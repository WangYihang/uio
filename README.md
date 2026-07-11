# uio

`uio` is a library for unified access to data across HTTP(S), the local file
system, and S3-compatible storage. It exposes a single `Open` function that
returns an `io.ReadWriteCloser`, so the same code can read from (or write to) any
supported backend. Files ending in `.gz`/`.gzip` are compressed and decompressed
transparently.

## Features

- Read from HTTP/HTTPS, the file system, S3, and standard input.
- Write to the file system, S3, and standard output.
- Automatic gzip compression/decompression based on the file extension.
- Silent by default; opt in to logging with `WithLogger`.

## Installation

```sh
go get github.com/WangYihang/uio
```

## Usage

```go
package main

import (
	"fmt"
	"io"

	"github.com/WangYihang/uio"
)

func main() {
	fd, err := uio.Open("http://example.com/data.txt")
	if err != nil {
		fmt.Println("Error opening resource:", err)
		return
	}
	defer fd.Close()

	data, err := io.ReadAll(fd)
	if err != nil {
		fmt.Println("Error reading data:", err)
		return
	}
	fmt.Println("Data:", string(data))
}
```

### Supported URIs

| URI                                   | Description                                  |
| ------------------------------------- | -------------------------------------------- |
| `http://host/data.txt`                | Read over HTTP (read-only)                   |
| `https://host/data.txt.gz`            | Read over HTTPS, decompressed automatically  |
| `file://path/to/data.txt`             | Read a local file (relative to the cwd)      |
| `file:///abs/path/data.txt?mode=write`| Write (truncate) a local file                |
| `s3://bucket/key.txt`                 | Read an S3 object                            |
| `s3://bucket/key.txt?mode=write`      | Write an S3 object                           |
| `-`                                   | Read from stdin / write to stdout            |

### File modes

Local file and S3 access accept a `mode` query parameter:

| Mode     | Behavior                                             |
| -------- | ---------------------------------------------------- |
| `read`   | Open for reading (**default**); errors if missing    |
| `write`  | Truncate (creating if needed) and write              |
| `append` | Append, creating the file if needed (local files)    |

`read` is the default so that opening a resource never creates or clobbers it by
accident. Pass `?mode=write` to write. Gzip is applied automatically when the
name ends in `.gz` or `.gzip`.

### S3 configuration

S3 credentials can be provided via query parameters or environment variables
(query parameters take precedence):

| Query parameter | Environment variable | Description                       |
| --------------- | -------------------- | --------------------------------- |
| `endpoint`      | `S3_ENDPOINT`        | S3 endpoint (host:port)           |
| `access_key`    | `S3_ACCESS_KEY`      | Access key                        |
| `secret_key`    | `S3_SECRET_KEY`      | Secret key                        |
| `insecure`      | `S3_INSECURE`        | Use HTTP instead of HTTPS         |

> Prefer environment variables for credentials. Values placed in the URL query
> may end up in logs or process listings.

### Logging

`Open` is silent by default. Pass a `*slog.Logger` to enable diagnostics:

```go
logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
fd, err := uio.Open("s3://bucket/key.txt", uio.WithLogger(logger))
```

## Development

The test suite is self-contained (HTTP tests use `httptest`). Run it with:

```sh
go test ./...
```

S3 integration tests are skipped unless `UIO_TEST_S3=1` is set and a compatible
endpoint is running:

```sh
docker compose up -d
UIO_TEST_S3=1 go test ./...
```
