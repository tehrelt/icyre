package domain

import (
	"errors"
	"math"
	"testing"
)

var song = Features{BPM: 128, BPMConfidence: 0.7, IntegratedLUFS: -9.4, LoudnessRangeLU: 5.2, TruePeakDBTP: 0.4, SilenceRatio: 0.02, AnalyzedMs: 215000}

func TestValidateAccepts(t *testing.T) {
	silence := Features{IntegratedLUFS: Floor, TruePeakDBTP: Floor, SilenceRatio: 1, AnalyzedMs: 1000}
	for name, f := range map[string]Features{"song": song, "silence": silence} {
		if err := f.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestValidateRejects(t *testing.T) {
	for name, mutate := range map[string]func(*Features){
		"slow bpm":         func(f *Features) { f.BPM = 30 },
		"fast bpm":         func(f *Features) { f.BPM = 250 },
		"nan":              func(f *Features) { f.IntegratedLUFS = math.NaN() },
		"inf peak":         func(f *Features) { f.TruePeakDBTP = math.Inf(-1) },
		"confidence":       func(f *Features) { f.BPMConfidence = 1.5 },
		"below floor":      func(f *Features) { f.IntegratedLUFS = -90 },
		"negative range":   func(f *Features) { f.LoudnessRangeLU = -1 },
		"silence ratio":    func(f *Features) { f.SilenceRatio = 2 },
		"no audio decoded": func(f *Features) { f.AnalyzedMs = 0 },
	} {
		f := song
		mutate(&f)
		if err := f.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}
