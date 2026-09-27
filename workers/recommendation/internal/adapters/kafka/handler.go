// Package kafka turns library.events, social.events and media.events into
// taste signals.
package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/libraryv1"
	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	"github.com/tehrelt/icyre/libs/contracts/events/socialv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/workers/recommendation/internal/adapters/postgres"
)

// Store is where signals land.
type Store interface {
	SaveLike(ctx context.Context, kind string, user, id uuid.UUID, at time.Time) error
	RemoveLike(ctx context.Context, kind string, user, id uuid.UUID, at time.Time) error
	SaveFollow(ctx context.Context, user, artist uuid.UUID, at time.Time) error
	RemoveFollow(ctx context.Context, user, artist uuid.UUID, at time.Time) error
	SaveFeatures(ctx context.Context, f postgres.Features) error
}

// Handler consumes likes (library.events), artist follows (social.events)
// and audio features (media.events); other events are ignored. Idempotent:
// likes and follows compare timestamps, features keep the latest master.
func Handler(s Store) platformkafka.Handler {
	return func(ctx context.Context, rec platformkafka.Record) error {
		env, err := events.Decode(rec.Value)
		if err != nil {
			return fmt.Errorf("%w: %v", platformkafka.ErrPermanent, err)
		}
		switch env.EventType {
		case libraryv1.TypeTrackSaved, libraryv1.TypeTrackRemoved, libraryv1.TypeAlbumSaved, libraryv1.TypeAlbumRemoved:
			if env.EventVersion != libraryv1.Version {
				return unsupported(env)
			}
			return like(ctx, s, env)
		case socialv1.TypeFollowed, socialv1.TypeUnfollowed:
			if env.EventVersion != socialv1.Version {
				return unsupported(env)
			}
			return follow(ctx, s, env)
		case mediav1.TypeAudioFeaturesExtracted:
			if env.EventVersion != mediav1.Version {
				return unsupported(env)
			}
			return features(ctx, s, env)
		}
		return nil
	}
}

func unsupported(env events.Envelope) error {
	return fmt.Errorf("%w: unsupported %s version %d", platformkafka.ErrPermanent, env.EventType, env.EventVersion)
}

func like(ctx context.Context, s Store, env events.Envelope) error {
	// All four payloads share userId and a timestamp; the item ID differs.
	var p struct {
		UserID    string    `json:"userId"`
		TrackID   string    `json:"trackId"`
		AlbumID   string    `json:"albumId"`
		SavedAt   time.Time `json:"savedAt"`
		RemovedAt time.Time `json:"removedAt"`
	}
	if err := env.DecodePayload(&p); err != nil {
		return fmt.Errorf("%w: payload: %v", platformkafka.ErrPermanent, err)
	}
	kind, rawID, at, save := postgres.KindTrack, p.TrackID, p.SavedAt, true
	switch env.EventType {
	case libraryv1.TypeTrackRemoved:
		at, save = p.RemovedAt, false
	case libraryv1.TypeAlbumSaved:
		kind, rawID = postgres.KindAlbum, p.AlbumID
	case libraryv1.TypeAlbumRemoved:
		kind, rawID, at, save = postgres.KindAlbum, p.AlbumID, p.RemovedAt, false
	}
	user, e1 := uuid.Parse(p.UserID)
	id, e2 := uuid.Parse(rawID)
	if e1 != nil || e2 != nil || at.IsZero() {
		return fmt.Errorf("%w: malformed %s", platformkafka.ErrPermanent, env.EventType)
	}
	if save {
		return s.SaveLike(ctx, kind, user, id, at)
	}
	return s.RemoveLike(ctx, kind, user, id, at)
}

// follow keeps artist follows; follows of users carry no taste signal yet.
func follow(ctx context.Context, s Store, env events.Envelope) error {
	var p struct {
		FollowerID   string    `json:"followerId"`
		TargetType   string    `json:"targetType"`
		TargetID     string    `json:"targetId"`
		FollowedAt   time.Time `json:"followedAt"`
		UnfollowedAt time.Time `json:"unfollowedAt"`
	}
	if err := env.DecodePayload(&p); err != nil {
		return fmt.Errorf("%w: payload: %v", platformkafka.ErrPermanent, err)
	}
	if p.TargetType != socialv1.TargetArtist {
		return nil
	}
	at := p.FollowedAt
	if env.EventType == socialv1.TypeUnfollowed {
		at = p.UnfollowedAt
	}
	user, e1 := uuid.Parse(p.FollowerID)
	artist, e2 := uuid.Parse(p.TargetID)
	if e1 != nil || e2 != nil || at.IsZero() {
		return fmt.Errorf("%w: malformed %s", platformkafka.ErrPermanent, env.EventType)
	}
	if env.EventType == socialv1.TypeFollowed {
		return s.SaveFollow(ctx, user, artist, at)
	}
	return s.RemoveFollow(ctx, user, artist, at)
}

func features(ctx context.Context, s Store, env events.Envelope) error {
	var p mediav1.AudioFeaturesExtracted
	if err := env.DecodePayload(&p); err != nil {
		return fmt.Errorf("%w: payload: %v", platformkafka.ErrPermanent, err)
	}
	track, err := uuid.Parse(p.TrackID)
	if err != nil || p.UploadedAt.IsZero() || p.AnalyzerVersion == "" {
		return fmt.Errorf("%w: malformed %s", platformkafka.ErrPermanent, env.EventType)
	}
	return s.SaveFeatures(ctx, postgres.Features{
		TrackID: track, BPM: p.BPM, IntegratedLUFS: p.IntegratedLUFS, LoudnessRangeLU: p.LoudnessRangeLU,
		SilenceRatio: p.SilenceRatio, AnalyzerVersion: p.AnalyzerVersion, UploadedAt: p.UploadedAt,
	})
}
