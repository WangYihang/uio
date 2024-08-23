package provider

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ExtractS3Params extracts S3 parameters from the URL query.
func extractS3Params(query url.Values) (endpoint, accessKey, secretKey string, insecure bool, download bool) {
	return query.Get("endpoint"), query.Get("access_key"), query.Get("secret_key"), query.Get("insecure") != "false", query.Get("download") == "true"
}

func OpenS3(uri *url.URL, logger *slog.Logger) (io.ReadCloser, error) {
	query := uri.Query()
	endpoint, accessKey, secretKey, insecure, download := extractS3Params(query)
	bucketName := uri.Host
	objectName := strings.TrimLeft(uri.Path, "/")

	logger.Info("Opening S3 object",
		slog.String("endpoint", endpoint),
		slog.String("accessKey", accessKey),
		slog.String("secretKey", strings.Repeat("*", len(secretKey))),
		slog.String("bucketName", bucketName),
		slog.String("objectName", objectName),
		slog.Bool("insecure", insecure),
		slog.Bool("download", false),
	)

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: !insecure,
	})
	if err != nil {
		logger.Error("Failed to create MinIO client", slog.String("error", err.Error()))
		return nil, err
	}

	if download {
		// Download to a temporary file
		path := filepath.Join(os.TempDir(), uuid.New().String())
		err := minioClient.FGetObject(
			context.Background(),
			bucketName,
			objectName,
			"",
			minio.GetObjectOptions{},
		)
		if err != nil {
			logger.Error("Failed to download S3 object", slog.String("error", err.Error()))
			return nil, err
		}
		source := fmt.Sprintf("file://%s", path)
		logger.Info("Downloaded S3 object", slog.String("url", source), slog.String("path", path))
		uri, err := url.Parse(source)
		if err != nil {
			logger.Error("Failed to parse URL", slog.String("error", err.Error()))
			return nil, err
		}
		return OpenFile(uri, logger)
	} else {
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

		return object, nil
	}
}
