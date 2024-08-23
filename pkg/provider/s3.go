package provider

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ExtractS3Params extracts S3 parameters from the URL query.
func extractS3Params(query url.Values) (endpoint, accessKey, secretKey string, insecure bool) {
	return query.Get("endpoint"), query.Get("access_key"), query.Get("secret_key"), query.Get("insecure") == "true"
}

// logS3Params logs the S3 connection parameters.
func logS3Params(logger *slog.Logger, endpoint, bucketName, objectName string, insecure bool) {
	logger.Info("Opening S3 object",
		slog.String("endpoint", endpoint),
		slog.String("bucketName", bucketName),
		slog.String("objectName", objectName),
		slog.Bool("insecure", insecure),
	)
}

func OpenS3(uri *url.URL, logger *slog.Logger) (io.ReadCloser, error) {
	query := uri.Query()
	endpoint, accessKey, secretKey, insecure := extractS3Params(query)
	bucketName := uri.Host
	objectName := strings.TrimLeft(uri.Path, "/")

	logS3Params(logger, endpoint, bucketName, objectName, insecure)

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: !insecure,
	})
	if err != nil {
		logger.Error("Failed to create MinIO client", slog.String("error", err.Error()))
		return nil, err
	}

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
