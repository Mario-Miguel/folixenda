package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Local is a Storage that keeps objects as files under a directory and serves
// them from the API itself (see Handler). Meant for development; production
// uses an S3-compatible service through MinIO (STORAGE_DRIVER=s3).
type Local struct {
	dir       string
	publicURL string
}

// NewLocal stores files under dir (created if missing) and builds URLs from
// publicURL, the base the directory is served at (e.g.
// http://localhost:8080/uploads).
func NewLocal(dir, publicURL string) (*Local, error) {
	if dir == "" {
		return nil, errors.New("local storage: empty directory")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("local storage dir %q: %w", dir, err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("local storage dir %q: %w", abs, err)
	}
	return &Local{dir: abs, publicURL: strings.TrimRight(publicURL, "/")}, nil
}

// Dir is the absolute directory files are stored in.
func (l *Local) Dir() string { return l.dir }

// path maps a key to a file inside dir, rejecting keys that would escape it.
func (l *Local) path(key string) (string, error) {
	clean := path.Clean("/" + key)[1:]
	if clean == "" || clean != strings.TrimLeft(key, "/") || strings.Contains(key, "\\") {
		return "", fmt.Errorf("invalid object key %q", key)
	}
	return filepath.Join(l.dir, filepath.FromSlash(clean)), nil
}

// Put writes the object atomically: to a temp file in the same directory,
// then renamed into place, so readers never see a partial file.
func (l *Local) Put(_ context.Context, key string, r io.Reader, size int64, _ string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	n, err := io.Copy(tmp, r)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if size >= 0 && n != size {
		return fmt.Errorf("short write for %q: wrote %d of %d bytes", key, n, size)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

// Delete removes the file. A missing file is not an error.
func (l *Local) Delete(_ context.Context, key string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	// Prune directories left empty (e.g. events/{id}/ after its last image),
	// stopping at the first non-empty one and never removing dir itself.
	for d := filepath.Dir(p); d != l.dir && strings.HasPrefix(d, l.dir+string(filepath.Separator)); d = filepath.Dir(d) {
		if os.Remove(d) != nil { // fails when not empty: stop
			break
		}
	}
	return nil
}

// URL is publicURL + "/" + key, with each segment escaped.
func (l *Local) URL(key string) string { return objectURL(l.publicURL, key) }

// contentTypes are the only files Handler serves: the image types uploads
// accept. Anything else (temp files, stray files) is a 404.
var contentTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

// Handler serves stored files. Mount it with the URL prefix stripped, e.g.
// mux.Handle("GET /uploads/", http.StripPrefix("/uploads", l.Handler())).
// It never lists directories and only serves the image types uploads accept.
func (l *Local) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/")
		ct, ok := contentTypes[strings.ToLower(path.Ext(key))]
		p, err := l.path(key)
		if !ok || err != nil || strings.HasPrefix(path.Base(key), ".") {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(p)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("Content-Type", ct)
		h.Set("Cache-Control", CacheControl)
		h.Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(w, r, "", info.ModTime(), f)
	})
}
