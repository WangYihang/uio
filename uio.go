package uio

import (
	"fmt"
	"io"
	"net/url"

	"github.com/WangYihang/uio/pkg/provider"
	"go.uber.org/zap"
)

var logger, _ = zap.NewDevelopment()

func logURIInfo(u *url.URL) {
	logger.Info("Parsed URI",
		zap.String("scheme", u.Scheme),
		zap.String("host", u.Host),
		zap.String("path", u.Path),
		zap.String("query", u.RawQuery),
	)
}

func Open(uri string) (io.ReadCloser, error) {
	logger.Info("Opening resource", zap.String("uri", uri))
	u, err := url.Parse(uri)
	if err != nil {
		logger.Error("Invalid URI", zap.Error(err))
		return nil, fmt.Errorf("invalid URI: %w", err)
	}
	logURIInfo(u)

	if f, ok := provider.SchemaMap[u.Scheme]; ok {
		return f(u, logger)
	}
	logger.Error("Unsupported scheme", zap.String("scheme", u.Scheme))
	return nil, fmt.Errorf("unsupported scheme: %s", u.Scheme)
}
