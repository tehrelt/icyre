package application

import (
	"context"
	"errors"
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
func (m *memRepo) AppendTrack(_ context.Context, id uuid.UUID, t domain.Track) error {
	if slices.ContainsFunc(m.tracks[id], func(x domain.Track) bool { return x.TrackID == t.TrackID }) {
		return nil
	}
	t.Position = len(m.tracks[id]) + 1
	m.tracks[id] = append(m.tracks[id], t)
	return nil
}
func (m *memRepo) RemoveTrack(_ context.Context, id, trackID uuid.UUID, _ time.Time) error {
	m.tracks[id] = slices.DeleteFunc(m.tracks[id], func(x domain.Track) bool { return x.TrackID == trackID })
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
	svc := New(repo, catalog{track: true})
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
}
