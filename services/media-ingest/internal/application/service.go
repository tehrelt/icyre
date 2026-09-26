// Package application implements the upload use cases.
package application

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/libs/platform/objectstore"
	"github.com/tehrelt/icyre/services/media-ingest/internal/domain"
)

// Repository stores upload sessions.
type Repository interface {
	Create(ctx context.Context, u domain.Upload) error
	// Get returns domain.ErrNotFound for an unknown ID.
	Get(ctx context.Context, id uuid.UUID) (domain.Upload, error)
	// Settle stores a final status if the upload is still pending;
	// false means another request settled it first.
	Settle(ctx context.Context, u domain.Upload) (bool, error)
}

// Catalog confirms that a track exists (domain.ErrTrackNotFound otherwise).
type Catalog interface {
	TrackExists(ctx context.Context, id uuid.UUID) error
}

// Storage is the media bucket.
type Storage interface {
	PresignUpload(ctx context.Context, key string, ttl time.Duration, o objectstore.UploadOptions) (objectstore.SignedURL, error)
	// Stat and ReadHead return objectstore.ErrNotFound for a missing object.
	Stat(ctx context.Context, key string) (objectstore.ObjectInfo, error)
	ReadHead(ctx context.Context, key string, n int64) ([]byte, error)
	Remove(ctx context.Context, key string) error
}

// Publisher announces settled uploads.
type Publisher interface {
	Uploaded(ctx context.Context, u domain.Upload) error
	Failed(ctx context.Context, u domain.Upload) error
}

// Transactor runs fn as one unit of work: the status change and its event
// (transactional outbox) commit or roll back together.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// Actor is the authenticated caller.
type Actor struct {
	ID     uuid.UUID
	Artist bool
	Admin  bool
}

// Limits bound uploads.
type Limits struct {
	MaxSize int64
	// URLTTL is the lifetime of the upload URL; an upload whose object has
	// not arrived by then fails as EXPIRED on complete.
	URLTTL time.Duration
}

// Service implements the use cases.
type Service struct {
	repo    Repository
	catalog Catalog
	store   Storage
	pub     Publisher
	tx      Transactor
	limits  Limits
	log     *slog.Logger
	now     func() time.Time
}

// New returns a Service.
func New(repo Repository, catalog Catalog, store Storage, pub Publisher, tx Transactor, limits Limits, log *slog.Logger) *Service {
	if tx == nil {
		tx = noTx{}
	}
	return &Service{repo: repo, catalog: catalog, store: store, pub: pub, tx: tx, limits: limits, log: log, now: time.Now}
}

// Created is a new session with the URL to upload to.
type Created struct {
	Upload domain.Upload
	URL    objectstore.SignedURL
}

// Create opens an upload session for an existing track. The client PUTs the
// file straight to object storage with the returned URL and headers; the
// store rejects a body whose SHA-256 differs from the declared one.
func (s *Service) Create(ctx context.Context, actor Actor, req domain.Request) (Created, error) {
	if !actor.Artist && !actor.Admin {
		return Created{}, domain.ErrForbidden
	}
	format, sum, err := req.Validate(s.limits.MaxSize)
	if err != nil {
		return Created{}, err
	}
	if err := s.catalog.TrackExists(ctx, req.TrackID); err != nil {
		return Created{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Created{}, err
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	u := domain.Upload{
		ID: id, TrackID: req.TrackID, UploaderID: actor.ID,
		ContentType: format.ContentType, SizeBytes: req.SizeBytes, SHA256: sum,
		Key:    media.TrackOriginalKey(req.TrackID.String(), id.String(), format.Ext),
		Status: domain.StatusPending, CreatedAt: now, ExpiresAt: now.Add(s.limits.URLTTL),
	}
	url, err := s.store.PresignUpload(ctx, u.Key, s.limits.URLTTL, objectstore.UploadOptions{ContentType: u.ContentType, SHA256: sum})
	if err != nil {
		return Created{}, err
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return Created{}, err
	}
	return Created{Upload: u, URL: url}, nil
}

// Get returns a session visible to the actor: its uploader or an admin.
// Others get domain.ErrNotFound, so IDs do not leak.
func (s *Service) Get(ctx context.Context, actor Actor, id uuid.UUID) (domain.Upload, error) {
	u, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Upload{}, err
	}
	if u.UploaderID != actor.ID && !actor.Admin {
		return domain.Upload{}, domain.ErrNotFound
	}
	return u, nil
}

// Complete verifies the stored object (size, content type, checksum, magic
// bytes) and settles the session: COMPLETED publishes track.uploaded,
// anything else FAILED with media.ingest.failed and a *domain.RejectedError.
// Completing a settled session repeats its outcome without new events.
func (s *Service) Complete(ctx context.Context, actor Actor, id uuid.UUID) (domain.Upload, error) {
	u, err := s.Get(ctx, actor, id)
	if err != nil {
		return domain.Upload{}, err
	}
	if u.Status != domain.StatusPending {
		return outcome(u)
	}
	reason, err := s.verify(ctx, u)
	if err != nil {
		return domain.Upload{}, err
	}
	settled := u.Settle(reason, s.now().UTC().Truncate(time.Microsecond))
	var changed bool
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if changed, err = s.repo.Settle(ctx, settled); err != nil || !changed {
			return err
		}
		if reason == "" {
			return s.pub.Uploaded(ctx, settled)
		}
		return s.pub.Failed(ctx, settled)
	})
	if err != nil {
		return domain.Upload{}, err
	}
	if !changed { // a concurrent complete won
		if settled, err = s.repo.Get(ctx, id); err != nil {
			return domain.Upload{}, err
		}
		return outcome(settled)
	}
	if reason != "" && reason != mediav1.ReasonExpired {
		// Rejected bytes are never processed; drop them.
		if err := s.store.Remove(ctx, u.Key); err != nil {
			s.log.WarnContext(ctx, "rejected upload not removed", "upload_id", u.ID, "key", u.Key, "error", err)
		}
	}
	return outcome(settled)
}

func outcome(u domain.Upload) (domain.Upload, error) {
	if u.Status == domain.StatusFailed {
		return u, &domain.RejectedError{Reason: u.FailureReason}
	}
	return u, nil
}

// verify returns a failure reason, or "" when the object is acceptable.
func (s *Service) verify(ctx context.Context, u domain.Upload) (string, error) {
	info, err := s.store.Stat(ctx, u.Key)
	if errors.Is(err, objectstore.ErrNotFound) {
		if s.now().After(u.ExpiresAt) {
			return mediav1.ReasonExpired, nil
		}
		return "", domain.ErrNotUploaded
	}
	if err != nil {
		return "", err
	}
	switch {
	case info.Size != u.SizeBytes || info.Size > s.limits.MaxSize:
		return mediav1.ReasonSizeMismatch, nil
	case info.ContentType != u.ContentType:
		return mediav1.ReasonContentTypeMismatch, nil
	case info.SHA256 != base64.StdEncoding.EncodeToString(u.SHA256):
		return mediav1.ReasonChecksumMismatch, nil
	}
	head, err := s.store.ReadHead(ctx, u.Key, domain.SniffLen)
	if err != nil {
		return "", fmt.Errorf("sniff: %w", err)
	}
	if f, _ := domain.FormatOf(u.ContentType); !f.Matches(head) {
		return mediav1.ReasonUnrecognizedFormat, nil
	}
	return "", nil
}
