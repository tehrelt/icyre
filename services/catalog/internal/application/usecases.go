package application

import (
	"context"
	"errors"
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
	if err := s.store(ctx, func(ctx context.Context) error { return s.d.Artists.Create(ctx, a) }, domain.ArtistCreated{Artist: a}); err != nil {
		return domain.Artist{}, fmt.Errorf("store artist: %w", err)
	}
	return a, nil
}

// GetArtist returns an artist by ID.
func (s *Service) GetArtist(ctx context.Context, id uuid.UUID) (domain.Artist, error) {
	return s.d.Artists.Get(ctx, id)
}

// MaxBatchIDs bounds batch lookups.
const MaxBatchIDs = 100

// ListArtists returns the existing artists among ids (batch lookup for
// aggregators such as the Web BFF). Unknown IDs are skipped.
func (s *Service) ListArtists(ctx context.Context, ids []uuid.UUID) ([]domain.Artist, error) {
	if len(ids) > MaxBatchIDs {
		return nil, &domain.ValidationError{Fields: map[string]string{"ids": "at most 100 IDs per request"}}
	}
	if len(ids) == 0 {
		return []domain.Artist{}, nil
	}
	artists, err := s.d.Artists.ListByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list artists: %w", err)
	}
	return artists, nil
}

// ListTracks returns the live tracks among ids (batch lookup for aggregators,
// e.g. a playlist page). Unknown and deleted IDs are skipped.
func (s *Service) ListTracks(ctx context.Context, ids []uuid.UUID) ([]domain.Track, error) {
	if len(ids) > MaxBatchIDs {
		return nil, &domain.ValidationError{Fields: map[string]string{"ids": "at most 100 IDs per request"}}
	}
	if len(ids) == 0 {
		return []domain.Track{}, nil
	}
	tracks, err := s.d.Tracks.ListByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list tracks: %w", err)
	}
	return tracks, nil
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
	return pageAlbums(q.Limit, func(limit int) ([]domain.Album, error) {
		return s.d.Albums.ListByArtist(ctx, q.ArtistID, q.After, limit)
	})
}

// ListAlbums is the input of Service.ListAlbums.
type ListAlbums struct {
	After *ports.AlbumCursor
	Limit int
}

// ListAlbums pages through the whole catalogue, newest releases first.
func (s *Service) ListAlbums(ctx context.Context, q ListAlbums) (AlbumPage, error) {
	return pageAlbums(q.Limit, func(limit int) ([]domain.Album, error) {
		return s.d.Albums.List(ctx, q.After, limit)
	})
}

// pageAlbums applies page-size limits and computes the next cursor by
// fetching one extra row.
func pageAlbums(limit int, fetch func(limit int) ([]domain.Album, error)) (AlbumPage, error) {
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}
	albums, err := fetch(limit + 1)
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
	if err := s.store(ctx, func(ctx context.Context) error { return s.d.Albums.Create(ctx, a) }, domain.AlbumCreated{Album: a}); err != nil {
		return domain.Album{}, fmt.Errorf("store album: %w", err)
	}
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
	if err := s.store(ctx, func(ctx context.Context) error { return s.d.Tracks.Create(ctx, t) }, domain.TrackCreated{Track: t}); err != nil {
		return domain.Track{}, fmt.Errorf("store track: %w", err)
	}
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
// Transitions into or out of BLOCKED are rejected; moderation uses ModerateTrack.
func (s *Service) UpdateTrack(ctx context.Context, cmd UpdateTrack) (domain.Track, error) {
	return s.modifyTrack(ctx, cmd.ID, func(t *domain.Track) (bool, error) {
		if t.Status == domain.TrackStatusDeleted {
			return false, domain.ErrTrackDeleted
		}
		if cmd.Changes.Status != nil {
			next := *cmd.Changes.Status
			if next == domain.TrackStatusBlocked || (t.Status == domain.TrackStatusBlocked && next != t.Status) {
				return false, fmt.Errorf("%w: %s → %s", domain.ErrInvalidStatusTransition, t.Status, next)
			}
		}
		return t.Apply(cmd.Changes, s.d.Now())
	})
}

// ListTracksPage returns a keyset-paginated slice of tracks ordered by
// (created_at DESC, id DESC), excluding DELETED unless status == DELETED.
func (s *Service) ListTracksPage(ctx context.Context, status *domain.TrackStatus, afterCreatedAt *time.Time, afterID *uuid.UUID, limit int) ([]domain.Track, error) {
	if status != nil && !status.Valid() {
		return nil, &domain.ValidationError{Fields: map[string]string{"status": "unknown status"}}
	}
	if (afterCreatedAt == nil) != (afterID == nil) {
		return nil, &domain.ValidationError{Fields: map[string]string{"cursor": "afterCreatedAt and afterID must both be set"}}
	}
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize+1 {
		limit = MaxPageSize + 1
	}
	tracks, err := s.d.Tracks.ListTracksPage(ctx, status, afterCreatedAt, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list tracks page: %w", err)
	}
	return tracks, nil
}

type trackCacheInvalidator interface {
	Invalidate(ctx context.Context, trackID, albumID uuid.UUID)
}

// ModerateTrack idempotently blocks or unblocks a track by operationID.
func (s *Service) ModerateTrack(ctx context.Context, operationID, trackID uuid.UUID, blocked bool) (domain.ModerationReceipt, error) {
	if err := validateModerationIDs(operationID, trackID); err != nil {
		return domain.ModerationReceipt{}, err
	}
	var (
		out     domain.ModerationReceipt
		albumID uuid.UUID
	)
	err := s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		rec, found, err := s.findModerationReceipt(ctx, operationID, trackID, blocked)
		if err != nil {
			return err
		}
		if found {
			out = rec
			return nil
		}
		rec, albumID, err = s.applyModeration(ctx, operationID, trackID, blocked)
		if err != nil {
			return err
		}
		out = rec
		return nil
	})
	if err != nil {
		return domain.ModerationReceipt{}, err
	}
	if out.Changed {
		if inv, ok := s.d.Tracks.(trackCacheInvalidator); ok {
			inv.Invalidate(ctx, out.TrackID, albumID)
		}
	}
	return out, nil
}

func (s *Service) applyModeration(ctx context.Context, operationID, trackID uuid.UUID, blocked bool) (domain.ModerationReceipt, uuid.UUID, error) {
	t, err := s.d.Tracks.GetForUpdate(ctx, trackID)
	if err != nil {
		return domain.ModerationReceipt{}, uuid.Nil, err
	}
	if rec, found, err := s.findModerationReceipt(ctx, operationID, trackID, blocked); err != nil || found {
		return rec, t.AlbumID, err
	}
	prev := t.Status
	now := s.d.Now().UTC()
	changed, err := t.ModerateBlock(blocked, now)
	if err != nil {
		return domain.ModerationReceipt{}, uuid.Nil, err
	}
	if changed {
		if err := s.d.Tracks.Update(ctx, t); err != nil {
			return domain.ModerationReceipt{}, uuid.Nil, fmt.Errorf("store track: %w", err)
		}
		if err := s.d.Publisher.Publish(ctx, domain.TrackUpdated{Track: t}); err != nil {
			return domain.ModerationReceipt{}, uuid.Nil, fmt.Errorf("record events: %w", err)
		}
	}
	rec := domain.ModerationReceipt{
		OperationID: operationID, TrackID: t.ID,
		PreviousStatus: prev, CurrentStatus: t.Status, Changed: changed, CreatedAt: now,
	}
	if err := s.d.Tracks.SaveModerationReceipt(ctx, rec); err != nil {
		return domain.ModerationReceipt{}, uuid.Nil, fmt.Errorf("save moderation receipt: %w", err)
	}
	return rec, t.AlbumID, nil
}

func (s *Service) findModerationReceipt(ctx context.Context, operationID, trackID uuid.UUID, blocked bool) (domain.ModerationReceipt, bool, error) {
	rec, err := s.d.Tracks.GetModerationReceipt(ctx, operationID)
	if errors.Is(err, domain.ErrModerationReceiptNotFound) {
		return domain.ModerationReceipt{}, false, nil
	}
	if err != nil {
		return domain.ModerationReceipt{}, false, fmt.Errorf("get moderation receipt: %w", err)
	}
	if rec.TrackID != trackID || blocked != (rec.CurrentStatus == domain.TrackStatusBlocked) {
		return domain.ModerationReceipt{}, false, domain.ErrModerationConflict
	}
	return rec, true, nil
}

func validateModerationIDs(operationID, trackID uuid.UUID) error {
	fields := map[string]string{}
	if operationID == uuid.Nil {
		fields["operationId"] = "must be set"
	}
	if trackID == uuid.Nil {
		fields["trackId"] = "must be set"
	}
	if len(fields) > 0 {
		return &domain.ValidationError{Fields: fields}
	}
	return nil
}

// AdvanceTrackMedia applies a media pipeline signal (media.events) to the
// track status. Redelivered and out-of-date signals change nothing and
// publish nothing (domain.Track.AdvanceMedia).
func (s *Service) AdvanceTrackMedia(ctx context.Context, id uuid.UUID, stage domain.MediaStage) (domain.Track, error) {
	return s.modifyTrack(ctx, id, func(t *domain.Track) (bool, error) {
		return t.AdvanceMedia(stage, s.d.Now()), nil
	})
}

// modifyTrack is a read-modify-write of one track under a row lock; a change
// is stored together with its track.updated event.
func (s *Service) modifyTrack(ctx context.Context, id uuid.UUID, modify func(t *domain.Track) (bool, error)) (domain.Track, error) {
	var out domain.Track
	err := s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		t, err := s.d.Tracks.GetForUpdate(ctx, id)
		if err != nil {
			return err
		}
		changed, err := modify(&t)
		if err != nil {
			return err
		}
		out = t
		if !changed {
			return nil
		}
		if err := s.d.Tracks.Update(ctx, t); err != nil {
			return fmt.Errorf("store track: %w", err)
		}
		if err := s.d.Publisher.Publish(ctx, domain.TrackUpdated{Track: t}); err != nil {
			return fmt.Errorf("record events: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Track{}, err
	}
	return out, nil
}

// ListGenres returns the curated genre list.
func (s *Service) ListGenres(ctx context.Context) ([]domain.Genre, error) {
	return s.d.Genres.List(ctx)
}
