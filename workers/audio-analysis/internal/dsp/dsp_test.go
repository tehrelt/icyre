package dsp

import (
	"encoding/binary"
	"math"
	"math/cmplx"
	"math/rand/v2"
	"testing"
)

// clicks is a drum-like track: a decaying 1 kHz burst on every beat, with
// an accent every fourth beat and quiet noise underneath.
func clicks(bpm float64, seconds int) []float32 {
	rng := rand.New(rand.NewPCG(1, 2))
	out := make([]float32, seconds*SampleRate)
	period := 60 / bpm
	for i := range out {
		t := float64(i) / SampleRate
		beat := math.Floor(t / period)
		since := t - beat*period
		amp := 0.5
		if int(beat)%4 == 0 {
			amp = 0.9
		}
		out[i] = float32(amp*math.Exp(-since*40)*math.Sin(2*math.Pi*1000*t) + 0.01*rng.NormFloat64())
	}
	return out
}

func analyze(s []float32) Rhythm {
	a := NewAnalyzer()
	a.WriteSamples(s)
	return a.Result()
}

func TestTempoOfClickTracks(t *testing.T) {
	for _, bpm := range []float64{72, 90, 100, 120, 128, 140, 174} {
		r := analyze(clicks(bpm, 30))
		if math.Abs(r.BPM-bpm) > 1 || r.Confidence < 0.3 {
			t.Errorf("%v BPM: got %v (confidence %.2f)", bpm, r.BPM, r.Confidence)
		}
	}
}

func TestSilenceHasNoTempo(t *testing.T) {
	r := analyze(make([]float32, 10*SampleRate))
	if r.BPM != 0 || r.Confidence != 0 || r.SilenceRatio != 1 || r.DurationMs() != 10000 {
		t.Fatalf("silence: %+v", r)
	}
}

func TestNoiseHasNoTempo(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	s := make([]float32, 20*SampleRate)
	for i := range s {
		s[i] = float32(0.3 * rng.NormFloat64())
	}
	if r := analyze(s); r.BPM != 0 || r.SilenceRatio != 0 {
		t.Fatalf("noise: %+v", r)
	}
}

func TestShortAudioHasNoTempo(t *testing.T) {
	if r := analyze(clicks(120, 3)); r.BPM != 0 || r.DurationMs() != 3000 {
		t.Fatalf("3 s: %+v", r)
	}
}

func TestSilenceRatio(t *testing.T) {
	s := append(clicks(120, 10), make([]float32, 10*SampleRate)...)
	if r := analyze(s); math.Abs(r.SilenceRatio-0.5) > 0.01 {
		t.Fatalf("half silent: ratio %v", r.SilenceRatio)
	}
}

func TestWriteSplitsSamplesAcrossCalls(t *testing.T) {
	s := clicks(120, 8)
	raw := make([]byte, 4*len(s))
	for i, v := range s {
		binary.LittleEndian.PutUint32(raw[4*i:], math.Float32bits(v))
	}
	a := NewAnalyzer()
	for p := raw; len(p) > 0; {
		n := min(len(p), 7) // never a whole number of samples
		if w, err := a.Write(p[:n]); err != nil || w != n {
			t.Fatalf("Write = %d, %v", w, err)
		}
		p = p[n:]
	}
	if got, want := a.Result(), analyze(s); got != want {
		t.Fatalf("bytes %+v != samples %+v", got, want)
	}
}

func TestFFTMatchesDFT(t *testing.T) {
	const n = 16
	re, im := make([]float64, n), make([]float64, n)
	in := make([]complex128, n)
	for i := range re {
		re[i] = math.Sin(float64(i)) + float64(i%3)
		in[i] = complex(re[i], 0)
	}
	newFFT(n).transform(re, im)
	for k := 0; k < n; k++ {
		var want complex128
		for j, x := range in {
			want += x * cmplx.Exp(complex(0, -2*math.Pi*float64(j*k)/n))
		}
		if cmplx.Abs(complex(re[k], im[k])-want) > 1e-9 {
			t.Fatalf("bin %d: %v, want %v", k, complex(re[k], im[k]), want)
		}
	}
}
