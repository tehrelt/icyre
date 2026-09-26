package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/search"
)

// Playlist is what the indexer reads from the Playlist Service.
type Playlist struct {
	ID         string
	OwnerID    string
	Title      string
	TrackCount int
	UpdatedAt  time.Time
}

// Playlists is the source of truth for the playlists index.
type Playlists interface {
	// Playlist reads one playlist (ErrNotFound when it is gone).
	Playlist(ctx context.Context, id string) (Playlist, error)
	EachPlaylist(ctx context.Context, fn func(Playlist) error) error
}

// Profiles resolves owners' display names; unknown users are absent.
type Profiles interface {
	DisplayNames(ctx context.Context, userIDs []string) (map[string]string, error)
}

// PlaylistChanged handles every playlist.* event except playlist.deleted.
// Payloads only name the playlist: the document is rebuilt from the Playlist
// Service, so the event order inside one version does not matter. A playlist
// that is gone by now is removed.
func (x *Indexer) PlaylistChanged(ctx context.Context, id string, at time.Time) error {
	p, err := x.playlists.Playlist(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return x.PlaylistDeleted(ctx, id, at)
	}
	if err != nil {
		return fmt.Errorf("playlist %s: %w", id, err)
	}
	owners, err := x.profiles.DisplayNames(ctx, []string{p.OwnerID})
	if err != nil {
		return err
	}
	doc := playlistDoc(p, owners)
	return x.index.Apply(ctx, []Write{{Index: search.AliasPlaylists, ID: doc.ID, Version: version(at), Doc: doc}})
}

// PlaylistDeleted handles playlist.deleted.
func (x *Indexer) PlaylistDeleted(ctx context.Context, id string, at time.Time) error {
	return x.index.Apply(ctx, []Write{{Index: search.AliasPlaylists, ID: id, Version: version(at), Delete: true}})
}

// rebuildPlaylists adds every playlist to the playlists index of a rebuild.
func (x *Indexer) rebuildPlaylists(ctx context.Context, index string, add func(Write) error) (int, error) {
	n := 0
	owners := map[string]string{}
	err := x.playlists.EachPlaylist(ctx, func(p Playlist) error {
		if _, ok := owners[p.OwnerID]; !ok {
			names, err := x.profiles.DisplayNames(ctx, []string{p.OwnerID})
			if err != nil {
				return err
			}
			owners[p.OwnerID] = names[p.OwnerID]
		}
		doc := playlistDoc(p, owners)
		n++
		return add(Write{Index: index, ID: doc.ID, Version: version(p.UpdatedAt), Doc: doc})
	})
	return n, err
}

func playlistDoc(p Playlist, owners map[string]string) search.Playlist {
	return search.Playlist{ID: p.ID, Title: p.Title, OwnerName: owners[p.OwnerID], TrackCount: p.TrackCount, UpdatedAt: p.UpdatedAt.UTC()}
}
