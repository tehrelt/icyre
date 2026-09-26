// Package domain holds the technical metadata of an uploaded master.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrInvalid means a probe result that cannot be stored.
var ErrInvalid = errors.New("invalid metadata")

// Probe is what ffprobe reports about one file.
type Probe struct {
	// Container is the demuxer name ("flac", "wav", "mp3").
	Container string
	// Codec is the audio stream's codec ("flac", "pcm_s16le", "mp3").
	Codec        string
	DurationMs   int64
	BitrateBps   int64 // stream, else container bitrate; 0: not declared
	SampleRateHz int
	Channels     int
}

// Metadata is the probe of one upload's master.
type Metadata struct {
	UploadID     uuid.UUID
	TrackID      uuid.UUID
	SourceSHA256 string
	Probe
	UploadedAt  time.Time
	ExtractedAt time.Time
}

// Validate rejects probes no audio file can have.
func (p Probe) Validate() error {
	switch {
	case p.Container == "" || p.Codec == "":
		return errors.Join(ErrInvalid, errors.New("container and codec are required"))
	case p.DurationMs <= 0:
		return errors.Join(ErrInvalid, errors.New("duration must be positive"))
	case p.BitrateBps < 0:
		return errors.Join(ErrInvalid, errors.New("bitrate must not be negative"))
	case p.SampleRateHz <= 0:
		return errors.Join(ErrInvalid, errors.New("sample rate must be positive"))
	case p.Channels <= 0:
		return errors.Join(ErrInvalid, errors.New("channels must be positive"))
	}
	return nil
}
