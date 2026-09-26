package dsp

import (
	"math"
	"math/rand/v2"
	"testing"
)

// drums is a rock beat: kick on 1 and 3, snare on 2 and 4, closed hi-hat
// on every eighth note.
func drums(bpm float64, seconds int) []float32 {
	rng := rand.New(rand.NewPCG(5, 6))
	out := make([]float32, seconds*SampleRate)
	beat := 60 / bpm
	for i := range out {
		t := float64(i) / SampleRate
		n := int(math.Floor(t / beat))
		sinceBeat := t - float64(n)*beat
		sinceEighth := math.Mod(t, beat/2)
		v := 0.0
		if n%2 == 0 { // kick: pitch-dropping sine
			v += 0.9 * math.Exp(-sinceBeat*25) * math.Sin(2*math.Pi*(50+80*math.Exp(-sinceBeat*30))*sinceBeat)
		} else { // snare: noise burst
			v += 0.5 * math.Exp(-sinceBeat*20) * rng.NormFloat64()
		}
		v += 0.12 * math.Exp(-sinceEighth*80) * rng.NormFloat64() // hi-hat
		out[i] = float32(v)
	}
	return out
}

func TestTempoOfDrumBeats(t *testing.T) {
	for _, bpm := range []float64{85, 100, 120, 135} {
		r := analyze(drums(bpm, 30))
		if math.Abs(r.BPM-bpm) > 1.5 {
			t.Errorf("%v BPM: got %v (confidence %.2f)", bpm, r.BPM, r.Confidence)
		}
	}
}

// A fast beat whose kick-snare bar is far more periodic than its beat is
// read an octave down (the usual octave ambiguity of tempo estimation):
// 160 or 80, never anything else.
func TestTempoOctaveAmbiguity(t *testing.T) {
	r := analyze(drums(160, 30))
	if math.Abs(r.BPM-160) > 1.5 && math.Abs(r.BPM-80) > 1 {
		t.Fatalf("160 BPM: got %v", r.BPM)
	}
}
