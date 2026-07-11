package uio

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// objectStore is the subset of object storage the s3 provider relies on. The
// seam lets the provider be tested without a live S3 endpoint.
type objectStore interface {
	// Get returns a reader for the object, or an error if it does not exist.
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	// Put uploads the file at filePath to the object.
	Put(ctx context.Context, bucket, key, filePath string) error
}

// s3Provider opens S3 objects. newStore builds the objectStore for a request
// and is overridable in tests.
type s3Provider struct {
	newStore func(ctx context.Context, u *url.URL, o Options) (objectStore, error)
	retry    retryPolicy
}

func newS3Provider() s3Provider {
	return s3Provider{
		newStore: newMinioStore,
		retry:    retryPolicy{attempts: 5, base: 100 * time.Millisecond},
	}
}

func bucketAndKey(u *url.URL) (string, string) {
	return u.Host, strings.TrimLeft(u.Path, "/")
}

// Open returns a reader for the S3 object.
func (p s3Provider) Open(ctx context.Context, u *url.URL, o Options) (io.ReadCloser, error) {
	store, err := p.newStore(ctx, u, o)
	if err != nil {
		return nil, err
	}
	bucket, key := bucketAndKey(u)
	o.Logger.Info("s3 get", slog.String("bucket", bucket), slog.String("key", key))
	return store.Get(ctx, bucket, key)
}

// Create buffers writes in a temporary file and uploads it to S3 on Close.
func (p s3Provider) Create(ctx context.Context, u *url.URL, o Options) (io.WriteCloser, error) {
	store, err := p.newStore(ctx, u, o)
	if err != nil {
		return nil, err
	}
	bucket, key := bucketAndKey(u)
	tmp, err := os.CreateTemp("", "uio-s3-*")
	if err != nil {
		return nil, err
	}
	o.Logger.Info("s3 create", slog.String("bucket", bucket), slog.String("key", key), slog.String("temp", tmp.Name()))
	return &s3Upload{
		ctx:    ctx,
		file:   tmp,
		store:  store,
		bucket: bucket,
		key:    key,
		retry:  p.retry,
		logger: o.Logger,
	}, nil
}

// s3Upload buffers written bytes in a temporary file and uploads it on Close.
type s3Upload struct {
	ctx    context.Context
	file   *os.File
	store  objectStore
	bucket string
	key    string
	retry  retryPolicy
	logger *slog.Logger
}

func (w *s3Upload) Write(p []byte) (int, error) {
	return w.file.Write(p)
}

// Close flushes the buffer, uploads it to S3 with retries, and removes the
// temporary file.
func (w *s3Upload) Close() error {
	name := w.file.Name()
	defer func() { _ = os.Remove(name) }()

	if err := w.file.Close(); err != nil {
		return fmt.Errorf("uio: s3: close temp file: %w", err)
	}

	w.logger.Info("s3 put", slog.String("bucket", w.bucket), slog.String("key", w.key))
	err := retry(w.ctx, w.retry, func(ctx context.Context) error {
		return w.store.Put(ctx, w.bucket, w.key, name)
	})
	if err != nil {
		return fmt.Errorf("uio: s3: upload %s/%s: %w", w.bucket, w.key, err)
	}
	return nil
}

// minioStore is the production objectStore backed by a MinIO/S3 client.
type minioStore struct{ client *minio.Client }

func newMinioStore(_ context.Context, u *url.URL, o Options) (objectStore, error) {
	cfg := resolveS3(u, o)
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("uio: s3: no endpoint configured")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
	})
	if err != nil {
		return nil, fmt.Errorf("uio: s3: %w", err)
	}
	return minioStore{client: client}, nil
}

func (m minioStore) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	// Stat first so a missing object fails here rather than on first Read.
	if _, err := m.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{}); err != nil {
		return nil, err
	}
	obj, err := m.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (m minioStore) Put(ctx context.Context, bucket, key, filePath string) error {
	_, err := m.client.FPutObject(ctx, bucket, key, filePath, minio.PutObjectOptions{})
	return err
}

// s3FromEnv reads S3 settings from the S3_* environment variables.
func s3FromEnv() S3Options {
	insecure, _ := strconv.ParseBool(os.Getenv("S3_INSECURE"))
	return S3Options{
		Endpoint:  os.Getenv("S3_ENDPOINT"),
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
		Secure:    !insecure,
	}
}

// resolveS3 merges S3 settings. The base comes from WithS3Credentials when set,
// otherwise from the environment; URL query parameters override individual
// fields.
func resolveS3(u *url.URL, o Options) S3Options {
	s := o.S3
	if s.Endpoint == "" {
		s = s3FromEnv()
	}
	q := u.Query()
	if v := q.Get("endpoint"); v != "" {
		s.Endpoint = v
	}
	if v := q.Get("access_key"); v != "" {
		s.AccessKey = v
	}
	if v := q.Get("secret_key"); v != "" {
		s.SecretKey = v
	}
	if v := q.Get("insecure"); v != "" {
		s.Secure = v == "false"
	}
	return s
}
