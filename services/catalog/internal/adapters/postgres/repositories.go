package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tehrelt/icyre/services/catalog/internal/domain"
	"github.com/tehrelt/icyre/services/catalog/internal/ports"
)

// ArtistRepository implements ports.ArtistRepository.
type ArtistRepository struct{ pool *pgxpool.Pool }

// NewArtistRepository returns an ArtistRepository.
func NewArtistRepository(pool *pgxpool.Pool) *ArtistRepository { return &ArtistRepository{pool} }

// Create inserts an artist.
func (r *ArtistRepository) Create(ctx context.Context, a domain.Artist) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO catalog.artists (id, name, created_at, updated_at) VALUES ($1, $2, $3, $4)`,
		a.ID, a.Name, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert artist: %w", mapError(err))
	}
	return nil
}

// Get loads an artist.
func (r *ArtistRepository) Get(ctx context.Context, id uuid.UUID) (domain.Artist, error) {
	var a domain.Artist
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, created_at, updated_at FROM catalog.artists WHERE id = $1`, id,
	).Scan(&a.ID, &a.Name, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Artist{}, domain.ErrArtistNotFound
	}
	if err != nil {
		return domain.Artist{}, fmt.Errorf("select artist: %w", err)
	}
	return a, nil
}

// ListByIDs loads the existing artists among ids.
func (r *ArtistRepository) ListByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Artist, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, created_at, updated_at FROM catalog.artists WHERE id = ANY($1::uuid[])`, uuidStrings(ids))
	if err != nil {
		return nil, fmt.Errorf("select artists: %w", err)
	}
	artists, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Artist, error) {
		var a domain.Artist
		err := row.Scan(&a.ID, &a.Name, &a.CreatedAt, &a.UpdatedAt)
		return a, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan artists: %w", err)
	}
	return artists, nil
}

// AlbumRepository implements ports.AlbumRepository.
type AlbumRepository struct{ pool *pgxpool.Pool }

// NewAlbumRepository returns an AlbumRepository.
func NewAlbumRepository(pool *pgxpool.Pool) *AlbumRepository { return &AlbumRepository{pool} }

const albumColumns = `
	a.id, a.title, a.album_type, a.release_date, a.created_at, a.updated_at,
	ARRAY(SELECT aa.artist_id::text FROM catalog.album_artists aa WHERE aa.album_id = a.id ORDER BY aa.position),
	ARRAY(SELECT ag.genre_id::text FROM catalog.album_genres ag WHERE ag.album_id = a.id ORDER BY ag.position)`

// Create inserts an album and its credits in one transaction.
func (r *AlbumRepository) Create(ctx context.Context, a domain.Album) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO catalog.albums (id, title, album_type, release_date, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			a.ID, a.Title, string(a.Type), a.ReleaseDate, a.CreatedAt, a.UpdatedAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO catalog.album_artists (album_id, artist_id, position)
			 SELECT $1, u.id, u.ord - 1 FROM unnest($2::uuid[]) WITH ORDINALITY AS u(id, ord)`,
			a.ID, uuidStrings(a.ArtistIDs)); err != nil {
			return err
		}
		if len(a.GenreIDs) > 0 {
			if _, err := tx.Exec(ctx,
				`INSERT INTO catalog.album_genres (album_id, genre_id, position)
				 SELECT $1, u.id, u.ord - 1 FROM unnest($2::uuid[]) WITH ORDINALITY AS u(id, ord)`,
				a.ID, uuidStrings(a.GenreIDs)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("insert album: %w", mapError(err))
	}
	return nil
}

// Get loads an album with its credits.
func (r *AlbumRepository) Get(ctx context.Context, id uuid.UUID) (domain.Album, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+albumColumns+` FROM catalog.albums a WHERE a.id = $1`, id)
	if err != nil {
		return domain.Album{}, fmt.Errorf("select album: %w", err)
	}
	albums, err := collectAlbums(rows)
	if err != nil {
		return domain.Album{}, err
	}
	if len(albums) == 0 {
		return domain.Album{}, domain.ErrAlbumNotFound
	}
	return albums[0], nil
}

// ListByArtist pages through an artist's albums, newest first.
func (r *AlbumRepository) ListByArtist(ctx context.Context, artistID uuid.UUID, after *ports.AlbumCursor, limit int) ([]domain.Album, error) {
	var (
		afterDate *time.Time
		afterID   *uuid.UUID
	)
	if after != nil {
		afterDate, afterID = &after.ReleaseDate, &after.ID
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+albumColumns+`
		FROM catalog.albums a
		JOIN catalog.album_artists credit ON credit.album_id = a.id AND credit.artist_id = $1
		WHERE $2::date IS NULL OR (a.release_date, a.id) < ($2::date, $3::uuid)
		ORDER BY a.release_date DESC, a.id DESC
		LIMIT $4`,
		artistID, afterDate, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("select artist albums: %w", err)
	}
	return collectAlbums(rows)
}

// List pages through all albums, newest first.
func (r *AlbumRepository) List(ctx context.Context, after *ports.AlbumCursor, limit int) ([]domain.Album, error) {
	var (
		afterDate *time.Time
		afterID   *uuid.UUID
	)
	if after != nil {
		afterDate, afterID = &after.ReleaseDate, &after.ID
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+albumColumns+`
		FROM catalog.albums a
		WHERE $1::date IS NULL OR (a.release_date, a.id) < ($1::date, $2::uuid)
		ORDER BY a.release_date DESC, a.id DESC
		LIMIT $3`,
		afterDate, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("select albums: %w", err)
	}
	return collectAlbums(rows)
}

func collectAlbums(rows pgx.Rows) ([]domain.Album, error) {
	albums, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Album, error) {
		var (
			a                 domain.Album
			albumType         string
			artists, genreIDs []string
		)
		if err := row.Scan(&a.ID, &a.Title, &albumType, &a.ReleaseDate, &a.CreatedAt, &a.UpdatedAt, &artists, &genreIDs); err != nil {
			return domain.Album{}, err
		}
		a.Type = domain.AlbumType(albumType)
		var err error
		if a.ArtistIDs, err = parseUUIDs(artists); err != nil {
			return domain.Album{}, err
		}
		if a.GenreIDs, err = parseUUIDs(genreIDs); err != nil {
			return domain.Album{}, err
		}
		return a, nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan albums: %w", err)
	}
	return albums, nil
}

// TrackRepository implements ports.TrackRepository.
type TrackRepository struct{ pool *pgxpool.Pool }

// NewTrackRepository returns a TrackRepository.
func NewTrackRepository(pool *pgxpool.Pool) *TrackRepository { return &TrackRepository{pool} }

const trackColumns = `
	t.id, t.album_id, t.title, t.duration_ms, t.track_number, t.disc_number, t.explicit,
	COALESCE(t.isrc, ''), t.status, t.created_at, t.updated_at,
	ARRAY(SELECT ta.artist_id::text FROM catalog.track_artists ta WHERE ta.track_id = t.id ORDER BY ta.position)`

// Create inserts a track and its artist credits in one transaction.
func (r *TrackRepository) Create(ctx context.Context, t domain.Track) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO catalog.tracks
				(id, album_id, title, duration_ms, track_number, disc_number, explicit, isrc, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, $10, $11)`,
			t.ID, t.AlbumID, t.Title, t.Duration.Milliseconds(), t.TrackNumber, t.DiscNumber, t.Explicit,
			t.ISRC, string(t.Status), t.CreatedAt, t.UpdatedAt); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO catalog.track_artists (track_id, artist_id, position)
			 SELECT $1, u.id, u.ord - 1 FROM unnest($2::uuid[]) WITH ORDINALITY AS u(id, ord)`,
			t.ID, uuidStrings(t.ArtistIDs))
		return err
	})
	if err != nil {
		return fmt.Errorf("insert track: %w", mapError(err))
	}
	return nil
}

// Get loads a track, including deleted ones.
func (r *TrackRepository) Get(ctx context.Context, id uuid.UUID) (domain.Track, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+trackColumns+` FROM catalog.tracks t WHERE t.id = $1`, id)
	if err != nil {
		return domain.Track{}, fmt.Errorf("select track: %w", err)
	}
	tracks, err := collectTracks(rows)
	if err != nil {
		return domain.Track{}, err
	}
	if len(tracks) == 0 {
		return domain.Track{}, domain.ErrTrackNotFound
	}
	return tracks[0], nil
}

// Update persists the mutable fields of a track.
func (r *TrackRepository) Update(ctx context.Context, t domain.Track) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE catalog.tracks SET title = $2, explicit = $3, status = $4, updated_at = $5
		WHERE id = $1`,
		t.ID, t.Title, t.Explicit, string(t.Status), t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update track: %w", mapError(err))
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTrackNotFound
	}
	return nil
}

// ListByAlbum returns the album's live tracks in play order.
func (r *TrackRepository) ListByAlbum(ctx context.Context, albumID uuid.UUID) ([]domain.Track, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+trackColumns+`
		FROM catalog.tracks t
		WHERE t.album_id = $1 AND t.status <> 'DELETED'
		ORDER BY t.disc_number, t.track_number`, albumID)
	if err != nil {
		return nil, fmt.Errorf("select album tracks: %w", err)
	}
	return collectTracks(rows)
}

func collectTracks(rows pgx.Rows) ([]domain.Track, error) {
	tracks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Track, error) {
		var (
			t          domain.Track
			durationMs int64
			status     string
			artists    []string
		)
		if err := row.Scan(&t.ID, &t.AlbumID, &t.Title, &durationMs, &t.TrackNumber, &t.DiscNumber, &t.Explicit,
			&t.ISRC, &status, &t.CreatedAt, &t.UpdatedAt, &artists); err != nil {
			return domain.Track{}, err
		}
		t.Duration = time.Duration(durationMs) * time.Millisecond
		t.Status = domain.TrackStatus(status)
		var err error
		t.ArtistIDs, err = parseUUIDs(artists)
		return t, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan tracks: %w", err)
	}
	return tracks, nil
}

// GenreRepository implements ports.GenreRepository.
type GenreRepository struct{ pool *pgxpool.Pool }

// NewGenreRepository returns a GenreRepository.
func NewGenreRepository(pool *pgxpool.Pool) *GenreRepository { return &GenreRepository{pool} }

// List returns all genres ordered by name.
func (r *GenreRepository) List(ctx context.Context) ([]domain.Genre, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, slug, name FROM catalog.genres ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("select genres: %w", err)
	}
	genres, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Genre, error) {
		var g domain.Genre
		err := row.Scan(&g.ID, &g.Slug, &g.Name)
		return g, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan genres: %w", err)
	}
	return genres, nil
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func parseUUIDs(ss []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, len(ss))
	for i, s := range ss {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("parse uuid %q: %w", s, err)
		}
		out[i] = id
	}
	return out, nil
}
