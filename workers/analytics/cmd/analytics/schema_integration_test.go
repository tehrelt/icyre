//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/tehrelt/icyre/libs/platform/clickhouse"
)

// Applies the analytics schema to a throwaway database on CLICKHOUSE_URL and
// checks that redelivered events collapse and a day aggregates as documented.
func TestSchema(t *testing.T) {
	url := os.Getenv("CLICKHOUSE_URL")
	if url == "" {
		t.Skip("CLICKHOUSE_URL not set")
	}
	ctx := context.Background()
	cfg := clickhouse.Config{URL: url, Username: os.Getenv("CLICKHOUSE_USERNAME"), Password: os.Getenv("CLICKHOUSE_PASSWORD")}
	db := fmt.Sprintf("it_analytics_%d", time.Now().UnixNano())
	if err := clickhouse.New(cfg, nil).Exec(ctx, "CREATE DATABASE "+db, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clickhouse.New(cfg, nil).Exec(context.Background(), "DROP DATABASE IF EXISTS "+db, nil) })
	cfg.Database = db
	ch := clickhouse.New(cfg, nil)
	if err := migrate(ctx, ch, slog.Default()); err != nil {
		t.Fatal(err)
	}

	const (
		track  = "00000000-0000-0000-0000-00000000000a"
		artist = "00000000-0000-0000-0000-0000000000a1"
		u1     = "00000000-0000-0000-0000-000000000001"
		u2     = "00000000-0000-0000-0000-000000000002"
	)
	ev := func(id int, typ, user string) map[string]any {
		return map[string]any{
			"event_id": fmt.Sprintf("00000000-0000-0000-0000-%012d", id), "event_type": typ,
			"playback_id": fmt.Sprintf("00000000-0000-0000-0001-%012d", id), "user_id": user, "track_id": track,
			"album_id": "00000000-0000-0000-0000-000000000000", "artist_ids": []string{artist}, "source": "album",
			"duration_ms": 200000, "listened_ms": 150000, "at": "2026-09-27 12:00:00.000",
		}
	}
	rows := []any{ev(1, "playback.started", u1), ev(2, "playback.finished", u1), ev(3, "playback.started", u2), ev(4, "playback.skipped", u2)}
	if err := ch.Insert(ctx, "playback_events", rows, "b1"); err != nil {
		t.Fatal(err)
	}
	// A redelivered event in another batch: ReplacingMergeTree collapses it.
	if err := ch.Insert(ctx, "playback_events", rows[:1], "b2"); err != nil {
		t.Fatal(err)
	}

	var got struct {
		Plays, Completions, Skips, Listeners json.Number
	}
	err := ch.Query(ctx, `SELECT countIf(event_type = 'playback.started')  AS Plays,
       countIf(event_type = 'playback.finished') AS Completions,
       countIf(event_type = 'playback.skipped')  AS Skips,
       uniqExact(user_id)                        AS Listeners
FROM playback_events FINAL
WHERE toDate(at) = {day:Date} AND track_id = {track:UUID}`,
		clickhouse.Params{"day": "2026-09-27", "track": track}, func(line []byte) error { return json.Unmarshal(line, &got) })
	if err != nil {
		t.Fatal(err)
	}
	if got.Plays != "2" || got.Completions != "1" || got.Skips != "1" || got.Listeners != "2" {
		t.Fatalf("aggregate %+v", got)
	}

	for _, table := range []string{"daily_track_stats", "daily_artist_stats", "daily_totals"} {
		if err := ch.Exec(ctx, "SELECT * FROM "+table+" LIMIT 0", nil); err != nil {
			t.Errorf("%s: %v", table, err)
		}
	}
}
