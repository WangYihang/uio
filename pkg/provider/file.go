package provider

import (
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// OpenFileMode represents the modes in which a file can be opened.
type OpenFileMode string

const (
	// ModeRead will open the file for reading.
	ModeRead OpenFileMode = "read"
	// ModeWrite will truncate the file before writing.
	ModeWrite OpenFileMode = "write"
	// ModeAppend will append to the file.
	ModeAppend OpenFileMode = "append"
)

// OpenFile opens a local file specified by the URL and supports reading and writing.
// If the path is "-", it returns os.Stdin for reading or os.Stdout for writing.
func OpenFile(uri *url.URL, logger *slog.Logger) (io.ReadWriteCloser, error) {
	if uri.Scheme == "" && uri.Path == "-" {
		// If the path is "-", determine if we should use stdin or stdout based on the desired mode.
		return &stdStreamWrapper{}, nil
	}

	var path string
	if uri.Scheme == "" {
		path = uri.Path
	} else {
		// Construct the file path from URL components.
		path = strings.Join([]string{
			uri.Host,
			strings.TrimLeft(uri.Path, "/"),
		}, "/")
	}

	mode := uri.Query().Get("mode")
	if mode == "" {
		mode = string(ModeRead) // Default to write mode
	}

	var flags int
	switch OpenFileMode(mode) {
	case ModeAppend:
		flags = os.O_CREATE | os.O_RDWR | os.O_APPEND
	case ModeWrite:
		flags = os.O_CREATE | os.O_RDWR
	case ModeRead:
		flags = os.O_RDONLY
	default:
		return nil, fmt.Errorf("invalid mode: %s", mode)
	}

	logger.Info("Opening file", slog.String("path", path), slog.String("mode", mode), slog.Int("flags", flags))

	// Check if the file extension is ".gzip" or ".gz".
	if strings.HasSuffix(path, ".gzip") || strings.HasSuffix(path, ".gz") {
		// Open the file with the appropriate flags.
		file, err := os.OpenFile(path, flags, 0644)
		if err != nil {
			logger.Error("Failed to open file", slog.String("error", err.Error()))
			return nil, err
		}

		if OpenFileMode(mode) == ModeWrite {
			// Create a gzip writer to compress the file.
			gzipWriter := gzip.NewWriter(file)
			if strings.HasSuffix(path, ".gzip") {
				gzipWriter.Name = strings.TrimSuffix(filepath.Base(path), ".gzip")
			} else {
				gzipWriter.Name = strings.TrimSuffix(filepath.Base(path), ".gz")
			}
			return &writeOnlyCloser{gzipWriter}, nil
		} else {
			// Create a gzip reader to decompress the file.
			gzipReader, err := gzip.NewReader(file)
			if err != nil {
				logger.Error("Failed to create gzip reader", slog.String("error", err.Error()))
				file.Close()
				return nil, err
			}
			return &readOnlyCloser{gzipReader}, nil
		}
	}

	// If the file extension is not ".gzip" or ".gz", open the file directly.
	file, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		logger.Error("Failed to open file", slog.String("error", err.Error()))
		return nil, err
	}

	return file, nil
}

// stdStreamWrapper is a wrapper for os.Stdin and os.Stdout to support ReadWriteCloser.
type stdStreamWrapper struct{}

func (s *stdStreamWrapper) Read(p []byte) (n int, err error) {
	return os.Stdin.Read(p)
}

func (s *stdStreamWrapper) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

func (s *stdStreamWrapper) Close() error {
	// No-op for stdin/stdout since we don't close these streams.
	return nil
}

// writeOnlyCloser is a wrapper that implements io.ReadWriteCloser but only supports writing.
type writeOnlyCloser struct {
	io.WriteCloser
}

func (w *writeOnlyCloser) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read operation not supported for file resources")
}
