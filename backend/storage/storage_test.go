package storage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConfigFromEnv(t *testing.T) {
	set := func(t *testing.T, kv map[string]string) {
		for _, k := range []string{"S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_BUCKET", "S3_USE_SSL",
			"S3_REGION", "S3_PUBLIC_URL", "S3_SKIP_BUCKET_SETUP"} {
			t.Setenv(k, kv[k])
		}
	}
	base := map[string]string{"S3_ENDPOINT": "localhost:9000", "S3_ACCESS_KEY": "a", "S3_SECRET_KEY": "s", "S3_BUCKET": "b"}

	t.Run("defaults", func(t *testing.T) {
		set(t, base)
		c, err := ConfigFromEnv()
		if err != nil {
			t.Fatal(err)
		}
		if c.Region != DefaultRegion || c.PublicURL != "http://localhost:9000/b" || c.UseSSL || c.SkipBucketSetup {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("explicit", func(t *testing.T) {
		kv := map[string]string{"S3_USE_SSL": "true", "S3_REGION": "auto", "S3_PUBLIC_URL": "https://cdn.example/x/",
			"S3_SKIP_BUCKET_SETUP": "1"}
		for k, v := range base {
			kv[k] = v
		}
		set(t, kv)
		c, err := ConfigFromEnv()
		if err != nil {
			t.Fatal(err)
		}
		if !c.UseSSL || c.Region != "auto" || c.PublicURL != "https://cdn.example/x" || !c.SkipBucketSetup {
			t.Fatalf("got %+v", c)
		}
	})
	t.Run("ssl default public url", func(t *testing.T) {
		kv := map[string]string{"S3_USE_SSL": "true"}
		for k, v := range base {
			kv[k] = v
		}
		set(t, kv)
		c, _ := ConfigFromEnv()
		if c.PublicURL != "https://localhost:9000/b" {
			t.Fatalf("got %q", c.PublicURL)
		}
	})
	t.Run("missing", func(t *testing.T) {
		set(t, map[string]string{"S3_BUCKET": "b"})
		_, err := ConfigFromEnv()
		if err == nil || !strings.Contains(err.Error(), "S3_ACCESS_KEY, S3_ENDPOINT, S3_SECRET_KEY") {
			t.Fatalf("err = %v", err)
		}
	})
	for _, name := range []string{"S3_USE_SSL", "S3_SKIP_BUCKET_SETUP"} {
		t.Run("bad "+name, func(t *testing.T) {
			kv := map[string]string{name: "maybe"}
			for k, v := range base {
				kv[k] = v
			}
			set(t, kv)
			if _, err := ConfigFromEnv(); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestURL(t *testing.T) {
	m, err := NewMinIO(Config{Endpoint: "localhost:9000", AccessKey: "a", SecretKey: "s", Bucket: "folixenda-events"})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.URL("events/e1/i1.jpg"); got != "http://localhost:9000/folixenda-events/events/e1/i1.jpg" {
		t.Fatalf("got %q", got)
	}
	if got := m.URL("/events/a b/ñ.png"); got != "http://localhost:9000/folixenda-events/events/a%20b/%C3%B1.png" {
		t.Fatalf("got %q", got)
	}
	if m.Bucket() != "folixenda-events" {
		t.Fatal(m.Bucket())
	}
}

func TestNewMinIOBadEndpoint(t *testing.T) {
	if _, err := NewMinIO(Config{Endpoint: "localhost:9000/with/path", Bucket: "b"}); err == nil {
		t.Fatal("expected error for an endpoint with a scheme")
	}
}

func TestPublicReadPolicy(t *testing.T) {
	var p struct {
		Version   string
		Statement []struct {
			Effect    string
			Principal struct{ AWS []string }
			Action    []string
			Resource  []string
		}
	}
	if err := json.Unmarshal([]byte(PublicReadPolicy("bkt")), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Statement) != 1 {
		t.Fatalf("got %+v", p)
	}
	s := p.Statement[0]
	if p.Version != "2012-10-17" || s.Effect != "Allow" || s.Principal.AWS[0] != "*" ||
		len(s.Action) != 1 || s.Action[0] != "s3:GetObject" || s.Resource[0] != "arn:aws:s3:::bkt/*" {
		t.Fatalf("got %+v", p)
	}
}

// fakeS3 answers the few S3 calls MinIO makes, recording them.
type fakeS3 struct {
	mu           sync.Mutex
	bucketExists bool
	fail         map[string]int // "METHOD kind" → status to return
	calls        []string
	putHeaders   http.Header
	putBody      string
	policy       string
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	kind := "object"
	if strings.Count(strings.Trim(r.URL.Path, "/"), "/") == 0 {
		kind = "bucket"
	}
	if _, ok := r.URL.Query()["policy"]; ok {
		kind = "policy"
	}
	call := r.Method + " " + kind
	f.calls = append(f.calls, call)
	if status, ok := f.fail[call]; ok {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(status)
		code := "AccessDenied"
		if status == 404 {
			code = "NoSuchKey"
		}
		io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>`+code+`</Code><Message>x</Message></Error>`)
		return
	}
	switch call {
	case "HEAD bucket":
		if !f.bucketExists {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	case "PUT bucket":
		f.bucketExists = true
	case "PUT policy":
		b, _ := io.ReadAll(r.Body)
		f.policy = string(b)
		w.WriteHeader(http.StatusNoContent)
		return
	case "PUT object":
		b, _ := io.ReadAll(r.Body)
		f.putBody = string(b)
		f.putHeaders = r.Header.Clone()
		w.Header().Set("ETag", `"abc"`)
	case "DELETE object":
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func newFake(t *testing.T, f *fakeS3, skipSetup bool) *MinIO {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	m, err := NewMinIO(Config{Endpoint: u.Host, AccessKey: "a", SecretKey: "s", Bucket: "bkt", SkipBucketSetup: skipSetup})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestEnsureBucket(t *testing.T) {
	ctx := context.Background()
	t.Run("creates bucket and policy", func(t *testing.T) {
		f := &fakeS3{}
		if err := newFake(t, f, false).EnsureBucket(ctx); err != nil {
			t.Fatal(err)
		}
		if strings.Join(f.calls, ",") != "HEAD bucket,PUT bucket,PUT policy" || !strings.Contains(f.policy, "s3:GetObject") {
			t.Fatalf("calls %v policy %s", f.calls, f.policy)
		}
	})
	t.Run("existing bucket only gets the policy", func(t *testing.T) {
		f := &fakeS3{bucketExists: true}
		if err := newFake(t, f, false).EnsureBucket(ctx); err != nil {
			t.Fatal(err)
		}
		if strings.Join(f.calls, ",") != "HEAD bucket,PUT policy" {
			t.Fatalf("calls %v", f.calls)
		}
	})
	t.Run("skip setup: exists", func(t *testing.T) {
		f := &fakeS3{bucketExists: true}
		if err := newFake(t, f, true).EnsureBucket(ctx); err != nil || len(f.calls) != 1 {
			t.Fatalf("err %v calls %v", err, f.calls)
		}
	})
	t.Run("skip setup: missing", func(t *testing.T) {
		if err := newFake(t, &fakeS3{}, true).EnsureBucket(ctx); err == nil || !strings.Contains(err.Error(), "does not exist") {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("unreachable", func(t *testing.T) {
		m, _ := NewMinIO(Config{Endpoint: "127.0.0.1:1", AccessKey: "a", SecretKey: "s", Bucket: "bkt"})
		short, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		if err := m.EnsureBucket(short); err == nil || !strings.Contains(err.Error(), "unreachable") {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("make bucket fails", func(t *testing.T) {
		f := &fakeS3{fail: map[string]int{"PUT bucket": 403}}
		if err := newFake(t, f, false).EnsureBucket(ctx); err == nil || !strings.Contains(err.Error(), "create bucket") {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("policy fails", func(t *testing.T) {
		f := &fakeS3{bucketExists: true, fail: map[string]int{"PUT policy": 403}}
		if err := newFake(t, f, false).EnsureBucket(ctx); err == nil || !strings.Contains(err.Error(), "policy") {
			t.Fatalf("err %v", err)
		}
	})
}

func TestPutDelete(t *testing.T) {
	ctx := context.Background()
	f := &fakeS3{bucketExists: true}
	m := newFake(t, f, false)
	if err := m.Put(ctx, "events/e1/i1.png", strings.NewReader("data"), 4, "image/png"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.putBody, "data") || f.putHeaders.Get("Content-Type") != "image/png" || f.putHeaders.Get("Cache-Control") != CacheControl {
		t.Fatalf("body %q headers %v", f.putBody, f.putHeaders)
	}
	if err := m.Delete(ctx, "events/e1/i1.png"); err != nil {
		t.Fatal(err)
	}

	f.fail = map[string]int{"DELETE object": 404}
	if err := m.Delete(ctx, "missing"); err != nil {
		t.Fatalf("missing key must not fail: %v", err)
	}
	f.fail = map[string]int{"DELETE object": 403}
	if err := m.Delete(ctx, "x"); err == nil {
		t.Fatal("expected error")
	}
}

var _ Storage = (*MinIO)(nil)
