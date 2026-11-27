package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var _ Storage = (*Local)(nil)

func newLocal(t *testing.T) *Local {
	t.Helper()
	l, err := NewLocal(filepath.Join(t.TempDir(), "uploads"), "http://localhost:8080/uploads/")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestNewLocal(t *testing.T) {
	if _, err := NewLocal("", "http://x"); err == nil {
		t.Fatal("empty dir must fail")
	}
	// A regular file where the directory should be.
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocal(filepath.Join(file, "sub"), "http://x"); err == nil {
		t.Fatal("dir under a file must fail")
	}
	l := newLocal(t)
	if info, err := os.Stat(l.Dir()); err != nil || !info.IsDir() {
		t.Fatalf("dir not created: %v", err)
	}
}

func TestLocalPutURLDelete(t *testing.T) {
	l := newLocal(t)
	ctx := context.Background()
	key := "events/ev 1/img.webp"
	if err := l.Put(ctx, key, strings.NewReader("data"), 4, "image/webp"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(l.Dir(), "events", "ev 1", "img.webp"))
	if err != nil || string(got) != "data" {
		t.Fatalf("stored %q, %v", got, err)
	}
	if u := l.URL(key); u != "http://localhost:8080/uploads/events/ev%201/img.webp" {
		t.Fatalf("URL = %q", u)
	}
	// Overwrite is atomic and leaves no temp files behind.
	if err := l.Put(ctx, key, strings.NewReader("new"), -1, ""); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(l.Dir(), "events", "ev 1"))
	if len(entries) != 1 {
		t.Fatalf("leftover files: %v", entries)
	}
	if err := l.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := l.Delete(ctx, key); err != nil {
		t.Fatalf("deleting a missing key must not fail: %v", err)
	}
	if _, err := os.Stat(filepath.Join(l.Dir(), "events")); !os.IsNotExist(err) {
		t.Fatal("empty parent directories must be pruned")
	}
	if _, err := os.Stat(l.Dir()); err != nil {
		t.Fatal("the storage dir itself must stay")
	}
	// A sibling keeps its directory.
	for _, k := range []string{"events/e2/a.png", "events/e2/b.png"} {
		if err := l.Put(ctx, k, strings.NewReader("x"), 1, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Delete(ctx, "events/e2/a.png"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(l.Dir(), "events", "e2", "b.png")); err != nil {
		t.Fatal("non-empty directory must be kept")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestLocalPutErrors(t *testing.T) {
	l := newLocal(t)
	ctx := context.Background()
	for _, key := range []string{"", "../escape.png", "a/../../b.png", "a\\b.png", "a//b.png", "./a.png"} {
		if err := l.Put(ctx, key, strings.NewReader("x"), 1, ""); err == nil {
			t.Errorf("Put(%q) must fail", key)
		}
		if err := l.Delete(ctx, key); err == nil {
			t.Errorf("Delete(%q) must fail", key)
		}
	}
	if err := l.Put(ctx, "a.png", strings.NewReader("abc"), 5, ""); err == nil {
		t.Error("size mismatch must fail")
	}
	if err := l.Put(ctx, "b.png", failingReader{}, 1, ""); err == nil {
		t.Error("reader error must fail")
	}
	if _, err := os.Stat(filepath.Join(l.Dir(), "a.png")); !os.IsNotExist(err) {
		t.Error("failed put must not leave a file")
	}
	// Parent path blocked by a regular file.
	if err := os.WriteFile(filepath.Join(l.Dir(), "blocker"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := l.Put(ctx, "blocker/x.png", strings.NewReader("x"), 1, ""); err == nil {
		t.Error("mkdir under a file must fail")
	}
	// A directory where the file should go: rename fails.
	if err := os.MkdirAll(filepath.Join(l.Dir(), "dir.png", "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := l.Put(ctx, "dir.png", strings.NewReader("x"), 1, ""); err == nil {
		t.Error("rename onto a non-empty dir must fail")
	}
	if err := l.Delete(ctx, "dir.png"); err == nil {
		t.Error("deleting a non-empty dir must fail")
	}
}

func TestLocalHandler(t *testing.T) {
	l := newLocal(t)
	ctx := context.Background()
	if err := l.Put(ctx, "events/e1/a.webp", strings.NewReader("webp-bytes"), -1, ""); err != nil {
		t.Fatal(err)
	}
	if err := l.Put(ctx, "events/e1/notes.txt", strings.NewReader("secret"), -1, ""); err != nil {
		t.Fatal(err)
	}
	if err := l.Put(ctx, "events/e1/.hidden.png", strings.NewReader("x"), -1, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(l.Dir(), "folder.png"), 0o755); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads", l.Handler()))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/uploads/events/e1/a.webp")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(body) != "webp-bytes" {
		t.Fatalf("status %d body %q", res.StatusCode, body)
	}
	for h, want := range map[string]string{"Content-Type": "image/webp", "Cache-Control": CacheControl,
		"X-Content-Type-Options": "nosniff"} {
		if got := res.Header.Get(h); got != want {
			t.Errorf("%s = %q, want %q", h, got, want)
		}
	}

	for _, p := range []string{
		"/uploads/events/e1/missing.png", // not found
		"/uploads/events/e1/notes.txt",   // not an image type
		"/uploads/events/e1/.hidden.png", // dotfile (temp files)
		"/uploads/events/e1/",            // directory listing
		"/uploads/folder.png",            // directory with image extension
		"/uploads/..%2f..%2fetc/passwd.png",
	} {
		res, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 404 {
			t.Errorf("GET %s = %d, want 404", p, res.StatusCode)
		}
	}
}
