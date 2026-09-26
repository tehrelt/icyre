package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/recommendation"
)

type fakeStore struct {
	sets map[string]recommendation.Set
	err  error
}

func (f fakeStore) Get(_ context.Context, key string) (recommendation.Set, bool, error) {
	s, ok := f.sets[key]
	return s, ok, f.err
}

func set(id string) recommendation.Set {
	return recommendation.Set{Version: recommendation.Version, Tracks: []recommendation.Item{{ID: id}}}
}

func TestForFallsBack(t *testing.T) {
	user, other, stale := uuid.New(), uuid.New(), uuid.New()
	old := set("old")
	old.Version = recommendation.Version + 1
	svc := New(fakeStore{sets: map[string]recommendation.Set{
		recommendation.UserKey(user.String()):  set("mine"),
		recommendation.UserKey(stale.String()): old,
		recommendation.PopularKey:              set("popular"),
	}})
	ctx := context.Background()
	for _, tc := range []struct {
		user   uuid.UUID
		source string
		first  string
	}{
		{user, SourcePersonal, "mine"},
		{other, SourcePopular, "popular"},
		{uuid.Nil, SourcePopular, "popular"},
		{stale, SourcePopular, "popular"}, // another schema version counts as absent
	} {
		r, err := svc.For(ctx, tc.user)
		if err != nil || r.Source != tc.source || r.Tracks[0].ID != tc.first || r.Artists == nil {
			t.Errorf("%v: %+v %v", tc.user, r, err)
		}
	}

	r, err := New(fakeStore{}).For(ctx, user)
	if err != nil || r.Source != SourceNone || r.Tracks == nil {
		t.Fatalf("nothing built: %+v %v", r, err)
	}
	if _, err := New(fakeStore{err: errors.New("down")}).For(ctx, user); err == nil {
		t.Fatal("want store error")
	}
}
