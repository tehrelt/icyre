package ffmpeg

import (
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tehrelt/icyre/workers/audio-analysis/internal/application"
	"github.com/tehrelt/icyre/workers/audio-analysis/internal/domain"
)

const summary = `[Parsed_ebur128_1 @ 0x55d] Summary:

  Integrated loudness:
    I:         -17.0 LUFS
    Threshold: -28.0 LUFS

  Loudness range:
    LRA:         6.4 LU
    Threshold: -38.0 LUFS
    LRA low:   -21.0 LUFS
    LRA high:  -14.6 LUFS

  True peak:
    Peak:       -2.0 dBFS
[out#0/null @ 0x55e] video:0KiB audio:2067KiB`

func TestParseLoudness(t *testing.T) {
	got, err := ParseLoudness("Input #0, wav\r\n" + strings.ReplaceAll(summary, "\n", "\r\n"))
	if err != nil || got != (Loudness{Integrated: -17, Range: 6.4, TruePeak: -2}) {
		t.Fatalf("ParseLoudness = %+v, %v", got, err)
	}
}

func TestParseLoudnessOfSilence(t *testing.T) {
	log := strings.NewReplacer("-17.0 LUFS", "-70.0 LUFS", "6.4 LU", "0.0 LU", "-2.0 dBFS", "-inf dBFS").Replace(summary)
	got, err := ParseLoudness(log)
	if err != nil || got != (Loudness{Integrated: domain.Floor, Range: 0, TruePeak: domain.Floor}) {
		t.Fatalf("ParseLoudness = %+v, %v", got, err)
	}
}

func TestParseLoudnessWithoutSummary(t *testing.T) {
	for name, log := range map[string]string{
		"no summary": "Input #0, wav",
		"no peak":    summary[:strings.Index(summary, "True peak")],
	} {
		if _, err := ParseLoudness(log); !errors.Is(err, application.ErrUndecodable) {
			t.Errorf("%s: err = %v, want ErrUndecodable", name, err)
		}
	}
}

func TestRedact(t *testing.T) {
	if got := redact("http://minio:9000/b/k.flac?X-Amz-Signature=secret"); got != "http://minio:9000/b/k.flac?REDACTED" {
		t.Fatalf("redact = %q", got)
	}
}

func TestTailKeepsTheEnd(t *testing.T) {
	tl := &tail{max: 4}
	_, _ = tl.Write([]byte("abc"))
	_, _ = tl.Write([]byte("defg"))
	if tl.String() != "defg" {
		t.Fatalf("tail = %q", tl.String())
	}
}

// generate renders a lavfi source to a file with the real ffmpeg.
func generate(t *testing.T, name, source string) string {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	path := filepath.Join(t.TempDir(), name)
	out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", source, path).CombinedOutput()
	if err != nil {
		t.Fatalf("generate %s: %v: %s", name, err, out)
	}
	return path
}

func TestAnalyzeClickTrack(t *testing.T) {
	// A decaying 1 kHz click every 0.5 s: 120 BPM, stereo FLAC.
	click := "0.8*exp(-40*mod(t,0.5))*sin(2*PI*1000*t)"
	src := generate(t, "click.flac", "aevalsrc='"+click+"|"+click+"':s=44100:d=20")
	f, err := New("ffmpeg").Analyze(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(f.BPM-120) > 1 || f.BPMConfidence < 0.3 || f.AnalyzedMs < 19900 || f.AnalyzedMs > 20100 {
		t.Fatalf("tempo: %+v", f)
	}
	if f.IntegratedLUFS > -10 || f.IntegratedLUFS < -30 || f.TruePeakDBTP > 0 || f.TruePeakDBTP < -5 {
		t.Fatalf("loudness: %+v", f)
	}
	if err := f.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzeSilence(t *testing.T) {
	src := generate(t, "silence.wav", "anullsrc=r=44100:cl=mono:d=8")
	f, err := New("ffmpeg").Analyze(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.Features{IntegratedLUFS: domain.Floor, TruePeakDBTP: domain.Floor, SilenceRatio: 1, AnalyzedMs: 8000}
	if f != want {
		t.Fatalf("silence = %+v, want %+v", f, want)
	}
}

func TestAnalyzeUndecodable(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	path := filepath.Join(t.TempDir(), "junk.flac")
	if err := os.WriteFile(path, []byte("not audio at all"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New("ffmpeg").Analyze(context.Background(), path); !errors.Is(err, application.ErrUndecodable) {
		t.Fatalf("err = %v, want ErrUndecodable", err)
	}
}

func TestAnalyzeWithoutBinaryIsRetryable(t *testing.T) {
	_, err := New("no-such-ffmpeg").Analyze(context.Background(), "x.flac")
	if err == nil || errors.Is(err, application.ErrUndecodable) {
		t.Fatalf("err = %v, want a retryable error", err)
	}
}
