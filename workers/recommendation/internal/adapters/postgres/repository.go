// Package postgres stores the worker's taste signals (likes, audio
// features) with pgx.
package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tehrelt/icyre/workers/recommendation/internal/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Schema owned by the Recommendation Worker.
const Schema = "recommendation"

// Migrations returns the embedded migrations.
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		panic(err)
	}
	return sub
}

// Repository implements application.SignalStore.
type Repository struct{ pool *pgxpool.Pool }

// New returns a Repository.
func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Like kinds: tracks and albums.
const (
	KindTrack = "track"
	KindAlbum = "album"
)

// tables by like kind; fixed identifiers, never user input.
var likeTables = map[string]struct{ table, column string }{
	KindTrack: {"recommendation.liked_tracks", "track_id"},
	KindAlbum: {"recommendation.liked_albums", "album_id"},
}

// SaveLike records a like.
func (r *Repository) SaveLike(ctx context.Context, kind string, user, id uuid.UUID, at time.Time) error {
	return r.setLike(ctx, kind, user, id, true, at)
}

// RemoveLike records a removal.
func (r *Repository) RemoveLike(ctx context.Context, kind string, user, id uuid.UUID, at time.Time) error {
	return r.setLike(ctx, kind, user, id, false, at)
}

// setLike applies a save or removal unless a newer one is already stored:
// the last event by time wins whatever the delivery order.
func (r *Repository) setLike(ctx context.Context, kind string, user, id uuid.UUID, liked bool, at time.Time) error {
	t, ok := likeTables[kind]
	if !ok {
		return fmt.Errorf("unknown like kind %q", kind)
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO `+t.table+` (user_id, `+t.column+`, liked, changed_at) VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, `+t.column+`) DO UPDATE SET liked = excluded.liked, changed_at = excluded.changed_at
WHERE excluded.changed_at >= `+t.table+`.changed_at`, user, id, liked, at)
	if err != nil {
		return fmt.Errorf("set %s like: %w", kind, err)
	}
	return nil
}

// Features are raw audio features of a track's master.
type Features struct {
	TrackID         uuid.UUID
	BPM             *float64
	IntegratedLUFS  float64
	LoudnessRangeLU float64
	SilenceRatio    float64
	AnalyzerVersion string
	UploadedAt      time.Time
}

// SaveFeatures keeps the features of the track's latest master.
func (r *Repository) SaveFeatures(ctx context.Context, f Features) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO recommendation.track_features
    (track_id, bpm, integrated_lufs, loudness_range_lu, silence_ratio, analyzer_version, uploaded_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (track_id) DO UPDATE SET
    bpm = excluded.bpm, integrated_lufs = excluded.integrated_lufs, loudness_range_lu = excluded.loudness_range_lu,
    silence_ratio = excluded.silence_ratio, analyzer_version = excluded.analyzer_version, uploaded_at = excluded.uploaded_at
WHERE excluded.uploaded_at >= recommendation.track_features.uploaded_at`,
		f.TrackID, f.BPM, f.IntegratedLUFS, f.LoudnessRangeLU, f.SilenceRatio, f.AnalyzerVersion, f.UploadedAt)
	if err != nil {
		return fmt.Errorf("save features: %w", err)
	}
	return nil
}

// Sounds returns the normalised features of every analysed track.
func (r *Repository) Sounds(ctx context.Context) (map[uuid.UUID]*domain.Sound, error) {
	rows, err := r.pool.Query(ctx, `SELECT track_id, bpm, integrated_lufs, loudness_range_lu, silence_ratio, analyzer_version
FROM recommendation.track_features`)
	if err != nil {
		return nil, fmt.Errorf("load features: %w", err)
	}
	defer rows.Close()
	out := map[uuid.UUID]*domain.Sound{}
	for rows.Next() {
		var (
			id                 uuid.UUID
			bpm                *float64
			lufs, lra, silence float64
			version            string
		)
		if err := rows.Scan(&id, &bpm, &lufs, &lra, &silence, &version); err != nil {
			return nil, fmt.Errorf("scan features: %w", err)
		}
		out[id] = domain.NewSound(version, bpm, lufs, lra, silence)
	}
	return out, rows.Err()
}

// Likes returns every user's liked tracks and albums.
func (r *Repository) Likes(ctx context.Context) (tracks, albums map[uuid.UUID]map[uuid.UUID]bool, err error) {
	if tracks, err = r.likes(ctx, `SELECT user_id, track_id FROM recommendation.liked_tracks WHERE liked`); err != nil {
		return nil, nil, err
	}
	if albums, err = r.likes(ctx, `SELECT user_id, album_id FROM recommendation.liked_albums WHERE liked`); err != nil {
		return nil, nil, err
	}
	return tracks, albums, nil
}

func (r *Repository) likes(ctx context.Context, q string) (map[uuid.UUID]map[uuid.UUID]bool, error) {
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("load likes: %w", err)
	}
	defer rows.Close()
	out := map[uuid.UUID]map[uuid.UUID]bool{}
	for rows.Next() {
		var user, id uuid.UUID
		if err := rows.Scan(&user, &id); err != nil {
			return nil, fmt.Errorf("scan like: %w", err)
		}
		if out[user] == nil {
			out[user] = map[uuid.UUID]bool{}
		}
		out[user][id] = true
	}
	return out, rows.Err()
}
