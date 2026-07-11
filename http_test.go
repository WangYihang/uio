package uio_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/WangYihang/uio"
)

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestHTTPRead(t *testing.T) {
	payload := []byte("Hello World!")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hello.txt":
			_, _ = w.Write(payload)
		case "/hello.txt.gz":
			// Raw gzip bytes, no Content-Encoding: uio decompresses by extension.
			_, _ = w.Write(gzipBytes(t, payload))
		case "/", "/ab":
			// Short paths that previously caused an index-out-of-range panic.
			_, _ = w.Write([]byte("root"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	cases := []struct {
		name string
		uri  string
		want []byte
	}{
		{"plain", srv.URL + "/hello.txt", payload},
		{"gzip", srv.URL + "/hello.txt.gz", payload},
		{"root path no panic", srv.URL + "/", []byte("root")},
		{"short path no panic", srv.URL + "/ab", []byte("root")},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := uio.Open(context.Background(), tc.uri)
			if err != nil {
				t.Fatalf("Open(%q): %v", tc.uri, err)
			}
			defer func() { _ = r.Close() }()
			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Errorf("Open(%q) = %q, want %q", tc.uri, got, tc.want)
			}
		})
	}
}

func TestHTTPNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := uio.Open(context.Background(), srv.URL+"/missing"); err == nil {
		t.Fatal("expected an error for a 404 response")
	}
}

func TestHTTPContextCancel(t *testing.T) {
	// Server blocks until the request context is cancelled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	if _, err := uio.Open(ctx, srv.URL+"/blocks"); err == nil {
		t.Fatal("expected an error when the context is cancelled")
	}
}
