package uio_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/WangYihang/uio"
	"github.com/google/uuid"
)

// TestIntegrationS3 exercises the real MinIO-backed provider end to end. It is
// skipped unless UIO_TEST_S3 is set, since it needs the services from
// docker-compose.yaml running:
//
//	docker compose up -d
//	UIO_TEST_S3=1 go test -run TestIntegrationS3 ./...
func TestIntegrationS3(t *testing.T) {
	if os.Getenv("UIO_TEST_S3") == "" {
		t.Skip("set UIO_TEST_S3=1 and start docker-compose to run S3 integration tests")
	}

	const creds = "endpoint=127.0.0.1:9000&access_key=minioadmin&secret_key=minioadmin&insecure=true"
	ctx := context.Background()

	for _, name := range []string{"plain.txt", "compressed.txt.gz"} {
		key := fmt.Sprintf("test_%s_%s", uuid.NewString(), name)
		uri := fmt.Sprintf("s3://uio/%s?%s", key, creds)
		payload := []byte("Hello from the S3 integration test!")

		w, err := uio.Create(ctx, uri)
		if err != nil {
			t.Fatalf("Create(%q): %v", uri, err)
		}
		if _, err := w.Write(payload); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		r, err := uio.Open(ctx, uri)
		if err != nil {
			t.Fatalf("Open(%q): %v", uri, err)
		}
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		_ = r.Close()

		if !bytes.Equal(got, payload) {
			t.Errorf("round trip %q = %q, want %q", uri, got, payload)
		}
	}
}
