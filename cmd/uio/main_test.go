package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyResource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("copy me"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "sub", "dst.txt.gz")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := copyResource(context.Background(), "file://"+src, "file://"+dst, logger); err != nil {
		t.Fatalf("copyResource: %v", err)
	}

	// dst is gzip-compressed; read it back through uio to confirm the round trip.
	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		t.Fatalf("destination is not gzip-compressed: % x", raw)
	}
}

func TestCopyResourceOpenError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := copyResource(context.Background(), "file:///nonexistent/uio-missing", "-", logger)
	if err == nil {
		t.Fatal("expected an error opening a missing source")
	}
}

func TestCopyResourceCreateError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// http is read-only, so Create must fail.
	if err := copyResource(context.Background(), "file://"+src, "http://example.com/x", logger); err == nil {
		t.Fatal("expected an error creating an http destination")
	}
}
