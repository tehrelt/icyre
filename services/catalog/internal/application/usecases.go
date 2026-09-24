package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/catalog/internal/domain"
	"github.com/tehrelt/icyre/services/catalog/internal/ports"
)

// CreateArtist is the input of Service.CreateArtist.
type CreateArtist struct {
	Name string
}

// CreateArtist stores a new artist and announces it.
func (s *Service) CreateArtist(ctx context.Context, cmd CreateArtist) (domain.Artist, error) {
	id, err := s.newID()
	if err != nil {
		return domain.Artist{}, err
	}
	a, err := domain.NewArtist(id, cmd.Name, s.d.Now())
	if err != nil {
		return domain.Artist{}, err
	}
	if err := s.d.Artists.Create(ctx, a); err != nil {
		return domain.Artist{}, fmt.Errorf("store artist: %w", err)
	}
	s.publish(ctx, domain.ArtistCreated{Artist: a})
	return a, nil
}

// GetArtist returns an artist by ID.
func (s *Service) GetArtist(ctx context.Context, id uuid.UUID) (domain.Artist, error) {
	return s.d.Artists.Get(ctx, id)
}

// ListArtistAlbums is the input of Service.ListArtistAlbums.
type ListArtistAlbums struct {
	ArtistID uuid.UUID
	After    *ports.AlbumCursor
	Limit    int
}

// Page limits.
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// AlbumPage is one page of albums.
type AlbumPage struct {
	Albums []domain.Album
	// Next is the cursor of the following page, nil on the last page.
	Next *ports.AlbumCursor
}

// ListArtistAlbums pages through an artist's releases, newest first.
func (s *Service) ListArtistAlbums(ctx context.Context, q ListArtistAlbums) (AlbumPage, error) {
	if _, err := s.d.Artists.Get(ctx, q.ArtistID); err != nil {
		return AlbumPage{}, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}

	// Fetch one extra row to know whether another page exists.
	albums, err := s.d.Albums.ListByArtist(ctx, q.ArtistID, q.After, limit+1)
	if err != nil {
		return AlbumPage{}, fmt.Errorf("list albums: %w", err)
	}
	page := AlbumPage{Albums: albums}
	if len(albums) > limit {
		page.Albums = albums[:limit]
		last := page.Albums[limit-1]
		page.Next = &ports.AlbumCursor{ReleaseDate: last.ReleaseDate, ID: last.ID}
	}
	return page, nil
}

// CreateAlbum is the input of Service.CreateAlbum.
type CreateAlbum struct {
	Title       string
	Type        domain.AlbumType
	ReleaseDate time.Time
	ArtistIDs   []uuid.UUID
	GenreIDs    []uuid.UUID
}

// CreateAlbum stores a release credited to existing artists.
func (s *Service) CreateAlbum(ctx context.Context, cmd CreateAlbum) (domain.Album, error) {
	id, err := s.newID()
	if err != nil {
		return domain.Album{}, err
	}
	a, err := domain.NewAlbum(domain.NewAlbumParams{
		ID: id, Title: cmd.Title, Type: cmd.Type, ReleaseDate: cmd.ReleaseDate,
		ArtistIDs: cmd.ArtistIDs, GenreIDs: cmd.GenreIDs,
	}, s.d.Now())
	if err != nil {
		return domain.Album{}, err
	}
	if err := s.d.Albums.Create(ctx, a); err != nil {
		return domain.Album{}, fmt.Errorf("store album: %w", err)
	}
	s.publish(ctx, domain.AlbumCreated{Album: a})
	return a, nil
}

// GetAlbum returns an album by ID.
func (s *Service) GetAlbum(ctx context.Context, id uuid.UUID) (domain.Album, error) {
	return s.d.Albums.Get(ctx, id)
}

// ListAlbumTracks returns the album's tracks in play order.
func (s *Service) ListAlbumTracks(ctx context.Context, albumID uuid.UUID) ([]domain.Track, error) {
	if _, err := s.d.Albums.Get(ctx, albumID); err != nil {
		return nil, err
	}
	tracks, err := s.d.Tracks.ListByAlbum(ctx, albumID)
	if err != nil {
		return nil, fmt.Errorf("list tracks: %w", err)
	}
	return tracks, nil
}

// CreateTrack is the input of Service.CreateTrack.
type CreateTrack struct {
	AlbumID uuid.UUID
	// ArtistIDs defaults to the album's artists when empty.
	ArtistIDs   []uuid.UUID
	Title       string
	Duration    time.Duration
	TrackNumber int
	DiscNumber  int
	Explicit    bool
	ISRC        string
}

// CreateTrack adds a DRAFT track to an existing album.
func (s *Service) CreateTrack(ctx context.Context, cmd CreateTrack) (domain.Track, error) {
	album, err := s.d.Albums.Get(ctx, cmd.AlbumID)
	if err != nil {
		return domain.Track{}, asReference(err, "albumId", domain.ErrAlbumNotFound)
	}
	artists := cmd.ArtistIDs
	if len(artists) == 0 {
		artists = album.ArtistIDs
	}

	id, err := s.newID()
	if err != nil {
		return domain.Track{}, err
	}
	t, err := domain.NewTrack(domain.NewTrackParams{
		ID: id, AlbumID: album.ID, ArtistIDs: artists, Title: cmd.Title, Duration: cmd.Duration,
		TrackNumber: cmd.TrackNumber, DiscNumber: cmd.DiscNumber, Explicit: cmd.Explicit, ISRC: cmd.ISRC,
	}, s.d.Now())
	if err != nil {
		return domain.Track{}, err
	}
	if err := s.d.Tracks.Create(ctx, t); err != nil {
		return domain.Track{}, fmt.Errorf("store track: %w", err)
	}
	s.publish(ctx, domain.TrackCreated{Track: t})
	return t, nil
}

// GetTrack returns a track by ID.
func (s *Service) GetTrack(ctx context.Context, id uuid.UUID) (domain.Track, error) {
	return s.d.Tracks.Get(ctx, id)
}

// UpdateTrack is the input of Service.UpdateTrack.
type UpdateTrack struct {
	ID      uuid.UUID
	Changes domain.TrackChanges
}

// UpdateTrack applies a partial update, including status transitions.
func (s *Service) UpdateTrack(ctx context.Context, cmd UpdateTrack) (domain.Track, error) {
	t, err := s.d.Tracks.Get(ctx, cmd.ID)
	if err != nil {
		return domain.Track{}, err
	}
	changed, err := t.Apply(cmd.Changes, s.d.Now())
	if err != nil {
		return domain.Track{}, err
	}
	if !changed {
		return t, nil
	}
	if err := s.d.Tracks.Update(ctx, t); err != nil {
		return domain.Track{}, fmt.Errorf("store track: %w", err)
	}
	s.publish(ctx, domain.TrackUpdated{Track: t})
	return t, nil
}

// ListGenres returns the curated genre list.
func (s *Service) ListGenres(ctx context.Context) ([]domain.Genre, error) {
	return s.d.Genres.List(ctx)
}
