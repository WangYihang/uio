package provider

import (
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
)

// OpenFileMode represents the modes in which a file can be opened.
type OpenFileMode string

const (
	// ModeWrite will truncate the file before writing.
	ModeWrite OpenFileMode = "write"
	// ModeAppend will append to the file.
	ModeAppend OpenFileMode = "append"
)

// OpenFile opens a local file specified by the URL and supports both reading and writing.
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
		mode = string(ModeWrite) // Default to write mode
	}

	var flags int
	switch OpenFileMode(mode) {
	case ModeAppend:
		flags = os.O_APPEND | os.O_CREATE | os.O_RDWR
	case ModeWrite:
		flags = os.O_TRUNC | os.O_CREATE | os.O_RDWR
	default:
		return nil, fmt.Errorf("invalid mode: %s", mode)
	}

	logger.Info("Opening file", slog.String("path", path), slog.String("mode", mode))

	// Open the file with the appropriate flags.
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
