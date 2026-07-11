package uio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

var errTransient = errors.New("transient upload failure")

// memStore is an in-memory objectStore used to test the s3 provider without a
// live endpoint.
type memStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	failPut int // number of initial Put calls to fail with errTransient
}

func newMemStore() *memStore { return &memStore{objects: map[string][]byte{}} }

func (m *memStore) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[bucket+"/"+key]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), b...))), nil
}

func (m *memStore) Put(ctx context.Context, bucket, key, filePath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	if m.failPut > 0 {
		m.failPut--
		m.mu.Unlock()
		return errTransient
	}
	m.mu.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.objects[bucket+"/"+key] = data
	m.mu.Unlock()
	return nil
}

func fakeS3(store objectStore) s3Provider {
	return s3Provider{
		newStore: func(context.Context, *url.URL, Options) (objectStore, error) {
			return store, nil
		},
		retry: retryPolicy{attempts: 5, base: time.Millisecond},
	}
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return u
}

func TestS3ProviderRoundTrip(t *testing.T) {
	store := newMemStore()
	p := fakeS3(store)
	ctx := context.Background()
	u := mustURL(t, "s3://bucket/key.txt")

	w, err := p.Create(ctx, u, Options{Logger: discardLogger()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := io.WriteString(w, "payload"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := p.Open(ctx, u, Options{Logger: discardLogger()})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, _ := io.ReadAll(r)
	_ = r.Close()
	if string(got) != "payload" {
		t.Fatalf("round trip = %q, want %q", got, "payload")
	}
}

func TestS3ProviderReadMissing(t *testing.T) {
	p := fakeS3(newMemStore())
	_, err := p.Open(context.Background(), mustURL(t, "s3://bucket/absent"), Options{Logger: discardLogger()})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open missing = %v, want fs.ErrNotExist", err)
	}
}

func TestS3ProviderUploadRetry(t *testing.T) {
	store := newMemStore()
	store.failPut = 2 // fail twice, then succeed
	p := fakeS3(store)
	u := mustURL(t, "s3://bucket/key.txt")

	w, err := p.Create(context.Background(), u, Options{Logger: discardLogger()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, _ = io.WriteString(w, "with retries")
	if err := w.Close(); err != nil {
		t.Fatalf("Close should succeed after retries: %v", err)
	}
	if string(store.objects["bucket/key.txt"]) != "with retries" {
		t.Fatalf("stored = %q", store.objects["bucket/key.txt"])
	}
}

func TestS3ProviderUploadExhausted(t *testing.T) {
	store := newMemStore()
	store.failPut = 100 // always fail within the attempt budget
	p := fakeS3(store)
	u := mustURL(t, "s3://bucket/key.txt")

	w, _ := p.Create(context.Background(), u, Options{Logger: discardLogger()})
	_, _ = io.WriteString(w, "doomed")
	if err := w.Close(); !errors.Is(err, errTransient) {
		t.Fatalf("Close err = %v, want errTransient", err)
	}
	if _, err := os.Stat("bucket/key.txt"); err == nil {
		t.Fatal("temp file must be cleaned up")
	}
}

func TestS3ProviderContextCancel(t *testing.T) {
	p := fakeS3(newMemStore())
	ctx, cancel := context.WithCancel(context.Background())

	w, _ := p.Create(ctx, mustURL(t, "s3://bucket/key.txt"), Options{Logger: discardLogger()})
	_, _ = io.WriteString(w, "data")
	cancel() // cancel before upload
	if err := w.Close(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Close err = %v, want context.Canceled", err)
	}
}

// TestS3ThroughPublicAPI wires the fake store into the registry so the full
// stack (routing + central compression + provider) is exercised end to end.
func TestS3ThroughPublicAPI(t *testing.T) {
	store := newMemStore()
	Register("s3mem", fakeS3(store))
	ctx := context.Background()

	for _, name := range []string{"plain.txt", "compressed.txt.gz"} {
		uri := "s3mem://bucket/" + name

		wc, err := Create(ctx, uri)
		if err != nil {
			t.Fatalf("Create(%q): %v", uri, err)
		}
		if _, err := io.WriteString(wc, "hello s3"); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if err := wc.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		rc, err := Open(ctx, uri)
		if err != nil {
			t.Fatalf("Open(%q): %v", uri, err)
		}
		got, _ := io.ReadAll(rc)
		_ = rc.Close()
		if string(got) != "hello s3" {
			t.Fatalf("round trip %q = %q", uri, got)
		}
	}

	// The gzip object must actually be compressed on the wire.
	raw := store.objects["bucket/compressed.txt.gz"]
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		t.Fatalf("stored .gz object is not gzip-compressed: % x", raw)
	}
}
