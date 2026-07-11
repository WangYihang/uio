// Package uio provides unified access to data across HTTP(S), the local file
// system, and S3-compatible object storage behind a single, small API.
//
// Open returns an [io.ReadCloser] and Create returns an [io.WriteCloser], so
// the same code can move bytes to or from any supported backend:
//
//	r, err := uio.Open(ctx, "s3://bucket/key.txt.gz")
//	w, err := uio.Create(ctx, "file:///tmp/out.txt")
//
// The scheme selects the backend: http/https fetch over HTTP (read-only), s3
// reads and writes objects, and file (or a bare path, or "-" for the standard
// streams) uses the local file system. Resources whose name ends in .gz or
// .gzip are transparently compressed and decompressed; see [Compression] to
// override that.
//
// uio is silent unless a logger is supplied with [WithLogger]. Additional
// schemes can be added with [Register].
package uio

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
)

// route parses uri and finds the provider responsible for its scheme.
func route(uri string) (Provider, *url.URL, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, nil, fmt.Errorf("uio: invalid URI %q: %w", uri, err)
	}
	p, ok := providerFor(u.Scheme)
	if !ok {
		return nil, u, fmt.Errorf("%w: %q", ErrUnsupportedScheme, u.Scheme)
	}
	return p, u, nil
}

// Open opens the resource identified by uri for reading. The caller must Close
// the returned reader. Resources ending in .gz or .gzip are decompressed
// transparently unless overridden with [WithCompression] or [WithoutCompression].
func Open(ctx context.Context, uri string, opts ...Option) (io.ReadCloser, error) {
	o := resolve(opts)
	p, u, err := route(uri)
	if err != nil {
		o.Logger.Error("open failed to route", slog.String("uri", uri), slog.String("error", err.Error()))
		return nil, err
	}
	o.Logger.Info("open", slog.String("scheme", u.Scheme), slog.String("host", u.Host), slog.String("path", u.Path))

	rc, err := p.Open(ctx, u, o)
	if err != nil {
		return nil, err
	}
	if !wantGzip(o.Compression, u.Path) {
		return rc, nil
	}

	dec, err := decompress(rc)
	if err != nil {
		_ = rc.Close()
		return nil, fmt.Errorf("uio: gzip: %w", err)
	}
	return dec, nil
}

// Create opens the resource identified by uri for writing, creating or
// truncating it (use [WithAppend] for append semantics on files). The caller
// must Close the returned writer to flush buffered data and, for S3, to upload.
// Resources ending in .gz or .gzip are compressed transparently unless
// overridden with [WithCompression] or [WithoutCompression].
func Create(ctx context.Context, uri string, opts ...Option) (io.WriteCloser, error) {
	o := resolve(opts)
	p, u, err := route(uri)
	if err != nil {
		o.Logger.Error("create failed to route", slog.String("uri", uri), slog.String("error", err.Error()))
		return nil, err
	}
	o.Logger.Info("create", slog.String("scheme", u.Scheme), slog.String("host", u.Host), slog.String("path", u.Path))

	wc, err := p.Create(ctx, u, o)
	if err != nil {
		return nil, err
	}
	if !wantGzip(o.Compression, u.Path) {
		return wc, nil
	}

	enc, err := compress(wc, gzipInnerName(u.Path), o.GzipLevel)
	if err != nil {
		_ = wc.Close()
		return nil, fmt.Errorf("uio: gzip: %w", err)
	}
	return enc, nil
}
