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
	"github.com/tehrelt/icyre/workers/audio-analysis/internal/domain"
)

func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("AUDIO_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("AUDIO_TEST_DATABASE_DSN is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("audio_it_%d", time.Now().UnixNano())
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

func encode(_ context.Context, a domain.Analysis) (platformkafka.Message, error) {
	return platformkafka.Message{Topic: "media.events", Key: []byte(a.TrackID.String()), Value: []byte(a.UploadID.String())}, nil
}

func TestRepositorySaveIsIdempotent(t *testing.T) {
	pool := newTestDB(t)
	repo := New(pool, encode)
	ctx := context.Background()
	at := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	m := domain.Analysis{
		UploadID: uuid.New(), TrackID: uuid.New(), SourceSHA256: "ab12", AnalyzerVersion: domain.AnalyzerVersion,
		Features: domain.Features{
			BPM: 128, BPMConfidence: 0.71, IntegratedLUFS: -9.4, LoudnessRangeLU: 5.2, TruePeakDBTP: -0.3, SilenceRatio: 0.02, AnalyzedMs: 215000,
		},
		UploadedAt: at, AnalyzedAt: at.Add(time.Second),
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

	var got domain.Analysis
	var bpm *float64
	err := pool.QueryRow(ctx, `SELECT track_id, bpm, bpm_confidence, integrated_lufs, loudness_range_lu, true_peak_dbtp,
			silence_ratio, analyzed_ms, analyzer_version, source_sha256, uploaded_at
		FROM audio_features.track_features WHERE upload_id = $1`, m.UploadID).
		Scan(&got.TrackID, &bpm, &got.BPMConfidence, &got.IntegratedLUFS, &got.LoudnessRangeLU, &got.TruePeakDBTP,
			&got.SilenceRatio, &got.AnalyzedMs, &got.AnalyzerVersion, &got.SourceSHA256, &got.UploadedAt)
	if err != nil {
		t.Fatal(err)
	}
	if bpm != nil {
		got.BPM = *bpm
	}
	if got.TrackID != m.TrackID || got.Features != m.Features || got.AnalyzerVersion != "1" || got.SourceSHA256 != "ab12" || !got.UploadedAt.Equal(at) {
		t.Fatalf("row = %+v", got)
	}

	// Exactly one event: the redelivery wrote none.
	var n int
	var value string
	if err := pool.QueryRow(ctx, `SELECT count(*), max(convert_from(value, 'UTF8')) FROM audio_features.outbox`).Scan(&n, &value); err != nil {
		t.Fatal(err)
	}
	if n != 1 || value != m.UploadID.String() {
		t.Fatalf("outbox = %d rows, value %q", n, value)
	}
}

func TestRepositoryStoresNoTempoAsNull(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	at := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	m := domain.Analysis{
		UploadID: uuid.New(), TrackID: uuid.New(), SourceSHA256: "cd34", AnalyzerVersion: domain.AnalyzerVersion,
		Features:   domain.Features{IntegratedLUFS: domain.Floor, TruePeakDBTP: domain.Floor, SilenceRatio: 1, AnalyzedMs: 10000},
		UploadedAt: at, AnalyzedAt: at,
	}
	if saved, err := New(pool, encode).Save(ctx, m); err != nil || !saved {
		t.Fatalf("Save = %v, %v", saved, err)
	}
	var null bool
	if err := pool.QueryRow(ctx, `SELECT bpm IS NULL FROM audio_features.track_features WHERE upload_id = $1`, m.UploadID).Scan(&null); err != nil || !null {
		t.Fatalf("bpm IS NULL = %v, %v", null, err)
	}
}
