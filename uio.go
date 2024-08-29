package uio

import (
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"

	"github.com/WangYihang/uio/pkg/provider"
)

func Open(uri string) (io.ReadWriteCloser, error) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	logger.Info("Opening resource", slog.String("uri", uri))
	u, err := url.Parse(uri)
	if err != nil {
		logger.Error("Invalid URI", slog.String("error", err.Error()))
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	logger.Info("Parsed URI",
		slog.String("scheme", u.Scheme),
		slog.String("host", u.Host),
		slog.String("path", u.Path),
		slog.String("query", u.RawQuery),
	)

	if f, ok := provider.SchemaMap[u.Scheme]; ok {
		return f(u, logger)
	} else {
		logger.Error("Fallback to file", slog.String("file", "stdio"))
		return provider.OpenFile(u, logger)
	}
}
