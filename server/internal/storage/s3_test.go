package storage

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type fakePresignClient struct {
	lastKey string
	url     string
	err     error
}

func (f *fakePresignClient) PresignGetObject(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	if params != nil && params.Key != nil {
		f.lastKey = aws.ToString(params.Key)
	}
	if f.err != nil {
		return nil, f.err
	}
	return &v4.PresignedHTTPRequest{URL: f.url}, nil
}

func TestS3StorageKeyFromURL_CustomEndpointPreservesNestedKey(t *testing.T) {
	s := &S3Storage{
		bucket:      "test-bucket",
		endpointURL: "http://localhost:9000",
	}

	rawURL := "http://localhost:9000/test-bucket/uploads/abc/file.png"

	if got := s.KeyFromURL(rawURL); got != "uploads/abc/file.png" {
		t.Fatalf("KeyFromURL(%q) = %q, want %q", rawURL, got, "uploads/abc/file.png")
	}
}

func TestS3StorageKeyFromURL_CustomEndpointWithTrailingSlash(t *testing.T) {
	s := &S3Storage{
		bucket:      "test-bucket",
		endpointURL: "http://localhost:9000/",
	}

	rawURL := "http://localhost:9000/test-bucket/uploads/abc/file.png"

	if got := s.KeyFromURL(rawURL); got != "uploads/abc/file.png" {
		t.Fatalf("KeyFromURL(%q) = %q, want %q", rawURL, got, "uploads/abc/file.png")
	}
}

func TestS3StorageSignedDownloadURL_CustomEndpoint(t *testing.T) {
	presign := &fakePresignClient{url: "http://localhost:9000/presigned"}
	s := &S3Storage{
		bucket:        "test-bucket",
		endpointURL:   "http://localhost:9000",
		presignClient: presign,
	}

	rawURL := "http://localhost:9000/test-bucket/uploads/abc/file.png"
	got, err := s.SignedDownloadURL(context.Background(), rawURL, 5*time.Minute)
	if err != nil {
		t.Fatalf("SignedDownloadURL returned error: %v", err)
	}
	if got != "http://localhost:9000/presigned" {
		t.Fatalf("SignedDownloadURL() = %q, want %q", got, "http://localhost:9000/presigned")
	}
	if presign.lastKey != "uploads/abc/file.png" {
		t.Fatalf("presign key = %q, want %q", presign.lastKey, "uploads/abc/file.png")
	}
}

func TestS3StorageSignedDownloadURL_NoPresignClient(t *testing.T) {
	s := &S3Storage{bucket: "test-bucket"}
	if _, err := s.SignedDownloadURL(context.Background(), "https://example.com/file.png", 5*time.Minute); err == nil {
		t.Fatal("SignedDownloadURL expected error when presign client is not configured")
	}
}
