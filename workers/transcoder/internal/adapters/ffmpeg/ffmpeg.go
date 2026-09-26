// Package ffmpeg probes and encodes audio with the ffmpeg/ffprobe binaries.
package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/workers/transcoder/internal/application"
)

// Output format of every variant: AAC-LC in ADTS, stereo, 44.1 kHz.
const (
	sampleRate = "44100"
	channels   = "2"
)

// FFmpeg implements application.Encoder.
type FFmpeg struct {
	ffmpeg, ffprobe string
}

// New returns an encoder using the given binaries (names on PATH or paths).
func New(ffmpeg, ffprobe string) *FFmpeg { return &FFmpeg{ffmpeg: ffmpeg, ffprobe: ffprobe} }

// Version checks that both binaries run (startup check).
func (f *FFmpeg) Version(ctx context.Context) error {
	for _, bin := range []string{f.ffmpeg, f.ffprobe} {
		if err := exec.CommandContext(ctx, bin, "-version").Run(); err != nil {
			return fmt.Errorf("%s: %w", bin, err)
		}
	}
	return nil
}

type stream struct {
	CodecType string `json:"codec_type"`
}

type probe struct {
	Streams []stream `json:"streams"`
	Format  struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// Probe implements application.Encoder. ffprobe rejecting the file, or a
// file without an audio stream or duration, is ErrUndecodable; a probe
// that could not run at all is a plain (retryable) error.
func (f *FFmpeg) Probe(ctx context.Context, src string) (int64, error) {
	out, err := run(ctx, f.ffprobe, "-v", "error", "-select_streams", "a:0",
		"-show_entries", "stream=codec_type:format=duration", "-of", "json", src)
	if ctx.Err() != nil {
		return 0, ctx.Err() // killed by the job deadline, not the file's fault
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return 0, fmt.Errorf("%w: %v", application.ErrUndecodable, err)
	}
	if err != nil {
		return 0, err
	}
	var p probe
	if err := json.Unmarshal(out, &p); err != nil {
		return 0, fmt.Errorf("%w: ffprobe output: %v", application.ErrUndecodable, err)
	}
	if !slices.ContainsFunc(p.Streams, func(s stream) bool {
		return s.CodecType == "audio"
	}) {
		return 0, fmt.Errorf("%w: no audio stream", application.ErrUndecodable)
	}
	secs, err := strconv.ParseFloat(p.Format.Duration, 64)
	if err != nil || secs <= 0 || math.IsInf(secs, 0) {
		return 0, fmt.Errorf("%w: duration %q", application.ErrUndecodable, p.Format.Duration)
	}
	return int64(math.Round(secs * 1000)), nil
}

// Encode implements application.Encoder: one decode, one output per
// variant, metadata stripped.
func (f *FFmpeg) Encode(ctx context.Context, src string, dst map[media.Quality]string) error {
	args := []string{"-nostdin", "-v", "error", "-y", "-i", src}
	for _, q := range media.Qualities {
		path, ok := dst[q]
		if !ok {
			continue
		}
		args = append(args, "-map", "0:a:0", "-vn", "-map_metadata", "-1",
			"-ac", channels, "-ar", sampleRate, "-c:a", "aac", "-b:a", q.String()+"k", "-f", "adts", path)
	}
	_, err := run(ctx, f.ffmpeg, args...)
	return err
}

func run(ctx context.Context, bin string, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%s: %w: %s", bin, err, truncate(msg, 500))
		}
		return nil, fmt.Errorf("%s: %w", bin, err)
	}
	return stdout.Bytes(), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
