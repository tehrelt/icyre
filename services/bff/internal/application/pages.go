// Package application aggregates upstream services into page contracts:
// parallel fan-out inside a per-page time budget, with optional sections
// degrading instead of failing the page.
package application

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/tehrelt/icyre/services/bff/internal/ports"
	"github.com/tehrelt/icyre/services/bff/internal/views"
)

// ErrNotFound means the page's primary resource does not exist.
var ErrNotFound = errors.New("page resource not found")

// ErrUpstream means a required upstream call failed.
var ErrUpstream = errors.New("upstream unavailable")

// Config tunes aggregation.
type Config struct {
	// PageBudget bounds the whole aggregation of one page.
	PageBudget time.Duration
	// GenreCacheTTL keeps the (small, rarely changing) genre list in memory.
	GenreCacheTTL time.Duration
	// NewReleases is the size of the Home "New releases" shelf.
	NewReleases int
	// MoreByArtist is the size of the album page "More by" shelf.
	MoreByArtist int
	// RecentlyPlayed is the size of the Home "Recently played" row.
	RecentlyPlayed int
}

// Pages builds page view models.
type Pages struct {
	catalog ports.Catalog
	library ports.Library // nil: no personalization
	history ports.History // nil: no "Recently played"
	// playlists: nil disables the playlist page and playlists in "Recently
	// played"; profiles: nil leaves owner names blank.
	playlists ports.Playlists
	profiles  ports.Profiles
	cfg       Config
	log       *slog.Logger
	now       func() time.Time

	genreMu      sync.Mutex
	genres       map[string]ports.Genre
	genresExpire time.Time
}

// Personal holds the per-listener upstreams; nil members switch the
// matching page features off.
type Personal struct {
	Library   ports.Library
	History   ports.History
	Playlists ports.Playlists
	Profiles  ports.Profiles
}

// New returns a Pages service.
func New(catalog ports.Catalog, personal Personal, cfg Config, log *slog.Logger) *Pages {
	if cfg.PageBudget <= 0 {
		cfg.PageBudget = time.Second
	}
	if cfg.GenreCacheTTL <= 0 {
		cfg.GenreCacheTTL = 10 * time.Minute
	}
	if cfg.NewReleases <= 0 {
		cfg.NewReleases = 6
	}
	if cfg.MoreByArtist <= 0 {
		cfg.MoreByArtist = 6
	}
	if cfg.RecentlyPlayed <= 0 {
		cfg.RecentlyPlayed = 6
	}
	return &Pages{
		catalog: catalog, library: personal.Library, history: personal.History,
		playlists: personal.Playlists, profiles: personal.Profiles, cfg: cfg, log: log, now: time.Now,
	}
}

// Album aggregates GET /api/v1/pages/albums/{id}:
//
//	album ──┬─ tracks            (required)
//	        ├─ more by artist     (optional)
//	        └─ genres → tags      (optional, cached)
//	then artists of album + tracks in one batch (optional: names degrade)
func (p *Pages) Album(ctx context.Context, id string) (views.AlbumPage, error) {
	ctx, cancel := context.WithTimeout(ctx, p.cfg.PageBudget)
	defer cancel()

	album, err := p.catalog.GetAlbum(ctx, id)
	if err != nil {
		return views.AlbumPage{}, required(err, "album")
	}

	var (
		tracks   []ports.Track
		more     []ports.Album
		genres   map[string]ports.Genre
		degraded = newDegraded()
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		tracks, err = p.catalog.AlbumTracks(gctx, album.ID)
		return required(err, "tracks")
	})
	if len(album.ArtistIDs) > 0 {
		g.Go(func() error {
			res, err := p.catalog.ArtistAlbums(gctx, album.ArtistIDs[0], p.cfg.MoreByArtist+1)
			if err != nil {
				degraded.add(gctx, p.log, "moreByArtist", err)
				return nil
			}
			more = res
			return nil
		})
	}
	g.Go(func() error {
		res, err := p.genreIndex(gctx)
		if err != nil {
			degraded.add(gctx, p.log, "tags", err)
			return nil
		}
		genres = res
		return nil
	})
	if err := g.Wait(); err != nil {
		return views.AlbumPage{}, err
	}

	names := p.artistNames(ctx, append(append([]string{}, album.ArtistIDs...), trackArtistIDs(tracks)...), degraded)
	liked := p.savedTracks(ctx, tracks, degraded)

	page := views.AlbumPage{
		Album:        p.albumHeader(album, tracks, genres),
		Artist:       artistRef(album, names),
		Tracks:       make([]views.Track, 0, len(tracks)),
		MoreByArtist: []views.AlbumCard{},
	}
	for _, t := range tracks {
		v := trackView(t, album, names)
		v.Liked = liked[t.ID]
		page.Tracks = append(page.Tracks, v)
	}
	for _, a := range more {
		if a.ID == album.ID || len(page.MoreByArtist) == p.cfg.MoreByArtist {
			continue
		}
		card := albumCard(a, names)
		card.ArtistName = page.Artist.Name
		page.MoreByArtist = append(page.MoreByArtist, card)
	}
	page.Unavailable = degraded.list()
	return page, nil
}

// Home aggregates GET /api/v1/pages/home. Only "New releases" has a source
// today (Catalog); the other sections wait for their services.
func (p *Pages) Home(ctx context.Context) (views.HomePage, error) {
	ctx, cancel := context.WithTimeout(ctx, p.cfg.PageBudget)
	defer cancel()

	degraded := newDegraded()
	for _, s := range []string{"albumOfTheWeek", "recommended", "trending", "madeForYou", "followedArtists"} {
		degraded.set(s) // no upstream service yet
	}
	if p.history == nil {
		degraded.set("recentlyPlayed")
	}

	page := views.HomePage{
		RecentlyPlayed:  []any{},
		Recommended:     views.Shelf[any]{Items: []any{}},
		NewReleases:     views.Shelf[views.AlbumCard]{Kicker: "Out this week", Items: []views.AlbumCard{}},
		Trending:        views.Trending{Today: []views.Track{}, Week: []views.Track{}},
		MadeForYou:      views.MadeForYou{Playlists: []any{}},
		FollowedArtists: []any{},
	}

	// "New releases" and "Recently played" load in parallel; both are optional.
	var (
		latest    []ports.Album
		latestErr error
		recent    []recentItem
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		latest, latestErr = p.catalog.LatestAlbums(gctx, p.cfg.NewReleases)
		return nil
	})
	g.Go(func() error {
		recent = p.recentItems(gctx, degraded)
		return nil
	})
	_ = g.Wait()

	var ids, owners []string
	for _, a := range latest {
		ids = append(ids, a.ArtistIDs...)
	}
	for _, it := range recent {
		if it.album != nil {
			ids = append(ids, it.album.ArtistIDs...)
		} else {
			owners = append(owners, it.playlist.OwnerID)
		}
	}
	names := p.artistNames(ctx, ids, degraded)
	ownerNames := p.ownerNames(ctx, owners, degraded)
	for _, it := range recent {
		if it.album != nil {
			page.RecentlyPlayed = append(page.RecentlyPlayed, albumCard(*it.album, names))
		} else {
			page.RecentlyPlayed = append(page.RecentlyPlayed, playlistCard(*it.playlist, ownerNames))
		}
	}

	if latestErr != nil {
		degraded.add(ctx, p.log, "newReleases", latestErr)
	} else {
		weekAgo := p.now().AddDate(0, 0, -7)
		for _, a := range latest {
			card := albumCard(a, names)
			if !a.ReleaseDate.Before(weekAgo) && !a.ReleaseDate.After(p.now()) {
				card.Badge = "new"
			}
			page.NewReleases.Items = append(page.NewReleases.Items, card)
		}
	}
	page.Unavailable = degraded.list()
	return page, nil
}

// recentItem is one "Recently played" entry: an album or a playlist.
type recentItem struct {
	album    *ports.Album
	playlist *ports.Playlist
}

// recentItems resolves the listener's recently played sources in History
// order: albums from Catalog, playlists from the Playlist Service. Other
// kinds (artists) wait for their pages; sources that no longer exist are
// skipped; a History failure degrades the row.
func (p *Pages) recentItems(ctx context.Context, degraded *degradedSet) []recentItem {
	if p.history == nil || ports.UserToken(ctx) == "" {
		return nil
	}
	sources, err := p.history.RecentSources(ctx, p.cfg.RecentlyPlayed)
	if err != nil {
		degraded.add(ctx, p.log, "recentlyPlayed", err)
		return nil
	}
	items := make([]recentItem, len(sources))
	g, gctx := errgroup.WithContext(ctx)
	for i, s := range sources {
		kind, id, _ := strings.Cut(s, ":")
		g.Go(func() error {
			switch {
			case kind == "album":
				a, err := p.catalog.GetAlbum(gctx, id)
				if err == nil {
					items[i].album = &a
				} else if !errors.Is(err, ports.ErrNotFound) {
					p.log.WarnContext(gctx, "recently played album unavailable", "album_id", id, "error", err)
				}
			case kind == "playlist" && p.playlists != nil:
				pl, err := p.playlists.GetPlaylist(gctx, id)
				if err == nil {
					items[i].playlist = &pl
				} else if !errors.Is(err, ports.ErrNotFound) {
					p.log.WarnContext(gctx, "recently played playlist unavailable", "playlist_id", id, "error", err)
				}
			}
			return nil
		})
	}
	_ = g.Wait()
	out := make([]recentItem, 0, len(items))
	for _, it := range items {
		if it.album != nil || it.playlist != nil {
			out = append(out, it)
		}
	}
	return out
}

// artistNames resolves artist IDs in one batch. On failure names degrade to
// "Unknown artist" instead of failing the page.
func (p *Pages) artistNames(ctx context.Context, ids []string, degraded *degradedSet) map[string]string {
	uniq := dedupe(ids)
	names := make(map[string]string, len(uniq))
	if len(uniq) == 0 {
		return names
	}
	artists, err := p.catalog.Artists(ctx, uniq)
	if err != nil {
		degraded.add(ctx, p.log, "artistNames", err)
		return names
	}
	for _, a := range artists {
		names[a.ID] = a.Name
	}
	return names
}

// savedTracks marks the listener's liked tracks. Anonymous visitors have
// none; a Library failure degrades to "not liked" instead of failing the page.
func (p *Pages) savedTracks(ctx context.Context, tracks []ports.Track, degraded *degradedSet) map[string]bool {
	if p.library == nil || ports.UserToken(ctx) == "" || len(tracks) == 0 {
		return nil
	}
	ids := make([]string, len(tracks))
	for i, t := range tracks {
		ids[i] = t.ID
	}
	saved, err := p.library.SavedTracks(ctx, ids)
	if err != nil {
		degraded.add(ctx, p.log, "liked", err)
		return nil
	}
	return saved
}

func (p *Pages) genreIndex(ctx context.Context) (map[string]ports.Genre, error) {
	p.genreMu.Lock()
	if p.genres != nil && p.now().Before(p.genresExpire) {
		g := p.genres
		p.genreMu.Unlock()
		return g, nil
	}
	p.genreMu.Unlock()

	list, err := p.catalog.Genres(ctx)
	if err != nil {
		return nil, err
	}
	idx := make(map[string]ports.Genre, len(list))
	for _, g := range list {
		idx[g.ID] = g
	}
	p.genreMu.Lock()
	p.genres, p.genresExpire = idx, p.now().Add(p.cfg.GenreCacheTTL)
	p.genreMu.Unlock()
	return idx, nil
}

func (p *Pages) albumHeader(a ports.Album, tracks []ports.Track, genres map[string]ports.Genre) views.AlbumHeader {
	var total time.Duration
	for _, t := range tracks {
		total += t.Duration
	}
	tags := []string{}
	for _, id := range a.GenreIDs {
		if g, ok := genres[id]; ok {
			tags = append(tags, g.Name)
		}
	}
	return views.AlbumHeader{
		ID:          a.ID,
		Title:       a.Title,
		AlbumType:   a.AlbumType,
		Year:        a.ReleaseDate.Year(),
		ReleaseDate: a.ReleaseDate.Format(time.DateOnly),
		TrackCount:  len(tracks),
		DurationSec: seconds(total),
		Tags:        tags,
		Art:         artIndex(a.ID),
	}
}

func artistRef(a ports.Album, names map[string]string) views.ArtistRef {
	if len(a.ArtistIDs) == 0 {
		return views.ArtistRef{Name: "Various artists"}
	}
	id := a.ArtistIDs[0]
	return views.ArtistRef{ID: id, Name: nameOr(names, id), Art: artIndex(id)}
}

func albumCard(a ports.Album, names map[string]string) views.AlbumCard {
	return views.AlbumCard{
		Kind:       "album",
		ID:         a.ID,
		Title:      a.Title,
		ArtistName: joinNames(a.ArtistIDs, names),
		Year:       a.ReleaseDate.Year(),
		Art:        artIndex(a.ID),
		AlbumType:  a.AlbumType,
	}
}

func trackView(t ports.Track, a ports.Album, names map[string]string) views.Track {
	return views.Track{
		ID:          t.ID,
		Title:       t.Title,
		ArtistName:  joinNames(t.ArtistIDs, names),
		AlbumID:     a.ID,
		AlbumTitle:  a.Title,
		DurationSec: seconds(t.Duration),
		Explicit:    t.Explicit,
		Art:         artIndex(a.ID),
		// Only READY tracks have audio; DRAFT/PROCESSING/BLOCKED are shown but not playable.
		Available: t.Status == "READY",
	}
}

// required maps an upstream failure of a mandatory call.
func required(err error, what string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ports.ErrNotFound):
		return fmt.Errorf("%s: %w", what, ErrNotFound)
	default:
		return fmt.Errorf("%s: %w: %w", what, ErrUpstream, err)
	}
}

// artIndex picks a stable Design System fallback cover (0–7) for an entity
// without artwork, so the same album always looks the same.
func artIndex(id string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return int(h.Sum32() % 8)
}

func seconds(d time.Duration) int { return int(math.Round(d.Seconds())) }

func nameOr(names map[string]string, id string) string {
	if n, ok := names[id]; ok {
		return n
	}
	return "Unknown artist"
}

func joinNames(ids []string, names map[string]string) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, nameOr(names, id))
	}
	if len(parts) == 0 {
		return "Unknown artist"
	}
	return strings.Join(parts, " · ")
}

func trackArtistIDs(tracks []ports.Track) []string {
	var ids []string
	for _, t := range tracks {
		ids = append(ids, t.ArtistIDs...)
	}
	return ids
}

func dedupe(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok || id == "" {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// degradedSet collects sections that were left empty.
type degradedSet struct {
	mu    sync.Mutex
	names []string
}

func newDegraded() *degradedSet { return &degradedSet{} }

func (d *degradedSet) set(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.names = append(d.names, name)
}

func (d *degradedSet) add(ctx context.Context, log *slog.Logger, name string, err error) {
	log.WarnContext(ctx, "page section degraded", "section", name, "error", err)
	d.set(name)
}

func (d *degradedSet) list() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.names...)
}
