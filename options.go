package uio

import (
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/klauspost/compress/gzip"
)

// defaultHTTPClient is used when no HTTP client is supplied. Its timeout is a
// backstop; per-call cancellation is handled through context.
var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

// discardLogger drops all log records, making uio silent by default.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// Options holds the resolved settings for a single Open or Create call. It is
// passed to providers. Most callers configure it through Option values rather
// than constructing it directly.
type Options struct {
	// Logger receives progress and error records. Defaults to a silent logger.
	Logger *slog.Logger
	// HTTPClient is used by the http/https provider.
	HTTPClient *http.Client
	// Compression controls gzip handling. Defaults to CompressionAuto.
	Compression Compression
	// GzipLevel is the gzip level used on write. Defaults to
	// gzip.DefaultCompression.
	GzipLevel int
	// Append requests append semantics on Create (file provider only).
	Append bool
	// S3 holds credentials and endpoint settings for the s3 provider. URL query
	// parameters override these fields.
	S3 S3Options
}

// S3Options holds S3 connection settings.
type S3Options struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Secure    bool
}

// Option configures an Options value.
type Option func(*Options)

func defaultOptions() Options {
	return Options{
		Logger:      discardLogger(),
		HTTPClient:  defaultHTTPClient,
		Compression: CompressionAuto,
		GzipLevel:   gzip.DefaultCompression,
		S3:          S3Options{Secure: true},
	}
}

func resolve(opts []Option) Options {
	o := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	if o.Logger == nil {
		o.Logger = discardLogger()
	}
	if o.HTTPClient == nil {
		o.HTTPClient = defaultHTTPClient
	}
	return o
}

// WithLogger sets the logger used to report progress and errors. A nil logger
// is ignored. By default uio is silent.
func WithLogger(logger *slog.Logger) Option {
	return func(o *Options) {
		if logger != nil {
			o.Logger = logger
		}
	}
}

// WithHTTPClient sets the HTTP client used by the http/https provider. A nil
// client is ignored.
func WithHTTPClient(client *http.Client) Option {
	return func(o *Options) {
		if client != nil {
			o.HTTPClient = client
		}
	}
}

// WithCompression overrides how gzip is applied.
func WithCompression(c Compression) Option {
	return func(o *Options) { o.Compression = c }
}

// WithoutCompression disables gzip regardless of the file extension.
func WithoutCompression() Option {
	return func(o *Options) { o.Compression = CompressionNone }
}

// WithGzipLevel sets the gzip compression level used on write. See the gzip
// package for valid levels.
func WithGzipLevel(level int) Option {
	return func(o *Options) { o.GzipLevel = level }
}

// WithAppend requests append semantics on Create. Only the file provider
// supports it; other providers return ErrWriteNotSupported.
func WithAppend() Option {
	return func(o *Options) { o.Append = true }
}

// WithS3Credentials sets the endpoint and credentials for the s3 provider. URL
// query parameters still take precedence over these values.
func WithS3Credentials(endpoint, accessKey, secretKey string, secure bool) Option {
	return func(o *Options) {
		o.S3 = S3Options{
			Endpoint:  endpoint,
			AccessKey: accessKey,
			SecretKey: secretKey,
			Secure:    secure,
		}
	}
}
