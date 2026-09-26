package application

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/tehrelt/icyre/services/bff/internal/ports"
	"github.com/tehrelt/icyre/services/bff/internal/views"
)

// Playlist aggregates GET /api/v1/pages/playlists/{id}:
//
//	playlist ──┬─ tracks (Catalog batch)   (required)
//	           └─ owner name               (optional)
//	then the tracks' albums (parallel), artists in one batch and liked marks
//	(all optional: rows degrade instead of failing the page)
func (p *Pages) Playlist(ctx context.Context, id string) (views.PlaylistPage, error) {
	if p.playlists == nil {
		return views.PlaylistPage{}, ErrUpstream
	}
	ctx, cancel := context.WithTimeout(ctx, p.cfg.PageBudget)
	defer cancel()

	pl, err := p.playlists.GetPlaylist(ctx, id)
	if err != nil {
		return views.PlaylistPage{}, required(err, "playlist")
	}

	degraded := newDegraded()
	var (
		found []ports.Track
		owner string
	)
	g, gctx := errgroup.WithContext(ctx)
	if len(pl.TrackIDs) > 0 {
		g.Go(func() error {
			var err error
			found, err = p.catalog.Tracks(gctx, pl.TrackIDs)
			return required(err, "tracks")
		})
	}
	g.Go(func() error {
		owner = p.ownerNames(gctx, []string{pl.OwnerID}, degraded)[pl.OwnerID]
		return nil
	})
	if err := g.Wait(); err != nil {
		return views.PlaylistPage{}, err
	}

	tracks := inOrder(pl.TrackIDs, found)
	albums := p.albumsOf(ctx, tracks, degraded)
	names := p.artistNames(ctx, trackArtistIDs(tracks), degraded)
	liked := p.savedTracks(ctx, tracks, degraded)

	page := views.PlaylistPage{Tracks: make([]views.Track, 0, len(tracks))}
	var total time.Duration
	for _, t := range tracks {
		album, ok := albums[t.AlbumID]
		if !ok {
			album = ports.Album{ID: t.AlbumID}
		}
		v := trackView(t, album, names)
		v.Liked = liked[t.ID]
		page.Tracks = append(page.Tracks, v)
		total += t.Duration
	}
	page.Playlist = views.PlaylistHeader{
		ID: pl.ID, Title: pl.Title, OwnerID: pl.OwnerID, Owner: owner,
		TrackCount: len(tracks), DurationSec: seconds(total),
		UpdatedAt: pl.UpdatedAt.UTC().Format(time.RFC3339), Art: artIndex(pl.ID),
	}
	page.Unavailable = degraded.list()
	return page, nil
}

// inOrder arranges Catalog's tracks in playlist order, dropping the ones
// Catalog no longer has.
func inOrder(order []string, found []ports.Track) []ports.Track {
	byID := make(map[string]ports.Track, len(found))
	for _, t := range found {
		byID[t.ID] = t
	}
	out := make([]ports.Track, 0, len(found))
	for _, id := range order {
		if t, ok := byID[id]; ok {
			out = append(out, t)
		}
	}
	return out
}

// albumsOf loads the distinct albums of tracks in parallel. A missing album
// only blanks the album column of its rows.
func (p *Pages) albumsOf(ctx context.Context, tracks []ports.Track, degraded *degradedSet) map[string]ports.Album {
	ids := make([]string, 0, len(tracks))
	for _, t := range tracks {
		ids = append(ids, t.AlbumID)
	}
	var (
		mu     sync.Mutex
		out    = map[string]ports.Album{}
		failed error
	)
	g, gctx := errgroup.WithContext(ctx)
	for _, id := range dedupe(ids) {
		g.Go(func() error {
			a, err := p.catalog.GetAlbum(gctx, id)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				out[id] = a
			case !errors.Is(err, ports.ErrNotFound):
				failed = err
			}
			return nil
		})
	}
	_ = g.Wait()
	if failed != nil {
		degraded.add(ctx, p.log, "albums", failed)
	}
	return out
}

// ownerNames resolves playlist owners' display names. Unknown users stay
// blank; a User Profile failure degrades the "owners" section once.
func (p *Pages) ownerNames(ctx context.Context, ids []string, degraded *degradedSet) map[string]string {
	out := map[string]string{}
	if p.profiles == nil {
		return out
	}
	var (
		mu     sync.Mutex
		failed error
	)
	g, gctx := errgroup.WithContext(ctx)
	for _, id := range dedupe(ids) {
		g.Go(func() error {
			name, err := p.profiles.DisplayName(gctx, id)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				out[id] = name
			case !errors.Is(err, ports.ErrNotFound):
				failed = err
			}
			return nil
		})
	}
	_ = g.Wait()
	if failed != nil {
		degraded.add(ctx, p.log, "owners", failed)
	}
	return out
}

func playlistCard(pl ports.Playlist, owners map[string]string) views.PlaylistCard {
	return views.PlaylistCard{Kind: "playlist", ID: pl.ID, Title: pl.Title, Owner: owners[pl.OwnerID], TrackCount: len(pl.TrackIDs), Art: artIndex(pl.ID)}
}
