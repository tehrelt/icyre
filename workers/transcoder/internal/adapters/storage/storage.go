// Package storage reads masters from and writes variants to object storage.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/libs/platform/objectstore"
	"github.com/tehrelt/icyre/workers/transcoder/internal/application"
)

// User metadata keys stored with every variant (canonical header form).
const (
	metaUploadID   = "Upload-Id"
	metaSourceSHA  = "Source-Sha256"
	metaUploadedAt = "Uploaded-At"
	metaDurationMs = "Duration-Ms"
)

// Store is the part of the platform object store used here.
type Store interface {
	Stat(ctx context.Context, bucket, key string) (objectstore.ObjectInfo, error)
	Download(ctx context.Context, bucket, key string, w io.Writer) (int64, error)
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, o objectstore.PutOptions) (objectstore.ObjectInfo, error)
}

// Media implements application.Storage.
type Media struct {
	store  Store
	bucket string
}

// New returns Media writing variants to bucket.
func New(store Store, bucket string) *Media { return &Media{store: store, bucket: bucket} }

// Bucket implements application.Storage.
func (m *Media) Bucket() string { return m.bucket }

// Download implements application.Storage: the file is hashed while written.
func (m *Media) Download(ctx context.Context, bucket, key, path string) (string, error) {
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = m.store.Download(ctx, bucket, key, io.MultiWriter(f, h))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if errors.Is(err, objectstore.ErrNotFound) {
		return "", fmt.Errorf("%s: %w", key, application.ErrSourceMissing)
	}
	if err != nil {
		return "", fmt.Errorf("download %s: %w", key, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Variant implements application.Storage. A variant without provenance
// (e.g. written by scripts/seed-media.ts) counts as foreign: its zero
// UploadedAt never wins over a real upload.
func (m *Media) Variant(ctx context.Context, trackID string, q media.Quality) (application.Stored, bool, error) {
	key := media.TrackAudioKey(trackID, q)
	info, err := m.store.Stat(ctx, m.bucket, key)
	if errors.Is(err, objectstore.ErrNotFound) {
		return application.Stored{}, false, nil
	}
	if err != nil {
		return application.Stored{}, false, err
	}
	md := info.Metadata
	at, _ := time.Parse(time.RFC3339Nano, md[metaUploadedAt])
	dur, _ := strconv.ParseInt(md[metaDurationMs], 10, 64)
	return application.Stored{
		Provenance: application.Provenance{UploadID: md[metaUploadID], SourceSHA256: md[metaSourceSHA], UploadedAt: at, DurationMs: dur},
		Quality:    q, Key: key, SizeBytes: info.Size,
	}, true, nil
}

// PutVariant implements application.Storage. Variants are overwritten in
// place: the key is stable per track, the metadata names the master.
func (m *Media) PutVariant(ctx context.Context, trackID string, q media.Quality, path string, p application.Provenance) (application.Stored, error) {
	f, err := os.Open(path)
	if err != nil {
		return application.Stored{}, err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return application.Stored{}, err
	}
	key := media.TrackAudioKey(trackID, q)
	_, err = m.store.Put(ctx, m.bucket, key, f, st.Size(), objectstore.PutOptions{
		ContentType: media.AudioContentType,
		Metadata: map[string]string{
			metaUploadID:   p.UploadID,
			metaSourceSHA:  p.SourceSHA256,
			metaUploadedAt: p.UploadedAt.UTC().Format(time.RFC3339Nano),
			metaDurationMs: strconv.FormatInt(p.DurationMs, 10),
		},
	})
	if err != nil {
		return application.Stored{}, err
	}
	return application.Stored{Provenance: p, Quality: q, Key: key, SizeBytes: st.Size()}, nil
}
