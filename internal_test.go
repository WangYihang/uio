package uio

import (
	"context"
	"errors"
	"io"
	"net/url"
	"testing"
	"time"
)

// recordingCloser records the order in which closers are closed and can inject
// an error.
type recordingCloser struct {
	id     int
	log    *[]int
	err    error
	closed bool
}

func (c *recordingCloser) Close() error {
	c.closed = true
	*c.log = append(*c.log, c.id)
	return c.err
}

func TestCloseAll(t *testing.T) {
	var order []int
	c1 := &recordingCloser{id: 1, log: &order}
	c2 := &recordingCloser{id: 2, log: &order, err: errors.New("boom")}
	c3 := &recordingCloser{id: 3, log: &order, err: errors.New("later")}

	err := closeAll(c1, nil, c2, c3)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("closeAll returned %v, want first error \"boom\"", err)
	}
	if want := []int{1, 2, 3}; !equalInts(order, want) {
		t.Fatalf("close order = %v, want %v", order, want)
	}
	if !c1.closed || !c2.closed || !c3.closed {
		t.Fatal("all closers must be closed even after an error")
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRetry(t *testing.T) {
	fast := retryPolicy{attempts: 5, base: time.Millisecond}

	t.Run("succeeds first try", func(t *testing.T) {
		calls := 0
		err := retry(context.Background(), fast, func(context.Context) error {
			calls++
			return nil
		})
		if err != nil || calls != 1 {
			t.Fatalf("err=%v calls=%d, want nil/1", err, calls)
		}
	})

	t.Run("succeeds after transient failures", func(t *testing.T) {
		calls := 0
		err := retry(context.Background(), fast, func(context.Context) error {
			calls++
			if calls < 3 {
				return errors.New("transient")
			}
			return nil
		})
		if err != nil || calls != 3 {
			t.Fatalf("err=%v calls=%d, want nil/3", err, calls)
		}
	})

	t.Run("exhausts attempts", func(t *testing.T) {
		calls := 0
		want := errors.New("permanent")
		err := retry(context.Background(), retryPolicy{attempts: 3, base: time.Millisecond}, func(context.Context) error {
			calls++
			return want
		})
		if !errors.Is(err, want) || calls != 3 {
			t.Fatalf("err=%v calls=%d, want permanent/3", err, calls)
		}
	})

	t.Run("stops on context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		calls := 0
		err := retry(ctx, fast, func(context.Context) error {
			calls++
			return errors.New("fail")
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v, want context.Canceled", err)
		}
		if calls != 1 {
			t.Fatalf("calls=%d, want 1 before cancellation observed", calls)
		}
	})
}

func TestWantGzip(t *testing.T) {
	cases := []struct {
		c    Compression
		path string
		want bool
	}{
		{CompressionAuto, "x.gz", true},
		{CompressionAuto, "x.gzip", true},
		{CompressionAuto, "x.txt", false},
		{CompressionNone, "x.gz", false},
		{CompressionGzip, "x.txt", true},
	}
	for _, tc := range cases {
		if got := wantGzip(tc.c, tc.path); got != tc.want {
			t.Errorf("wantGzip(%v, %q) = %v, want %v", tc.c, tc.path, got, tc.want)
		}
	}
}

func TestGzipInnerName(t *testing.T) {
	cases := map[string]string{
		"a.txt.gz":          "a.txt",
		"a.txt.gzip":        "a.txt",
		"/path/to/a.txt.gz": "a.txt",
		"a.txt":             "a.txt",
		".gz":               "",
	}
	for in, want := range cases {
		if got := gzipInnerName(in); got != want {
			t.Errorf("gzipInnerName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFilePath(t *testing.T) {
	cases := map[string]string{
		"file://data/x.txt": "data/x.txt",
		"file:///abs/x.txt": "/abs/x.txt",
		"data/x.txt":        "data/x.txt",
		"/abs/x.txt":        "/abs/x.txt",
	}
	for in, want := range cases {
		u, err := url.Parse(in)
		if err != nil {
			t.Fatalf("parse %q: %v", in, err)
		}
		if got := filePath(u); got != want {
			t.Errorf("filePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRoute(t *testing.T) {
	t.Run("known scheme", func(t *testing.T) {
		p, u, err := route("s3://bucket/key")
		if err != nil || p == nil || u.Host != "bucket" {
			t.Fatalf("route s3 = %v, %v, %v", p, u, err)
		}
	})
	t.Run("empty scheme falls to file", func(t *testing.T) {
		p, _, err := route("data/x.txt")
		if err != nil || p == nil {
			t.Fatalf("route bare path = %v, %v", p, err)
		}
	})
	t.Run("unknown scheme", func(t *testing.T) {
		_, _, err := route("ftp://host/x")
		if !errors.Is(err, ErrUnsupportedScheme) {
			t.Fatalf("route ftp err = %v, want ErrUnsupportedScheme", err)
		}
	})
	t.Run("invalid uri", func(t *testing.T) {
		if _, _, err := route("://nope"); err == nil {
			t.Fatal("route of invalid URI should error")
		}
	})
}

func TestResolveDefaults(t *testing.T) {
	o := resolve(nil)
	if o.Logger == nil || o.HTTPClient == nil {
		t.Fatal("defaults must set Logger and HTTPClient")
	}
	if o.Compression != CompressionAuto {
		t.Errorf("default compression = %v, want Auto", o.Compression)
	}
	if !o.S3.Secure {
		t.Error("default S3.Secure should be true")
	}

	o = resolve([]Option{WithoutCompression(), WithAppend()})
	if o.Compression != CompressionNone || !o.Append {
		t.Errorf("options not applied: %+v", o)
	}
}

func TestResolveS3(t *testing.T) {
	t.Setenv("S3_ENDPOINT", "env:9000")
	t.Setenv("S3_ACCESS_KEY", "envkey")
	t.Setenv("S3_SECRET_KEY", "envsecret")
	t.Setenv("S3_INSECURE", "true")

	t.Run("env base", func(t *testing.T) {
		u, _ := url.Parse("s3://bucket/key")
		s := resolveS3(u, Options{})
		if s.Endpoint != "env:9000" || s.AccessKey != "envkey" || s.Secure {
			t.Fatalf("env resolve = %+v", s)
		}
	})

	t.Run("query overrides env", func(t *testing.T) {
		u, _ := url.Parse("s3://bucket/key?endpoint=q:9000&access_key=qkey&insecure=false")
		s := resolveS3(u, Options{})
		if s.Endpoint != "q:9000" || s.AccessKey != "qkey" || !s.Secure {
			t.Fatalf("query override = %+v", s)
		}
	})

	t.Run("option base overrides env", func(t *testing.T) {
		u, _ := url.Parse("s3://bucket/key")
		o := Options{S3: S3Options{Endpoint: "opt:9000", AccessKey: "optkey", Secure: true}}
		s := resolveS3(u, o)
		if s.Endpoint != "opt:9000" || s.AccessKey != "optkey" || !s.Secure {
			t.Fatalf("option base = %+v", s)
		}
	})
}

// verify the wrappers satisfy the standard interfaces.
var (
	_ io.ReadCloser  = readerCloser{}
	_ io.WriteCloser = writerCloser{}
	_ io.WriteCloser = nopWriteCloser{}
)
