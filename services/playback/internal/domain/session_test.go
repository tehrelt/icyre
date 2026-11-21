package domain

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

func reverseShuffle(n int, swap func(i, j int)) {
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		swap(i, j)
	}
}

func TestNewSessionAndShuffle(t *testing.T) {
	userID := uuid.New()
	t1, t2, t3 := uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

	sess, q, tr, err := NewSession(userID, StartSessionRequest{
		Tracks:     []uuid.UUID{t1, t2, t3},
		StartIndex: 1,
		Source:     "album:alb-1",
		DurationMs: 180_000,
		Shuffle:    true,
	}, now, reverseShuffle)
	if err != nil {
		t.Fatal(err)
	}
	if sess.TrackID != t2 || sess.QueueIndex != 1 || !sess.IsPlaying || sess.RepeatMode != RepeatOff {
		t.Fatalf("unexpected session: %+v", sess)
	}
	if q.Cursor != 0 || q.Order[0] != 1 || !slices.Equal(q.Order, []int{1, 2, 0}) {
		t.Fatalf("unexpected shuffled order: %+v", q)
	}
	if len(tr.Events) != 1 || tr.Events[0].Kind != Started || tr.Events[0].TrackID != t2 {
		t.Fatalf("unexpected transition events: %+v", tr.Events)
	}
}

func TestNewSessionValidation(t *testing.T) {
	userID := uuid.New()
	now := time.Now()
	cases := map[string]StartSessionRequest{
		"tracks":     {Tracks: nil},
		"startIndex": {Tracks: []uuid.UUID{uuid.New()}, StartIndex: 2},
		"source":     {Tracks: []uuid.UUID{uuid.New()}, Source: "http://bad"},
		"durationMs": {Tracks: []uuid.UUID{uuid.New()}, DurationMs: -1},
		"repeatMode": {Tracks: []uuid.UUID{uuid.New()}, RepeatMode: "forever"},
	}
	for field, req := range cases {
		_, _, _, err := NewSession(userID, req, now, nil)
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Fields[field] == "" {
			t.Errorf("%s: got %v", field, err)
		}
	}
}

func TestApplyQueue(t *testing.T) {
	userID := uuid.New()
	t1, t2, t3 := uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	sess, q, _, err := NewSession(userID, StartSessionRequest{
		Tracks: []uuid.UUID{t1, t2}, StartIndex: 0, Source: "album:a1", DurationMs: 100_000,
	}, now, nil)
	if err != nil {
		t.Fatal(err)
	}
	sess.PositionMs = 25_000

	idx := 1
	nextSess, nextQ, tr, err := ApplyQueue(sess, q, SetQueueRequest{
		Tracks: []uuid.UUID{t2, t3}, StartIndex: &idx, Source: "playlist:p1",
	}, now.Add(time.Minute), nil)
	if err != nil {
		t.Fatal(err)
	}
	if nextSess.TrackID != t3 || nextSess.QueueIndex != 1 || nextSess.Source != "playlist:p1" || len(nextQ.Tracks) != 2 {
		t.Fatalf("unexpected queue update: %+v %+v", nextSess, nextQ)
	}
	if len(tr.Events) != 2 || tr.Events[0].Kind != Skipped || tr.Events[0].ListenedMs != 25_000 || tr.Events[1].Kind != Started {
		t.Fatalf("unexpected events: %+v", tr.Events)
	}

	// Clearing queue stops playback and emits skipped.
	clearedSess, clearedQ, tr, err := ApplyQueue(nextSess, nextQ, SetQueueRequest{Tracks: []uuid.UUID{}}, now.Add(2*time.Minute), nil)
	if err != nil {
		t.Fatal(err)
	}
	if clearedSess.IsPlaying || clearedSess.TrackID != uuid.Nil || clearedQ.Cursor != -1 || len(tr.Events) != 1 || tr.Events[0].Kind != Skipped {
		t.Fatalf("unexpected clear: %+v %+v %+v", clearedSess, clearedQ, tr.Events)
	}
}

func TestApplyStateNavigationShuffleAndRepeat(t *testing.T) {
	userID := uuid.New()
	t1, t2, t3 := uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	sess, q, _, _ := NewSession(userID, StartSessionRequest{
		Tracks: []uuid.UUID{t1, t2, t3}, StartIndex: 0, DurationMs: 120_000,
	}, now, nil)

	// Progress update emits Progress event.
	pos := int64(15_000)
	sess, q, tr, err := ApplyState(sess, q, StatePatch{PositionMs: &pos}, now.Add(10*time.Second), nil)
	if err != nil || sess.PositionMs != 15_000 || len(tr.Events) != 1 || tr.Events[0].Kind != Progress {
		t.Fatalf("progress: %+v %+v %v", sess, tr.Events, err)
	}

	// Previous when position > 3s restarts current track without changing track.
	prev := ActionPrevious
	sess, q, tr, err = ApplyState(sess, q, StatePatch{Action: &prev}, now.Add(11*time.Second), nil)
	if err != nil || sess.TrackID != t1 || sess.PositionMs != 0 || len(tr.Events) != 0 {
		t.Fatalf("restart on previous: %+v %+v %v", sess, tr.Events, err)
	}

	// Toggle shuffle on keeps current track first and shuffles rest.
	shuf := true
	sess, q, _, err = ApplyState(sess, q, StatePatch{Shuffle: &shuf}, now.Add(12*time.Second), reverseShuffle)
	if err != nil || !sess.Shuffle || !slices.Equal(q.Order, []int{0, 2, 1}) || q.Cursor != 0 {
		t.Fatalf("shuffle on: %+v %+v %v", sess, q, err)
	}

	// Next moves along shuffled order (0 -> 2 -> t3).
	next := ActionNext
	sess, q, tr, err = ApplyState(sess, q, StatePatch{Action: &next}, now.Add(13*time.Second), reverseShuffle)
	if err != nil || sess.TrackID != t3 || sess.QueueIndex != 2 || len(tr.Events) != 2 || tr.Events[0].Kind != Skipped || tr.Events[1].Kind != Started {
		t.Fatalf("next shuffled: %+v %+v %v", sess, tr.Events, err)
	}

	// Toggle shuffle off restores sequential order at current track index (2).
	shufOff := false
	sess, q, _, err = ApplyState(sess, q, StatePatch{Shuffle: &shufOff}, now.Add(14*time.Second), nil)
	if err != nil || sess.Shuffle || !slices.Equal(q.Order, []int{0, 1, 2}) || q.Cursor != 2 {
		t.Fatalf("shuffle off: %+v %+v %v", sess, q, err)
	}

	// Next at end with RepeatOff finishes playback.
	sess, q, tr, err = ApplyState(sess, q, StatePatch{Action: &next}, now.Add(15*time.Second), nil)
	if err != nil || sess.IsPlaying || len(tr.Events) != 1 || tr.Events[0].Kind != Finished {
		t.Fatalf("end of queue: %+v %+v %v", sess, tr.Events, err)
	}

	// RepeatAll wraps around on next.
	repAll := RepeatAll
	playing := true
	sess, q, _, err = ApplyState(sess, q, StatePatch{RepeatMode: &repAll, IsPlaying: &playing}, now.Add(16*time.Second), nil)
	if err != nil {
		t.Fatal(err)
	}
	sess, q, tr, err = ApplyState(sess, q, StatePatch{Action: &next}, now.Add(17*time.Second), nil)
	if err != nil || sess.TrackID != t1 || sess.QueueIndex != 0 || len(tr.Events) != 2 {
		t.Fatalf("repeat all wrap: %+v %+v %v", sess, tr.Events, err)
	}
}
