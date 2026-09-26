// Package postgres stores track metadata with pgx.
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
	"github.com/tehrelt/icyre/workers/metadata/internal/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Schema owned by the Metadata Worker.
const Schema = "media_metadata"

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

// Encoder turns stored metadata into its media.metadata_extracted message.
type Encoder func(ctx context.Context, m domain.Metadata) (platformkafka.Message, error)

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
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM media_metadata.track_metadata WHERE upload_id = $1)`, uploadID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("metadata exists: %w", err)
	}
	return ok, nil
}

// Save implements application.Repository: the row and its event commit
// together, and only the delivery that inserted the row writes the event.
func (r *Repository) Save(ctx context.Context, m domain.Metadata) (bool, error) {
	msg, err := r.encode(ctx, m)
	if err != nil {
		return false, err
	}
	saved := false
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			INSERT INTO media_metadata.track_metadata
				(upload_id, track_id, container, codec, duration_ms, bitrate_bps, sample_rate_hz, channels, source_sha256, uploaded_at, extracted_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (upload_id) DO NOTHING`,
			m.UploadID, m.TrackID, m.Container, m.Codec, m.DurationMs, m.BitrateBps, m.SampleRateHz, m.Channels,
			m.SourceSHA256, m.UploadedAt, m.ExtractedAt)
		if err != nil {
			return fmt.Errorf("insert metadata: %w", err)
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
