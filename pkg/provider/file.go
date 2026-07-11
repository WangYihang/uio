package provider

import (
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
)

// OpenFileMode represents the mode in which a file is opened.
type OpenFileMode string

const (
	// ModeRead opens the file for reading. This is the default.
	ModeRead OpenFileMode = "read"
	// ModeWrite truncates the file (creating it if necessary) before writing.
	ModeWrite OpenFileMode = "write"
	// ModeAppend appends to the file, creating it if necessary.
	ModeAppend OpenFileMode = "append"
)

// OpenFile opens a local file identified by uri. The "mode" query parameter
// selects read, write, or append (read is the default). A path of "-" maps to
// standard input/output. Files whose name ends in .gz or .gzip are transparently
// compressed on write/append and decompressed on read.
func OpenFile(uri *url.URL, logger *slog.Logger) (io.ReadWriteCloser, error) {
	if uri.Scheme == "" && uri.Path == "-" {
		// A path of "-" reads from stdin and writes to stdout.
		return &stdStreamWrapper{}, nil
	}

	var path string
	if uri.Scheme == "" {
		path = uri.Path
	} else {
		// Reconstruct the file path from the URL host and path components.
		path = strings.Join([]string{
			uri.Host,
			strings.TrimLeft(uri.Path, "/"),
		}, "/")
	}

	mode := OpenFileMode(uri.Query().Get("mode"))
	if mode == "" {
		mode = ModeRead // Default to read so callers never create/clobber by accident.
	}

	var flags int
	switch mode {
	case ModeRead:
		flags = os.O_RDONLY
	case ModeWrite:
		flags = os.O_CREATE | os.O_RDWR | os.O_TRUNC
	case ModeAppend:
		flags = os.O_CREATE | os.O_RDWR | os.O_APPEND
	default:
		return nil, fmt.Errorf("invalid mode: %s", mode)
	}

	logger.Info("opening file", slog.String("path", path), slog.String("mode", string(mode)))

	file, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		logger.Error("failed to open file", slog.String("error", err.Error()))
		return nil, err
	}

	// Non-gzip files are used directly.
	if !isGzip(path) {
		return file, nil
	}

	// Gzip files are decompressed on read and compressed on write/append.
	if mode == ModeRead {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			_ = file.Close()
			logger.Error("failed to create gzip reader", slog.String("error", err.Error()))
			return nil, err
		}
		return &readCloser{Reader: gzipReader, closers: []io.Closer{gzipReader, file}}, nil
	}

	gzipWriter, err := gzip.NewWriterLevel(file, gzip.BestCompression)
	if err != nil {
		_ = file.Close()
		logger.Error("failed to create gzip writer", slog.String("error", err.Error()))
		return nil, err
	}
	gzipWriter.Name = gzipInnerName(path)
	return &writeCloser{Writer: gzipWriter, closers: []io.Closer{gzipWriter, file}}, nil
}

// stdStreamWrapper adapts standard input/output to io.ReadWriteCloser. Reads
// come from os.Stdin and writes go to os.Stdout; Close is a no-op so the
// standard streams stay open.
type stdStreamWrapper struct{}

func (s *stdStreamWrapper) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (s *stdStreamWrapper) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (s *stdStreamWrapper) Close() error                { return nil }
