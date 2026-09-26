//go:build integration

package postgres

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	platformpg "github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/workers/metadata/internal/domain"
)

func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("METADATA_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("METADATA_TEST_DATABASE_DSN is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("metadata_it_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	cfg, _ := pgxpool.ParseConfig(dsn)
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	})
	if err := platformpg.Migrate(ctx, pool, Schema, Migrations(), slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	return pool
}

func encode(_ context.Context, m domain.Metadata) (platformkafka.Message, error) {
	return platformkafka.Message{Topic: "media.events", Key: []byte(m.TrackID.String()), Value: []byte(m.UploadID.String())}, nil
}

func TestRepositorySaveIsIdempotent(t *testing.T) {
	pool := newTestDB(t)
	repo := New(pool, encode)
	ctx := context.Background()
	at := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	m := domain.Metadata{
		UploadID: uuid.New(), TrackID: uuid.New(), SourceSHA256: "ab12",
		Probe:      domain.Probe{Container: "flac", Codec: "flac", DurationMs: 215000, BitrateBps: 910000, SampleRateHz: 44100, Channels: 2},
		UploadedAt: at, ExtractedAt: at.Add(time.Second),
	}

	if ok, err := repo.Exists(ctx, m.UploadID); err != nil || ok {
		t.Fatalf("Exists before save = %v, %v", ok, err)
	}
	if saved, err := repo.Save(ctx, m); err != nil || !saved {
		t.Fatalf("first Save = %v, %v", saved, err)
	}
	if saved, err := repo.Save(ctx, m); err != nil || saved {
		t.Fatalf("second Save = %v, %v", saved, err)
	}
	if ok, err := repo.Exists(ctx, m.UploadID); err != nil || !ok {
		t.Fatalf("Exists after save = %v, %v", ok, err)
	}

	var got domain.Metadata
	err := pool.QueryRow(ctx, `SELECT track_id, container, codec, duration_ms, bitrate_bps, sample_rate_hz, channels, source_sha256, uploaded_at
		FROM media_metadata.track_metadata WHERE upload_id = $1`, m.UploadID).
		Scan(&got.TrackID, &got.Container, &got.Codec, &got.DurationMs, &got.BitrateBps, &got.SampleRateHz, &got.Channels, &got.SourceSHA256, &got.UploadedAt)
	if err != nil {
		t.Fatal(err)
	}
	if got.TrackID != m.TrackID || got.Probe != m.Probe || got.SourceSHA256 != "ab12" || !got.UploadedAt.Equal(at) {
		t.Fatalf("row = %+v", got)
	}

	// Exactly one event: the redelivery wrote none.
	var n int
	var value string
	if err := pool.QueryRow(ctx, `SELECT count(*), max(convert_from(value, 'UTF8')) FROM media_metadata.outbox`).Scan(&n, &value); err != nil {
		t.Fatal(err)
	}
	if n != 1 || value != m.UploadID.String() {
		t.Fatalf("outbox = %d rows, value %q", n, value)
	}
}
