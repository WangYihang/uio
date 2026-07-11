package uio

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
)

// fileProvider reads and writes local files. It also handles "-", which maps to
// standard input (read) and standard output (write).
type fileProvider struct{}

// Open opens the file for reading. A missing file is an error; it is never
// created on read.
func (fileProvider) Open(_ context.Context, u *url.URL, o Options) (io.ReadCloser, error) {
	if isStdio(u) {
		return io.NopCloser(os.Stdin), nil
	}
	path := filePath(u)
	o.Logger.Info("open file", slog.String("path", path))
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Create opens the file for writing, truncating it (or appending when
// o.Append is set) and creating it if necessary.
func (fileProvider) Create(_ context.Context, u *url.URL, o Options) (io.WriteCloser, error) {
	if isStdio(u) {
		return nopWriteCloser{os.Stdout}, nil
	}
	path := filePath(u)
	flag := os.O_CREATE | os.O_WRONLY
	if o.Append {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	o.Logger.Info("create file", slog.String("path", path), slog.Bool("append", o.Append))
	f, err := os.OpenFile(path, flag, 0o644)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// isStdio reports whether u refers to the standard streams ("-").
func isStdio(u *url.URL) bool {
	return u.Scheme == "" && u.Path == "-"
}

// filePath converts a file URL (or bare path) into a local filesystem path.
//
//	file://data/x.txt   -> data/x.txt   (relative to the working directory)
//	file:///abs/x.txt    -> /abs/x.txt
//	data/x.txt           -> data/x.txt
func filePath(u *url.URL) string {
	if u.Scheme == "" {
		return u.Path
	}
	if u.Host == "" {
		return u.Path
	}
	return u.Host + "/" + strings.TrimLeft(u.Path, "/")
}
