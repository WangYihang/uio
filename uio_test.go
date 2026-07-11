package uio_test

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/WangYihang/uio"
	"github.com/google/uuid"
)

// gzipBytes returns data compressed with gzip.
func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// readAll opens uri and returns its full contents.
func readAll(t *testing.T, uri string) []byte {
	t.Helper()
	fd, err := uio.Open(uri)
	if err != nil {
		t.Fatalf("Open(%q) returned error: %v", uri, err)
	}
	defer func() { _ = fd.Close() }()
	got, err := io.ReadAll(fd)
	if err != nil {
		t.Fatalf("io.ReadAll(%q) returned error: %v", uri, err)
	}
	return got
}

func TestFileRead(t *testing.T) {
	testcases := []struct {
		name     string
		uri      string
		expected []byte
	}{
		{
			name:     "plain",
			uri:      "file://data/test_read_from_file.txt",
			expected: []byte("Hello World!"),
		},
		{
			name:     "gzip",
			uri:      "file://data/test_read_from_file.txt.gz",
			expected: []byte("Hello World!"),
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := readAll(t, tc.uri); !bytes.Equal(got, tc.expected) {
				t.Errorf("for URI %q, expected %q, got %q", tc.uri, tc.expected, got)
			}
		})
	}
}

func TestHTTPRead(t *testing.T) {
	payload := []byte("Hello World!")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hello.txt":
			_, _ = w.Write(payload)
		case "/hello.txt.gz":
			// Serve raw gzip bytes without Content-Encoding so uio decompresses
			// based on the extension rather than the transport doing it.
			_, _ = w.Write(gzipBytes(t, payload))
		case "/", "/ab":
			// Short paths that previously triggered an index-out-of-range panic.
			_, _ = w.Write([]byte("root"))
		default:
			http.NotFound(w, r)
		}
	}))
	// Cleanup (not defer) so the server outlives the parallel subtests below.
	t.Cleanup(srv.Close)

	testcases := []struct {
		name     string
		uri      string
		expected []byte
	}{
		{name: "plain", uri: srv.URL + "/hello.txt", expected: payload},
		{name: "gzip", uri: srv.URL + "/hello.txt.gz", expected: payload},
		{name: "root path (no panic)", uri: srv.URL + "/", expected: []byte("root")},
		{name: "short path (no panic)", uri: srv.URL + "/ab", expected: []byte("root")},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := readAll(t, tc.uri); !bytes.Equal(got, tc.expected) {
				t.Errorf("for URI %q, expected %q, got %q", tc.uri, tc.expected, got)
			}
		})
	}
}

func TestHTTPNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := uio.Open(srv.URL + "/missing.txt"); err == nil {
		t.Fatal("expected an error for a 404 response, got nil")
	}
}

func TestFileWriteRoundTrip(t *testing.T) {
	payload := []byte("Hello World!")
	dir := t.TempDir()
	testcases := []struct {
		name string
		file string
	}{
		{name: "plain", file: "out.txt"},
		{name: "gzip", file: "out.txt.gz"},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(dir, tc.file)
			writeURI := "file://" + path + "?mode=write"
			readURI := "file://" + path

			fd, err := uio.Open(writeURI)
			if err != nil {
				t.Fatalf("Open(%q) returned error: %v", writeURI, err)
			}
			if _, err := fd.Write(payload); err != nil {
				t.Fatalf("Write returned error: %v", err)
			}
			if err := fd.Close(); err != nil {
				t.Fatalf("Close returned error: %v", err)
			}

			if got := readAll(t, readURI); !bytes.Equal(got, payload) {
				t.Errorf("round trip for %q: expected %q, got %q", path, payload, got)
			}
		})
	}
}

// TestFileWriteTruncates guards against mode=write leaving trailing bytes from a
// previous, longer file (i.e. a missing O_TRUNC).
func TestFileWriteTruncates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trunc.txt")
	if err := os.WriteFile(path, []byte("Hello World!"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	writeURI := "file://" + path + "?mode=write"
	fd, err := uio.Open(writeURI)
	if err != nil {
		t.Fatalf("Open(%q) returned error: %v", writeURI, err)
	}
	if _, err := fd.Write([]byte("Hi")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := fd.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	got := readAll(t, "file://"+path)
	if !bytes.Equal(got, []byte("Hi")) {
		t.Errorf("expected truncated content %q, got %q", "Hi", got)
	}
}

// TestReadMissingFileErrors verifies that the default (read) mode does not create
// a file for a resource that does not exist.
func TestReadMissingFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does_not_exist.txt")
	if _, err := uio.Open("file://" + path); err == nil {
		t.Fatalf("expected an error opening a missing file, got nil")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("opening a missing file for read must not create it, stat err = %v", err)
	}
}

// TestS3 exercises the S3 provider. It is skipped unless UIO_TEST_S3 is set,
// since it requires a running S3-compatible endpoint (see docker-compose.yaml).
func TestS3(t *testing.T) {
	if os.Getenv("UIO_TEST_S3") == "" {
		t.Skip("set UIO_TEST_S3=1 and start docker-compose to run S3 tests")
	}

	suffix := uuid.New().String()
	payload := []byte("Hello World!")
	base := "s3://uio/test_write_to_s3_%s.txt?endpoint=127.0.0.1:9000&access_key=minioadmin&secret_key=minioadmin&insecure=true"
	writeURI := fmt.Sprintf(base+"&mode=write", suffix)
	readURI := fmt.Sprintf(base, suffix)

	fd, err := uio.Open(writeURI)
	if err != nil {
		t.Fatalf("Open(%q) returned error: %v", writeURI, err)
	}
	if _, err := fd.Write(payload); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := fd.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	if got := readAll(t, readURI); !bytes.Equal(got, payload) {
		t.Errorf("S3 round trip: expected %q, got %q", payload, got)
	}
}
