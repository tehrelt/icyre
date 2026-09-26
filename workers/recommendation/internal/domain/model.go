// Package domain holds the Recommendation Worker's model and the MVP
// scoring (specs/workers/recommendation-worker.md):
//
//	score = style*0.35 + artist_affinity*0.30 + popularity*0.20 + freshness*0.15
//
// where style is genre similarity, blended with sound (audio features)
// similarity when both sides have features.
package domain

import (
	"math"
	"time"

	"github.com/google/uuid"
)

// Track is a recommendable track with what scoring needs about it.
type Track struct {
	ID        uuid.UUID
	ArtistIDs []uuid.UUID
	AlbumID   uuid.UUID
	GenreIDs  []uuid.UUID // the album's genres
	Released  time.Time   // zero when unknown
	// Plays in the popularity window.
	Plays uint64
	// Sound is nil when the track has not been analysed.
	Sound *Sound
}

// Sound is a track's audio features, normalised to [0, 1] per dimension.
// Features of different analyzer versions are not comparable.
type Sound struct {
	Version string
	Vector  [4]float64 // tempo, loudness, loudness range, silence
}

// NewSound normalises raw features (BPM may be nil: no detectable tempo).
func NewSound(version string, bpm *float64, lufs, lraLU, silence float64) *Sound {
	tempo := 0.5 // no tempo: the middle, neither slow nor fast
	if bpm != nil {
		tempo = clamp01((*bpm - 60) / 140) // 60..200 BPM
	}
	return &Sound{Version: version, Vector: [4]float64{
		tempo,
		clamp01((lufs + 70) / 70), // -70..0 LUFS
		clamp01(lraLU / 20),       // 0..20 LU
		clamp01(silence),
	}}
}

// Interaction is what one user did with one track in the history window.
type Interaction struct {
	Plays       uint64 // playback.started
	Completions uint64 // playback.finished
	Skips       uint64 // playback.skipped
}

// Signals are everything known about one user's taste.
type Signals struct {
	UserID      uuid.UUID
	History     map[uuid.UUID]Interaction
	LikedTracks map[uuid.UUID]bool
	LikedAlbums map[uuid.UUID]bool
	// FollowedArtists: follows arrive with the Social Service (EPIC-015).
	FollowedArtists map[uuid.UUID]bool
}

// Empty reports that there is nothing to personalise on.
func (s Signals) Empty() bool {
	return len(s.History) == 0 && len(s.LikedTracks) == 0 && len(s.LikedAlbums) == 0 && len(s.FollowedArtists) == 0
}

func clamp01(v float64) float64 {
	return math.Min(1, math.Max(0, v))
}
