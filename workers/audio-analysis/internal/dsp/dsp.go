// Package dsp measures the tempo and the silence of a mono PCM stream.
//
// Tempo: the onset envelope is the spectral flux of log-magnitude spectra
// (frameSize-sample Hann frames every hopSize samples); its local mean is
// removed and the rest half-wave rectified, then the normalized
// autocorrelation over lags of MinBPM..MaxBPM, weighted by a log-normal
// tempo prior around 120 BPM, picks the beat period, and the faster octave
// wins when it is almost as periodic. The peak value is the confidence; a
// weak peak means no tempo.
package dsp

import (
	"encoding/binary"
	"math"

	"github.com/tehrelt/icyre/workers/audio-analysis/internal/domain"
)

// SampleRate is the rate the stream must be resampled to.
const SampleRate = 11025

const (
	frameSize = 1024
	hopSize   = 128
	// fps is the onset envelope rate, ~86 values per second.
	fps = float64(SampleRate) / hopSize
	// silenceRMS is -60 dBFS: a hop quieter than it is silence.
	silenceRMS = 0.001
	// minConfidence: a weaker autocorrelation peak is not a tempo.
	minConfidence = 0.1
	// minTempoSeconds of audio are needed to see a few beats of 60 BPM.
	minTempoSeconds = 6
	// priorBPM and priorOctaves shape the tempo prior (octave errors).
	priorBPM     = 120
	priorOctaves = 1.0
	// octaveRatio: half the picked lag must be about as periodic to win;
	// weaker off-beats (hi-hats between kicks and snares) must not.
	octaveRatio = 0.95
	// logGain compresses magnitudes before the flux.
	logGain = 100
)

// Rhythm is what the analyzer measured.
type Rhythm struct {
	// BPM is 0 when no tempo is found.
	BPM          float64
	Confidence   float64
	SilenceRatio float64
	Samples      int64
}

// DurationMs is the analyzed audio length.
func (r Rhythm) DurationMs() int64 { return r.Samples * 1000 / SampleRate }

// Analyzer consumes mono float32 little-endian PCM at SampleRate through
// Write; memory is bounded by the onset envelope (~86 values a second).
type Analyzer struct {
	frame    []float64 // the last frameSize samples
	hop      []float64 // samples of the hop being filled
	window   []float64
	re, im   []float64
	prevMag  []float64
	envelope []float64
	fft      *fft

	hops, silentHops int
	samples          int64
	rest             []byte // a sample split between two writes
}

// NewAnalyzer returns an empty analyzer.
func NewAnalyzer() *Analyzer {
	w := make([]float64, frameSize)
	for i := range w {
		w[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/frameSize)
	}
	return &Analyzer{
		frame: make([]float64, frameSize), hop: make([]float64, 0, hopSize), window: w,
		re: make([]float64, frameSize), im: make([]float64, frameSize), prevMag: make([]float64, frameSize/2+1),
		fft: newFFT(frameSize),
	}
}

// Write implements io.Writer over float32 little-endian samples.
func (a *Analyzer) Write(p []byte) (int, error) {
	n := len(p)
	if len(a.rest) > 0 {
		need := 4 - len(a.rest)
		if len(p) < need {
			a.rest = append(a.rest, p...)
			return n, nil
		}
		a.rest = append(a.rest, p[:need]...)
		a.sample(math.Float32frombits(binary.LittleEndian.Uint32(a.rest)))
		a.rest, p = a.rest[:0], p[need:]
	}
	for ; len(p) >= 4; p = p[4:] {
		a.sample(math.Float32frombits(binary.LittleEndian.Uint32(p)))
	}
	a.rest = append(a.rest, p...)
	return n, nil
}

// WriteSamples feeds samples directly (tests, callers with decoded audio).
func (a *Analyzer) WriteSamples(s []float32) {
	for _, v := range s {
		a.sample(v)
	}
}

func (a *Analyzer) sample(v float32) {
	x := float64(v)
	if math.IsNaN(x) || math.IsInf(x, 0) {
		x = 0
	}
	a.samples++
	a.hop = append(a.hop, x)
	if len(a.hop) == hopSize {
		a.endHop()
	}
}

func (a *Analyzer) endHop() {
	a.countSilence(a.hop)
	copy(a.frame, a.frame[hopSize:])
	copy(a.frame[frameSize-hopSize:], a.hop)
	a.hop = a.hop[:0]
	a.envelope = append(a.envelope, a.flux())
}

func (a *Analyzer) countSilence(hop []float64) {
	var sum float64
	for _, x := range hop {
		sum += x * x
	}
	a.hops++
	if math.Sqrt(sum/float64(len(hop))) < silenceRMS {
		a.silentHops++
	}
}

// flux is the positive log-magnitude change of the current frame.
func (a *Analyzer) flux() float64 {
	for i, x := range a.frame {
		a.re[i], a.im[i] = x*a.window[i], 0
	}
	a.fft.transform(a.re, a.im)
	var sum float64
	for k := range a.prevMag {
		mag := math.Log1p(logGain * math.Hypot(a.re[k], a.im[k]))
		if d := mag - a.prevMag[k]; d > 0 && len(a.envelope) > 0 {
			sum += d
		}
		a.prevMag[k] = mag
	}
	return sum
}

// Result finishes the stream and measures it.
func (a *Analyzer) Result() Rhythm {
	if len(a.hop) > 0 {
		a.countSilence(a.hop)
		a.hop = a.hop[:0]
	}
	r := Rhythm{Samples: a.samples}
	if a.hops > 0 {
		r.SilenceRatio = float64(a.silentHops) / float64(a.hops)
	}
	r.BPM, r.Confidence = Tempo(a.envelope)
	return r
}

// Tempo estimates the beat of an onset envelope sampled at fps; 0, 0 when
// it has no tempo.
func Tempo(envelope []float64) (bpm, confidence float64) {
	minLag := int(math.Floor(60 * fps / domain.MaxBPM))
	maxLag := int(math.Ceil(60 * fps / domain.MinBPM))
	if len(envelope) < minTempoSeconds*SampleRate/hopSize || len(envelope) < 2*(maxLag+1) {
		return 0, 0
	}
	e := onsets(envelope)
	var r0 float64
	for _, v := range e {
		r0 += v * v
	}
	if r0 == 0 {
		return 0, 0
	}
	n := float64(len(e))
	acf := make([]float64, maxLag+2)
	for lag := minLag - 1; lag <= maxLag+1; lag++ {
		var s float64
		for i := 0; i+lag < len(e); i++ {
			s += e[i] * e[i+lag]
		}
		// Unbiased and normalized: a perfectly periodic envelope scores 1.
		acf[lag] = s / (n - float64(lag)) * n / r0
	}

	best, bestScore := 0, 0.0
	for lag := minLag; lag <= maxLag; lag++ {
		bpm := 60 * fps / float64(lag)
		if bpm < domain.MinBPM || bpm > domain.MaxBPM || acf[lag] <= 0 {
			continue
		}
		if s := acf[lag] * prior(bpm); s > bestScore {
			best, bestScore = lag, s
		}
	}
	if best == 0 || acf[best] < minConfidence {
		return 0, 0
	}
	// Beats at every period also repeat at twice the period, so the slower
	// octave scores as high as the true tempo; when half the lag is almost as
	// periodic, the events between the picked beats are beats too.
	if half := int(math.Round(float64(best) / 2)); half >= minLag && acf[half] >= octaveRatio*acf[best] {
		best = half
	}
	lag := float64(best) + parabolic(acf[best-1], acf[best], acf[best+1])
	bpm = math.Min(math.Max(60*fps/lag, domain.MinBPM), domain.MaxBPM)
	return math.Round(bpm*10) / 10, math.Min(acf[best], 1)
}

// onsets removes the envelope's local mean (half a second around each
// value), keeps what rises above it and centers the result.
func onsets(env []float64) []float64 {
	half := SampleRate / hopSize / 4
	prefix := make([]float64, len(env)+1)
	for i, v := range env {
		prefix[i+1] = prefix[i] + v
	}
	out := make([]float64, len(env))
	var mean float64
	for i, v := range env {
		lo, hi := max(0, i-half), min(len(env), i+half+1)
		out[i] = math.Max(0, v-(prefix[hi]-prefix[lo])/float64(hi-lo))
		mean += out[i]
	}
	mean /= float64(len(out))
	for i := range out {
		out[i] -= mean
	}
	return out
}

// prior is a log-normal tempo weight: humans tap near priorBPM.
func prior(bpm float64) float64 {
	o := math.Log2(bpm/priorBPM) / priorOctaves
	return math.Exp(-0.5 * o * o)
}

// parabolic is the offset of the vertex of the parabola through three
// equally spaced points, in [-0.5, 0.5].
func parabolic(l, c, r float64) float64 {
	d := l - 2*c + r
	if d >= 0 {
		return 0
	}
	return math.Max(-0.5, math.Min(0.5, 0.5*(l-r)/d))
}
