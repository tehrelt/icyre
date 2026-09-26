package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// TrackStatus is the lifecycle state of a track.
type TrackStatus string

// Track lifecycle (specs/services/catalog.md).
const (
	TrackStatusDraft      TrackStatus = "DRAFT"
	TrackStatusProcessing TrackStatus = "PROCESSING"
	TrackStatusReady      TrackStatus = "READY"
	TrackStatusBlocked    TrackStatus = "BLOCKED"
	TrackStatusDeleted    TrackStatus = "DELETED"
)

// allowedTransitions is the track state machine.
//
//	DRAFT ──► PROCESSING ──► READY ◄──► BLOCKED
//	  ▲            │
//	  └── failed ──┘          any non-deleted ──► DELETED (terminal)
var allowedTransitions = map[TrackStatus][]TrackStatus{
	TrackStatusDraft:      {TrackStatusProcessing, TrackStatusDeleted},
	TrackStatusProcessing: {TrackStatusReady, TrackStatusDraft, TrackStatusDeleted},
	TrackStatusReady:      {TrackStatusBlocked, TrackStatusDeleted},
	TrackStatusBlocked:    {TrackStatusReady, TrackStatusDeleted},
	TrackStatusDeleted:    nil,
}

// Valid reports whether s is a known status.
func (s TrackStatus) Valid() bool {
	_, ok := allowedTransitions[s]
	return ok
}

// CanTransitionTo reports whether the state machine allows s → next.
func (s TrackStatus) CanTransitionTo(next TrackStatus) bool {
	for _, t := range allowedTransitions[s] {
		if t == next {
			return true
		}
	}
	return false
}

// Track limits.
const (
	MaxTrackDuration = 24 * time.Hour
	MaxTrackNumber   = 999
	MaxDiscNumber    = 99
)

var isrcPattern = regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{3}[0-9]{7}$`)

// Track is one recording on an album.
type Track struct {
	ID          uuid.UUID
	AlbumID     uuid.UUID
	ArtistIDs   []uuid.UUID
	Title       string
	Duration    time.Duration
	TrackNumber int
	DiscNumber  int
	Explicit    bool
	ISRC        string
	Status      TrackStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewTrackParams are the inputs of NewTrack.
type NewTrackParams struct {
	ID          uuid.UUID
	AlbumID     uuid.UUID
	ArtistIDs   []uuid.UUID
	Title       string
	Duration    time.Duration
	TrackNumber int
	DiscNumber  int
	Explicit    bool
	ISRC        string
}

// NewTrack validates and creates a track in DRAFT status. Audio is not
// attached yet; media ingest moves it forward.
func NewTrack(p NewTrackParams, now time.Time) (Track, error) {
	title := strings.TrimSpace(p.Title)
	isrc := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(p.ISRC), "-", ""))
	artists := dedupe(p.ArtistIDs)
	disc := p.DiscNumber
	if disc == 0 {
		disc = 1
	}

	v := validator{}
	v.check(p.ID != uuid.Nil, "id", "must be set")
	v.check(p.AlbumID != uuid.Nil, "albumId", "must be set")
	v.check(len(artists) > 0, "artistIds", "must contain at least one artist")
	v.check(len(artists) <= MaxCredits, "artistIds", "has too many artists")
	v.check(!containsNil(artists), "artistIds", "must not contain empty IDs")
	validateTitle(v, title)
	v.check(p.Duration > 0, "durationMs", "must be positive")
	v.check(p.Duration <= MaxTrackDuration, "durationMs", "is too long")
	v.check(p.TrackNumber >= 1 && p.TrackNumber <= MaxTrackNumber, "trackNumber", fmt.Sprintf("must be between 1 and %d", MaxTrackNumber))
	v.check(disc >= 1 && disc <= MaxDiscNumber, "discNumber", fmt.Sprintf("must be between 1 and %d", MaxDiscNumber))
	v.check(isrc == "" || isrcPattern.MatchString(isrc), "isrc", "must be a 12-character ISRC like USRC17607839")
	if err := v.err(); err != nil {
		return Track{}, err
	}

	now = now.UTC()
	return Track{
		ID:          p.ID,
		AlbumID:     p.AlbumID,
		ArtistIDs:   artists,
		Title:       title,
		Duration:    p.Duration.Truncate(time.Millisecond),
		TrackNumber: p.TrackNumber,
		DiscNumber:  disc,
		Explicit:    p.Explicit,
		ISRC:        isrc,
		Status:      TrackStatusDraft,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func validateTitle(v validator, title string) {
	v.check(title != "", "title", "must not be empty")
	v.check(utf8.RuneCountInString(title) <= MaxNameLength, "title", "is too long")
}

// MediaStage is a step of the media pipeline reported on media.events.
type MediaStage int

// Media pipeline steps (specs/services/catalog.md).
const (
	// MediaUploaded: a verified master is stored; transcoding starts.
	MediaUploaded MediaStage = iota + 1
	// MediaTranscoded: every audio variant is stored; the track is playable.
	MediaTranscoded
	// MediaFailed: the master cannot be transcoded; a new upload is needed.
	MediaFailed
)

// AdvanceMedia moves the track along the media pipeline and reports whether
// its status changed. Signals that do not fit the current status are
// ignored, not errors: redeliveries repeat, a READY track stays playable
// while a re-upload is processed, BLOCKED and DELETED tracks never come back.
//
// MediaTranscoded also finishes a DRAFT track: the event itself proves the
// audio went through processing (e.g. a failed upload followed by a
// successful one).
func (t *Track) AdvanceMedia(stage MediaStage, now time.Time) bool {
	var next TrackStatus
	switch {
	case stage == MediaUploaded && t.Status == TrackStatusDraft:
		next = TrackStatusProcessing
	case stage == MediaTranscoded && (t.Status == TrackStatusDraft || t.Status == TrackStatusProcessing):
		next = TrackStatusReady
	case stage == MediaFailed && t.Status == TrackStatusProcessing:
		next = TrackStatusDraft
	default:
		return false
	}
	t.Status = next
	t.UpdatedAt = now.UTC()
	return true
}

// TrackChanges describes a partial update. Nil fields stay unchanged.
type TrackChanges struct {
	Title    *string
	Explicit *bool
	Status   *TrackStatus
}

// Apply mutates the track. It reports whether anything changed.
func (t *Track) Apply(c TrackChanges, now time.Time) (bool, error) {
	if t.Status == TrackStatusDeleted {
		return false, ErrTrackDeleted
	}

	next := *t
	if c.Title != nil {
		title := strings.TrimSpace(*c.Title)
		v := validator{}
		validateTitle(v, title)
		if err := v.err(); err != nil {
			return false, err
		}
		next.Title = title
	}
	if c.Explicit != nil {
		next.Explicit = *c.Explicit
	}
	if c.Status != nil && *c.Status != t.Status {
		if !c.Status.Valid() {
			return false, &ValidationError{Fields: map[string]string{"status": "unknown status"}}
		}
		if !t.Status.CanTransitionTo(*c.Status) {
			return false, fmt.Errorf("%w: %s → %s", ErrInvalidStatusTransition, t.Status, *c.Status)
		}
		next.Status = *c.Status
	}

	changed := next.Title != t.Title || next.Explicit != t.Explicit || next.Status != t.Status
	if changed {
		next.UpdatedAt = now.UTC()
		*t = next
	}
	return changed, nil
}
