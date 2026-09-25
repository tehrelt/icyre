// Package storage finds audio variants in object storage and signs URLs.
package storage

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/libs/platform/objectstore"
)

// Store is the part of objectstore.Store used here.
type Store interface {
	Stat(ctx context.Context, bucket, key string) (objectstore.ObjectInfo, error)
	PresignDownload(ctx context.Context, bucket, key string, ttl time.Duration, o objectstore.DownloadOptions) (objectstore.SignedURL, error)
}

// Media implements application.Variants and application.Signer.
type Media struct {
	store  Store
	bucket string
}

// New returns Media over bucket.
func New(store Store, bucket string) *Media { return &Media{store: store, bucket: bucket} }

// Available checks each variant key (HEAD requests; variants are immutable,
// so callers cache the result).
func (m *Media) Available(ctx context.Context, trackID string) ([]media.Quality, error) {
	var out []media.Quality
	for _, q := range media.Qualities {
		_, err := m.store.Stat(ctx, m.bucket, media.TrackAudioKey(trackID, q))
		switch {
		case err == nil:
			out = append(out, q)
		case errors.Is(err, objectstore.ErrNotFound):
		default:
			return nil, fmt.Errorf("variants of %s: %w", trackID, err)
		}
	}
	return out, nil
}

// Sign issues a presigned GET. Cache-Control lets the browser (and a CDN in
// front of the origin) keep the bytes no longer than the URL lives.
func (m *Media) Sign(ctx context.Context, trackID string, q media.Quality, ttl time.Duration) (string, time.Time, error) {
	u, err := m.store.PresignDownload(ctx, m.bucket, media.TrackAudioKey(trackID, q), ttl, objectstore.DownloadOptions{
		CacheControl: "private, max-age=" + strconv.Itoa(int(ttl.Seconds())),
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return u.URL, u.ExpiresAt, nil
}
