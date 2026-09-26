// Package storage binds the object store to the media bucket.
package storage

import (
	"context"
	"time"

	"github.com/tehrelt/icyre/libs/platform/objectstore"
)

// Bucket implements application.Storage on one bucket.
type Bucket struct {
	store *objectstore.Store
	name  string
}

// New returns a Bucket.
func New(store *objectstore.Store, name string) *Bucket { return &Bucket{store: store, name: name} }

// PresignUpload signs a PUT of key.
func (b *Bucket) PresignUpload(ctx context.Context, key string, ttl time.Duration, o objectstore.UploadOptions) (objectstore.SignedURL, error) {
	return b.store.PresignUpload(ctx, b.name, key, ttl, o)
}

// Stat returns object metadata.
func (b *Bucket) Stat(ctx context.Context, key string) (objectstore.ObjectInfo, error) {
	return b.store.Stat(ctx, b.name, key)
}

// ReadHead returns the first n bytes.
func (b *Bucket) ReadHead(ctx context.Context, key string, n int64) ([]byte, error) {
	return b.store.ReadHead(ctx, b.name, key, n)
}

// Remove deletes the object.
func (b *Bucket) Remove(ctx context.Context, key string) error {
	return b.store.Remove(ctx, b.name, key)
}
