package storage

import (
	"context"
	"time"
)

type Storage interface {
	Upload(ctx context.Context, key string, data []byte, contentType string, filename string) (string, error)
	Delete(ctx context.Context, key string)
	DeleteKeys(ctx context.Context, keys []string)
	KeyFromURL(rawURL string) string
}

// DownloadURLSigner is optional and implemented by storages that can generate
// short-lived signed download URLs for private objects.
type DownloadURLSigner interface {
	SignedDownloadURL(ctx context.Context, rawURL string, expiry time.Duration) (string, error)
}
