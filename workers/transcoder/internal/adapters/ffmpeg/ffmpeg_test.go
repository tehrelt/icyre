package ffmpeg

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/workers/transcoder/internal/application"
)

func encoder(t *testing.T) *FFmpeg {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s is not on PATH", bin)
		}
	}
	return New("ffmpeg", "ffprobe")
}

// master writes a 3-second FLAC tone.
func master(t *testing.T, dir string) string {
	t.Helper()
	src := filepath.Join(dir, "master.flac")
	out, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=3",
		"-ac", "1", "-ar", "48000", src).CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return src
}

func TestProbeAndEncode(t *testing.T) {
	f := encoder(t)
	ctx := context.Background()
	dir := t.TempDir()
	src := master(t, dir)

	ms, err := f.Probe(ctx, src)
	if err != nil || ms < 2900 || ms > 3100 {
		t.Fatalf("duration %d %v", ms, err)
	}

	dst := map[media.Quality]string{}
	for _, q := range media.Qualities {
		dst[q] = filepath.Join(dir, q.String()+".aac")
	}
	if err := f.Encode(ctx, src, dst); err != nil {
		t.Fatal(err)
	}
	var prev int64
	for _, q := range media.Qualities {
		st, err := os.Stat(dst[q])
		if err != nil || st.Size() == 0 {
			t.Fatalf("%d kbps: %v", q, err)
		}
		if st.Size() <= prev {
			t.Fatalf("%d kbps is not larger than the lower variant: %d <= %d", q, st.Size(), prev)
		}
		prev = st.Size()
		// Variants are decodable ADTS with the source duration.
		head, _ := os.ReadFile(dst[q])
		if head[0] != 0xFF || head[1]&0xF0 != 0xF0 {
			t.Fatalf("%d kbps: no ADTS sync word", q)
		}
		if ms, err := f.Probe(ctx, dst[q]); err != nil || ms < 2900 || ms > 3200 {
			t.Fatalf("%d kbps: duration %d %v", q, ms, err)
		}
	}
}

func TestProbeRejectsNonAudio(t *testing.T) {
	f := encoder(t)
	bad := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(bad, []byte("definitely not audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Probe(context.Background(), bad); !errors.Is(err, application.ErrUndecodable) {
		t.Fatal(err)
	}
}

func TestMissingBinaryIsNotUndecodable(t *testing.T) {
	f := New("ffmpeg-does-not-exist", "ffprobe-does-not-exist")
	_, err := f.Probe(context.Background(), "x")
	if err == nil || errors.Is(err, application.ErrUndecodable) {
		t.Fatal(err)
	}
	if f.Version(context.Background()) == nil {
		t.Fatal("version check passed without binaries")
	}
}
