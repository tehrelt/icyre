package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/playback/internal/domain"
)

type recorder struct {
	events []domain.Event
	err    error
}

func (r *recorder) Publish(_ context.Context, e domain.Event) error {
	if r.err != nil {
		return r.err
	}
	r.events = append(r.events, e)
	return nil
}

type memStore struct {
	sessions map[uuid.UUID]domain.Session
	queues   map[uuid.UUID]domain.Queue
	progress map[string]time.Time
	getErr   error
	saveErr  error
	allowErr error
	now      func() time.Time
}

func newMemStore(now func() time.Time) *memStore {
	return &memStore{
		sessions: map[uuid.UUID]domain.Session{},
		queues:   map[uuid.UUID]domain.Queue{},
		progress: map[string]time.Time{},
		now:      now,
	}
}

func (m *memStore) GetState(_ context.Context, userID uuid.UUID) (domain.Session, domain.Queue, error) {
	if m.getErr != nil {
		return domain.Session{}, domain.Queue{}, m.getErr
	}
	s, ok := m.sessions[userID]
	if !ok {
		return domain.Session{}, domain.Queue{}, domain.ErrSessionNotFound
	}
	return s, m.queues[userID], nil
}

func (m *memStore) SaveState(_ context.Context, sess domain.Session, q domain.Queue) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.sessions[sess.UserID] = sess
	m.queues[sess.UserID] = q
	return nil
}

func (m *memStore) AllowProgress(_ context.Context, userID, playbackID uuid.UUID, interval time.Duration) (bool, error) {
	if m.allowErr != nil {
		return false, m.allowErr
	}
	key := userID.String() + ":" + playbackID.String()
	now := m.now()
	if until, ok := m.progress[key]; ok && now.Before(until) {
		return false, nil
	}
	m.progress[key] = now.Add(interval)
	return true, nil
}

func TestReport(t *testing.T) {
	rec := &recorder{}
	svc := New(rec)
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	e := domain.Event{Kind: domain.Started, PlaybackID: uuid.New(), UserID: uuid.New(), TrackID: uuid.New(), DurationMs: 1000, At: now.Add(-time.Hour)}
	if err := svc.Report(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if len(rec.events) != 1 || !rec.events[0].At.Equal(now) {
		t.Fatalf("server time must win: %+v", rec.events)
	}
	var ve *domain.ValidationError
	if err := svc.Report(context.Background(), domain.Event{Kind: "x"}); !errors.As(err, &ve) || len(rec.events) != 1 {
		t.Fatalf("invalid report published: %v", err)
	}
}

func TestReportProgressThrottling(t *testing.T) {
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	rec := &recorder{}
	store := newMemStore(func() time.Time { return now })
	svc := New(rec).WithStateStore(store, 10*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.now = func() time.Time { return now }

	userID, playbackID, trackID := uuid.New(), uuid.New(), uuid.New()
	ev := domain.Event{Kind: domain.Progress, PlaybackID: playbackID, UserID: userID, TrackID: trackID, DurationMs: 180_000, ListenedMs: 10_000}

	if err := svc.Report(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if err := svc.Report(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(rec.events) != 1 {
		t.Fatalf("expected second progress within interval to be throttled, got %d events", len(rec.events))
	}

	now = now.Add(11 * time.Second)
	if err := svc.Report(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(rec.events) != 2 {
		t.Fatalf("expected progress after interval to publish, got %d events", len(rec.events))
	}

	// Redis error degrades open so telemetry is not dropped silently.
	store.allowErr = errors.New("redis down")
	if err := svc.Report(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(rec.events) != 3 {
		t.Fatalf("expected fail-open on redis error, got %d events", len(rec.events))
	}
}

func TestSessionLifecycle(t *testing.T) {
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	rec := &recorder{}
	store := newMemStore(func() time.Time { return now })
	svc := New(rec).WithStateStore(store, 10*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.now = func() time.Time { return now }
	svc.shuffle = func(n int, swap func(i, j int)) {
		for i := 0; i < n/2; i++ {
			swap(i, n-1-i)
		}
	}

	userID := uuid.New()
	t1, t2, t3 := uuid.New(), uuid.New(), uuid.New()

	if _, err := svc.GetSession(context.Background(), userID); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}

	st, err := svc.StartSession(context.Background(), userID, domain.StartSessionRequest{
		Tracks:     []uuid.UUID{t1, t2},
		StartIndex: 0,
		Source:     "album:a1",
		DurationMs: 200_000,
	})
	if err != nil || st.Session.TrackID != t1 || !st.Session.IsPlaying {
		t.Fatalf("start session: %+v %v", st, err)
	}
	if len(rec.events) != 1 || rec.events[0].Kind != domain.Started {
		t.Fatalf("expected started event, got %+v", rec.events)
	}

	got, err := svc.GetSession(context.Background(), userID)
	if err != nil || got.Session.SessionID != st.Session.SessionID {
		t.Fatalf("get session: %+v %v", got, err)
	}

	// Update position emits throttled progress.
	pos := int64(15_000)
	st, err = svc.UpdateState(context.Background(), userID, domain.StatePatch{PositionMs: &pos})
	if err != nil || st.Session.PositionMs != 15_000 {
		t.Fatalf("update position: %+v %v", st, err)
	}
	pos2 := int64(18_000)
	st, err = svc.UpdateState(context.Background(), userID, domain.StatePatch{PositionMs: &pos2})
	if err != nil || st.Session.PositionMs != 18_000 {
		t.Fatalf("update position 2: %+v %v", st, err)
	}
	if len(rec.events) != 2 || rec.events[1].Kind != domain.Progress {
		t.Fatalf("expected 1 throttled progress event, got %+v", rec.events)
	}

	// Next track emits skipped + started.
	next := domain.ActionNext
	st, err = svc.UpdateState(context.Background(), userID, domain.StatePatch{Action: &next})
	if err != nil || st.Session.TrackID != t2 {
		t.Fatalf("next: %+v %v", st, err)
	}
	if len(rec.events) != 4 || rec.events[2].Kind != domain.Skipped || rec.events[3].Kind != domain.Started {
		t.Fatalf("expected skipped+started, got %+v", rec.events)
	}

	// Replace queue.
	idx := 1
	st, err = svc.SetQueue(context.Background(), userID, domain.SetQueueRequest{
		Tracks:     []uuid.UUID{t1, t3},
		StartIndex: &idx,
		Source:     "playlist:p1",
	})
	if err != nil || st.Session.TrackID != t3 || st.Queue.Source != "playlist:p1" {
		t.Fatalf("set queue: %+v %v", st, err)
	}
}

func TestSessionWithoutStoreReturnsUnavailable(t *testing.T) {
	svc := New(&recorder{})
	if _, err := svc.GetSession(context.Background(), uuid.New()); !errors.Is(err, domain.ErrStoreUnavailable) {
		t.Fatalf("expected ErrStoreUnavailable, got %v", err)
	}
}
