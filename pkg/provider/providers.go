package provider

import (
	"io"
	"log/slog"
	"net/url"
)

var SchemaMap = map[string]func(uri *url.URL, logger *slog.Logger) (io.ReadCloser, error){
	"http":  OpenHTTP,
	"https": OpenHTTP,
	"file":  OpenFile,
	"s3":    OpenS3,
}
