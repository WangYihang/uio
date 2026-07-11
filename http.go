package uio

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

// httpProvider fetches resources over HTTP/HTTPS. It is read-only.
type httpProvider struct{}

// Open issues a GET request and returns the response body. The request honours
// ctx, so cancelling ctx aborts the transfer.
func (httpProvider) Open(ctx context.Context, u *url.URL, o Options) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	o.Logger.Info("http get", slog.String("url", u.String()))

	resp, err := o.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("uio: http %s: unexpected status %d %s", u.String(), resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return resp.Body, nil
}

// Create always fails: HTTP resources are read-only in uio.
func (httpProvider) Create(_ context.Context, _ *url.URL, _ Options) (io.WriteCloser, error) {
	return nil, fmt.Errorf("%w: http(s) is read-only", ErrWriteNotSupported)
}
