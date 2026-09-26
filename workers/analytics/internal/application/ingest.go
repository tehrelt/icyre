// Package application holds the Analytics Worker's use cases: ingesting
// playback events into ClickHouse and recomputing the daily aggregates.
package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/workers/analytics/internal/domain"
)

// Catalog resolves tracks; unknown (deleted) tracks are absent from the map.
type Catalog interface {
	Tracks(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.TrackInfo, error)
}

// EventStore persists raw events. A repeated dedupToken makes the insert a
// no-op; redelivered events collapse by event ID anyway.
type EventStore interface {
	InsertEvents(ctx context.Context, evs []domain.Event, dedupToken string) error
}

// Ingester writes batches of playback events.
type Ingester struct {
	catalog Catalog
	store   EventStore
	log     *slog.Logger
}

// NewIngester returns an Ingester.
func NewIngester(c Catalog, s EventStore, log *slog.Logger) *Ingester {
	return &Ingester{catalog: c, store: s, log: log}
}

// Ingest enriches evs with album and artists and inserts them as one block.
// A Catalog failure fails the batch (it is retried): events stored without
// artists would be missing from artist charts for good.
func (i *Ingester) Ingest(ctx context.Context, evs []domain.Event) error {
	if len(evs) == 0 {
		return nil
	}
	seen := map[uuid.UUID]bool{}
	var ids []uuid.UUID
	for _, e := range evs {
		if !seen[e.TrackID] {
			seen[e.TrackID] = true
			ids = append(ids, e.TrackID)
		}
	}
	tracks, err := i.catalog.Tracks(ctx, ids)
	if err != nil {
		return fmt.Errorf("resolve tracks: %w", err)
	}
	out := make([]domain.Event, len(evs))
	unknown := 0
	for n, e := range evs {
		t, ok := tracks[e.TrackID]
		if !ok {
			unknown++
		}
		out[n] = e.Enrich(t)
	}
	if unknown > 0 {
		i.log.WarnContext(ctx, "events of tracks unknown to catalog", "events", unknown)
	}
	return i.store.InsertEvents(ctx, out, dedupToken(evs))
}

// dedupToken identifies a batch by its events: a retry of the same batch
// carries the same token.
func dedupToken(evs []domain.Event) string {
	h := sha256.New()
	for _, e := range evs {
		h.Write(e.EventID[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TrackCache is a Catalog that remembers answers for ttl. Albums and artists
// of a track practically never change; unknown tracks are cached too.
type TrackCache struct {
	next Catalog
	ttl  time.Duration
	max  int
	now  func() time.Time

	mu      sync.Mutex
	entries map[uuid.UUID]cached
}

type cached struct {
	info    domain.TrackInfo
	known   bool
	expires time.Time
}

// NewTrackCache wraps next; max bounds the entries (the cache is emptied
// when full — cheap, and a refill is one Catalog request per 100 tracks).
func NewTrackCache(next Catalog, ttl time.Duration, max int) *TrackCache {
	return &TrackCache{next: next, ttl: ttl, max: max, now: time.Now, entries: map[uuid.UUID]cached{}}
}

// Tracks implements Catalog.
func (c *TrackCache) Tracks(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.TrackInfo, error) {
	now := c.now()
	out := map[uuid.UUID]domain.TrackInfo{}
	var missing []uuid.UUID
	c.mu.Lock()
	for _, id := range ids {
		e, ok := c.entries[id]
		switch {
		case !ok || now.After(e.expires):
			missing = append(missing, id)
		case e.known:
			out[id] = e.info
		}
	}
	c.mu.Unlock()
	if len(missing) == 0 {
		return out, nil
	}

	found, err := c.next.Tracks(ctx, missing)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries)+len(missing) > c.max {
		c.entries = map[uuid.UUID]cached{}
	}
	for _, id := range missing {
		info, known := found[id]
		c.entries[id] = cached{info: info, known: known, expires: now.Add(c.ttl)}
		if known {
			out[id] = info
		}
	}
	return out, nil
}
