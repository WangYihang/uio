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

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ExtractS3Params extracts S3 parameters from the URL query.
func extractS3Params(query url.Values) (endpoint, accessKey, secretKey string, insecure bool, mode string) {
	return query.Get("endpoint"), query.Get("access_key"), query.Get("secret_key"), query.Get("insecure") != "false", query.Get("mode")
}

// s3ReadOnlyCloser is a custom wrapper that handles only read operations with S3.
type s3ReadOnlyCloser struct {
	object io.ReadCloser
	logger *slog.Logger
}

// Read reads data from the S3 object.
func (s *s3ReadOnlyCloser) Read(p []byte) (n int, err error) {
	return s.object.Read(p)
}

// Write writes data to the S3 object.
func (s *s3ReadOnlyCloser) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("write operation not supported")
}

// Close closes the S3 object.
func (s *s3ReadOnlyCloser) Close() error {
	return s.object.Close()
}

// s3ReadWriteCloser is a custom wrapper that handles both read and write operations with S3.
type s3ReadWriteCloser struct {
	*os.File
	client     *minio.Client
	bucketName string
	objectName string
	logger     *slog.Logger
	isModified bool // Indicates if the file has been modified
}

// Read reads data from the temporary file.
func (s *s3ReadWriteCloser) Read(p []byte) (n int, err error) {
	return s.File.Read(p)
}

// Write writes data to the temporary file.
func (s *s3ReadWriteCloser) Write(p []byte) (n int, err error) {
	s.isModified = true // Mark the file as modified
	return s.File.Write(p)
}

// s3ReadWriteCloser's Close method should handle the file upload if modified
func (s *s3ReadWriteCloser) Close() error {
	if !s.isModified {
		s.logger.Info("No modifications detected, skipping upload")
		return s.File.Close()
	}

	defer os.Remove(s.Name()) // Clean up the temporary file after upload

	s.logger.Info("Uploading file to S3", slog.String("bucket", s.bucketName), slog.String("object", s.objectName))

	// Exponential backoff for retry
	var err error
	maxRetries := 5
	backoff := time.Millisecond * 100

	for i := 0; i < maxRetries; i++ {
		_, err = s.client.FPutObject(
			context.Background(),
			s.bucketName,
			s.objectName,
			s.Name(),
			minio.PutObjectOptions{},
		)
		if err == nil {
			s.logger.Info("Successfully uploaded file to S3", slog.String("bucket", s.bucketName), slog.String("object", s.objectName))
			return s.File.Close()
		}

		s.logger.Error("Failed to upload file to S3, retrying...", slog.Int("attempt", i+1), slog.String("error", err.Error()))
		time.Sleep(backoff)
		backoff *= 2 // Exponential backoff
	}

	// If we exit the loop, it means all retries failed
	return fmt.Errorf("failed to upload file to S3 after %d retries: %w", maxRetries, err)
}

// OpenS3 opens an S3 object for reading or reading and writing.
func OpenS3(uri *url.URL, logger *slog.Logger) (io.ReadWriteCloser, error) {
	query := uri.Query()
	endpoint, accessKey, secretKey, insecure, mode := extractS3Params(query)
	bucketName := uri.Host
	objectName := strings.TrimLeft(uri.Path, "/")

	logger.Info("Opening S3 object",
		slog.String("endpoint", endpoint),
		slog.String("accessKey", accessKey),
		slog.String("secretKey", strings.Repeat("*", len(secretKey))),
		slog.String("bucketName", bucketName),
		slog.String("objectName", objectName),
		slog.Bool("insecure", insecure),
		slog.String("mode", mode),
	)

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: !insecure,
	})
	if err != nil {
		logger.Error("Failed to create MinIO client", slog.String("error", err.Error()))
		return nil, err
	}

	if mode == "write" {
		// Create a temporary file for read-write operations
		tempFile, err := os.CreateTemp("", "s3-*")
		if err != nil {
			logger.Error("Failed to create temporary file", slog.String("error", err.Error()))
			return nil, err
		}

		// No need to check if the object exists; just create a temporary file for write operations
		logger.Info("Opened temporary file for S3 write operations", slog.String("path", tempFile.Name()))

		return &s3ReadWriteCloser{
			File:       tempFile,
			client:     minioClient,
			bucketName: bucketName,
			objectName: objectName,
			logger:     logger,
			isModified: false, // Initially, the file is not modified
		}, nil
	} else {
		// Only read operations
		object, err := minioClient.GetObject(
			context.Background(),
			bucketName,
			objectName,
			minio.GetObjectOptions{},
		)
		if err != nil {
			logger.Error("Failed to get S3 object", slog.String("error", err.Error()))
			return nil, err
		}

		return &s3ReadOnlyCloser{
			object: object,
			logger: logger,
		}, nil
	}
}
