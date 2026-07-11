package uio

import (
	"net/url"
	"testing"
)

// FuzzRoute ensures URI routing never panics and keeps its invariants for
// arbitrary input. This class of test would have caught the historical
// index-out-of-range panic in gzip-suffix handling.
func FuzzRoute(f *testing.F) {
	seeds := []string{
		"", "-", "/", "/ab", "file://data/x.txt", "file:///abs/x.txt.gz",
		"http://h/", "https://h/a.gz", "s3://bucket/key?insecure=true",
		"ftp://h/x", "://bad", "file://" + string(rune(0)),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, uri string) {
		p, u, err := route(uri)
		if err != nil {
			return
		}
		if p == nil || u == nil {
			t.Fatalf("route(%q) returned nil provider/url without error", uri)
		}
	})
}

// FuzzPureHelpers exercises the extension helpers on arbitrary paths.
func FuzzPureHelpers(f *testing.F) {
	for _, s := range []string{"", ".", "/", ".gz", "a.txt", "a.txt.gzip", "x/y.gz"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, path string) {
		_ = isGzipPath(path)
		_ = gzipInnerName(path)
		_ = wantGzip(CompressionAuto, path)
		if u, err := url.Parse(path); err == nil {
			_ = filePath(u)
		}
	})
}
