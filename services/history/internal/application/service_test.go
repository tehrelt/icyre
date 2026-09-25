package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/history/internal/domain"
)

type memRepo struct{ rows map[uuid.UUID]domain.Listen }

func (m *memRepo) Record(_ context.Context, l domain.Listen) (bool, error) {
	if _, ok := m.rows[l.PlaybackID]; ok {
		return false, nil
	}
	m.rows[l.PlaybackID] = l
	return true, nil
}
func (m *memRepo) Tracks(context.Context, uuid.UUID, *domain.Cursor, int) ([]domain.Listen, error) {
	return nil, nil
}
func (m *memRepo) RecentSources(context.Context, uuid.UUID, int) ([]domain.RecentSource, error) {
	return nil, nil
}

func TestPlayEnded(t *testing.T) {
	repo := &memRepo{rows: map[uuid.UUID]domain.Listen{}}
	svc := New(repo, domain.DefaultRule)
	ctx := context.Background()
	l := domain.Listen{PlaybackID: uuid.New(), UserID: uuid.New(), TrackID: uuid.New(), DurationMs: 240_000, ListenedMs: 45_000, PlayedAt: time.Now()}

	if ok, err := svc.PlayEnded(ctx, l); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if ok, _ := svc.PlayEnded(ctx, l); ok {
		t.Fatal("redelivered event recorded twice")
	}
	short := l
	short.PlaybackID, short.ListenedMs = uuid.New(), 5_000
	if ok, _ := svc.PlayEnded(ctx, short); ok || len(repo.rows) != 1 {
		t.Fatal("a 5 s skip counted as a listen")
	}
}
