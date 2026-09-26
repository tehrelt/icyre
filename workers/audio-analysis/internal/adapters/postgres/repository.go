// Package postgres stores audio features with pgx.
package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/libs/platform/outbox"
	"github.com/tehrelt/icyre/workers/audio-analysis/internal/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Schema owned by the Audio Analysis Worker.
const Schema = "audio_features"

// OutboxTable holds media.events messages until the relay sends them.
const OutboxTable = Schema + ".outbox"

// Migrations returns the embedded migrations.
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		panic(err)
	}
	return sub
}

// Encoder turns stored features into their audio.features_extracted message.
type Encoder func(ctx context.Context, a domain.Analysis) (platformkafka.Message, error)

// Repository implements application.Repository.
type Repository struct {
	pool   *pgxpool.Pool
	encode Encoder
}

// New returns a Repository.
func New(pool *pgxpool.Pool, encode Encoder) *Repository {
	return &Repository{pool: pool, encode: encode}
}

// Exists implements application.Repository.
func (r *Repository) Exists(ctx context.Context, uploadID uuid.UUID) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM audio_features.track_features WHERE upload_id = $1)`, uploadID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("features exist: %w", err)
	}
	return ok, nil
}

// Save implements application.Repository: the row and its event commit
// together, and only the delivery that inserted the row writes the event.
func (r *Repository) Save(ctx context.Context, a domain.Analysis) (bool, error) {
	msg, err := r.encode(ctx, a)
	if err != nil {
		return false, err
	}
	saved := false
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			INSERT INTO audio_features.track_features
				(upload_id, track_id, bpm, bpm_confidence, integrated_lufs, loudness_range_lu, true_peak_dbtp,
				 silence_ratio, analyzed_ms, analyzer_version, source_sha256, uploaded_at, analyzed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (upload_id) DO NOTHING`,
			a.UploadID, a.TrackID, nullBPM(a.BPM), a.BPMConfidence, a.IntegratedLUFS, a.LoudnessRangeLU, a.TruePeakDBTP,
			a.SilenceRatio, a.AnalyzedMs, a.AnalyzerVersion, a.SourceSHA256, a.UploadedAt, a.AnalyzedAt)
		if err != nil {
			return fmt.Errorf("insert features: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		saved = true
		return outbox.Write(ctx, tx, OutboxTable, msg)
	})
	if err != nil {
		return false, err
	}
	return saved, nil
}

// nullBPM stores "no tempo" as NULL.
func nullBPM(bpm float64) *float64 {
	if bpm == 0 {
		return nil
	}
	return &bpm
}
