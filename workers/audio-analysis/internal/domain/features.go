// Package domain holds the audio features of an uploaded master.
package domain

import (
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
)

// ErrInvalid means features that cannot be stored.
var ErrInvalid = errors.New("invalid features")

// AnalyzerVersion identifies the algorithms behind Features; it changes
// when they do, so features of different versions are not mixed up.
const AnalyzerVersion = "1"

// Tempo range the analysis searches, in beats per minute.
const (
	MinBPM = 60
	MaxBPM = 200
)

// Floor is the lowest reported level (LUFS, dBTP): the EBU R128 absolute
// gate. Silence measures -70, not -inf.
const Floor = -70.0

// Features are what the analysis measures on one file.
type Features struct {
	// BPM is the dominant tempo; 0 when the track has none (silence, noise).
	BPM float64
	// BPMConfidence is the tempo's periodicity strength in [0, 1].
	BPMConfidence float64
	// IntegratedLUFS is the EBU R128 integrated loudness.
	IntegratedLUFS float64
	// LoudnessRangeLU is the EBU R128 loudness range (LRA).
	LoudnessRangeLU float64
	// TruePeakDBTP is the maximum true peak over all channels.
	TruePeakDBTP float64
	// SilenceRatio is the share of the track below -60 dBFS, in [0, 1].
	SilenceRatio float64
	// AnalyzedMs is the decoded audio length.
	AnalyzedMs int64
}

// Analysis is the features of one upload's master.
type Analysis struct {
	UploadID     uuid.UUID
	TrackID      uuid.UUID
	SourceSHA256 string
	Features
	AnalyzerVersion string
	UploadedAt      time.Time
	AnalyzedAt      time.Time
}

// Validate rejects features no audio file can have.
func (f Features) Validate() error {
	for _, v := range []float64{f.BPM, f.BPMConfidence, f.IntegratedLUFS, f.LoudnessRangeLU, f.TruePeakDBTP, f.SilenceRatio} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return errors.Join(ErrInvalid, errors.New("features must be finite"))
		}
	}
	switch {
	case f.BPM != 0 && (f.BPM < MinBPM || f.BPM > MaxBPM):
		return errors.Join(ErrInvalid, errors.New("bpm out of range"))
	case f.BPMConfidence < 0 || f.BPMConfidence > 1:
		return errors.Join(ErrInvalid, errors.New("bpm confidence must be in [0, 1]"))
	case f.IntegratedLUFS < Floor || f.TruePeakDBTP < Floor:
		return errors.Join(ErrInvalid, errors.New("levels must not be below the floor"))
	case f.LoudnessRangeLU < 0:
		return errors.Join(ErrInvalid, errors.New("loudness range must not be negative"))
	case f.SilenceRatio < 0 || f.SilenceRatio > 1:
		return errors.Join(ErrInvalid, errors.New("silence ratio must be in [0, 1]"))
	case f.AnalyzedMs <= 0:
		return errors.Join(ErrInvalid, errors.New("analyzed duration must be positive"))
	}
	return nil
}
