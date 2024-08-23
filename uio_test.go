package uio_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/WangYihang/uio"
)

func TestUniversalRead(t *testing.T) {
	testcases := []struct {
		name     string
		uri      string
		expected []byte
	}{
		{
			name:     "HTTP example",
			uri:      "http://127.0.0.1:9090/example.txt",
			expected: []byte("Hello World!"),
		},
		{
			name:     "File example",
			uri:      "file://data/example.txt",
			expected: []byte("Hello World!"),
		},
		{
			name:     "S3 example",
			uri:      "s3://uio/example.txt?endpoint=127.0.0.1:9000&access_key=xO7mW9YDxixgIqFqY4He&secret_key=UrEjic8z7qXKFy6gkH5jC5gcurJAbhaFhdsoW8KK&insecure=true",
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
