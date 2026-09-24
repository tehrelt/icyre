// Package postgres implements the Catalog repositories with pgx.
package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tehrelt/icyre/services/catalog/internal/domain"
)

const (
	pgForeignKeyViolation = "23503"
	pgUniqueViolation     = "23505"
)

// constraintErrors maps database constraints to domain errors, so the
// database stays the final guard of every invariant.
var constraintErrors = map[string]error{
	"album_artists_artist_id_fkey": &domain.ReferenceError{Field: "artistIds", Err: domain.ErrArtistNotFound},
	"track_artists_artist_id_fkey": &domain.ReferenceError{Field: "artistIds", Err: domain.ErrArtistNotFound},
	"album_genres_genre_id_fkey":   &domain.ReferenceError{Field: "genreIds", Err: domain.ErrGenreNotFound},
	"tracks_album_id_fkey":         &domain.ReferenceError{Field: "albumId", Err: domain.ErrAlbumNotFound},
	"tracks_album_position_key":    domain.ErrTrackPositionTaken,
}

// mapError translates known constraint violations; other errors pass through.
func mapError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	if pgErr.Code != pgForeignKeyViolation && pgErr.Code != pgUniqueViolation {
		return err
	}
	if mapped, ok := constraintErrors[pgErr.ConstraintName]; ok {
		return mapped
	}
	return err
}
