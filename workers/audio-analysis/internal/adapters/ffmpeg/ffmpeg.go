// Package ffmpeg analyzes audio with the ffmpeg binary: one decoding pass
// measures EBU R128 loudness (ebur128 filter) and streams mono PCM to the
// tempo and silence analysis (internal/dsp).
package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/tehrelt/icyre/workers/audio-analysis/internal/application"
	"github.com/tehrelt/icyre/workers/audio-analysis/internal/domain"
	"github.com/tehrelt/icyre/workers/audio-analysis/internal/dsp"
)

// stderrTail bounds the kept ffmpeg log; the loudness summary is its end.
const stderrTail = 16 << 10

// FFmpeg implements application.Analyzer.
type FFmpeg struct{ bin string }

// New returns an analyzer using bin (a name on PATH or a path).
func New(bin string) *FFmpeg { return &FFmpeg{bin: bin} }

// Version checks that the binary runs (startup check).
func (f *FFmpeg) Version(ctx context.Context) error {
	if err := exec.CommandContext(ctx, f.bin, "-version").Run(); err != nil {
		return fmt.Errorf("%s: %w", f.bin, err)
	}
	return nil
}

// Analyze implements application.Analyzer: src is a local path or an
// http(s) URL, read once from start to end. ffmpeg rejecting the input
// (no audio stream included) is ErrUndecodable; ffmpeg that could not run
// at all is retryable.
func (f *FFmpeg) Analyze(ctx context.Context, src string) (domain.Features, error) {
	graph := fmt.Sprintf("[0:a:0]asplit=2[l][p];[l]ebur128=peak=true:framelog=verbose[lo];"+
		"[p]aresample=%d,aformat=sample_fmts=flt:channel_layouts=mono[po]", dsp.SampleRate)
	// Only plain files and HTTP(S): a crafted master (an HLS or concat
	// playlist) must not make ffmpeg open other files or hosts.
	cmd := exec.CommandContext(ctx, f.bin, "-hide_banner", "-nostdin", "-nostats", "-loglevel", "info",
		"-protocol_whitelist", "file,http,https,tcp,tls", "-i", src,
		"-filter_complex", graph, "-map", "[lo]", "-f", "null", "-", "-map", "[po]", "-f", "f32le", "-")
	stderr := &tail{max: stderrTail}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return domain.Features{}, err
	}
	if err := cmd.Start(); err != nil {
		return domain.Features{}, fmt.Errorf("%s: %w", f.bin, err)
	}
	analyzer := dsp.NewAnalyzer()
	_, copyErr := io.Copy(analyzer, stdout)
	err = cmd.Wait()
	if ctx.Err() != nil {
		return domain.Features{}, ctx.Err() // killed by the job deadline, not the file's fault
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		msg := strings.ReplaceAll(lastLines(stderr.String(), 5), src, redact(src))
		return domain.Features{}, fmt.Errorf("%w: %s", application.ErrUndecodable, truncate(msg, 300))
	}
	if err != nil {
		return domain.Features{}, fmt.Errorf("%s: %w", f.bin, err)
	}
	if copyErr != nil {
		return domain.Features{}, fmt.Errorf("read pcm: %w", copyErr)
	}
	loud, err := ParseLoudness(stderr.String())
	if err != nil {
		return domain.Features{}, err
	}
	r := analyzer.Result()
	if r.Samples == 0 {
		return domain.Features{}, fmt.Errorf("%w: no audio decoded", application.ErrUndecodable)
	}
	return domain.Features{
		BPM: r.BPM, BPMConfidence: round(r.Confidence, 3), SilenceRatio: round(r.SilenceRatio, 4), AnalyzedMs: r.DurationMs(),
		IntegratedLUFS: loud.Integrated, LoudnessRangeLU: loud.Range, TruePeakDBTP: loud.TruePeak,
	}, nil
}

// Loudness is the ebur128 summary.
type Loudness struct {
	Integrated float64 // LUFS
	Range      float64 // LU
	TruePeak   float64 // dBTP
}

var (
	reIntegrated = regexp.MustCompile(`(?m)^\s*I:\s+(\S+) LUFS`)
	reRange      = regexp.MustCompile(`(?m)^\s*LRA:\s+(\S+) LU`)
	rePeak       = regexp.MustCompile(`(?m)^\s*Peak:\s+(\S+) dBFS`)
)

// ParseLoudness reads the summary the ebur128 filter logs at the end of
// the stream. Levels of silence (-inf, or below the gate) are domain.Floor.
func ParseLoudness(log string) (Loudness, error) {
	_, summary, ok := strings.Cut(log, "Summary:")
	if !ok {
		return Loudness{}, fmt.Errorf("%w: no loudness summary", application.ErrUndecodable)
	}
	var l Loudness
	for _, f := range []struct {
		re  *regexp.Regexp
		dst *float64
	}{{reIntegrated, &l.Integrated}, {reRange, &l.Range}, {rePeak, &l.TruePeak}} {
		m := f.re.FindStringSubmatch(summary)
		if m == nil {
			return Loudness{}, fmt.Errorf("%w: loudness summary without %s", application.ErrUndecodable, f.re)
		}
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return Loudness{}, fmt.Errorf("%w: loudness value %q", application.ErrUndecodable, m[1])
		}
		*f.dst = v
	}
	l.Integrated, l.TruePeak = floor(l.Integrated), floor(l.TruePeak)
	if l.Range < 0 || math.IsNaN(l.Range) {
		l.Range = 0
	}
	return l, nil
}

func floor(v float64) float64 {
	if math.IsNaN(v) || v < domain.Floor {
		return domain.Floor
	}
	return v
}

func round(v float64, digits int) float64 {
	p := math.Pow(10, float64(digits))
	return math.Round(v*p) / p
}

// tail keeps the last max bytes written.
type tail struct {
	buf []byte
	max int
}

func (t *tail) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if over := len(t.buf) - t.max; over > 0 {
		t.buf = append(t.buf[:0], t.buf[over:]...)
	}
	return len(p), nil
}

func (t *tail) String() string { return string(t.buf) }

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}

// redact drops the query of a URL: a presigned URL's signature must not
// reach logs through ffmpeg's error output.
func redact(src string) string {
	if base, _, ok := strings.Cut(src, "?"); ok {
		return base + "?REDACTED"
	}
	return src
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
