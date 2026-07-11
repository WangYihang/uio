package uio_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/WangYihang/uio"
)

// Example demonstrates a gzip round trip: Create compresses transparently
// because the name ends in .gz, and Open decompresses it again.
func Example() {
	dir, _ := os.MkdirTemp("", "uio-example")
	defer func() { _ = os.RemoveAll(dir) }()
	ctx := context.Background()
	uri := "file://" + filepath.Join(dir, "greeting.txt.gz")

	w, err := uio.Create(ctx, uri)
	if err != nil {
		panic(err)
	}
	_, _ = io.WriteString(w, "Hello, uio!")
	_ = w.Close() // flushes the gzip stream

	r, err := uio.Open(ctx, uri)
	if err != nil {
		panic(err)
	}
	defer func() { _ = r.Close() }()

	data, _ := io.ReadAll(r)
	fmt.Println(string(data))
	// Output: Hello, uio!
}
