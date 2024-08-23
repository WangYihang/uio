# uio

`uio` is a library designed for unified access to various data sources, including HTTP, the file system, and S3 storage. It provides a simple interface for opening and reading resources from different protocols.

## Features

- Supports reading data from HTTP/HTTPS, file system, and S3 storage.
- Provides a straightforward API for resource access and handling.
- Includes customizable logging for debugging and error tracking.

## Installation

Install the `uio` library using `go get`:

```sh
go get github.com/WangYihang/uio
```

## Usage

Here's an example of how to use the `uio` library to read data from an HTTP resource:

```go
package main

import (
    "fmt"
    "github.com/WangYihang/uio"
)

func main() {
    // Open an HTTP resource
    fd, err := uio.Open("http://example.com/data.txt")

    // Open a file resource
    // fd, err := uio.Open("file:///path/to/data.txt")

    // Open an S3 resource
    // fd, err := uio.Open("s3://bucket-name/data.txt")

    if err != nil {
        fmt.Println("Error opening resource:", err)
        return
    }
    defer fd.Close()

    // Read data from the resource
    data, err := fd.ReadAll()
    if err != nil {
        fmt.Println("Error reading data:", err)
        return
    }

    fmt.Println("Data:", string(data))
}
```