package provider

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

// httpClient is a configurable HTTP client.
var httpClient = &http.Client{
	Timeout: 30 * time.Second, // Set a reasonable timeout for HTTP requests.
}

// openHTTP fetches the HTTP/HTTPS resource specified by the URL.
func openHTTP(uri *url.URL, logger *zap.Logger) (io.ReadCloser, error) {
	logger.Info("Fetching HTTP/HTTPS resource", zap.String("url", uri.String()))

	// Perform the HTTP GET request using the configured client.
	resp, err := httpClient.Get(uri.String())
	if err != nil {
		logger.Error("Failed to fetch HTTP/HTTPS resource", zap.Error(err))
		return nil, err
	}

	// Check for non-2xx status codes and handle errors accordingly.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg := "Received non-2xx response code"
		logger.Error(errMsg, zap.Int("statusCode", resp.StatusCode))
		resp.Body.Close() // Close the body before returning the error.
		return nil, fmt.Errorf("%s: %d %s", errMsg, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return resp.Body, nil
}
