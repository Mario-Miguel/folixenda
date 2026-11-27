package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var pathEscape = url.PathEscape

// MinIO is a Storage backed by any S3-compatible service through minio-go.
type MinIO struct {
	client *minio.Client
	cfg    Config
}

// NewMinIO builds the client. It doesn't contact the server; call
// EnsureBucket to check connectivity and prepare the bucket.
func NewMinIO(cfg Config) (*MinIO, error) {
	cfg = cfg.withDefaults()
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("object storage client for %q: %w", cfg.Endpoint, err)
	}
	return &MinIO{client: client, cfg: cfg}, nil
}

// EnsureBucket creates the bucket when missing and gives it an anonymous
// read-only policy on its objects, so URLs work in <img src>. With
// SkipBucketSetup it only checks that the bucket exists.
func (m *MinIO) EnsureBucket(ctx context.Context) error {
	exists, err := m.client.BucketExists(ctx, m.cfg.Bucket)
	if err != nil {
		return fmt.Errorf("object storage at %s unreachable: %w", m.cfg.Endpoint, err)
	}
	if m.cfg.SkipBucketSetup {
		if !exists {
			return fmt.Errorf("bucket %q does not exist", m.cfg.Bucket)
		}
		return nil
	}
	if !exists {
		if err := m.client.MakeBucket(ctx, m.cfg.Bucket, minio.MakeBucketOptions{Region: m.cfg.Region}); err != nil {
			return fmt.Errorf("create bucket %q: %w", m.cfg.Bucket, err)
		}
	}
	if err := m.client.SetBucketPolicy(ctx, m.cfg.Bucket, PublicReadPolicy(m.cfg.Bucket)); err != nil {
		return fmt.Errorf("set public-read policy on bucket %q: %w", m.cfg.Bucket, err)
	}
	return nil
}

// PublicReadPolicy is an S3 bucket policy that lets anyone GET the bucket's
// objects (but not list them or write).
func PublicReadPolicy(bucket string) string {
	policy := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{{
			"Effect":    "Allow",
			"Principal": map[string]any{"AWS": []string{"*"}},
			"Action":    []string{"s3:GetObject"},
			"Resource":  []string{"arn:aws:s3:::" + bucket + "/*"},
		}},
	}
	b, _ := json.Marshal(policy) // a static map always marshals
	return string(b)
}

func (m *MinIO) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := m.client.PutObject(ctx, m.cfg.Bucket, key, r, size, minio.PutObjectOptions{
		ContentType:  contentType,
		CacheControl: CacheControl,
	})
	return err
}

func (m *MinIO) Delete(ctx context.Context, key string) error {
	err := m.client.RemoveObject(ctx, m.cfg.Bucket, key, minio.RemoveObjectOptions{})
	var resp minio.ErrorResponse
	if errors.As(err, &resp) && resp.Code == "NoSuchKey" {
		return nil
	}
	return err
}

func (m *MinIO) URL(key string) string {
	return objectURL(m.cfg.PublicURL, key)
}

// Bucket is the configured bucket name.
func (m *MinIO) Bucket() string { return m.cfg.Bucket }
