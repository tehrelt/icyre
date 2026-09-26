package domain

import (
	"time"

	"github.com/google/uuid"
)

// Stats are playback metrics over some period.
// Plays count playback.started, Completions playback.finished and Skips
// playback.skipped.
type Stats struct {
	Plays           uint64 `json:"plays"`
	Completions     uint64 `json:"completions"`
	Skips           uint64 `json:"skips"`
	UniqueListeners uint64 `json:"uniqueListeners"`
	ListenedMs      uint64 `json:"listenedMs"`
}

// CompletionRate is the share of ended plays that were listened to the end;
// 0 when no play has ended.
func (s Stats) CompletionRate() float64 {
	ended := s.Completions + s.Skips
	if ended == 0 {
		return 0
	}
	return float64(s.Completions) / float64(ended)
}

// Ranked is a track or artist with its metrics, as in a popularity chart.
type Ranked struct {
	ID uuid.UUID `json:"id"`
	Stats
	CompletionRate float64 `json:"completionRate"`
}

// Report summarizes the days [From, To].
type Report struct {
	From   time.Time `json:"from"`
	To     time.Time `json:"to"`
	Totals Stats     `json:"totals"`
	// CompletionRate is Totals.CompletionRate().
	CompletionRate float64 `json:"completionRate"`
	// DailyActiveListeners is the mean of the days' active listeners.
	DailyActiveListeners float64  `json:"dailyActiveListeners"`
	TopTracks            []Ranked `json:"topTracks"`
	TopArtists           []Ranked `json:"topArtists"`
}
