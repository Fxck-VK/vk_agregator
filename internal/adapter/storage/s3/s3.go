// Package s3 provides an S3/MinIO-backed object store used to persist artifact
// bytes before delivery. It wraps the MinIO Go client, which speaks the S3 API
// against both AWS S3 and self-hosted MinIO.
package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
)

// Config holds the connection settings for the object store.
type Config struct {
	// Endpoint is the host:port or URL of the S3-compatible server.
	Endpoint string
	// AccessKey and SecretKey authenticate requests.
	AccessKey string
	SecretKey string
	// UseSSL enables HTTPS to the endpoint.
	UseSSL bool
	// Region is the optional bucket region.
	Region string
	// AddressingStyle controls bucket addressing: path, virtual-hosted or auto.
	AddressingStyle string
}

// Store is an S3/MinIO object store.
type Store struct {
	client *minio.Client
	region string
}

// New creates an object-store client.
//
// Bucket reachability is checked by EnsureBucket/BucketReady instead of a
// ListBuckets startup probe. Managed S3 credentials are often scoped to a
// single private bucket and may not have ListAllMyBuckets permissions.
func New(ctx context.Context, cfg Config) (*Store, error) {
	_ = ctx
	endpoint, secure, err := normalizeEndpoint(cfg.Endpoint, cfg.UseSSL)
	if err != nil {
		return nil, err
	}
	bucketLookup, err := bucketLookupForAddressingStyle(cfg.AddressingStyle)
	if err != nil {
		return nil, err
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       secure,
		Region:       strings.TrimSpace(cfg.Region),
		BucketLookup: bucketLookup,
	})
	if err != nil {
		return nil, fmt.Errorf("s3: new client: %w", err)
	}
	return &Store{client: client, region: strings.TrimSpace(cfg.Region)}, nil
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
	if err := s.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: s.region}); err != nil {
		return fmt.Errorf("s3: make bucket %s: %w", bucket, err)
	}
	return nil
}

// BucketReady verifies that the configured artifact bucket is reachable.
// Error messages intentionally avoid bucket names because readiness failures may
// be surfaced in deployment logs.
func (s *Store) BucketReady(ctx context.Context, bucket string) error {
	exists, err := s.client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("s3: bucket readiness check: %w", err)
	}
	if !exists {
		return errors.New("s3: bucket is missing")
	}
	return nil
}

// SetRetention configures (or clears) an object-expiry lifecycle rule on the
// bucket so stored artifacts are purged after the given number of days. A
// non-positive value removes the rule. This bounds storage growth and limits
// how long generated media is retained (audit ST1).
func (s *Store) SetRetention(ctx context.Context, bucket string, days int) error {
	cfg := lifecycle.NewConfiguration()
	if days > 0 {
		cfg.Rules = []lifecycle.Rule{{
			ID:         "artifact-expiry",
			Status:     "Enabled",
			Expiration: lifecycle.Expiration{Days: lifecycle.ExpirationDays(days)},
		}}
	}
	if err := s.client.SetBucketLifecycle(ctx, bucket, cfg); err != nil {
		return fmt.Errorf("s3: set lifecycle %s: %w", bucket, err)
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

// GetObject downloads an object's bytes from bucket/key.
func (s *Store) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("s3: get %s/%s: %w", bucket, key, err)
	}
	defer func() {
		_ = obj.Close()
	}()
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("s3: read %s/%s: %w", bucket, key, err)
	}
	return data, nil
}

// DeleteObject removes an object from storage. Error messages intentionally do
// not include bucket/key because maintenance errors may be logged.
func (s *Store) DeleteObject(ctx context.Context, bucket, key string) error {
	if err := s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3: delete object: %w", err)
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

func normalizeEndpoint(endpoint string, useSSL bool) (string, bool, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", false, errors.New("s3: endpoint is required")
	}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return "", false, fmt.Errorf("s3: parse endpoint: %w", err)
		}
		if parsed.Host == "" {
			return "", false, fmt.Errorf("s3: endpoint URL must include host")
		}
		if parsed.Path != "" && parsed.Path != "/" {
			return "", false, fmt.Errorf("s3: endpoint URL must not include path")
		}
		return parsed.Host, parsed.Scheme == "https", nil
	}
	endpoint = strings.TrimRight(endpoint, "/")
	if strings.Contains(endpoint, "/") {
		return "", false, fmt.Errorf("s3: endpoint must not include path")
	}
	return endpoint, useSSL, nil
}

func bucketLookupForAddressingStyle(style string) (minio.BucketLookupType, error) {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "", "path":
		return minio.BucketLookupPath, nil
	case "auto":
		return minio.BucketLookupAuto, nil
	case "virtual-hosted", "virtual", "dns":
		return minio.BucketLookupDNS, nil
	default:
		return minio.BucketLookupAuto, fmt.Errorf("s3: invalid addressing style %q", style)
	}
}
