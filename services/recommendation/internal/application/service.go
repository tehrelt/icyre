// Package application serves ready recommendation sets: the user's personal
// set, or the popular fallback when there is none.
package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/recommendation"
)

// Store reads sets by key; ok is false when the key is absent.
type Store interface {
	Get(ctx context.Context, key string) (set recommendation.Set, ok bool, err error)
}

// Sources of a served set.
const (
	SourcePersonal = "personal" // built for this user
	SourcePopular  = "popular"  // fallback: no personal set (new user, guest, expired)
	SourceNone     = "none"     // nothing built yet
)

// Result is a set with where it came from.
type Result struct {
	Source      string
	Algorithm   string
	GeneratedAt time.Time
	Tracks      []recommendation.Item
	Artists     []recommendation.Item
}

// Service reads recommendations.
type Service struct{ store Store }

// New returns a Service.
func New(s Store) *Service { return &Service{store: s} }

// For returns the set of user (uuid.Nil for a guest), falling back to the
// popular set, then to an empty one.
func (s *Service) For(ctx context.Context, user uuid.UUID) (Result, error) {
	if user != uuid.Nil {
		set, ok, err := s.get(ctx, recommendation.UserKey(user.String()))
		if err != nil {
			return Result{}, err
		}
		if ok {
			return result(SourcePersonal, set), nil
		}
	}
	set, ok, err := s.get(ctx, recommendation.PopularKey)
	if err != nil {
		return Result{}, err
	}
	if !ok {
		return Result{Source: SourceNone, Tracks: []recommendation.Item{}, Artists: []recommendation.Item{}}, nil
	}
	return result(SourcePopular, set), nil
}

// get treats a set of another schema version as absent.
func (s *Service) get(ctx context.Context, key string) (recommendation.Set, bool, error) {
	set, ok, err := s.store.Get(ctx, key)
	if err != nil || !ok || set.Version != recommendation.Version {
		return recommendation.Set{}, false, err
	}
	return set, true, nil
}

func result(source string, set recommendation.Set) Result {
	r := Result{Source: source, Algorithm: set.Algorithm, GeneratedAt: set.GeneratedAt, Tracks: set.Tracks, Artists: set.Artists}
	if r.Tracks == nil {
		r.Tracks = []recommendation.Item{}
	}
	if r.Artists == nil {
		r.Artists = []recommendation.Item{}
	}
	return r
}
