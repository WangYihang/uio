package uio

import (
	"context"
	"errors"
	"io"
	"time"
)

// Sentinel errors returned by the package. Callers can test for them with
// errors.Is.
var (
	// ErrUnsupportedScheme is returned when no provider is registered for a
	// URI's scheme.
	ErrUnsupportedScheme = errors.New("uio: unsupported scheme")
	// ErrReadNotSupported is returned when a resource cannot be read.
	ErrReadNotSupported = errors.New("uio: read not supported for this resource")
	// ErrWriteNotSupported is returned when a resource cannot be written.
	ErrWriteNotSupported = errors.New("uio: write not supported for this resource")
)

// closeAll closes every non-nil closer in order and returns the first error.
func closeAll(closers ...io.Closer) error {
	var firstErr error
	for _, c := range closers {
		if c == nil {
			continue
		}
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// readerCloser couples a reader with the closers that release it (for example a
// gzip reader together with the underlying file or network stream). Close
// releases them in order, returning the first error.
type readerCloser struct {
	io.Reader
	closers []io.Closer
}

func (r readerCloser) Close() error { return closeAll(r.closers...) }

// writerCloser couples a writer with the closers that release it. Closers must
// be supplied outermost-first (for example the gzip writer before the file) so
// that buffered data is flushed before the underlying resource is closed.
type writerCloser struct {
	io.Writer
	closers []io.Closer
}

func (w writerCloser) Close() error { return closeAll(w.closers...) }

// nopWriteCloser adds a no-op Close to a writer, used to keep os.Stdout open.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// retryPolicy configures bounded retries with exponential backoff.
type retryPolicy struct {
	attempts int
	base     time.Duration
}

// retry invokes fn until it returns nil, the attempts are exhausted, or ctx is
// cancelled. Between attempts it waits base, then doubles the wait each time.
func retry(ctx context.Context, p retryPolicy, fn func(context.Context) error) error {
	if p.attempts < 1 {
		p.attempts = 1
	}
	backoff := p.base
	var err error
	for i := 0; i < p.attempts; i++ {
		if err = fn(ctx); err == nil {
			return nil
		}
		if i == p.attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return err
}
