package uio

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/klauspost/compress/gzip"
)

func TestOptionSetters(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := &http.Client{}

	o := resolve([]Option{
		WithLogger(logger),
		WithHTTPClient(client),
		WithGzipLevel(gzip.BestSpeed),
		WithCompression(CompressionGzip),
		WithS3Credentials("ep:9000", "ak", "sk", false),
	})

	if o.Logger != logger {
		t.Error("WithLogger not applied")
	}
	if o.HTTPClient != client {
		t.Error("WithHTTPClient not applied")
	}
	if o.GzipLevel != gzip.BestSpeed {
		t.Errorf("GzipLevel = %d, want %d", o.GzipLevel, gzip.BestSpeed)
	}
	if o.Compression != CompressionGzip {
		t.Errorf("Compression = %v, want Gzip", o.Compression)
	}
	if o.S3.Endpoint != "ep:9000" || o.S3.AccessKey != "ak" || o.S3.SecretKey != "sk" || o.S3.Secure {
		t.Errorf("S3 credentials = %+v", o.S3)
	}
}

func TestOptionNilIgnored(t *testing.T) {
	o := resolve([]Option{WithLogger(nil), WithHTTPClient(nil), nil})
	if o.Logger == nil {
		t.Error("nil logger must fall back to the default")
	}
	if o.HTTPClient == nil {
		t.Error("nil client must fall back to the default")
	}
}
