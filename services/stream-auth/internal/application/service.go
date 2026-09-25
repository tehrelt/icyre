// Package application implements stream authorization.
package application

import (
	"context"
	"errors"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/services/stream-auth/internal/domain"
)

// Tracks reads track status from Catalog (domain.ErrTrackNotFound when missing).
type Tracks interface {
	Status(ctx context.Context, trackID string) (string, error)
}

// Variants lists the audio variants present in object storage.
type Variants interface {
	Available(ctx context.Context, trackID string) ([]media.Quality, error)
}

// Signer issues a signed URL for one variant.
type Signer interface {
	Sign(ctx context.Context, trackID string, q media.Quality, ttl time.Duration) (url string, expiresAt time.Time, err error)
}

// Decision is one authorization outcome, for the audit trail.
type Decision struct {
	UserID    string
	SessionID string
	TrackID   string
	Requested media.Quality
	Granted   media.Quality // 0 when denied
	Err       error         // nil when granted
}

// Auditor records every decision (security log + metrics).
type Auditor interface {
	Record(ctx context.Context, d Decision)
}

// Request asks to play a track.
type Request struct {
	UserID    string
	SessionID string
	TrackID   string
	Quality   media.Quality // 0 = default
}

// Service authorizes playback.
type Service struct {
	tracks   Tracks
	variants Variants
	signer   Signer
	audit    Auditor
	ttl      time.Duration
}

// New returns a Service issuing URLs valid for ttl.
func New(tracks Tracks, variants Variants, signer Signer, audit Auditor, ttl time.Duration) *Service {
	return &Service{tracks: tracks, variants: variants, signer: signer, audit: audit, ttl: ttl}
}

// Authorize checks the track and returns a short-lived URL for the best
// available variant. The audio itself never passes through this service.
func (s *Service) Authorize(ctx context.Context, r Request) (domain.Grant, error) {
	requested := r.Quality
	if requested == 0 {
		requested = domain.DefaultQuality
	}
	grant, err := s.authorize(ctx, r.TrackID, requested)
	s.audit.Record(ctx, Decision{UserID: r.UserID, SessionID: r.SessionID, TrackID: r.TrackID, Requested: requested, Granted: grant.Quality, Err: err})
	return grant, err
}

func (s *Service) authorize(ctx context.Context, trackID string, requested media.Quality) (domain.Grant, error) {
	status, err := s.tracks.Status(ctx, trackID)
	if err != nil {
		return domain.Grant{}, err
	}
	if err := domain.CheckPlayable(status); err != nil {
		return domain.Grant{}, err
	}
	available, err := s.variants.Available(ctx, trackID)
	if err != nil {
		return domain.Grant{}, err
	}
	q, err := domain.ChooseVariant(requested, available)
	if err != nil {
		return domain.Grant{}, err
	}
	url, expiresAt, err := s.signer.Sign(ctx, trackID, q, s.ttl)
	if err != nil {
		return domain.Grant{}, err
	}
	return domain.Grant{TrackID: trackID, Quality: q, URL: url, ExpiresAt: expiresAt}, nil
}

// Denied reports whether err is a business denial (as opposed to a failure
// of a dependency).
func Denied(err error) bool {
	for _, e := range []error{domain.ErrTrackNotFound, domain.ErrTrackNotReady, domain.ErrTrackBlocked, domain.ErrNoVariant, domain.ErrInvalidRequest} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
