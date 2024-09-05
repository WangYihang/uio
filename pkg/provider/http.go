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

// httpClient is a configurable HTTP client.
var httpClient = &http.Client{
	Timeout: 30 * time.Second, // Set a reasonable timeout for HTTP requests.
}

// readOnlyCloser is a wrapper that implements io.ReadWriteCloser but only supports reading.
type readOnlyCloser struct {
	io.ReadCloser
}

func (r *readOnlyCloser) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("write operation not supported for HTTP/HTTPS resources")
}

// OpenHTTP fetches the HTTP/HTTPS resource specified by the URL.
func OpenHTTP(uri *url.URL, logger *slog.Logger) (io.ReadWriteCloser, error) {
	logger.Info("Fetching HTTP/HTTPS resource", slog.String("url", uri.String()))

	// Perform the HTTP GET request using the configured client.
	resp, err := httpClient.Get(uri.String())
	if err != nil {
		logger.Error("Failed to fetch HTTP/HTTPS resource", slog.String("error", err.Error()))
		return nil, err
	}

	// Check for non-2xx status codes and handle errors accordingly.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg := "Received non-2xx response code"
		logger.Error(errMsg, slog.Int("statusCode", resp.StatusCode))
		resp.Body.Close() // Close the body before returning the error.
		return nil, fmt.Errorf("%s: %d %s", errMsg, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	// Check if the opened file is compressed using gzip using url extension.
	if uri.Path != "" && (uri.Path[len(uri.Path)-3:] == ".gz" || uri.Path[len(uri.Path)-5:] == ".gzip") {
		// Create a gzip reader to decompress the file.
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			logger.Error("Failed to create gzip reader", slog.String("error", err.Error()))
			resp.Body.Close()
			return nil, err
		}
		return &readOnlyCloser{gzipReader}, nil
	}

	// Wrap the response body in a readOnlyCloser and return it.
	return &readOnlyCloser{resp.Body}, nil
}
