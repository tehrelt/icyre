package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/playback/internal/domain"
)

type recorder struct{ events []domain.Event }

func (r *recorder) Publish(_ context.Context, e domain.Event) error {
	r.events = append(r.events, e)
	return nil
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
