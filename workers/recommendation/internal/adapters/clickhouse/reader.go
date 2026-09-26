// Package clickhouse reads listening history and popularity from the
// analytics store (schema: libs/contracts/analytics).
package clickhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/analytics"
	"github.com/tehrelt/icyre/libs/platform/clickhouse"
	"github.com/tehrelt/icyre/workers/recommendation/internal/domain"
)

// Reader implements application.Popularity and application.HistorySource.
type Reader struct{ ch *clickhouse.Client }

// New returns a Reader.
func New(ch *clickhouse.Client) *Reader { return &Reader{ch: ch} }

// Plays returns each track's plays over the last days, from the daily
// aggregates.
func (r *Reader) Plays(ctx context.Context, days int) (map[uuid.UUID]uint64, error) {
	out := map[uuid.UUID]uint64{}
	err := r.ch.Query(ctx, `SELECT toString(track_id) AS id, sum(plays) AS plays
FROM `+analytics.TableDailyTrackStats+` FINAL
WHERE day > today() - {days:UInt32}
GROUP BY track_id`, clickhouse.Params{"days": strconv.Itoa(days)}, func(line []byte) error {
		var row struct {
			ID    string `json:"id"`
			Plays uint64 `json:"plays"`
		}
		if err := json.Unmarshal(line, &row); err != nil {
			return err
		}
		id, err := uuid.Parse(row.ID)
		if err != nil {
			return err
		}
		out[id] = row.Plays
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("popularity: %w", err)
	}
	return out, nil
}

// History returns what every user did with every track over the last days.
// FINAL collapses redelivered events.
func (r *Reader) History(ctx context.Context, days int) (map[uuid.UUID]map[uuid.UUID]domain.Interaction, error) {
	out := map[uuid.UUID]map[uuid.UUID]domain.Interaction{}
	err := r.ch.Query(ctx, `SELECT toString(user_id) AS user, toString(track_id) AS track,
    countIf(event_type = 'playback.started')  AS plays,
    countIf(event_type = 'playback.finished') AS completions,
    countIf(event_type = 'playback.skipped')  AS skips
FROM `+analytics.TablePlaybackEvents+` FINAL
WHERE at > now64(3) - toIntervalDay({days:UInt32})
GROUP BY user_id, track_id`, clickhouse.Params{"days": strconv.Itoa(days)}, func(line []byte) error {
		var row struct {
			User        string `json:"user"`
			Track       string `json:"track"`
			Plays       uint64 `json:"plays"`
			Completions uint64 `json:"completions"`
			Skips       uint64 `json:"skips"`
		}
		if err := json.Unmarshal(line, &row); err != nil {
			return err
		}
		user, e1 := uuid.Parse(row.User)
		track, e2 := uuid.Parse(row.Track)
		if e1 != nil || e2 != nil {
			return fmt.Errorf("history row %s/%s", row.User, row.Track)
		}
		if out[user] == nil {
			out[user] = map[uuid.UUID]domain.Interaction{}
		}
		out[user][track] = domain.Interaction{Plays: row.Plays, Completions: row.Completions, Skips: row.Skips}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}
	return out, nil
}
