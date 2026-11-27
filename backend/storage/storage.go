// Package storage keeps uploaded files (event images). Local stores them on
// disk and serves them from the API (development); MinIO talks to any
// S3-compatible service (AWS S3, Cloudflare R2, MinIO...) configured through
// the S3_* environment variables. main.go picks one with STORAGE_DRIVER.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
)

// Storage stores objects by key and gives their public URL.
type Storage interface {
	// Put uploads size bytes from r under key.
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	// Delete removes the object at key. Deleting a missing key is not an error.
	Delete(ctx context.Context, key string) error
	// URL is the public URL of key, usable as <img src>.
	URL(key string) string
}

// CacheControl is sent with every uploaded object. Keys are never reused (a
// new upload gets a new id), so objects can be cached forever.
const CacheControl = "public, max-age=31536000, immutable"

// DefaultRegion is used when S3_REGION is empty.
const DefaultRegion = "us-east-1"

// Config selects and authenticates the object store.
type Config struct {
	Endpoint  string // host[:port], no scheme (S3_ENDPOINT)
	AccessKey string // S3_ACCESS_KEY
	SecretKey string // S3_SECRET_KEY
	Bucket    string // S3_BUCKET
	UseSSL    bool   // S3_USE_SSL
	Region    string // S3_REGION, DefaultRegion when empty
	// PublicURL is the base URL objects are served from (S3_PUBLIC_URL); keys
	// are appended to it. Defaults to http(s)://Endpoint/Bucket.
	PublicURL string
	// SkipBucketSetup skips creating the bucket and its public-read policy on
	// startup (S3_SKIP_BUCKET_SETUP), for providers where that is managed
	// outside the API.
	SkipBucketSetup bool
}

// ConfigFromEnv reads the S3_* variables. Endpoint, keys and bucket are
// required.
func ConfigFromEnv() (Config, error) {
	c := Config{
		Endpoint:  strings.TrimSpace(os.Getenv("S3_ENDPOINT")),
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
		Bucket:    strings.TrimSpace(os.Getenv("S3_BUCKET")),
		Region:    strings.TrimSpace(os.Getenv("S3_REGION")),
		PublicURL: strings.TrimSpace(os.Getenv("S3_PUBLIC_URL")),
	}
	var err error
	if c.UseSSL, err = envBool("S3_USE_SSL"); err != nil {
		return c, err
	}
	if c.SkipBucketSetup, err = envBool("S3_SKIP_BUCKET_SETUP"); err != nil {
		return c, err
	}
	var missing []string
	for name, v := range map[string]string{"S3_ENDPOINT": c.Endpoint, "S3_ACCESS_KEY": c.AccessKey,
		"S3_SECRET_KEY": c.SecretKey, "S3_BUCKET": c.Bucket} {
		if v == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return c, fmt.Errorf("object storage is not configured: missing %s", strings.Join(slices.Sorted(slices.Values(missing)), ", "))
	}
	return c.withDefaults(), nil
}

func envBool(name string) (bool, error) {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s=%q: expected true or false", name, v)
	}
	return b, nil
}

// withDefaults fills Region and PublicURL.
func (c Config) withDefaults() Config {
	if c.Region == "" {
		c.Region = DefaultRegion
	}
	if c.PublicURL == "" {
		scheme := "http"
		if c.UseSSL {
			scheme = "https"
		}
		c.PublicURL = scheme + "://" + c.Endpoint + "/" + c.Bucket
	}
	c.PublicURL = strings.TrimRight(c.PublicURL, "/")
	return c
}

// objectURL joins base and key, escaping each key segment.
func objectURL(base, key string) string {
	parts := strings.Split(strings.TrimLeft(key, "/"), "/")
	for i, p := range parts {
		parts[i] = pathEscape(p)
	}
	return base + "/" + strings.Join(parts, "/")
}
