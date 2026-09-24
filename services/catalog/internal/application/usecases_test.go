package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/catalog/internal/domain"
	"github.com/tehrelt/icyre/services/catalog/internal/ports"
)

// In-memory fakes: use cases are tested without any infrastructure.

type memArtists map[uuid.UUID]domain.Artist

func (m memArtists) Create(_ context.Context, a domain.Artist) error { m[a.ID] = a; return nil }
func (m memArtists) Get(_ context.Context, id uuid.UUID) (domain.Artist, error) {
	a, ok := m[id]
	if !ok {
		return domain.Artist{}, domain.ErrArtistNotFound
	}
	return a, nil
}

type memAlbums struct {
	artists memArtists
	byID    map[uuid.UUID]domain.Album
}

func (m *memAlbums) Create(_ context.Context, a domain.Album) error {
	for _, id := range a.ArtistIDs {
		if _, ok := m.artists[id]; !ok {
			return &domain.ReferenceError{Field: "artistIds", Err: domain.ErrArtistNotFound}
		}
	}
	m.byID[a.ID] = a
	return nil
}

func (m *memAlbums) Get(_ context.Context, id uuid.UUID) (domain.Album, error) {
	a, ok := m.byID[id]
	if !ok {
		return domain.Album{}, domain.ErrAlbumNotFound
	}
	return a, nil
}

func (m *memAlbums) ListByArtist(_ context.Context, artistID uuid.UUID, after *ports.AlbumCursor, limit int) ([]domain.Album, error) {
	var out []domain.Album
	for _, a := range m.byID {
		for _, id := range a.ArtistIDs {
			if id == artistID {
				out = append(out, a)
			}
		}
	}
	less := func(a, b domain.Album) bool { // DESC by (date, id)
		if !a.ReleaseDate.Equal(b.ReleaseDate) {
			return a.ReleaseDate.After(b.ReleaseDate)
		}
		return a.ID.String() > b.ID.String()
	}
	sort.Slice(out, func(i, j int) bool { return less(out[i], out[j]) })
	if after != nil {
		cur := domain.Album{ReleaseDate: after.ReleaseDate, ID: after.ID}
		for len(out) > 0 && !less(cur, out[0]) {
			out = out[1:]
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type memTracks map[uuid.UUID]domain.Track

func (m memTracks) Create(_ context.Context, t domain.Track) error {
	for _, x := range m {
		if x.AlbumID == t.AlbumID && x.DiscNumber == t.DiscNumber && x.TrackNumber == t.TrackNumber {
			return domain.ErrTrackPositionTaken
		}
	}
	m[t.ID] = t
	return nil
}
func (m memTracks) Get(_ context.Context, id uuid.UUID) (domain.Track, error) {
	t, ok := m[id]
	if !ok {
		return domain.Track{}, domain.ErrTrackNotFound
	}
	return t, nil
}
func (m memTracks) Update(_ context.Context, t domain.Track) error { m[t.ID] = t; return nil }
func (m memTracks) ListByAlbum(_ context.Context, albumID uuid.UUID) ([]domain.Track, error) {
	var out []domain.Track
	for _, t := range m {
		if t.AlbumID == albumID {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TrackNumber < out[j].TrackNumber })
	return out, nil
}

type memGenres []domain.Genre

func (m memGenres) List(context.Context) ([]domain.Genre, error) { return m, nil }

type recordingPublisher struct {
	events []domain.Event
	err    error
}

func (p *recordingPublisher) Publish(_ context.Context, events ...domain.Event) error {
	p.events = append(p.events, events...)
	return p.err
}

type fixture struct {
	svc     *Service
	artists memArtists
	tracks  memTracks
	pub     *recordingPublisher
	clock   time.Time
}

func newFixture() *fixture {
	f := &fixture{artists: memArtists{}, tracks: memTracks{}, pub: &recordingPublisher{}, clock: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)}
	f.svc = New(Deps{
		Artists:   f.artists,
		Albums:    &memAlbums{artists: f.artists, byID: map[uuid.UUID]domain.Album{}},
		Tracks:    f.tracks,
		Genres:    memGenres{},
		Publisher: f.pub,
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:       func() time.Time { return f.clock },
	})
	return f
}

func (f *fixture) album(t *testing.T, date time.Time, artists ...uuid.UUID) domain.Album {
	t.Helper()
	a, err := f.svc.CreateAlbum(context.Background(), CreateAlbum{Title: "Prism Hours", Type: domain.AlbumTypeAlbum, ReleaseDate: date, ArtistIDs: artists})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestCreateTrackPublishesAndInheritsAlbumArtists(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	artist, err := f.svc.CreateArtist(ctx, CreateArtist{Name: "Nova Hale"})
	if err != nil {
		t.Fatal(err)
	}
	album := f.album(t, time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC), artist.ID)

	tr, err := f.svc.CreateTrack(ctx, CreateTrack{AlbumID: album.ID, Title: "Glass Tides", Duration: 227 * time.Second, TrackNumber: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.ArtistIDs) != 1 || tr.ArtistIDs[0] != artist.ID || tr.Status != domain.TrackStatusDraft {
		t.Fatalf("unexpected track %+v", tr)
	}
	if len(f.pub.events) != 3 {
		t.Fatalf("expected artist, album and track events, got %d", len(f.pub.events))
	}
	if ev, ok := f.pub.events[2].(domain.TrackCreated); !ok || ev.Track.ID != tr.ID {
		t.Fatalf("last event = %#v", f.pub.events[2])
	}
}

func TestCreateTrackForMissingAlbumIsReferenceError(t *testing.T) {
	f := newFixture()
	_, err := f.svc.CreateTrack(context.Background(), CreateTrack{AlbumID: uuid.New(), Title: "x", Duration: time.Second, TrackNumber: 1})
	var ref *domain.ReferenceError
	if !errors.As(err, &ref) || ref.Field != "albumId" || !errors.Is(err, domain.ErrAlbumNotFound) {
		t.Fatalf("expected albumId reference error, got %v", err)
	}
	if len(f.pub.events) != 0 {
		t.Fatal("nothing must be published on failure")
	}
}

func TestCreateTrackRejectsTakenPosition(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	artist, _ := f.svc.CreateArtist(ctx, CreateArtist{Name: "Nova Hale"})
	album := f.album(t, time.Now(), artist.ID)
	cmd := CreateTrack{AlbumID: album.ID, Title: "One", Duration: time.Minute, TrackNumber: 1}
	if _, err := f.svc.CreateTrack(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateTrack(ctx, cmd); !errors.Is(err, domain.ErrTrackPositionTaken) {
		t.Fatalf("expected ErrTrackPositionTaken, got %v", err)
	}
}

func TestPublishFailureDoesNotFailTheWrite(t *testing.T) {
	f := newFixture()
	f.pub.err = errors.New("broker down")
	a, err := f.svc.CreateArtist(context.Background(), CreateArtist{Name: "Kai Frost"})
	if err != nil {
		t.Fatalf("write must succeed when publishing fails: %v", err)
	}
	if _, ok := f.artists[a.ID]; !ok {
		t.Fatal("artist was not stored")
	}
}

func TestUpdateTrackPublishesOnlyOnChange(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	artist, _ := f.svc.CreateArtist(ctx, CreateArtist{Name: "Nova Hale"})
	album := f.album(t, time.Now(), artist.ID)
	tr, _ := f.svc.CreateTrack(ctx, CreateTrack{AlbumID: album.ID, Title: "One", Duration: time.Minute, TrackNumber: 1})
	published := len(f.pub.events)

	processing := domain.TrackStatusProcessing
	f.clock = f.clock.Add(time.Minute)
	got, err := f.svc.UpdateTrack(ctx, UpdateTrack{ID: tr.ID, Changes: domain.TrackChanges{Status: &processing}})
	if err != nil || got.Status != processing {
		t.Fatalf("got %+v, err %v", got, err)
	}
	if len(f.pub.events) != published+1 {
		t.Fatal("expected one track.updated event")
	}
	if _, ok := f.pub.events[len(f.pub.events)-1].(domain.TrackUpdated); !ok {
		t.Fatal("last event is not TrackUpdated")
	}

	if _, err := f.svc.UpdateTrack(ctx, UpdateTrack{ID: tr.ID, Changes: domain.TrackChanges{Status: &processing}}); err != nil {
		t.Fatal(err)
	}
	if len(f.pub.events) != published+1 {
		t.Fatal("no-op update must not publish")
	}

	ready := domain.TrackStatusBlocked
	if _, err := f.svc.UpdateTrack(ctx, UpdateTrack{ID: tr.ID, Changes: domain.TrackChanges{Status: &ready}}); !errors.Is(err, domain.ErrInvalidStatusTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
}

func TestListArtistAlbumsPaginates(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	artist, _ := f.svc.CreateArtist(ctx, CreateArtist{Name: "Nova Hale"})
	for i := 0; i < 5; i++ {
		f.album(t, time.Date(2020+i, 1, 1, 0, 0, 0, 0, time.UTC), artist.ID)
	}

	var years []int
	var after *ports.AlbumCursor
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("pagination does not terminate")
		}
		page, err := f.svc.ListArtistAlbums(ctx, ListArtistAlbums{ArtistID: artist.ID, After: after, Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range page.Albums {
			years = append(years, a.ReleaseDate.Year())
		}
		if page.Next == nil {
			break
		}
		after = page.Next
	}
	want := []int{2024, 2023, 2022, 2021, 2020}
	if len(years) != len(want) {
		t.Fatalf("years = %v", years)
	}
	for i := range want {
		if years[i] != want[i] {
			t.Fatalf("years = %v, want %v", years, want)
		}
	}

	if _, err := f.svc.ListArtistAlbums(ctx, ListArtistAlbums{ArtistID: uuid.New()}); !errors.Is(err, domain.ErrArtistNotFound) {
		t.Fatalf("expected ErrArtistNotFound, got %v", err)
	}
}
