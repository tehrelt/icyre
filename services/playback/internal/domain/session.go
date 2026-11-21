package domain

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Session errors.
var (
	ErrSessionNotFound  = errors.New("playback session not found")
	ErrStoreUnavailable = errors.New("playback state store is unavailable")
)

// MaxQueueSize bounds the number of tracks in a user's playback queue.
const MaxQueueSize = 500

const restartThresholdMs int64 = 3_000

// RepeatMode controls queue wrap-around behavior.
type RepeatMode string

// Repeat modes.
const (
	RepeatOff RepeatMode = "off"
	RepeatAll RepeatMode = "all"
	RepeatOne RepeatMode = "one"
)

// Valid reports whether m is a supported repeat mode.
func (m RepeatMode) Valid() bool {
	switch m {
	case RepeatOff, RepeatAll, RepeatOne:
		return true
	default:
		return false
	}
}

// Action is a transport command applied to the current queue.
type Action string

// Transport actions.
const (
	ActionNext     Action = "next"
	ActionPrevious Action = "previous"
)

// Valid reports whether a is a supported transport action.
func (a Action) Valid() bool {
	switch a {
	case ActionNext, ActionPrevious:
		return true
	default:
		return false
	}
}

// Session holds the transient playback state of one listener.
type Session struct {
	SessionID  uuid.UUID  `json:"sessionId"`
	UserID     uuid.UUID  `json:"userId"`
	TrackID    uuid.UUID  `json:"trackId"`
	PlaybackID uuid.UUID  `json:"playbackId"`
	Source     string     `json:"source,omitempty"`
	IsPlaying  bool       `json:"isPlaying"`
	PositionMs int64      `json:"positionMs"`
	DurationMs int64      `json:"durationMs"`
	Shuffle    bool       `json:"shuffle"`
	RepeatMode RepeatMode `json:"repeatMode"`
	QueueIndex int        `json:"queueIndex"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

// Queue holds the listener's track list and play order.
type Queue struct {
	UserID    uuid.UUID   `json:"userId"`
	Tracks    []uuid.UUID `json:"tracks"`
	Order     []int       `json:"order"`
	Cursor    int         `json:"cursor"`
	Source    string      `json:"source,omitempty"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

// PlaybackState combines the active session and its queue.
type PlaybackState struct {
	Session Session
	Queue   Queue
}

// StartSessionRequest starts a new playback session with an initial queue.
type StartSessionRequest struct {
	Tracks     []uuid.UUID
	StartIndex int
	Source     string
	DurationMs int64
	Shuffle    bool
	RepeatMode RepeatMode
}

// SetQueueRequest replaces the active queue.
type SetQueueRequest struct {
	Tracks     []uuid.UUID
	StartIndex *int
	Source     string
}

// StatePatch is a partial update to the active playback state.
type StatePatch struct {
	IsPlaying  *bool
	PositionMs *int64
	DurationMs *int64
	Shuffle    *bool
	RepeatMode *RepeatMode
	QueueIndex *int
	Action     *Action
}

// StateTransition lists domain events produced by a session/state mutation.
type StateTransition struct {
	Events []Event
}

// ShuffleFunc permutes n elements in place (matches math/rand/v2.Shuffle).
type ShuffleFunc func(n int, swap func(i, j int))

// NewSession validates req and creates a new Session and Queue.
func NewSession(userID uuid.UUID, req StartSessionRequest, now time.Time, shuffleFn ShuffleFunc) (Session, Queue, StateTransition, error) {
	fields := map[string]string{}
	if userID == uuid.Nil {
		fields["userId"] = "must be a UUID"
	}
	validateTracks(req.Tracks, false, fields)
	if len(req.Tracks) > 0 && (req.StartIndex < 0 || req.StartIndex >= len(req.Tracks)) {
		fields["startIndex"] = "must be within tracks bounds"
	}
	if req.Source != "" && !sourcePattern.MatchString(req.Source) {
		fields["source"] = "must look like album:<id>"
	}
	if req.DurationMs < 0 || req.DurationMs > MaxDurationMs {
		fields["durationMs"] = "must be between 0 and 7200000"
	}
	repeat := req.RepeatMode
	if repeat == "" {
		repeat = RepeatOff
	} else if !repeat.Valid() {
		fields["repeatMode"] = "must be off, all or one"
	}
	if len(fields) > 0 {
		return Session{}, Queue{}, StateTransition{}, &ValidationError{Fields: fields}
	}

	ts := now.UTC().Truncate(time.Millisecond)
	order, cursor := buildOrder(len(req.Tracks), req.StartIndex, req.Shuffle, shuffleFn)
	tracks := slices.Clone(req.Tracks)
	playbackID := uuid.New()
	sessionID := uuid.New()
	trackID := tracks[req.StartIndex]

	sess := Session{
		SessionID:  sessionID,
		UserID:     userID,
		TrackID:    trackID,
		PlaybackID: playbackID,
		Source:     req.Source,
		IsPlaying:  true,
		PositionMs: 0,
		DurationMs: req.DurationMs,
		Shuffle:    req.Shuffle,
		RepeatMode: repeat,
		QueueIndex: req.StartIndex,
		UpdatedAt:  ts,
	}
	q := Queue{
		UserID:    userID,
		Tracks:    tracks,
		Order:     order,
		Cursor:    cursor,
		Source:    req.Source,
		UpdatedAt: ts,
	}
	tr := StateTransition{
		Events: []Event{{
			Kind:       Started,
			PlaybackID: playbackID,
			UserID:     userID,
			TrackID:    trackID,
			Source:     req.Source,
			DurationMs: req.DurationMs,
			ListenedMs: 0,
			At:         ts,
		}},
	}
	return sess, q, tr, nil
}

// ApplyQueue replaces the queue on an existing session without mutating inputs.
func ApplyQueue(sess Session, q Queue, req SetQueueRequest, now time.Time, shuffleFn ShuffleFunc) (Session, Queue, StateTransition, error) {
	fields := map[string]string{}
	validateTracks(req.Tracks, true, fields)
	startIdx := 0
	if req.StartIndex != nil {
		startIdx = *req.StartIndex
		if len(req.Tracks) == 0 || startIdx < 0 || startIdx >= len(req.Tracks) {
			fields["startIndex"] = "must be within tracks bounds"
		}
	}
	if req.Source != "" && !sourcePattern.MatchString(req.Source) {
		fields["source"] = "must look like album:<id>"
	}
	if len(fields) > 0 {
		return Session{}, Queue{}, StateTransition{}, &ValidationError{Fields: fields}
	}

	ts := now.UTC().Truncate(time.Millisecond)
	nextSess := sess
	nextSess.UpdatedAt = ts
	source := sess.Source
	if req.Source != "" {
		source = req.Source
	}
	nextSess.Source = source

	var events []Event
	if len(req.Tracks) == 0 {
		if sess.TrackID != uuid.Nil && sess.IsPlaying {
			events = append(events, Event{
				Kind:       Skipped,
				PlaybackID: sess.PlaybackID,
				UserID:     sess.UserID,
				TrackID:    sess.TrackID,
				Source:     sess.Source,
				DurationMs: sess.DurationMs,
				ListenedMs: clampListened(sess.PositionMs, sess.DurationMs),
				At:         ts,
			})
		}
		nextSess.TrackID = uuid.Nil
		nextSess.PlaybackID = uuid.Nil
		nextSess.IsPlaying = false
		nextSess.PositionMs = 0
		nextSess.DurationMs = 0
		nextSess.QueueIndex = -1
		nextQ := Queue{
			UserID:    sess.UserID,
			Tracks:    []uuid.UUID{},
			Order:     []int{},
			Cursor:    -1,
			Source:    source,
			UpdatedAt: ts,
		}
		return nextSess, nextQ, StateTransition{Events: events}, nil
	}

	tracks := slices.Clone(req.Tracks)
	order, cursor := buildOrder(len(tracks), startIdx, sess.Shuffle, shuffleFn)
	nextTrack := tracks[startIdx]
	if nextTrack != sess.TrackID || sess.PlaybackID == uuid.Nil {
		if sess.TrackID != uuid.Nil && sess.IsPlaying {
			events = append(events, Event{
				Kind:       Skipped,
				PlaybackID: sess.PlaybackID,
				UserID:     sess.UserID,
				TrackID:    sess.TrackID,
				Source:     sess.Source,
				DurationMs: sess.DurationMs,
				ListenedMs: clampListened(sess.PositionMs, sess.DurationMs),
				At:         ts,
			})
		}
		nextSess.PlaybackID = uuid.New()
		nextSess.TrackID = nextTrack
		nextSess.PositionMs = 0
		nextSess.DurationMs = 0
		if nextSess.IsPlaying {
			events = append(events, Event{
				Kind:       Started,
				PlaybackID: nextSess.PlaybackID,
				UserID:     nextSess.UserID,
				TrackID:    nextTrack,
				Source:     source,
				DurationMs: 0,
				ListenedMs: 0,
				At:         ts,
			})
		}
	}
	nextSess.QueueIndex = startIdx
	nextQ := Queue{
		UserID:    sess.UserID,
		Tracks:    tracks,
		Order:     order,
		Cursor:    cursor,
		Source:    source,
		UpdatedAt: ts,
	}
	return nextSess, nextQ, StateTransition{Events: events}, nil
}

// ApplyState validates and applies a partial state update without mutating inputs.
func ApplyState(sess Session, q Queue, patch StatePatch, now time.Time, shuffleFn ShuffleFunc) (Session, Queue, StateTransition, error) {
	fields := map[string]string{}
	if patch.RepeatMode != nil && !patch.RepeatMode.Valid() {
		fields["repeatMode"] = "must be off, all or one"
	}
	if patch.DurationMs != nil && (*patch.DurationMs < 0 || *patch.DurationMs > MaxDurationMs) {
		fields["durationMs"] = "must be between 0 and 7200000"
	}
	effectiveDuration := sess.DurationMs
	if patch.DurationMs != nil {
		effectiveDuration = *patch.DurationMs
	}
	if patch.PositionMs != nil {
		if *patch.PositionMs < 0 || *patch.PositionMs > MaxDurationMs {
			fields["positionMs"] = "must be between 0 and 7200000"
		} else if effectiveDuration > 0 && *patch.PositionMs > effectiveDuration {
			fields["positionMs"] = "must not exceed durationMs"
		}
	}
	if patch.Action != nil && patch.QueueIndex != nil {
		fields["action"] = "cannot be combined with queueIndex"
	}
	if patch.Action != nil {
		if !patch.Action.Valid() {
			fields["action"] = "must be next or previous"
		} else if len(q.Tracks) == 0 {
			fields["action"] = "queue is empty"
		}
	}
	if patch.QueueIndex != nil {
		if len(q.Tracks) == 0 || *patch.QueueIndex < 0 || *patch.QueueIndex >= len(q.Tracks) {
			fields["queueIndex"] = "must be within queue bounds"
		}
	}
	if len(fields) > 0 {
		return Session{}, Queue{}, StateTransition{}, &ValidationError{Fields: fields}
	}

	ts := now.UTC().Truncate(time.Millisecond)
	nextSess := sess
	nextSess.UpdatedAt = ts
	nextQ := Queue{
		UserID:    q.UserID,
		Tracks:    slices.Clone(q.Tracks),
		Order:     slices.Clone(q.Order),
		Cursor:    q.Cursor,
		Source:    q.Source,
		UpdatedAt: ts,
	}

	if patch.DurationMs != nil {
		nextSess.DurationMs = *patch.DurationMs
	}
	if patch.RepeatMode != nil {
		nextSess.RepeatMode = *patch.RepeatMode
	}
	if patch.IsPlaying != nil {
		nextSess.IsPlaying = *patch.IsPlaying
	}
	if patch.Shuffle != nil && *patch.Shuffle != sess.Shuffle {
		nextSess.Shuffle = *patch.Shuffle
		if len(nextQ.Tracks) > 0 && nextQ.Cursor >= 0 && nextQ.Cursor < len(nextQ.Order) {
			curOrig := nextQ.Order[nextQ.Cursor]
			nextQ.Order, nextQ.Cursor = buildOrder(len(nextQ.Tracks), curOrig, nextSess.Shuffle, shuffleFn)
		}
	}

	var events []Event
	trackChanged := false
	endedQueue := false

	switch {
	case patch.QueueIndex != nil:
		targetOrig := *patch.QueueIndex
		if idx := slices.Index(nextQ.Order, targetOrig); idx >= 0 {
			nextQ.Cursor = idx
		}
		trackChanged = targetOrig != sess.QueueIndex || nextQ.Tracks[targetOrig] != sess.TrackID
	case patch.Action != nil:
		switch *patch.Action {
		case ActionNext:
			if nextQ.Cursor+1 < len(nextQ.Order) {
				nextQ.Cursor++
				trackChanged = true
			} else if nextSess.RepeatMode == RepeatAll && len(nextQ.Order) > 0 {
				if nextSess.Shuffle && len(nextQ.Order) > 1 {
					nextQ.Order, nextQ.Cursor = buildOrder(len(nextQ.Tracks), nextQ.Order[0], true, shuffleFn)
				} else {
					nextQ.Cursor = 0
				}
				trackChanged = true
			} else {
				endedQueue = true
			}
		case ActionPrevious:
			if sess.PositionMs > restartThresholdMs && patch.PositionMs == nil {
				nextSess.PositionMs = 0
			} else if nextQ.Cursor > 0 {
				nextQ.Cursor--
				trackChanged = true
			} else if nextSess.RepeatMode == RepeatAll && len(nextQ.Order) > 0 {
				nextQ.Cursor = len(nextQ.Order) - 1
				trackChanged = true
			} else {
				nextSess.PositionMs = 0
			}
		}
	}

	if endedQueue {
		if sess.TrackID != uuid.Nil && sess.IsPlaying {
			events = append(events, Event{
				Kind:       Finished,
				PlaybackID: sess.PlaybackID,
				UserID:     sess.UserID,
				TrackID:    sess.TrackID,
				Source:     sess.Source,
				DurationMs: sess.DurationMs,
				ListenedMs: clampListened(sess.PositionMs, sess.DurationMs),
				At:         ts,
			})
		}
		nextSess.IsPlaying = false
		nextSess.PositionMs = 0
		return nextSess, nextQ, StateTransition{Events: events}, nil
	}

	if trackChanged && nextQ.Cursor >= 0 && nextQ.Cursor < len(nextQ.Order) {
		origIdx := nextQ.Order[nextQ.Cursor]
		nextTrack := nextQ.Tracks[origIdx]
		if sess.TrackID != uuid.Nil {
			events = append(events, Event{
				Kind:       Skipped,
				PlaybackID: sess.PlaybackID,
				UserID:     sess.UserID,
				TrackID:    sess.TrackID,
				Source:     sess.Source,
				DurationMs: sess.DurationMs,
				ListenedMs: clampListened(sess.PositionMs, sess.DurationMs),
				At:         ts,
			})
		}
		nextSess.PlaybackID = uuid.New()
		nextSess.TrackID = nextTrack
		nextSess.QueueIndex = origIdx
		nextSess.PositionMs = 0
		nextSess.DurationMs = 0
		if nextSess.IsPlaying {
			events = append(events, Event{
				Kind:       Started,
				PlaybackID: nextSess.PlaybackID,
				UserID:     nextSess.UserID,
				TrackID:    nextTrack,
				Source:     nextSess.Source,
				DurationMs: 0,
				ListenedMs: 0,
				At:         ts,
			})
		}
		return nextSess, nextQ, StateTransition{Events: events}, nil
	}

	if patch.PositionMs != nil {
		nextSess.PositionMs = *patch.PositionMs
	}
	if nextSess.TrackID != uuid.Nil && nextSess.IsPlaying {
		if !sess.IsPlaying && nextSess.PositionMs == 0 {
			events = append(events, Event{
				Kind:       Started,
				PlaybackID: nextSess.PlaybackID,
				UserID:     nextSess.UserID,
				TrackID:    nextSess.TrackID,
				Source:     nextSess.Source,
				DurationMs: nextSess.DurationMs,
				ListenedMs: 0,
				At:         ts,
			})
		} else if patch.PositionMs != nil {
			events = append(events, Event{
				Kind:       Progress,
				PlaybackID: nextSess.PlaybackID,
				UserID:     nextSess.UserID,
				TrackID:    nextSess.TrackID,
				Source:     nextSess.Source,
				DurationMs: nextSess.DurationMs,
				ListenedMs: clampListened(nextSess.PositionMs, nextSess.DurationMs),
				At:         ts,
			})
		}
	}
	return nextSess, nextQ, StateTransition{Events: events}, nil
}

func validateTracks(tracks []uuid.UUID, allowEmpty bool, fields map[string]string) {
	if !allowEmpty && len(tracks) == 0 {
		fields["tracks"] = "must contain at least 1 track"
		return
	}
	if len(tracks) > MaxQueueSize {
		fields["tracks"] = "must contain at most 500 tracks"
		return
	}
	if slices.Contains(tracks, uuid.Nil) {
		fields["tracks"] = "must contain valid track UUIDs"
	}
}

func buildOrder(n, startIdx int, shuffle bool, shuffleFn ShuffleFunc) ([]int, int) {
	if n == 0 {
		return []int{}, -1
	}
	order := make([]int, n)
	if !shuffle {
		for i := range n {
			order[i] = i
		}
		return order, startIdx
	}
	order[0] = startIdx
	pos := 1
	for i := range n {
		if i == startIdx {
			continue
		}
		order[pos] = i
		pos++
	}
	if shuffleFn != nil && n > 2 {
		rest := order[1:]
		shuffleFn(len(rest), func(i, j int) {
			rest[i], rest[j] = rest[j], rest[i]
		})
	}
	return order, 0
}

func clampListened(pos, dur int64) int64 {
	if pos < 0 {
		return 0
	}
	if dur > 0 && pos > dur {
		return dur
	}
	return pos
}
