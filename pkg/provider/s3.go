package provider

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env"
	"github.com/klauspost/compress/gzip"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Config holds S3 connection details.
type S3Config struct {
	Endpoint  string `env:"S3_ENDPOINT"`
	AccessKey string `env:"S3_ACCESS_KEY"`
	SecretKey string `env:"S3_SECRET_KEY"`
	Insecure  bool   `env:"S3_INSECURE"`
}

// LoadS3Config loads the configuration from environment variables, allowing URL
// query parameters to override individual fields.
func LoadS3Config(query url.Values) (*S3Config, error) {
	cfg := &S3Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	// Use URL parameters if provided, otherwise fall back to environment variables.
	if endpoint := query.Get("endpoint"); endpoint != "" {
		cfg.Endpoint = endpoint
	}
	if accessKey := query.Get("access_key"); accessKey != "" {
		cfg.AccessKey = accessKey
	}
	if secretKey := query.Get("secret_key"); secretKey != "" {
		cfg.SecretKey = secretKey
	}
	if insecure := query.Get("insecure"); insecure != "" {
		cfg.Insecure = insecure != "false"
	}

	return cfg, nil
}

// s3WriteCloser buffers writes in a temporary file and uploads it to S3 on
// Close (when modified). Objects whose name ends in .gz or .gzip are compressed.
type s3WriteCloser struct {
	file       *os.File
	gz         *gzip.Writer // nil unless the object is gzip-compressed
	client     *minio.Client
	bucketName string
	objectName string
	logger     *slog.Logger
	isModified bool
}

// Read reads previously buffered bytes from the temporary file.
func (s *s3WriteCloser) Read(p []byte) (int, error) {
	return s.file.Read(p)
}

// Write buffers data to the temporary file (compressing it when applicable).
func (s *s3WriteCloser) Write(p []byte) (int, error) {
	s.isModified = true
	if s.gz != nil {
		return s.gz.Write(p)
	}
	return s.file.Write(p)
}

// Close finalizes the buffer and uploads it to S3 if it has been modified.
func (s *s3WriteCloser) Close() error {
	name := s.file.Name()

	// Flush and close the compression layer so all bytes reach the temp file.
	if s.gz != nil {
		if err := s.gz.Close(); err != nil {
			_ = s.file.Close()
			_ = os.Remove(name)
			return fmt.Errorf("failed to finalize gzip stream: %w", err)
		}
	}

	if !s.isModified {
		s.logger.Info("no modifications detected, skipping upload")
		err := s.file.Close()
		_ = os.Remove(name)
		return err
	}

	// Close the temp file so its contents are fully flushed before upload.
	if err := s.file.Close(); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	defer func() { _ = os.Remove(name) }()

	s.logger.Info("uploading file to S3", slog.String("bucket", s.bucketName), slog.String("object", s.objectName))

	// Retry the upload with exponential backoff.
	var err error
	maxRetries := 5
	backoff := 100 * time.Millisecond
	for i := 0; i < maxRetries; i++ {
		_, err = s.client.FPutObject(
			context.Background(),
			s.bucketName,
			s.objectName,
			name,
			minio.PutObjectOptions{},
		)
		if err == nil {
			s.logger.Info("successfully uploaded file to S3", slog.String("bucket", s.bucketName), slog.String("object", s.objectName))
			return nil
		}

		s.logger.Error("failed to upload file to S3, retrying...", slog.Int("attempt", i+1), slog.String("error", err.Error()))
		time.Sleep(backoff)
		backoff *= 2
	}

	return fmt.Errorf("failed to upload file to S3 after %d retries: %w", maxRetries, err)
}

// OpenS3 opens an S3 object for reading, or for writing when mode=write.
func OpenS3(uri *url.URL, logger *slog.Logger) (io.ReadWriteCloser, error) {
	query := uri.Query()
	cfg, err := LoadS3Config(query)
	if err != nil {
		logger.Error("failed to load S3 configuration", slog.String("error", err.Error()))
		return nil, err
	}

	bucketName := uri.Host
	objectName := strings.TrimLeft(uri.Path, "/")
	mode := query.Get("mode")

	logger.Info("opening S3 object",
		slog.String("endpoint", cfg.Endpoint),
		slog.String("accessKey", cfg.AccessKey),
		slog.String("secretKey", strings.Repeat("*", len(cfg.SecretKey))),
		slog.String("bucketName", bucketName),
		slog.String("objectName", objectName),
		slog.Bool("insecure", cfg.Insecure),
		slog.String("mode", mode),
	)

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: !cfg.Insecure,
	})
	if err != nil {
		logger.Error("failed to create MinIO client", slog.String("error", err.Error()))
		return nil, err
	}

	if mode == "write" {
		tempFile, err := os.CreateTemp("", "s3-*")
		if err != nil {
			logger.Error("failed to create temporary file", slog.String("error", err.Error()))
			return nil, err
		}
		logger.Info("opened temporary file for S3 write operations", slog.String("path", tempFile.Name()))

		w := &s3WriteCloser{
			file:       tempFile,
			client:     client,
			bucketName: bucketName,
			objectName: objectName,
			logger:     logger,
		}
		if isGzip(objectName) {
			gzipWriter, err := gzip.NewWriterLevel(tempFile, gzip.BestCompression)
			if err != nil {
				_ = tempFile.Close()
				_ = os.Remove(tempFile.Name())
				logger.Error("failed to create gzip writer", slog.String("error", err.Error()))
				return nil, err
			}
			gzipWriter.Name = gzipInnerName(objectName)
			w.gz = gzipWriter
		}
		return w, nil
	}

	// Read mode.
	object, err := client.GetObject(
		context.Background(),
		bucketName,
		objectName,
		minio.GetObjectOptions{},
	)
	if err != nil {
		logger.Error("failed to get S3 object", slog.String("error", err.Error()))
		return nil, err
	}

	if isGzip(objectName) {
		gzipReader, err := gzip.NewReader(object)
		if err != nil {
			_ = object.Close()
			logger.Error("failed to create gzip reader", slog.String("error", err.Error()))
			return nil, err
		}
		return &readCloser{Reader: gzipReader, closers: []io.Closer{gzipReader, object}}, nil
	}

	return &readCloser{Reader: object, closers: []io.Closer{object}}, nil
}
