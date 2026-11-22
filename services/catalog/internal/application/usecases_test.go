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

func (m memArtists) ListByIDs(_ context.Context, ids []uuid.UUID) ([]domain.Artist, error) {
	var out []domain.Artist
	for _, id := range ids {
		if a, ok := m[id]; ok {
			out = append(out, a)
		}
	}
	return out, nil
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

func (m *memAlbums) List(ctx context.Context, after *ports.AlbumCursor, limit int) ([]domain.Album, error) {
	return m.ListByArtist(ctx, uuid.Nil, after, limit)
}

// ListByArtist filters by artist; uuid.Nil means every album.
func (m *memAlbums) ListByArtist(_ context.Context, artistID uuid.UUID, after *ports.AlbumCursor, limit int) ([]domain.Album, error) {
	var out []domain.Album
	for _, a := range m.byID {
		if artistID == uuid.Nil {
			out = append(out, a)
			continue
		}
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

type memTracks struct {
	byID     map[uuid.UUID]domain.Track
	receipts map[uuid.UUID]domain.ModerationReceipt
}

func newMemTracks() *memTracks {
	return &memTracks{
		byID:     map[uuid.UUID]domain.Track{},
		receipts: map[uuid.UUID]domain.ModerationReceipt{},
	}
}

func (m *memTracks) Create(_ context.Context, t domain.Track) error {
	for _, x := range m.byID {
		if x.AlbumID == t.AlbumID && x.DiscNumber == t.DiscNumber && x.TrackNumber == t.TrackNumber {
			return domain.ErrTrackPositionTaken
		}
	}
	m.byID[t.ID] = t
	return nil
}
func (m *memTracks) Get(_ context.Context, id uuid.UUID) (domain.Track, error) {
	t, ok := m.byID[id]
	if !ok {
		return domain.Track{}, domain.ErrTrackNotFound
	}
	return t, nil
}
func (m *memTracks) ListByIDs(_ context.Context, ids []uuid.UUID) ([]domain.Track, error) {
	var out []domain.Track
	for _, id := range ids {
		if t, ok := m.byID[id]; ok && t.Status != domain.TrackStatusDeleted {
			out = append(out, t)
		}
	}
	return out, nil
}
func (m *memTracks) GetForUpdate(ctx context.Context, id uuid.UUID) (domain.Track, error) {
	return m.Get(ctx, id)
}
func (m *memTracks) Update(_ context.Context, t domain.Track) error { m.byID[t.ID] = t; return nil }
func (m *memTracks) ListByAlbum(_ context.Context, albumID uuid.UUID) ([]domain.Track, error) {
	var out []domain.Track
	for _, t := range m.byID {
		if t.AlbumID == albumID {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TrackNumber < out[j].TrackNumber })
	return out, nil
}
func (m *memTracks) ListTracksPage(_ context.Context, status *domain.TrackStatus, afterCreatedAt *time.Time, afterID *uuid.UUID, limit int) ([]domain.Track, error) {
	var out []domain.Track
	for _, t := range m.byID {
		if status == nil && t.Status == domain.TrackStatusDeleted {
			continue
		}
		if status != nil && t.Status != *status {
			continue
		}
		out = append(out, t)
	}
	less := func(a, b domain.Track) bool {
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.After(b.CreatedAt)
		}
		return a.ID.String() > b.ID.String()
	}
	sort.Slice(out, func(i, j int) bool { return less(out[i], out[j]) })
	if afterCreatedAt != nil && afterID != nil {
		cur := domain.Track{CreatedAt: *afterCreatedAt, ID: *afterID}
		for len(out) > 0 && !less(cur, out[0]) {
			out = out[1:]
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (m *memTracks) GetModerationReceipt(_ context.Context, operationID uuid.UUID) (domain.ModerationReceipt, error) {
	rec, ok := m.receipts[operationID]
	if !ok {
		return domain.ModerationReceipt{}, domain.ErrModerationReceiptNotFound
	}
	return rec, nil
}
func (m *memTracks) SaveModerationReceipt(_ context.Context, rec domain.ModerationReceipt) error {
	m.receipts[rec.OperationID] = rec
	return nil
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
	tracks  *memTracks
	pub     *recordingPublisher
	clock   time.Time
}

func newFixture() *fixture {
	f := &fixture{artists: memArtists{}, tracks: newMemTracks(), pub: &recordingPublisher{}, clock: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)}
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

// spyTx records units of work and whether they ended in an error (a
// rollback for the real Transactor).
type spyTx struct {
	runs, rolledBack int
	inside           bool
}

func (s *spyTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	s.runs++
	s.inside = true
	defer func() { s.inside = false }()
	err := fn(ctx)
	if err != nil {
		s.rolledBack++
	}
	return err
}

// insideTx fails a publish that happens outside the unit of work.
type insideTx struct {
	tx  *spyTx
	err error
}

func (p insideTx) Publish(context.Context, ...domain.Event) error {
	if !p.tx.inside {
		return errors.New("event recorded outside the transaction")
	}
	return p.err
}

func TestEventsAreRecordedWithTheChange(t *testing.T) {
	f := newFixture()
	tx := &spyTx{}
	svc := New(Deps{Artists: f.artists, Publisher: insideTx{tx: tx}, Tx: tx, Now: func() time.Time { return f.clock }})
	if _, err := svc.CreateArtist(context.Background(), CreateArtist{Name: "Kai Frost"}); err != nil {
		t.Fatal(err)
	}
	if tx.runs != 1 || tx.rolledBack != 0 {
		t.Fatalf("unit of work: %+v", tx)
	}

	// The outbox write failing rolls the change back: no change without its event.
	svc = New(Deps{Artists: f.artists, Publisher: insideTx{tx: tx, err: errors.New("outbox insert failed")}, Tx: tx, Now: func() time.Time { return f.clock }})
	if _, err := svc.CreateArtist(context.Background(), CreateArtist{Name: "Mira Solen"}); err == nil {
		t.Fatal("a failed event write must fail the request")
	}
	if tx.rolledBack != 1 {
		t.Fatalf("expected a rollback: %+v", tx)
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

func TestListAlbumsAndArtists(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	nova, _ := f.svc.CreateArtist(ctx, CreateArtist{Name: "Nova Hale"})
	kai, _ := f.svc.CreateArtist(ctx, CreateArtist{Name: "Kai Frost"})
	f.album(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), nova.ID)
	f.album(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), kai.ID)
	f.album(t, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), nova.ID)

	page, err := f.svc.ListAlbums(ctx, ListAlbums{Limit: 2})
	if err != nil || len(page.Albums) != 2 || page.Next == nil || page.Albums[0].ReleaseDate.Year() != 2026 {
		t.Fatalf("page = %+v, err = %v", page, err)
	}
	rest, _ := f.svc.ListAlbums(ctx, ListAlbums{After: page.Next, Limit: 2})
	if len(rest.Albums) != 1 || rest.Next != nil || rest.Albums[0].ReleaseDate.Year() != 2024 {
		t.Fatalf("rest = %+v", rest)
	}

	artists, err := f.svc.ListArtists(ctx, []uuid.UUID{nova.ID, uuid.New(), kai.ID})
	if err != nil || len(artists) != 2 {
		t.Fatalf("artists = %v, err = %v", artists, err)
	}
	if _, err := f.svc.ListArtists(ctx, make([]uuid.UUID, MaxBatchIDs+1)); err == nil {
		t.Fatal("expected validation error for oversized batch")
	}
	if _, err := f.svc.ListTracks(ctx, make([]uuid.UUID, MaxBatchIDs+1)); err == nil {
		t.Fatal("expected validation error for oversized track batch")
	}
	if tracks, err := f.svc.ListTracks(ctx, nil); err != nil || len(tracks) != 0 {
		t.Fatalf("empty batch = %v, %v", tracks, err)
	}
}

func TestAdvanceTrackMediaWalksThePipeline(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	artist, _ := f.svc.CreateArtist(ctx, CreateArtist{Name: "Nova Hale"})
	album := f.album(t, time.Now(), artist.ID)
	tr, _ := f.svc.CreateTrack(ctx, CreateTrack{AlbumID: album.ID, Title: "One", Duration: time.Minute, TrackNumber: 1})
	published := len(f.pub.events)

	steps := []struct {
		stage domain.MediaStage
		want  domain.TrackStatus
		event bool
	}{
		{domain.MediaUploaded, domain.TrackStatusProcessing, true},
		{domain.MediaUploaded, domain.TrackStatusProcessing, false}, // redelivery
		{domain.MediaFailed, domain.TrackStatusDraft, true},
		{domain.MediaUploaded, domain.TrackStatusProcessing, true},
		{domain.MediaTranscoded, domain.TrackStatusReady, true},
		{domain.MediaTranscoded, domain.TrackStatusReady, false}, // redelivery
	}
	for i, st := range steps {
		got, err := f.svc.AdvanceTrackMedia(ctx, tr.ID, st.stage)
		if err != nil || got.Status != st.want || f.tracks.byID[tr.ID].Status != st.want {
			t.Fatalf("step %d: got %s (stored %s), err %v; want %s", i, got.Status, f.tracks.byID[tr.ID].Status, err, st.want)
		}
		if st.event {
			published++
			if _, ok := f.pub.events[len(f.pub.events)-1].(domain.TrackUpdated); !ok {
				t.Fatalf("step %d: last event is not TrackUpdated", i)
			}
		}
		if len(f.pub.events) != published {
			t.Fatalf("step %d: %d events, want %d", i, len(f.pub.events), published)
		}
	}

	if _, err := f.svc.AdvanceTrackMedia(ctx, uuid.New(), domain.MediaUploaded); !errors.Is(err, domain.ErrTrackNotFound) {
		t.Fatalf("unknown track: %v", err)
	}
}

func TestUpdateTrackRejectsBlockedTransitions(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	artist, _ := f.svc.CreateArtist(ctx, CreateArtist{Name: "Nova Hale"})
	album := f.album(t, time.Now(), artist.ID)
	tr, _ := f.svc.CreateTrack(ctx, CreateTrack{AlbumID: album.ID, Title: "One", Duration: time.Minute, TrackNumber: 1})
	_, _ = f.svc.AdvanceTrackMedia(ctx, tr.ID, domain.MediaTranscoded)
	published := len(f.pub.events)

	blocked := domain.TrackStatusBlocked
	if _, err := f.svc.UpdateTrack(ctx, UpdateTrack{ID: tr.ID, Changes: domain.TrackChanges{Status: &blocked}}); !errors.Is(err, domain.ErrInvalidStatusTransition) {
		t.Fatalf("READY -> BLOCKED via UpdateTrack: got %v, want ErrInvalidStatusTransition", err)
	}
	if len(f.pub.events) != published {
		t.Fatal("rejected UpdateTrack must not publish")
	}

	if _, err := f.svc.ModerateTrack(ctx, uuid.New(), tr.ID, true); err != nil {
		t.Fatal(err)
	}
	published = len(f.pub.events)

	ready := domain.TrackStatusReady
	if _, err := f.svc.UpdateTrack(ctx, UpdateTrack{ID: tr.ID, Changes: domain.TrackChanges{Status: &ready}}); !errors.Is(err, domain.ErrInvalidStatusTransition) {
		t.Fatalf("BLOCKED -> READY via UpdateTrack: got %v, want ErrInvalidStatusTransition", err)
	}
	deleted := domain.TrackStatusDeleted
	if _, err := f.svc.UpdateTrack(ctx, UpdateTrack{ID: tr.ID, Changes: domain.TrackChanges{Status: &deleted}}); !errors.Is(err, domain.ErrInvalidStatusTransition) {
		t.Fatalf("BLOCKED -> DELETED via UpdateTrack: got %v, want ErrInvalidStatusTransition", err)
	}
	title := "One (Edited)"
	got, err := f.svc.UpdateTrack(ctx, UpdateTrack{ID: tr.ID, Changes: domain.TrackChanges{Title: &title}})
	if err != nil || got.Title != title || got.Status != domain.TrackStatusBlocked {
		t.Fatalf("metadata update on blocked track: got %+v, err %v", got, err)
	}
	if len(f.pub.events) != published+1 {
		t.Fatalf("expected 1 event for title update, got %d", len(f.pub.events)-published)
	}
}

func TestModerateTrack(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	artist, _ := f.svc.CreateArtist(ctx, CreateArtist{Name: "Nova Hale"})
	album := f.album(t, time.Now(), artist.ID)
	tr, _ := f.svc.CreateTrack(ctx, CreateTrack{AlbumID: album.ID, Title: "One", Duration: time.Minute, TrackNumber: 1})

	// DRAFT cannot be moderated.
	if _, err := f.svc.ModerateTrack(ctx, uuid.New(), tr.ID, true); !errors.Is(err, domain.ErrInvalidModerationTransition) {
		t.Fatalf("DRAFT block: got %v", err)
	}

	_, _ = f.svc.AdvanceTrackMedia(ctx, tr.ID, domain.MediaTranscoded)
	published := len(f.pub.events)

	// READY -> BLOCKED.
	op1 := uuid.New()
	res, err := f.svc.ModerateTrack(ctx, op1, tr.ID, true)
	if err != nil || !res.Changed || res.PreviousStatus != domain.TrackStatusReady || res.CurrentStatus != domain.TrackStatusBlocked {
		t.Fatalf("block result = %+v, err = %v", res, err)
	}
	if len(f.pub.events) != published+1 {
		t.Fatalf("expected TrackUpdated event, got %d", len(f.pub.events)-published)
	}

	// Idempotent replay with same operationID returns stored receipt without publishing.
	replay, err := f.svc.ModerateTrack(ctx, op1, tr.ID, true)
	if err != nil || replay != res {
		t.Fatalf("replay = %+v, want %+v, err = %v", replay, res, err)
	}
	if len(f.pub.events) != published+1 {
		t.Fatal("idempotent replay must not publish")
	}

	// Reusing operationID for another track or different blocked flag returns ErrModerationConflict.
	if _, err := f.svc.ModerateTrack(ctx, op1, uuid.New(), true); !errors.Is(err, domain.ErrModerationConflict) {
		t.Fatalf("reused operationID for another track: got %v", err)
	}
	if _, err := f.svc.ModerateTrack(ctx, op1, tr.ID, false); !errors.Is(err, domain.ErrModerationConflict) {
		t.Fatalf("reused operationID with different blocked flag: got %v", err)
	}

	// BLOCKED -> BLOCKED with new operationID is a no-op (changed=false, no event, receipt saved).
	op2 := uuid.New()
	noop, err := f.svc.ModerateTrack(ctx, op2, tr.ID, true)
	if err != nil || noop.Changed || noop.PreviousStatus != domain.TrackStatusBlocked || noop.CurrentStatus != domain.TrackStatusBlocked {
		t.Fatalf("noop block = %+v, err = %v", noop, err)
	}
	if len(f.pub.events) != published+1 {
		t.Fatal("no-op moderation must not publish")
	}

	// BLOCKED -> READY.
	op3 := uuid.New()
	unblock, err := f.svc.ModerateTrack(ctx, op3, tr.ID, false)
	if err != nil || !unblock.Changed || unblock.PreviousStatus != domain.TrackStatusBlocked || unblock.CurrentStatus != domain.TrackStatusReady {
		t.Fatalf("unblock = %+v, err = %v", unblock, err)
	}
	if len(f.pub.events) != published+2 {
		t.Fatal("unblock must publish TrackUpdated")
	}

	// Validation and not-found checks.
	var ve *domain.ValidationError
	if _, err := f.svc.ModerateTrack(ctx, uuid.Nil, tr.ID, true); !errors.As(err, &ve) {
		t.Fatalf("nil operationID: got %v", err)
	}
	if _, err := f.svc.ModerateTrack(ctx, uuid.New(), uuid.New(), true); !errors.Is(err, domain.ErrTrackNotFound) {
		t.Fatalf("missing track: got %v", err)
	}
}

func TestListTracksPage(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	artist, _ := f.svc.CreateArtist(ctx, CreateArtist{Name: "Nova Hale"})
	album := f.album(t, time.Now(), artist.ID)

	var created []domain.Track
	for i := 1; i <= 4; i++ {
		f.clock = f.clock.Add(time.Minute)
		tr, err := f.svc.CreateTrack(ctx, CreateTrack{AlbumID: album.ID, Title: "T", Duration: time.Minute, TrackNumber: i})
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, tr)
	}
	// Mark track 2 READY, track 3 DELETED.
	_, _ = f.svc.AdvanceTrackMedia(ctx, created[1].ID, domain.MediaTranscoded)
	deleted := domain.TrackStatusDeleted
	_, _ = f.svc.UpdateTrack(ctx, UpdateTrack{ID: created[2].ID, Changes: domain.TrackChanges{Status: &deleted}})

	// Default excludes DELETED, ordered newest first: 4, 2, 1.
	page1, err := f.svc.ListTracksPage(ctx, nil, nil, nil, 2)
	if err != nil || len(page1) != 2 || page1[0].ID != created[3].ID || page1[1].ID != created[1].ID {
		t.Fatalf("page1 = %+v, err = %v", page1, err)
	}
	page2, err := f.svc.ListTracksPage(ctx, nil, &page1[1].CreatedAt, &page1[1].ID, 2)
	if err != nil || len(page2) != 1 || page2[0].ID != created[0].ID {
		t.Fatalf("page2 = %+v, err = %v", page2, err)
	}

	// Filter by READY.
	ready := domain.TrackStatusReady
	readyPage, err := f.svc.ListTracksPage(ctx, &ready, nil, nil, 10)
	if err != nil || len(readyPage) != 1 || readyPage[0].ID != created[1].ID {
		t.Fatalf("readyPage = %+v, err = %v", readyPage, err)
	}

	// Filter by DELETED includes deleted tracks.
	delPage, err := f.svc.ListTracksPage(ctx, &deleted, nil, nil, 10)
	if err != nil || len(delPage) != 1 || delPage[0].ID != created[2].ID {
		t.Fatalf("delPage = %+v, err = %v", delPage, err)
	}

	// Validation errors.
	var ve *domain.ValidationError
	bogus := domain.TrackStatus("BOGUS")
	if _, err := f.svc.ListTracksPage(ctx, &bogus, nil, nil, 10); !errors.As(err, &ve) {
		t.Fatalf("invalid status: got %v", err)
	}
	if _, err := f.svc.ListTracksPage(ctx, nil, &f.clock, nil, 10); !errors.As(err, &ve) {
		t.Fatalf("partial cursor: got %v", err)
	}
}

type invalidatingTracks struct {
	*memTracks
	tx          *spyTx
	invalidated []uuid.UUID
}

func (r *invalidatingTracks) Invalidate(_ context.Context, trackID, albumID uuid.UUID) {
	if r.tx.inside {
		panic("cache invalidated inside transaction")
	}
	r.invalidated = append(r.invalidated, trackID, albumID)
}

func TestModerateTrackInvalidatesCacheAfterCommitAndRejectsNonReady(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	tx := &spyTx{}
	tracks := &invalidatingTracks{memTracks: f.tracks, tx: tx}
	svc := New(Deps{
		Artists: f.artists, Albums: &memAlbums{artists: f.artists, byID: map[uuid.UUID]domain.Album{}},
		Tracks: tracks, Genres: memGenres{}, Publisher: f.pub, Tx: tx,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time { return f.clock },
	})

	artist, _ := svc.CreateArtist(ctx, CreateArtist{Name: "Nova Hale"})
	album, _ := svc.CreateAlbum(ctx, CreateAlbum{Title: "Prism", Type: domain.AlbumTypeAlbum, ReleaseDate: f.clock, ArtistIDs: []uuid.UUID{artist.ID}})
	tr, _ := svc.CreateTrack(ctx, CreateTrack{AlbumID: album.ID, Title: "One", Duration: time.Minute, TrackNumber: 1})

	// PROCESSING and DELETED cannot be moderated.
	_, _ = svc.AdvanceTrackMedia(ctx, tr.ID, domain.MediaUploaded)
	if _, err := svc.ModerateTrack(ctx, uuid.New(), tr.ID, true); !errors.Is(err, domain.ErrInvalidModerationTransition) {
		t.Fatalf("PROCESSING block: got %v", err)
	}
	_, _ = svc.AdvanceTrackMedia(ctx, tr.ID, domain.MediaTranscoded)

	// READY -> BLOCKED invalidates cache after commit.
	if _, err := svc.ModerateTrack(ctx, uuid.New(), tr.ID, true); err != nil {
		t.Fatal(err)
	}
	if len(tracks.invalidated) != 2 || tracks.invalidated[0] != tr.ID || tracks.invalidated[1] != album.ID {
		t.Fatalf("invalidated = %v, want [%s %s]", tracks.invalidated, tr.ID, album.ID)
	}

	// No-op does not invalidate again.
	if _, err := svc.ModerateTrack(ctx, uuid.New(), tr.ID, true); err != nil {
		t.Fatal(err)
	}
	if len(tracks.invalidated) != 2 {
		t.Fatalf("no-op should not invalidate cache: %v", tracks.invalidated)
	}

	// DELETED track cannot be moderated.
	tr2, _ := svc.CreateTrack(ctx, CreateTrack{AlbumID: album.ID, Title: "Two", Duration: time.Minute, TrackNumber: 2})
	del := domain.TrackStatusDeleted
	_, _ = svc.UpdateTrack(ctx, UpdateTrack{ID: tr2.ID, Changes: domain.TrackChanges{Status: &del}})
	if _, err := svc.ModerateTrack(ctx, uuid.New(), tr2.ID, true); !errors.Is(err, domain.ErrInvalidModerationTransition) {
		t.Fatalf("DELETED block: got %v", err)
	}
}
