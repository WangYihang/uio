package uio

import (
	"io"
	"path"
	"strings"

	"github.com/klauspost/compress/gzip"
)

// Compression selects how uio compresses on write and decompresses on read.
type Compression int

const (
	// CompressionAuto enables gzip based on the .gz or .gzip file extension.
	// It is the default.
	CompressionAuto Compression = iota
	// CompressionNone disables gzip regardless of the file extension.
	CompressionNone
	// CompressionGzip forces gzip regardless of the file extension.
	CompressionGzip
)

// isGzipPath reports whether path names a gzip file by its extension.
func isGzipPath(p string) bool {
	return strings.HasSuffix(p, ".gz") || strings.HasSuffix(p, ".gzip")
}

// wantGzip reports whether gzip should be applied for the given mode and path.
func wantGzip(c Compression, path string) bool {
	switch c {
	case CompressionNone:
		return false
	case CompressionGzip:
		return true
	default:
		return isGzipPath(path)
	}
}

// gzipInnerName derives the original file name (without the gzip extension) to
// store in the gzip header.
func gzipInnerName(p string) string {
	base := path.Base(p)
	base = strings.TrimSuffix(base, ".gzip")
	base = strings.TrimSuffix(base, ".gz")
	if base == "." || base == "/" {
		return ""
	}
	return base
}

// decompress wraps rc in a gzip reader. Closing the result closes both the gzip
// reader and rc.
func decompress(rc io.ReadCloser) (io.ReadCloser, error) {
	gr, err := gzip.NewReader(rc)
	if err != nil {
		return nil, err
	}
	return readerCloser{Reader: gr, closers: []io.Closer{gr, rc}}, nil
}

// compress wraps wc in a gzip writer using the given stored name and level.
// Closing the result flushes and closes the gzip writer, then closes wc.
func compress(wc io.WriteCloser, name string, level int) (io.WriteCloser, error) {
	gw, err := gzip.NewWriterLevel(wc, level)
	if err != nil {
		return nil, err
	}
	gw.Name = name
	return writerCloser{Writer: gw, closers: []io.Closer{gw, wc}}, nil
}
