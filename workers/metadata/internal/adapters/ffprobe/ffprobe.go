// Package ffprobe reads technical audio metadata with the ffprobe binary.
package ffprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tehrelt/icyre/workers/metadata/internal/application"
	"github.com/tehrelt/icyre/workers/metadata/internal/domain"
)

// FFprobe implements application.Prober.
type FFprobe struct{ bin string }

// New returns a prober using bin (a name on PATH or a path).
func New(bin string) *FFprobe { return &FFprobe{bin: bin} }

// Version checks that the binary runs (startup check).
func (f *FFprobe) Version(ctx context.Context) error {
	if err := exec.CommandContext(ctx, f.bin, "-version").Run(); err != nil {
		return fmt.Errorf("%s: %w", f.bin, err)
	}
	return nil
}

// Output is the part of `ffprobe -of json` used here. Numbers come as
// strings, and any of them may be absent or "N/A".
type Output struct {
	Streams []struct {
		CodecType  string `json:"codec_type"`
		CodecName  string `json:"codec_name"`
		SampleRate string `json:"sample_rate"`
		Channels   int    `json:"channels"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"streams"`
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
}

// Probe implements application.Prober: src is a local path or an
// http(s) URL (ffprobe reads only what it needs, with Range requests).
// ffprobe rejecting the input, or an input without an audio stream, is
// ErrUndecodable; a probe that could not run at all is retryable.
func (f *FFprobe) Probe(ctx context.Context, src string) (domain.Probe, error) {
	var stdout, stderr bytes.Buffer
	// Only plain files and HTTP(S): a crafted master (an HLS or concat
	// playlist) must not make ffprobe open other files or hosts.
	cmd := exec.CommandContext(ctx, f.bin, "-v", "error", "-protocol_whitelist", "file,http,https,tcp,tls", "-select_streams", "a:0",
		"-show_entries", "stream=codec_type,codec_name,sample_rate,channels,duration,bit_rate:format=format_name,duration,bit_rate",
		"-of", "json", src)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return domain.Probe{}, ctx.Err() // killed by the job deadline, not the file's fault
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		msg := strings.ReplaceAll(strings.TrimSpace(stderr.String()), src, redact(src))
		return domain.Probe{}, fmt.Errorf("%w: %s", application.ErrUndecodable, truncate(msg, 300))
	}
	if err != nil {
		return domain.Probe{}, fmt.Errorf("%s: %w", f.bin, err)
	}
	var out Output
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return domain.Probe{}, fmt.Errorf("%w: ffprobe output: %v", application.ErrUndecodable, err)
	}
	return Parse(out)
}

// Parse maps ffprobe output to a probe: the first audio stream, stream
// values before format ones (the container bitrate counts headers too),
// the first demuxer name of a list ("mov,mp4").
func Parse(out Output) (domain.Probe, error) {
	for _, s := range out.Streams {
		if s.CodecType != "audio" {
			continue
		}
		container, _, _ := strings.Cut(out.Format.FormatName, ",")
		p := domain.Probe{
			Container:    container,
			Codec:        s.CodecName,
			DurationMs:   millis(first(s.Duration, out.Format.Duration)),
			BitrateBps:   integer(first(s.BitRate, out.Format.BitRate)),
			SampleRateHz: int(integer(s.SampleRate)),
			Channels:     s.Channels,
		}
		if p.DurationMs <= 0 {
			return domain.Probe{}, fmt.Errorf("%w: no duration", application.ErrUndecodable)
		}
		return p, nil
	}
	return domain.Probe{}, fmt.Errorf("%w: no audio stream", application.ErrUndecodable)
}

// first returns the first value that is set.
func first(vs ...string) string {
	for _, v := range vs {
		if v != "" && v != "N/A" {
			return v
		}
	}
	return ""
}

func millis(secs string) int64 {
	f, err := strconv.ParseFloat(secs, 64)
	if err != nil || f <= 0 || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0
	}
	return int64(math.Round(f * 1000))
}

func integer(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// redact drops the query of a URL: a presigned URL's signature must not
// reach logs through ffprobe's error output.
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
