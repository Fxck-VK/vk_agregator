package s3

import (
	"context"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

func TestNormalizeEndpointAcceptsHostPort(t *testing.T) {
	endpoint, secure, err := normalizeEndpoint("minio:9000", false)
	if err != nil {
		t.Fatalf("normalizeEndpoint returned error: %v", err)
	}
	if endpoint != "minio:9000" || secure {
		t.Fatalf("endpoint=%q secure=%v, want minio:9000 false", endpoint, secure)
	}
}

func TestNormalizeEndpointAcceptsURL(t *testing.T) {
	endpoint, secure, err := normalizeEndpoint("https://s3.example.test", false)
	if err != nil {
		t.Fatalf("normalizeEndpoint returned error: %v", err)
	}
	if endpoint != "s3.example.test" || !secure {
		t.Fatalf("endpoint=%q secure=%v, want s3.example.test true", endpoint, secure)
	}
}

func TestNormalizeEndpointRejectsPath(t *testing.T) {
	if _, _, err := normalizeEndpoint("https://s3.example.test/bucket", true); err == nil {
		t.Fatal("expected endpoint path validation error")
	}
	if _, _, err := normalizeEndpoint("s3.example.test/bucket", true); err == nil {
		t.Fatal("expected bare endpoint path validation error")
	}
}

func TestBucketLookupForAddressingStyle(t *testing.T) {
	tests := map[string]minio.BucketLookupType{
		"":               minio.BucketLookupPath,
		"path":           minio.BucketLookupPath,
		"auto":           minio.BucketLookupAuto,
		"virtual-hosted": minio.BucketLookupDNS,
		"dns":            minio.BucketLookupDNS,
	}
	for input, want := range tests {
		got, err := bucketLookupForAddressingStyle(input)
		if err != nil {
			t.Fatalf("bucketLookupForAddressingStyle(%q) error: %v", input, err)
		}
		if got != want {
			t.Fatalf("bucketLookupForAddressingStyle(%q) = %v, want %v", input, got, want)
		}
	}
	if _, err := bucketLookupForAddressingStyle("bad"); err == nil {
		t.Fatal("expected invalid addressing style error")
	}
}

func TestPresignedGetURLUsesConfiguredEndpoint(t *testing.T) {
	store, err := New(context.Background(), Config{
		Endpoint:        "https://s3.example.test",
		AccessKey:       "access",
		SecretKey:       "secret",
		UseSSL:          false,
		Region:          "ru-1",
		AddressingStyle: "path",
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	url, err := store.PresignedGetURL(context.Background(), "artifacts", "jobs/1/output.png", time.Minute)
	if err != nil {
		t.Fatalf("PresignedGetURL returned error: %v", err)
	}
	if want := "https://s3.example.test/artifacts/jobs/1/output.png"; len(url) < len(want) || url[:len(want)] != want {
		t.Fatalf("presigned URL %q does not start with %q", url, want)
	}
}
