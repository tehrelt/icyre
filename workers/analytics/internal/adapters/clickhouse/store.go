// Package clickhouse stores playback events and daily aggregates in
// ClickHouse (schema: libs/contracts/analytics) and reads reports from them.
package clickhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/analytics"
	"github.com/tehrelt/icyre/libs/platform/clickhouse"
	"github.com/tehrelt/icyre/workers/analytics/internal/domain"
)

// Store implements application.EventStore and application.AggregateStore.
type Store struct {
	ch *clickhouse.Client
}

// New returns a Store.
func New(ch *clickhouse.Client) *Store {
	return &Store{ch: ch}
}

// timeLayout is how DateTime64(3) values are sent in JSONEachRow.
const timeLayout = "2006-01-02 15:04:05.000"

type eventRow struct {
	EventID    string   `json:"event_id"`
	EventType  string   `json:"event_type"`
	PlaybackID string   `json:"playback_id"`
	UserID     string   `json:"user_id"`
	TrackID    string   `json:"track_id"`
	AlbumID    string   `json:"album_id"`
	ArtistIDs  []string `json:"artist_ids"`
	Source     string   `json:"source"`
	DurationMs uint32   `json:"duration_ms"`
	ListenedMs uint32   `json:"listened_ms"`
	At         string   `json:"at"`
}

// InsertEvents writes evs as one block.
func (s *Store) InsertEvents(ctx context.Context, evs []domain.Event, dedupToken string) error {
	rows := make([]any, len(evs))
	for i, e := range evs {
		artists := make([]string, len(e.ArtistIDs))
		for j, a := range e.ArtistIDs {
			artists[j] = a.String()
		}
		rows[i] = eventRow{
			EventID: e.EventID.String(), EventType: e.Type, PlaybackID: e.PlaybackID.String(),
			UserID: e.UserID.String(), TrackID: e.TrackID.String(), AlbumID: e.AlbumID.String(),
			ArtistIDs: artists, Source: sourceKind(e.Source),
			DurationMs: clampMs(e.DurationMs), ListenedMs: clampMs(e.ListenedMs), At: e.At.UTC().Format(timeLayout),
		}
	}
	if err := s.ch.Insert(ctx, analytics.TablePlaybackEvents, rows, dedupToken); err != nil {
		return fmt.Errorf("insert playback events: %w", err)
	}
	return nil
}

// sourceKind keeps the kind of a source ("album:<id>" → "album"): the column
// is low-cardinality and per-entity sources are not analysed.
func sourceKind(s string) string {
	kind, _, _ := strings.Cut(s, ":")
	return kind
}

func clampMs(ms int64) uint32 {
	return uint32(min(max(ms, 0), math.MaxUint32))
}

// Plays, completions and skips by event type; listened time is carried by
// the events that end a play.
const statsColumns = `
    countIf(event_type = 'playback.started')        AS plays,
    countIf(event_type = 'playback.finished')       AS completions,
    countIf(event_type = 'playback.skipped')        AS skips,
    uniqExact(user_id)                              AS unique_listeners,
    sumIf(listened_ms, event_type != 'playback.started') AS listened_ms`

// recompute rebuilds each daily table for one day; FINAL collapses
// redelivered events before counting.
var recompute = []string{
	`INSERT INTO ` + analytics.TableDailyTrackStats + `
    (day, track_id, plays, completions, skips, unique_listeners, listened_ms, computed_at)
SELECT {day:Date} AS day, track_id,` + statsColumns + `, now64(3)
FROM ` + analytics.TablePlaybackEvents + ` FINAL
WHERE toDate(at) = {day:Date}
GROUP BY track_id`,

	`INSERT INTO ` + analytics.TableDailyArtistStats + `
    (day, artist_id, plays, completions, skips, unique_listeners, listened_ms, computed_at)
SELECT {day:Date} AS day, artist_id,` + statsColumns + `, now64(3)
FROM ` + analytics.TablePlaybackEvents + ` FINAL
ARRAY JOIN artist_ids AS artist_id
WHERE toDate(at) = {day:Date}
GROUP BY artist_id`,

	// HAVING drops the empty row an aggregate without GROUP BY yields for a
	// day without events.
	`INSERT INTO ` + analytics.TableDailyTotals + `
    (day, plays, completions, skips, active_listeners, listened_ms, computed_at)
SELECT {day:Date} AS day,` + statsColumns + `, now64(3)
FROM ` + analytics.TablePlaybackEvents + ` FINAL
WHERE toDate(at) = {day:Date}
HAVING count() > 0`,
}

// RecomputeDay rebuilds the daily aggregates of day (UTC).
func (s *Store) RecomputeDay(ctx context.Context, day time.Time) error {
	p := clickhouse.Params{"day": day.UTC().Format(time.DateOnly)}
	for _, q := range recompute {
		if err := s.ch.Exec(ctx, q, p); err != nil {
			return fmt.Errorf("recompute day: %w", err)
		}
	}
	return nil
}

type statsRow struct {
	ID              string  `json:"id"`
	Plays           uint64  `json:"plays"`
	Completions     uint64  `json:"completions"`
	Skips           uint64  `json:"skips"`
	UniqueListeners uint64  `json:"unique_listeners"`
	ListenedMs      uint64  `json:"listened_ms"`
	ActiveListeners float64 `json:"active_listeners"`
}

func (r statsRow) stats() domain.Stats {
	return domain.Stats{Plays: r.Plays, Completions: r.Completions, Skips: r.Skips, UniqueListeners: r.UniqueListeners, ListenedMs: r.ListenedMs}
}

// Report reads the metrics of the days [from, to] and the limit most played
// tracks and artists. Sums come from the daily tables; unique listeners over
// several days cannot be summed, so they are counted from the raw events
// (exact within raw retention).
func (s *Store) Report(ctx context.Context, from, to time.Time, limit int) (domain.Report, error) {
	p := clickhouse.Params{
		"from": from.UTC().Format(time.DateOnly), "to": to.UTC().Format(time.DateOnly), "limit": strconv.Itoa(limit),
	}
	rep := domain.Report{From: from, To: to, TopTracks: []domain.Ranked{}, TopArtists: []domain.Ranked{}}

	totals, err := s.rows(ctx, `SELECT '' AS id, sum(plays) AS plays, sum(completions) AS completions, sum(skips) AS skips,
    sum(listened_ms) AS listened_ms, avg(active_listeners) AS active_listeners
FROM `+analytics.TableDailyTotals+` FINAL
WHERE day BETWEEN {from:Date} AND {to:Date}`, p)
	if err != nil {
		return rep, err
	}
	listeners, err := s.rows(ctx, `SELECT '' AS id, uniqExact(user_id) AS unique_listeners
FROM `+analytics.TablePlaybackEvents+`
WHERE toDate(at) BETWEEN {from:Date} AND {to:Date}`, p)
	if err != nil {
		return rep, err
	}
	if len(totals) == 1 {
		rep.Totals = totals[0].stats()
		rep.DailyActiveListeners = totals[0].ActiveListeners
	}
	if len(listeners) == 1 {
		rep.Totals.UniqueListeners = listeners[0].UniqueListeners
	}
	rep.CompletionRate = rep.Totals.CompletionRate()

	if rep.TopTracks, err = s.top(ctx, p, analytics.TableDailyTrackStats, "track_id", ""); err != nil {
		return rep, err
	}
	if rep.TopArtists, err = s.top(ctx, p, analytics.TableDailyArtistStats, "artist_id", "ARRAY JOIN artist_ids AS artist_id"); err != nil {
		return rep, err
	}
	return rep, nil
}

// top ranks the entities of table by plays, then fills in their unique
// listeners over the whole period from the raw events.
func (s *Store) top(ctx context.Context, p clickhouse.Params, table, key, arrayJoin string) ([]domain.Ranked, error) {
	rows, err := s.rows(ctx, `SELECT toString(`+key+`) AS id, sum(plays) AS plays, sum(completions) AS completions,
    sum(skips) AS skips, sum(listened_ms) AS listened_ms
FROM `+table+` FINAL
WHERE day BETWEEN {from:Date} AND {to:Date}
GROUP BY `+key+`
ORDER BY plays DESC, `+key+`
LIMIT {limit:UInt32}`, p)
	if err != nil || len(rows) == 0 {
		return []domain.Ranked{}, err
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = "'" + r.ID + "'" // UUIDs produced by ClickHouse itself
	}
	lp := clickhouse.Params{"from": p["from"], "to": p["to"], "ids": "[" + strings.Join(ids, ",") + "]"}
	listeners, err := s.rows(ctx, `SELECT toString(`+key+`) AS id, uniqExact(user_id) AS unique_listeners
FROM `+analytics.TablePlaybackEvents+` `+arrayJoin+`
WHERE toDate(at) BETWEEN {from:Date} AND {to:Date} AND `+key+` IN {ids:Array(UUID)}
GROUP BY `+key, lp)
	if err != nil {
		return nil, err
	}
	uniq := make(map[string]uint64, len(listeners))
	for _, l := range listeners {
		uniq[l.ID] = l.UniqueListeners
	}
	out := make([]domain.Ranked, len(rows))
	for i, r := range rows {
		st := r.stats()
		st.UniqueListeners = uniq[r.ID]
		id, err := uuid.Parse(r.ID)
		if err != nil {
			return nil, fmt.Errorf("report: id %q: %w", r.ID, err)
		}
		out[i] = domain.Ranked{ID: id, Stats: st, CompletionRate: st.CompletionRate()}
	}
	return out, nil
}

func (s *Store) rows(ctx context.Context, q string, p clickhouse.Params) ([]statsRow, error) {
	var out []statsRow
	err := s.ch.Query(ctx, q, p, func(line []byte) error {
		var r statsRow
		if err := json.Unmarshal(line, &r); err != nil {
			return err
		}
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("report: %w", err)
	}
	return out, nil
}
