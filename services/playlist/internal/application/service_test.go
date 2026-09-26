package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/playlist/internal/domain"
)

type memRepo struct {
	lists  map[uuid.UUID]domain.Playlist
	tracks map[uuid.UUID][]domain.Track
}

func (m *memRepo) Create(_ context.Context, p domain.Playlist) error { m.lists[p.ID] = p; return nil }
func (m *memRepo) Get(_ context.Context, id uuid.UUID) (domain.Playlist, error) {
	p, ok := m.lists[id]
	if !ok {
		return p, domain.ErrNotFound
	}
	p.TrackCount = len(m.tracks[id])
	return p, nil
}
func (m *memRepo) ByOwner(context.Context, uuid.UUID) ([]domain.Playlist, error) { return nil, nil }
func (m *memRepo) Tracks(_ context.Context, id uuid.UUID) ([]domain.Track, error) {
	return m.tracks[id], nil
}
func (m *memRepo) UpdateTitle(_ context.Context, id uuid.UUID, title string, at time.Time) error {
	p := m.lists[id]
	p.Title, p.UpdatedAt = title, at
	m.lists[id] = p
	return nil
}
func (m *memRepo) Delete(_ context.Context, id uuid.UUID) (bool, error) {
	_, ok := m.lists[id]
	delete(m.lists, id)
	delete(m.tracks, id)
	return ok, nil
}
func (m *memRepo) AppendTrack(_ context.Context, id uuid.UUID, t domain.Track) (domain.Track, bool, error) {
	if slices.ContainsFunc(m.tracks[id], func(x domain.Track) bool { return x.TrackID == t.TrackID }) {
		return t, false, nil
	}
	t.Position = len(m.tracks[id]) + 1
	m.tracks[id] = append(m.tracks[id], t)
	return t, true, nil
}
func (m *memRepo) RemoveTrack(_ context.Context, id, trackID uuid.UUID, _ time.Time) (bool, error) {
	n := len(m.tracks[id])
	m.tracks[id] = slices.DeleteFunc(m.tracks[id], func(x domain.Track) bool { return x.TrackID == trackID })
	return len(m.tracks[id]) < n, nil
}
func (m *memRepo) Reorder(_ context.Context, id uuid.UUID, order []uuid.UUID, _ time.Time) error {
	if err := domain.CheckOrder(m.tracks[id], order); err != nil {
		return err
	}
	out := make([]domain.Track, len(order))
	for i, tid := range order {
		out[i] = domain.Track{TrackID: tid, Position: i + 1}
	}
	m.tracks[id] = out
	return nil
}

// recorder captures published event types.
type recorder struct{ events []string }

func (r *recorder) Created(context.Context, domain.Playlist) error {
	r.events = append(r.events, "created")
	return nil
}
func (r *recorder) Updated(context.Context, domain.Playlist) error {
	r.events = append(r.events, "updated")
	return nil
}
func (r *recorder) Deleted(context.Context, domain.Playlist, time.Time) error {
	r.events = append(r.events, "deleted")
	return nil
}
func (r *recorder) TrackAdded(context.Context, domain.Playlist, domain.Track) error {
	r.events = append(r.events, "track_added")
	return nil
}
func (r *recorder) TrackRemoved(context.Context, domain.Playlist, uuid.UUID, time.Time) error {
	r.events = append(r.events, "track_removed")
	return nil
}
func (r *recorder) TracksReordered(context.Context, domain.Playlist, []uuid.UUID, time.Time) error {
	r.events = append(r.events, "tracks_reordered")
	return nil
}

type catalog map[uuid.UUID]bool

func (c catalog) TrackExists(_ context.Context, id uuid.UUID) error {
	if !c[id] {
		return domain.ErrTrackNotFound
	}
	return nil
}

func TestPlaylistFlow(t *testing.T) {
	track := uuid.New()
	repo := &memRepo{lists: map[uuid.UUID]domain.Playlist{}, tracks: map[uuid.UUID][]domain.Track{}}
	pub := &recorder{}
	svc := New(repo, catalog{track: true}, pub, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	owner, stranger := uuid.New(), uuid.New()

	p, err := svc.Create(ctx, owner, " Late night ")
	if err != nil || p.Title != "Late night" {
		t.Fatal(p, err)
	}
	if _, err := svc.Create(ctx, owner, " "); !errors.Is(err, domain.ErrInvalidTitle) {
		t.Fatal(err)
	}
	for range 2 {
		if err := svc.AddTrack(ctx, owner, p.ID, track); err != nil {
			t.Fatal(err)
		}
	}
	if _, tracks, _ := svc.Get(ctx, p.ID); len(tracks) != 1 {
		t.Fatalf("idempotent add: %v", tracks)
	}
	if err := svc.AddTrack(ctx, owner, p.ID, uuid.New()); !errors.Is(err, domain.ErrTrackNotFound) {
		t.Fatal(err)
	}
	if err := svc.AddTrack(ctx, stranger, p.ID, track); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("stranger: %v", err)
	}
	if err := svc.RemoveTrack(ctx, stranger, p.ID, track); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal(err)
	}
	if err := svc.RemoveTrack(ctx, owner, p.ID, track); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Get(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}

	if !slices.Equal(pub.events, []string{"created", "track_added", "track_removed"}) {
		t.Fatalf("events: only real changes publish, got %v", pub.events)
	}
}

func TestRenameReorderDelete(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	repo := &memRepo{lists: map[uuid.UUID]domain.Playlist{}, tracks: map[uuid.UUID][]domain.Track{}}
	pub := &recorder{}
	svc := New(repo, catalog{a: true, b: true, c: true}, pub, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	owner, stranger := uuid.New(), uuid.New()
	p, _ := svc.Create(ctx, owner, "Mix")
	for _, id := range []uuid.UUID{a, b, c} {
		_ = svc.AddTrack(ctx, owner, p.ID, id)
	}
	pub.events = nil

	if got, err := svc.Rename(ctx, owner, p.ID, "  Road   trip "); err != nil || got.Title != "Road trip" {
		t.Fatal(got, err)
	}
	if _, err := svc.Rename(ctx, owner, p.ID, "Road trip"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rename(ctx, owner, p.ID, ""); !errors.Is(err, domain.ErrInvalidTitle) {
		t.Fatal(err)
	}
	if _, err := svc.Rename(ctx, stranger, p.ID, "Mine now"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal(err)
	}

	if err := svc.Reorder(ctx, owner, p.ID, []uuid.UUID{c, a, b}); err != nil {
		t.Fatal(err)
	}
	if _, tracks, _ := svc.Get(ctx, p.ID); tracks[0].TrackID != c || tracks[2].TrackID != b {
		t.Fatalf("order %+v", tracks)
	}
	if err := svc.Reorder(ctx, owner, p.ID, []uuid.UUID{c, a}); !errors.Is(err, domain.ErrOrderMismatch) {
		t.Fatal(err)
	}
	if err := svc.Reorder(ctx, stranger, p.ID, []uuid.UUID{a, b, c}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal(err)
	}

	if err := svc.Delete(ctx, stranger, p.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, owner, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, owner, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	if !slices.Equal(pub.events, []string{"updated", "tracks_reordered", "deleted"}) {
		t.Fatalf("events %v", pub.events)
	}
}
