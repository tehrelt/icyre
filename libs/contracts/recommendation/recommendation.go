// Package recommendation holds the Redis contract between the
// Recommendation Worker (writer) and the Recommendation Service (reader):
// keys and the JSON of a ready-made recommendation set.
package recommendation

import "time"

// Version is the schema version of Set; readers ignore sets of another one.
const Version = 1

// UserKey holds the personal set of one user.
func UserKey(userID string) string { return "recommendations:user:" + userID }

// PopularKey holds the fallback set for users without personal data.
const PopularKey = "recommendations:popular"

// Reasons an item was recommended (the largest score components).
const (
	ReasonArtist  = "artist"  // artists the user plays and likes
	ReasonStyle   = "style"   // genres and sound close to the user's taste
	ReasonPopular = "popular" // played a lot lately
	ReasonFresh   = "fresh"   // recently released
)

// Item is one recommended track or artist, best first within its list.
type Item struct {
	ID      string   `json:"id"`
	Score   float64  `json:"score"`
	Reasons []string `json:"reasons,omitempty"`
}

// Set is a ready recommendation set.
type Set struct {
	Version     int       `json:"version"`
	Algorithm   string    `json:"algorithm"`
	GeneratedAt time.Time `json:"generatedAt"`
	Tracks      []Item    `json:"tracks"`
	Artists     []Item    `json:"artists"`
}
