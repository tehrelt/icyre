// Package application builds recommendation sets: it gathers the catalog,
// popularity, history, likes and audio features, scores the catalog for
// every user with signals and publishes the sets.
package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/recommendation"
	"github.com/tehrelt/icyre/workers/recommendation/internal/domain"
)

// CatalogSource lists the recommendable (playable) tracks.
type CatalogSource interface {
	Tracks(ctx context.Context) ([]domain.Track, error)
}

// Analytics reads popularity and listening history.
type Analytics interface {
	Plays(ctx context.Context, days int) (map[uuid.UUID]uint64, error)
	History(ctx context.Context, days int) (map[uuid.UUID]map[uuid.UUID]domain.Interaction, error)
}

// SignalStore holds likes and audio features.
type SignalStore interface {
	Likes(ctx context.Context) (tracks, albums map[uuid.UUID]map[uuid.UUID]bool, err error)
	Sounds(ctx context.Context) (map[uuid.UUID]*domain.Sound, error)
}

// Publisher stores ready sets by key.
type Publisher interface {
	Publish(ctx context.Context, sets map[string]recommendation.Set) error
}

// Options size one build.
type Options struct {
	HistoryDays    int // listening history window
	PopularityDays int // popularity window
	Tracks         int // tracks per set
	Artists        int // artists per set
}

// Stats describe one build.
type Stats struct {
	Candidates int
	Users      int
	Took       time.Duration
}

// Builder runs builds.
type Builder struct {
	catalog   CatalogSource
	analytics Analytics
	signals   SignalStore
	out       Publisher
	opts      Options
	log       *slog.Logger
	now       func() time.Time
}

// NewBuilder returns a Builder.
func NewBuilder(c CatalogSource, a Analytics, s SignalStore, out Publisher, o Options, log *slog.Logger) *Builder {
	return &Builder{catalog: c, analytics: a, signals: s, out: out, opts: o, log: log, now: time.Now}
}

// Build recomputes every set: the popular fallback and one per user with
// signals. Users are scored against the whole catalog — O(users × tracks),
// fine for the MVP; candidate generation (artist and genre neighbourhoods)
// bounds it at scale.
func (b *Builder) Build(ctx context.Context) (Stats, error) {
	start := b.now()
	tracks, err := b.catalog.Tracks(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("catalog: %w", err)
	}
	plays, err := b.analytics.Plays(ctx, b.opts.PopularityDays)
	if err != nil {
		return Stats{}, err
	}
	sounds, err := b.signals.Sounds(ctx)
	if err != nil {
		return Stats{}, err
	}
	for i := range tracks {
		tracks[i].Plays = plays[tracks[i].ID]
		tracks[i].Sound = sounds[tracks[i].ID]
	}
	cat := domain.NewCatalog(tracks, start)

	history, err := b.analytics.History(ctx, b.opts.HistoryDays)
	if err != nil {
		return Stats{}, err
	}
	likedTracks, likedAlbums, err := b.signals.Likes(ctx)
	if err != nil {
		return Stats{}, err
	}

	sets := map[string]recommendation.Set{}
	pt, pa := cat.Popular(b.opts.Tracks, b.opts.Artists)
	sets[recommendation.PopularKey] = newSet(start, pt, pa)

	users := map[uuid.UUID]bool{}
	for u := range history {
		users[u] = true
	}
	for u := range likedTracks {
		users[u] = true
	}
	for u := range likedAlbums {
		users[u] = true
	}
	personal := 0
	for u := range users {
		s := domain.Signals{UserID: u, History: history[u], LikedTracks: likedTracks[u], LikedAlbums: likedAlbums[u]}
		if s.Empty() {
			continue
		}
		t, a := cat.Recommend(s, b.opts.Tracks, b.opts.Artists)
		if len(t) == 0 && len(a) == 0 {
			continue // the whole catalog is excluded: the popular set serves better
		}
		sets[recommendation.UserKey(u.String())] = newSet(start, t, a)
		personal++
	}
	if err := b.out.Publish(ctx, sets); err != nil {
		return Stats{}, err
	}
	return Stats{Candidates: cat.Len(), Users: personal, Took: b.now().Sub(start)}, nil
}

func newSet(at time.Time, tracks, artists []domain.Scored) recommendation.Set {
	return recommendation.Set{
		Version: recommendation.Version, Algorithm: domain.Algorithm, GeneratedAt: at.UTC(),
		Tracks: items(tracks), Artists: items(artists),
	}
}

func items(s []domain.Scored) []recommendation.Item {
	out := make([]recommendation.Item, len(s))
	for i, x := range s {
		out[i] = recommendation.Item{ID: x.ID.String(), Score: x.Score, Reasons: x.Reasons}
	}
	return out
}

// Run builds every period until ctx is cancelled; a failed build is logged
// and retried on the next tick. observe (optional) receives each outcome.
func (b *Builder) Run(ctx context.Context, every time.Duration, observe func(Stats, error)) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		st, err := b.Build(ctx)
		if ctx.Err() != nil {
			return
		}
		if observe != nil {
			observe(st, err)
		}
		if err != nil {
			b.log.ErrorContext(ctx, "recommendation build failed", "error", err)
		} else {
			b.log.InfoContext(ctx, "recommendations built", "users", st.Users, "candidates", st.Candidates, "took", st.Took.Round(time.Millisecond).String())
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
