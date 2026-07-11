package uio_test

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/WangYihang/uio"
)

// These tests swap the process standard streams, so they must not run in
// parallel with anything else.

func TestOpenStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	go func() {
		_, _ = io.WriteString(w, "from stdin")
		_ = w.Close()
	}()

	rc, err := uio.Open(context.Background(), "-")
	if err != nil {
		t.Fatalf("Open(-): %v", err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	_ = rc.Close()
	if string(got) != "from stdin" {
		t.Errorf("stdin read = %q, want %q", got, "from stdin")
	}
}

func TestCreateStdout(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	wc, err := uio.Create(context.Background(), "-")
	if err != nil {
		t.Fatalf("Create(-): %v", err)
	}
	if _, err := io.WriteString(wc, "to stdout"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Close must be a no-op that leaves os.Stdout usable.
	if err := wc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stdout.WriteString(""); err != nil {
		t.Fatalf("stdout should still be open after Close: %v", err)
	}
	_ = w.Close()
	os.Stdout = old

	got, _ := io.ReadAll(r)
	if string(got) != "to stdout" {
		t.Errorf("stdout write = %q, want %q", got, "to stdout")
	}
}
