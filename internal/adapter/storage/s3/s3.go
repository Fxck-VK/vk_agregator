// Package s3 provides an S3/MinIO-backed object store used to persist artifact
// bytes before delivery. It wraps the MinIO Go client, which speaks the S3 API
// against both AWS S3 and self-hosted MinIO.
package s3

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config holds the connection settings for the object store.
type Config struct {
	// Endpoint is the host:port of the S3/MinIO server (no scheme).
	Endpoint string
	// AccessKey and SecretKey authenticate requests.
	AccessKey string
	SecretKey string
	// UseSSL enables HTTPS to the endpoint.
	UseSSL bool
	// Region is the optional bucket region.
	Region string
}

// Store is an S3/MinIO object store.
type Store struct {
	client *minio.Client
}

// New connects to the object store and verifies the credentials/endpoint.
func New(ctx context.Context, cfg Config) (*Store, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("s3: new client: %w", err)
	}
	// A cheap call to confirm connectivity and credentials.
	if _, err := client.ListBuckets(ctx); err != nil {
		return nil, fmt.Errorf("s3: connectivity check: %w", err)
	}
	return &Store{client: client}, nil
}

// EnsureBucket creates the bucket if it does not already exist.
func (s *Store) EnsureBucket(ctx context.Context, bucket string) error {
	exists, err := s.client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("s3: bucket exists %s: %w", bucket, err)
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("s3: make bucket %s: %w", bucket, err)
	}
	return nil
}

// Put stores data under bucket/key with the given content type.
func (s *Store) Put(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, bucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("s3: put %s/%s: %w", bucket, key, err)
	}
	return nil
}

// PresignedGetURL returns a time-limited URL to download the object.
func (s *Store) PresignedGetURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("s3: presign %s/%s: %w", bucket, key, err)
	}
	return u.String(), nil
}
