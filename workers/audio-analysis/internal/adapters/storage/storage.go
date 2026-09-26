// Package storage gives ffmpeg read access to masters in object storage.
package storage

import (
	"context"
	"errors"
	"time"

	"github.com/tehrelt/icyre/libs/platform/objectstore"
	"github.com/tehrelt/icyre/workers/audio-analysis/internal/application"
)

// Source implements application.Source with presigned GET URLs: ffmpeg
// streams the master over HTTP, nothing is written to local disk.
type Source struct {
	store *objectstore.Store
	ttl   time.Duration
}

// New returns a Source whose URLs live for ttl (at least the analysis timeout).
func New(store *objectstore.Store, ttl time.Duration) *Source { return &Source{store: store, ttl: ttl} }

// URL implements application.Source.
func (s *Source) URL(ctx context.Context, bucket, key string) (string, error) {
	if _, err := s.store.Stat(ctx, bucket, key); errors.Is(err, objectstore.ErrNotFound) {
		return "", application.ErrSourceMissing
	} else if err != nil {
		return "", err
	}
	u, err := s.store.PresignDownload(ctx, bucket, key, s.ttl, objectstore.DownloadOptions{})
	if err != nil {
		return "", err
	}
	return u.URL, nil
}
