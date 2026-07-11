package uio

import (
	"fmt"
	"io"
	"log/slog"
	"net/url"

	"github.com/WangYihang/uio/pkg/provider"
)

// Option configures the behavior of Open.
type Option func(*options)

type options struct {
	logger *slog.Logger
}

// WithLogger sets the logger used to report progress and errors. By default
// Open is silent. A nil logger is ignored.
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) {
		if logger != nil {
			o.logger = logger
		}
	}
}

// Open opens the resource identified by uri and returns an io.ReadWriteCloser.
//
// The scheme selects the backend: http/https fetch over HTTP, s3 reads or writes
// an object in S3-compatible storage, and file (or a bare path, or "-" for the
// standard streams) uses the local file system. Resources whose name ends in
// .gz or .gzip are transparently compressed and decompressed.
//
// Open is silent unless a logger is supplied with WithLogger.
func Open(uri string, opts ...Option) (io.ReadWriteCloser, error) {
	o := &options{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	for _, opt := range opts {
		opt(o)
	}
	logger := o.logger

	logger.Info("opening resource", slog.String("uri", uri))
	u, err := url.Parse(uri)
	if err != nil {
		logger.Error("invalid URI", slog.String("error", err.Error()))
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	// Note: the raw query is intentionally not logged as it may carry secrets
	// (e.g. S3 access_key/secret_key).
	logger.Info("parsed URI",
		slog.String("scheme", u.Scheme),
		slog.String("host", u.Host),
		slog.String("path", u.Path),
	)

	if open, ok := provider.SchemaMap[u.Scheme]; ok {
		return open(u, logger)
	}
	// Unknown schemes fall back to the file system (covers bare paths and "-").
	return provider.OpenFile(u, logger)
}
