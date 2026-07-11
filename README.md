# uio

`uio` provides unified access to data across HTTP(S), the local file system, and
S3-compatible object storage behind one small API. [`Open`](https://pkg.go.dev/github.com/WangYihang/uio#Open)
returns an `io.ReadCloser` and [`Create`](https://pkg.go.dev/github.com/WangYihang/uio#Create)
returns an `io.WriteCloser`, so the same code can move bytes to or from any
supported backend. Files ending in `.gz`/`.gzip` are compressed and decompressed
transparently.

## Features

- Read from HTTP/HTTPS, the file system, S3, and standard input.
- Write to the file system, S3, and standard output.
- Automatic gzip compression/decompression based on the file extension.
- First-class `context.Context` for cancellation and timeouts.
- Silent by default; opt in to logging with `WithLogger`.
- Extensible: register your own scheme with `Register`.

## Installation

```sh
go get github.com/WangYihang/uio
```

Requires Go 1.25 or newer.

## Usage

```go
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/WangYihang/uio"
)

func main() {
	ctx := context.Background()

	// Read (decompressed automatically because of the .gz suffix).
	r, err := uio.Open(ctx, "https://example.com/data.txt.gz")
	if err != nil {
		panic(err)
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))

	// Write (compressed automatically because of the .gz suffix).
	w, err := uio.Create(ctx, "file:///tmp/out.txt.gz")
	if err != nil {
		panic(err)
	}
	if _, err := w.Write(data); err != nil {
		panic(err)
	}
	// Close flushes the gzip stream (and, for S3, performs the upload).
	if err := w.Close(); err != nil {
		panic(err)
	}
}
```

### Supported URIs

| URI                                    | `Open` | `Create` | Notes                              |
| -------------------------------------- | :----: | :------: | ---------------------------------- |
| `http(s)://host/data.txt`              |   ✅   |    —     | Read-only                          |
| `file://path/to/data.txt`              |   ✅   |    ✅    | Relative to the working directory  |
| `file:///abs/path/data.txt`            |   ✅   |    ✅    | Absolute path                      |
| `s3://bucket/key.txt`                  |   ✅   |    ✅    | S3-compatible object storage       |
| `-`                                    |   ✅   |    ✅    | Standard input / standard output   |

Any name ending in `.gz` or `.gzip` is gzip-encoded on `Create` and decoded on
`Open`.

### The command-line tool

`cmd/uio` is a universal copy tool: `uio <src> [dst]`. With `dst` omitted it
writes to standard output.

```sh
go run ./cmd/uio https://example.com/data.txt.gz        # print, decompressed
go run ./cmd/uio s3://bucket/key.txt ./key.txt          # download
go run ./cmd/uio ./key.txt s3://bucket/key.txt.gz       # upload, compressed
```

## Options

`Open` and `Create` accept functional options:

| Option                                  | Effect                                        |
| --------------------------------------- | --------------------------------------------- |
| `WithLogger(*slog.Logger)`              | Enable logging (silent by default)            |
| `WithHTTPClient(*http.Client)`          | Use a custom HTTP client                      |
| `WithCompression(Compression)`          | Force `CompressionGzip` / `CompressionNone`   |
| `WithoutCompression()`                  | Disable gzip regardless of extension          |
| `WithGzipLevel(int)`                    | Set the gzip level used on write              |
| `WithAppend()`                          | Append instead of truncating (file only)      |
| `WithS3Credentials(endpoint, ak, sk, secure)` | Configure the S3 endpoint and credentials |

```go
logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
w, err := uio.Create(ctx, "s3://bucket/key.txt",
	uio.WithLogger(logger),
	uio.WithAppend(), // ignored by S3, honoured by files
)
```

### S3 configuration

S3 settings resolve in order of precedence: **URL query parameters** override
values from `WithS3Credentials`, which override the environment variables.

| Query parameter | Environment variable | Description                 |
| --------------- | -------------------- | --------------------------- |
| `endpoint`      | `S3_ENDPOINT`        | Endpoint (`host:port`)      |
| `access_key`    | `S3_ACCESS_KEY`      | Access key                  |
| `secret_key`    | `S3_SECRET_KEY`      | Secret key                  |
| `insecure`      | `S3_INSECURE`        | Use HTTP instead of HTTPS   |

> Prefer environment variables or `WithS3Credentials` for secrets. Credentials
> placed in a URL query may leak into logs or process listings.

## Extending

Register a `Provider` to add a scheme:

```go
uio.Register("gcs", myGCSProvider{})
r, err := uio.Open(ctx, "gcs://bucket/object")
```

Compression is applied by uio around whatever reader or writer a provider
returns, so providers only deal with raw bytes.

## Development

The unit tests are self-contained — HTTP uses `httptest` and S3 uses an
in-memory store, so no external services are required:

```sh
make test      # or: go test ./...
make race      # race detector
make cover     # coverage summary
make lint      # golangci-lint
```

The S3 integration tests run against the `docker-compose` MinIO service and are
skipped unless `UIO_TEST_S3` is set:

```sh
make integration
```
