package provider

import (
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// httpClient is the shared HTTP client used for all HTTP/HTTPS requests.
var httpClient = &http.Client{
	Timeout: 30 * time.Second, // Set a reasonable timeout for HTTP requests.
}

// OpenHTTP fetches the HTTP/HTTPS resource identified by uri. The returned
// value is read-only; write attempts fail with errWriteNotSupported. Resources
// whose path ends in .gz or .gzip are transparently decompressed.
func OpenHTTP(uri *url.URL, logger *slog.Logger) (io.ReadWriteCloser, error) {
	logger.Info("fetching HTTP/HTTPS resource", slog.String("url", uri.String()))

	// Perform the HTTP GET request using the shared client.
	resp, err := httpClient.Get(uri.String())
	if err != nil {
		logger.Error("failed to fetch HTTP/HTTPS resource", slog.String("error", err.Error()))
		return nil, err
	}

	// Check for non-2xx status codes and handle errors accordingly.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		logger.Error("received non-2xx response code", slog.Int("statusCode", resp.StatusCode))
		return nil, fmt.Errorf("received non-2xx response code: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	// Transparently decompress gzip-compressed resources based on the extension.
	if isGzip(uri.Path) {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			_ = resp.Body.Close()
			logger.Error("failed to create gzip reader", slog.String("error", err.Error()))
			return nil, err
		}
		return &readCloser{Reader: gzipReader, closers: []io.Closer{gzipReader, resp.Body}}, nil
	}

	return &readCloser{Reader: resp.Body, closers: []io.Closer{resp.Body}}, nil
}
