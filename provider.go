package uio

import (
	"context"
	"io"
	"net/url"
	"sync"
)

// Provider opens resources for one or more URI schemes. Implementations are
// registered with Register and may be supplied by third parties.
//
// A provider that cannot serve a direction returns ErrReadNotSupported or
// ErrWriteNotSupported. Compression is applied by uio around the reader or
// writer a provider returns, so providers deal in raw bytes.
type Provider interface {
	// Open opens u for reading.
	Open(ctx context.Context, u *url.URL, o Options) (io.ReadCloser, error)
	// Create opens u for writing, creating or truncating the resource.
	Create(ctx context.Context, u *url.URL, o Options) (io.WriteCloser, error)
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Provider{}
)

// Register associates p with a URI scheme, replacing any existing provider for
// that scheme. The empty scheme handles bare paths and "-" (standard streams).
// Register is safe for concurrent use.
func Register(scheme string, p Provider) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[scheme] = p
}

func providerFor(scheme string) (Provider, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	p, ok := registry[scheme]
	return p, ok
}

func init() {
	file := fileProvider{}
	Register("", file)
	Register("file", file)

	http := httpProvider{}
	Register("http", http)
	Register("https", http)

	Register("s3", newS3Provider())
}
