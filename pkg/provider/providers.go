package provider

import (
	"io"
	"net/url"

	"go.uber.org/zap"
)

var SchemaMap = map[string]func(uri *url.URL, logger *zap.Logger) (io.ReadCloser, error){
	"http":  openHTTP,
	"https": openHTTP,
	"file":  openFile,
	"s3":    openS3,
}
