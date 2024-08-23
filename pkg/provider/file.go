package provider

import (
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// OpenFile opens a local file specified by the URL.
func OpenFile(uri *url.URL, logger *slog.Logger) (io.ReadCloser, error) {
	if uri.Scheme == "" && uri.Path == "-" {
		return os.Stdin, nil
	}

	var path string
	if uri.Scheme == "" {
		path = uri.Path
	} else {
		// Construct the file path from URL components.
		// Use filepath.Join to handle platform-specific path separators.
		path = filepath.Join(uri.Host, strings.TrimPrefix(uri.Path, "/"))
	}
	logger.Info("Opening file", slog.String("path", path))

	// Open the file.
	file, err := os.Open(path)
	if err != nil {
		logger.Error("Failed to open file", slog.String("error", err.Error()))
		return nil, err
	}

	return file, nil
}
