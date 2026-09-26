package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/recommendation"
	"github.com/tehrelt/icyre/workers/recommendation/internal/domain"
)

type fakes struct {
	tracks  []domain.Track
	plays   map[uuid.UUID]uint64
	history map[uuid.UUID]map[uuid.UUID]domain.Interaction
	liked   map[uuid.UUID]map[uuid.UUID]bool
	sounds  map[uuid.UUID]*domain.Sound
	err     error
	out     map[string]recommendation.Set
}

func (f *fakes) Tracks(context.Context) ([]domain.Track, error) { return f.tracks, f.err }
func (f *fakes) Plays(context.Context, int) (map[uuid.UUID]uint64, error) {
	return f.plays, nil
}
func (f *fakes) History(context.Context, int) (map[uuid.UUID]map[uuid.UUID]domain.Interaction, error) {
	return f.history, nil
}
func (f *fakes) Likes(context.Context) (map[uuid.UUID]map[uuid.UUID]bool, map[uuid.UUID]map[uuid.UUID]bool, error) {
	return f.liked, nil, nil
}
func (f *fakes) Sounds(context.Context) (map[uuid.UUID]*domain.Sound, error) { return f.sounds, nil }
func (f *fakes) Publish(_ context.Context, sets map[string]recommendation.Set) error {
	f.out = sets
	return nil
}

func TestBuildPublishesPopularAndPersonalSets(t *testing.T) {
	artist := uuid.New()
	t1, t2, t3 := uuid.New(), uuid.New(), uuid.New()
	listener, liker, stranger := uuid.New(), uuid.New(), uuid.New()
	f := &fakes{
		tracks: []domain.Track{
			{ID: t1, ArtistIDs: []uuid.UUID{artist}},
			{ID: t2, ArtistIDs: []uuid.UUID{artist}},
			{ID: t3, ArtistIDs: []uuid.UUID{uuid.New()}},
		},
		plays:   map[uuid.UUID]uint64{t3: 100},
		history: map[uuid.UUID]map[uuid.UUID]domain.Interaction{listener: {t1: {Plays: 1, Completions: 1}}},
		liked:   map[uuid.UUID]map[uuid.UUID]bool{liker: {t2: true}},
		sounds:  map[uuid.UUID]*domain.Sound{t1: domain.NewSound("v1", nil, -10, 5, 0)},
	}
	b := NewBuilder(f, f, f, f, Options{Tracks: 2, Artists: 2}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	st, err := b.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Users != 2 || st.Candidates != 3 || len(f.out) != 3 {
		t.Fatalf("stats %+v sets %d", st, len(f.out))
	}
	pop := f.out[recommendation.PopularKey]
	if pop.Version != recommendation.Version || pop.Tracks[0].ID != t3.String() {
		t.Fatalf("popular %+v", pop)
	}
	mine := f.out[recommendation.UserKey(listener.String())]
	if mine.Tracks[0].ID != t2.String() || mine.Algorithm != domain.Algorithm {
		t.Fatalf("listener set %+v", mine)
	}
	if _, ok := f.out[recommendation.UserKey(stranger.String())]; ok {
		t.Fatal("no set for a user without signals")
	}
	if _, ok := f.out[recommendation.UserKey(liker.String())]; !ok {
		t.Fatal("likes alone must personalise")
	}
}

func TestBuildFailsWithoutCatalog(t *testing.T) {
	f := &fakes{err: errors.New("down")}
	b := NewBuilder(f, f, f, f, Options{Tracks: 1, Artists: 1}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	b.now = func() time.Time { return time.Unix(0, 0) }
	if _, err := b.Build(context.Background()); err == nil || f.out != nil {
		t.Fatalf("err %v, published %v", err, f.out)
	}
}
