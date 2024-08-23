package provider

import (
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// openFile opens a local file specified by the URL.
func openFile(uri *url.URL, logger *zap.Logger) (io.ReadCloser, error) {
	// Construct the file path from URL components.
	// Use filepath.Join to handle platform-specific path separators.
	path := filepath.Join(uri.Host, strings.TrimPrefix(uri.Path, "/"))

	logger.Info("Opening file", zap.String("path", path))

	// Open the file.
	file, err := os.Open(path)
	if err != nil {
		logger.Error("Failed to open file", zap.Error(err))
		return nil, err
	}

	return file, nil
}
