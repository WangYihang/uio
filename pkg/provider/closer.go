package provider

import (
	"errors"
	"io"
	"path/filepath"
	"strings"
)

// errWriteNotSupported is returned when writing to a read-only resource.
var errWriteNotSupported = errors.New("uio: write operation not supported for this resource")

// errReadNotSupported is returned when reading from a write-only resource.
var errReadNotSupported = errors.New("uio: read operation not supported for this resource")

// isGzip reports whether path names a gzip-compressed file by its extension.
func isGzip(path string) bool {
	return strings.HasSuffix(path, ".gz") || strings.HasSuffix(path, ".gzip")
}

// gzipInnerName returns the original file name (with the gzip extension
// stripped) to embed in the gzip header's Name field.
func gzipInnerName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".gzip")
	base = strings.TrimSuffix(base, ".gz")
	return base
}

// closeAll closes every closer in order, returning the first error encountered.
func closeAll(closers []io.Closer) error {
	var firstErr error
	for _, c := range closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// readCloser adapts an io.Reader to io.ReadWriteCloser. Writes are rejected,
// and Close closes every wrapped closer in order (so a decompressor and its
// underlying source are both released), returning the first error.
type readCloser struct {
	io.Reader
	closers []io.Closer
}

func (r *readCloser) Write([]byte) (int, error) { return 0, errWriteNotSupported }

func (r *readCloser) Close() error { return closeAll(r.closers) }

// writeCloser adapts an io.Writer to io.ReadWriteCloser. Reads are rejected,
// and Close closes every wrapped closer in order. Closers must be supplied
// outermost-first (e.g. the gzip writer before the file) so buffered data is
// flushed before the underlying resource is closed.
type writeCloser struct {
	io.Writer
	closers []io.Closer
}

func (w *writeCloser) Read([]byte) (int, error) { return 0, errReadNotSupported }

func (w *writeCloser) Close() error { return closeAll(w.closers) }
