package uio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/WangYihang/uio"
)

// writeVia writes data to uri through uio.Create.
func writeVia(t *testing.T, uri string, data []byte, opts ...uio.Option) {
	t.Helper()
	w, err := uio.Create(context.Background(), uri, opts...)
	if err != nil {
		t.Fatalf("Create(%q): %v", uri, err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("Write(%q): %v", uri, err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close(%q): %v", uri, err)
	}
}

// readVia reads uri through uio.Open.
func readVia(t *testing.T, uri string, opts ...uio.Option) []byte {
	t.Helper()
	r, err := uio.Open(context.Background(), uri, opts...)
	if err != nil {
		t.Fatalf("Open(%q): %v", uri, err)
	}
	defer func() { _ = r.Close() }()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll(%q): %v", uri, err)
	}
	return b
}

func TestFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		file string
	}{
		{"plain", "data.txt"},
		{"gzip", "data.txt.gz"},
		{"gzip long ext", "data.txt.gzip"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uri := "file://" + filepath.Join(dir, tc.file)
			payload := []byte("Hello World!")
			writeVia(t, uri, payload)
			if got := readVia(t, uri); !bytes.Equal(got, payload) {
				t.Errorf("round trip = %q, want %q", got, payload)
			}
		})
	}
}

func TestFileGzipIsCompressedOnDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.txt.gz")
	writeVia(t, "file://"+path, bytes.Repeat([]byte("A"), 1000))

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		t.Fatalf("file is not gzip-compressed: % x", raw[:min(4, len(raw))])
	}
	if len(raw) >= 1000 {
		t.Errorf("compressed size %d should be well under 1000", len(raw))
	}
}

func TestCreateTruncates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trunc.txt")
	writeVia(t, "file://"+path, []byte("Hello World!"))
	writeVia(t, "file://"+path, []byte("Hi")) // must truncate, not overwrite in place
	if got := readVia(t, "file://"+path); !bytes.Equal(got, []byte("Hi")) {
		t.Errorf("after truncate = %q, want %q", got, "Hi")
	}
}

func TestCreateAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.txt")
	writeVia(t, "file://"+path, []byte("one\n"))
	writeVia(t, "file://"+path, []byte("two\n"), uio.WithAppend())
	if got := readVia(t, "file://"+path); !bytes.Equal(got, []byte("one\ntwo\n")) {
		t.Errorf("after append = %q", got)
	}
}

func TestWithoutCompression(t *testing.T) {
	// A .gz name with compression disabled stores raw bytes.
	path := filepath.Join(t.TempDir(), "raw.gz")
	writeVia(t, "file://"+path, []byte("not gzip"), uio.WithoutCompression())
	if got := readVia(t, "file://"+path, uio.WithoutCompression()); !bytes.Equal(got, []byte("not gzip")) {
		t.Errorf("raw round trip = %q", got)
	}
	raw, _ := os.ReadFile(path)
	if !bytes.Equal(raw, []byte("not gzip")) {
		t.Errorf("on disk = %q, want raw bytes", raw)
	}
}

func TestForcedCompression(t *testing.T) {
	// A plain name with gzip forced is compressed and reads back with the same option.
	path := filepath.Join(t.TempDir(), "forced.bin")
	writeVia(t, "file://"+path, []byte("force gzip"), uio.WithCompression(uio.CompressionGzip))
	raw, _ := os.ReadFile(path)
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		t.Fatalf("forced gzip not applied: % x", raw)
	}
	if got := readVia(t, "file://"+path, uio.WithCompression(uio.CompressionGzip)); !bytes.Equal(got, []byte("force gzip")) {
		t.Errorf("forced round trip = %q", got)
	}
}

func TestOpenMissingFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")
	if _, err := uio.Open(context.Background(), "file://"+path); err == nil {
		t.Fatal("opening a missing file should error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("opening for read must not create the file")
	}
}

func TestUnsupportedScheme(t *testing.T) {
	_, err := uio.Open(context.Background(), "ftp://host/file")
	if !errors.Is(err, uio.ErrUnsupportedScheme) {
		t.Fatalf("err = %v, want ErrUnsupportedScheme", err)
	}
}

func TestHTTPCreateUnsupported(t *testing.T) {
	_, err := uio.Create(context.Background(), "http://example.com/x")
	if !errors.Is(err, uio.ErrWriteNotSupported) {
		t.Fatalf("err = %v, want ErrWriteNotSupported", err)
	}
}
