// Command uio copies data between any two locations uio understands.
//
//	uio <src> [dst]
//
// When dst is omitted it defaults to "-" (standard output), so "uio <src>"
// prints a resource. Both src and dst may be any supported URI, for example:
//
//	uio https://example.com/data.txt.gz            # print, decompressed
//	uio s3://bucket/key.txt ./key.txt              # download
//	uio ./key.txt s3://bucket/key.txt.gz           # upload, compressed
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/WangYihang/uio"
)

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Fprintln(os.Stderr, "usage: uio <src> [dst]")
		os.Exit(2)
	}
	dst := "-"
	if len(os.Args) == 3 {
		dst = os.Args[2]
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	if err := copyResource(context.Background(), os.Args[1], dst, logger); err != nil {
		fmt.Fprintf(os.Stderr, "uio: %v\n", err)
		os.Exit(1)
	}
}

func copyResource(ctx context.Context, src, dst string, logger *slog.Logger) error {
	r, err := uio.Open(ctx, src, uio.WithLogger(logger))
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	w, err := uio.Create(ctx, dst, uio.WithLogger(logger))
	if err != nil {
		return err
	}

	if _, err := io.Copy(w, r); err != nil {
		_ = w.Close()
		return err
	}
	// Closing the writer flushes compression buffers and performs the S3 upload.
	return w.Close()
}
