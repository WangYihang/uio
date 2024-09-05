package uio_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/WangYihang/uio"
	"github.com/google/uuid"
)

func TestUniversalRead(t *testing.T) {
	testcases := []struct {
		name     string
		uri      string
		expected []byte
	}{

		{
			name:     "File example",
			uri:      "file://data/test_read_from_file.txt",
			expected: []byte("Hello World!"),
		},
		{
			name:     "File Gzip example",
			uri:      "file://data/test_read_from_file.txt.gz",
			expected: []byte("Hello World!"),
		},
		{
			name:     "HTTP example",
			uri:      "http://127.0.0.1:9090/test_read_from_http.txt",
			expected: []byte("Hello World!"),
		},
		{
			name:     "HTTP Gzip example",
			uri:      "http://127.0.0.1:9090/test_read_from_http.txt.gz",
			expected: []byte("Hello World!"),
		},
		{
			name:     "S3 example",
			uri:      "s3://uio/test_read_from_s3.txt?endpoint=127.0.0.1:9000&access_key=minioadmin&secret_key=minioadmin&insecure=true",
			expected: []byte("Hello World!"),
		},
		{
			name:     "S3 Gzip example",
			uri:      "s3://uio/test_read_from_s3.txt.gz?endpoint=127.0.0.1:9000&access_key=minioadmin&secret_key=minioadmin&insecure=true",
			expected: []byte("Hello World!"),
		},
	}
	for _, tc := range testcases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fd, err := uio.Open(tc.uri)
			if err != nil {
				t.Fatalf("Open(%q) returned error: %v", tc.uri, err)
			}
			defer fd.Close()

			got, err := io.ReadAll(fd)
			if err != nil {
				t.Fatalf("io.ReadAll returned error: %v", err)
			}
			if !bytes.Equal(got, tc.expected) {
				t.Errorf("for URI %q, expected %v, got %v", tc.uri, tc.expected, got)
			}
		})
	}
}

func TestUniversalWrite(t *testing.T) {
	// Test cases for write operations
	suffix := uuid.New().String()
	testcases := []struct {
		name      string
		uri       string
		data      []byte
		verifyURI string
	}{
		{
			name:      "File example",
			uri:       "file://data/test_write_to_file.txt",
			data:      []byte("Hello World!"),
			verifyURI: "file://data/test_write_to_file.txt",
		},
		{
			name:      "S3 example",
			uri:       fmt.Sprintf("s3://uio/test_write_to_s3_%s.txt?endpoint=127.0.0.1:9000&access_key=minioadmin&secret_key=minioadmin&insecure=true&mode=write", suffix),
			data:      []byte("Hello World!"),
			verifyURI: fmt.Sprintf("s3://uio/test_write_to_s3_%s.txt?endpoint=127.0.0.1:9000&access_key=minioadmin&secret_key=minioadmin&insecure=true&mode=read", suffix),
		},
	}
	for _, tc := range testcases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Write data to the file/S3
			fd, err := uio.Open(tc.uri)
			if err != nil {
				t.Fatalf("Open(%q) returned error: %v", tc.uri, err)
			}
			_, err = fd.Write(tc.data)
			if err != nil {
				t.Fatalf("Write returned error: %v", err)
			}
			fd.Close()

			// Read the written data
			verifyFd, err := uio.Open(tc.verifyURI)
			if err != nil {
				t.Fatalf("Open(%q) for verification returned error: %v", tc.verifyURI, err)
			}
			got, err := io.ReadAll(verifyFd)
			if err != nil {
				t.Fatalf("io.ReadAll returned error: %v", err)
			}
			verifyFd.Close()

			// Verify the written data
			if !bytes.Equal(got, tc.data) {
				t.Errorf("for URI %q, expected %v, got %v", tc.verifyURI, tc.data, got)
			}
		})
	}
}
