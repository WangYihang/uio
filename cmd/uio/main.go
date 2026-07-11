package main

import (
	"fmt"
	"io"
	"os"

	"github.com/WangYihang/uio"
)

// uio is a small cat-like tool: it opens the resource named by the single
// argument and copies its contents to standard output.
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: uio <uri>")
		os.Exit(1)
	}

	fd, err := uio.Open(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open resource: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = fd.Close() }()

	if _, err := io.Copy(os.Stdout, fd); err != nil {
		fmt.Fprintf(os.Stderr, "failed to read resource: %v\n", err)
		os.Exit(1)
	}
}
