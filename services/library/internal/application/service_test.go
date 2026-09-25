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

	"github.com/tehrelt/icyre/services/library/internal/domain"
)

type memRepo struct{ items []domain.Item }

func (m *memRepo) find(u uuid.UUID, k domain.Kind, id uuid.UUID) int {
	return slices.IndexFunc(m.items, func(i domain.Item) bool { return i.UserID == u && i.Kind == k && i.EntityID == id })
}
func (m *memRepo) Save(_ context.Context, it domain.Item) (domain.Change, error) {
	if i := m.find(it.UserID, it.Kind, it.EntityID); i >= 0 {
		return domain.Change{Item: m.items[i]}, nil
	}
	m.items = append(m.items, it)
	return domain.Change{Item: it, Changed: true}, nil
}
func (m *memRepo) Remove(_ context.Context, u uuid.UUID, k domain.Kind, id uuid.UUID, at time.Time) (domain.Change, error) {
	i := m.find(u, k, id)
	if i < 0 {
		return domain.Change{Item: domain.Item{UserID: u, Kind: k, EntityID: id}}, nil
	}
	it := m.items[i]
	m.items = slices.Delete(m.items, i, i+1)
	return domain.Change{Item: it, Changed: true}, nil
}
func (m *memRepo) List(_ context.Context, u uuid.UUID, k domain.Kind, after *domain.Cursor, limit int) ([]domain.Item, error) {
	var out []domain.Item
	for i := len(m.items) - 1; i >= 0; i-- { // newest first
		it := m.items[i]
		if it.UserID != u || it.Kind != k {
			continue
		}
		if after != nil && !it.SavedAt.Before(after.SavedAt) {
			continue
		}
		out = append(out, it)
	}
	return out[:min(limit, len(out))], nil
}
func (m *memRepo) Contains(_ context.Context, u uuid.UUID, k domain.Kind, ids []uuid.UUID) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, id := range ids {
		if m.find(u, k, id) >= 0 {
			out = append(out, id)
		}
	}
	return out, nil
}
func (m *memRepo) Counts(_ context.Context, u uuid.UUID) (domain.Counts, error) {
	var c domain.Counts
	for _, it := range m.items {
		if it.UserID == u {
			if it.Kind == domain.KindTrack {
				c.Tracks++
			} else {
				c.Albums++
			}
		}
	}
	return c, nil
}

type fakeCatalog map[uuid.UUID]bool

func (f fakeCatalog) Exists(_ context.Context, _ domain.Kind, id uuid.UUID) error {
	if !f[id] {
		return domain.ErrNotFound
	}
	return nil
}

type events struct{ saved, removed int }

func (e *events) Saved(context.Context, domain.Item) error   { e.saved++; return nil }
func (e *events) Removed(context.Context, domain.Item) error { e.removed++; return nil }

func TestIdempotentSaveRemove(t *testing.T) {
	track := uuid.New()
	repo, ev := &memRepo{}, &events{}
	svc := New(repo, fakeCatalog{track: true}, ev, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	user := uuid.New()

	first, err := svc.Save(ctx, user, domain.KindTrack, track)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := svc.Save(ctx, user, domain.KindTrack, track)
	if !again.SavedAt.Equal(first.SavedAt) || ev.saved != 1 {
		t.Fatalf("second save changed state: %v vs %v, events %d", again.SavedAt, first.SavedAt, ev.saved)
	}
	if _, err := svc.Save(ctx, user, domain.KindTrack, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown track: %v", err)
	}
	_ = svc.Remove(ctx, user, domain.KindTrack, track)
	if err := svc.Remove(ctx, user, domain.KindTrack, track); err != nil || ev.removed != 1 {
		t.Fatalf("second remove: %v, events %d", err, ev.removed)
	}
}

func TestListPaginationAndContains(t *testing.T) {
	repo := &memRepo{}
	user := uuid.New()
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var ids []uuid.UUID
	for i := range 5 {
		id := uuid.New()
		ids = append(ids, id)
		repo.items = append(repo.items, domain.Item{UserID: user, Kind: domain.KindTrack, EntityID: id, SavedAt: t0.Add(time.Duration(i) * time.Hour)})
	}
	svc := New(repo, fakeCatalog{}, &events{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	p1, _ := svc.List(ctx, user, domain.KindTrack, nil, 2)
	if len(p1.Items) != 2 || p1.Items[0].EntityID != ids[4] || p1.Next == nil {
		t.Fatalf("page 1 %+v", p1)
	}
	p3, _ := svc.List(ctx, user, domain.KindTrack, &domain.Cursor{SavedAt: ids0(repo, ids[1]).SavedAt, EntityID: ids[1]}, 2)
	if len(p3.Items) != 1 || p3.Next != nil {
		t.Fatalf("last page %+v", p3)
	}
	got, _ := svc.Contains(ctx, user, domain.KindTrack, []uuid.UUID{ids[0], uuid.New()})
	if len(got) != 1 || got[0] != ids[0] {
		t.Fatal(got)
	}
	c, _ := svc.Counts(ctx, user)
	if c.Tracks != 5 || c.Albums != 0 {
		t.Fatal(c)
	}
}

func ids0(r *memRepo, id uuid.UUID) domain.Item {
	for _, it := range r.items {
		if it.EntityID == id {
			return it
		}
	}
	return domain.Item{}
}
