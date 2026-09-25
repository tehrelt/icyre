// Package media is the object-storage contract shared by the media pipeline
// (ingest, transcoder) and delivery (stream authorization, origin):
// bucket layout, object keys and audio variants (specs/data/object-storage.md).
package media

import (
	"fmt"
	"slices"
	"strconv"
)

// Bucket holds all track media. Objects are private; clients get
// short-lived signed URLs.
const Bucket = "icyre-media"

// Quality is an audio variant bitrate in kbps.
type Quality int

// Variants produced by the transcoder, lowest first.
const (
	Quality64  Quality = 64
	Quality128 Quality = 128
	Quality256 Quality = 256
)

// Qualities lists every variant, lowest first.
var Qualities = []Quality{Quality64, Quality128, Quality256}

// AudioContentType is the MIME type of variants (AAC in ADTS).
const AudioContentType = "audio/aac"

// ParseQuality accepts "64", "128" or "256".
func ParseQuality(s string) (Quality, error) {
	n, err := strconv.Atoi(s)
	if err != nil || !slices.Contains(Qualities, Quality(n)) {
		return 0, fmt.Errorf("unknown quality %q", s)
	}
	return Quality(n), nil
}

func (q Quality) String() string { return strconv.Itoa(int(q)) }

// TrackAudioKey is the object key of an audio variant:
// tracks/{trackId}/audio/{quality}.aac. Variants are immutable.
func TrackAudioKey(trackID string, q Quality) string {
	return fmt.Sprintf("tracks/%s/audio/%d.aac", trackID, q)
}

// TrackOriginalKey is where an uploaded master lives: tracks/{trackId}/original/source.{ext}.
func TrackOriginalKey(trackID, ext string) string {
	return fmt.Sprintf("tracks/%s/original/source.%s", trackID, ext)
}

// TrackCoverKey is the cover image: tracks/{trackId}/cover/cover.webp.
func TrackCoverKey(trackID string) string {
	return fmt.Sprintf("tracks/%s/cover/cover.webp", trackID)
}
