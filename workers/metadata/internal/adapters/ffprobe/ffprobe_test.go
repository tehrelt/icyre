package ffprobe

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tehrelt/icyre/workers/metadata/internal/application"
	"github.com/tehrelt/icyre/workers/metadata/internal/domain"
)

func parse(t *testing.T, raw string) (domain.Probe, error) {
	t.Helper()
	var out Output
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	return Parse(out)
}

func TestParse(t *testing.T) {
	cases := map[string]struct {
		raw  string
		want domain.Probe
	}{
		"flac": {
			raw:  `{"streams":[{"codec_type":"audio","codec_name":"flac","sample_rate":"44100","channels":2}],"format":{"format_name":"flac","duration":"215.012000","bit_rate":"912345"}}`,
			want: domain.Probe{Container: "flac", Codec: "flac", DurationMs: 215012, BitrateBps: 912345, SampleRateHz: 44100, Channels: 2},
		},
		"stream duration wins, format list trimmed": {
			raw:  `{"streams":[{"codec_type":"audio","codec_name":"aac","sample_rate":"48000","channels":1,"duration":"10.5","bit_rate":"128000"}],"format":{"format_name":"mov,mp4,m4a","duration":"10.6","bit_rate":"N/A"}}`,
			want: domain.Probe{Container: "mov", Codec: "aac", DurationMs: 10500, BitrateBps: 128000, SampleRateHz: 48000, Channels: 1},
		},
		"no bitrate": {
			raw:  `{"streams":[{"codec_type":"audio","codec_name":"mp3","sample_rate":"44100","channels":2,"duration":"N/A"}],"format":{"format_name":"mp3","duration":"3.0"}}`,
			want: domain.Probe{Container: "mp3", Codec: "mp3", DurationMs: 3000, SampleRateHz: 44100, Channels: 2},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := parse(t, tc.raw)
			if err != nil || got != tc.want {
				t.Fatalf("Parse = %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	for name, raw := range map[string]string{
		"no audio stream": `{"streams":[],"format":{"format_name":"flac","duration":"1.0"}}`,
		"no duration":     `{"streams":[{"codec_type":"audio","codec_name":"flac","sample_rate":"44100","channels":2}],"format":{"format_name":"flac","duration":"N/A"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parse(t, raw); !errors.Is(err, application.ErrUndecodable) {
				t.Fatalf("err = %v, want ErrUndecodable", err)
			}
		})
	}
}

// wav writes a silent 16-bit PCM WAV file.
func wav(t *testing.T, rate, channels, seconds int) string {
	t.Helper()
	data := make([]byte, rate*channels*2*seconds)
	hdr := make([]byte, 44)
	copy(hdr, "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(36+len(data)))
	copy(hdr[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(hdr[16:], 16)
	binary.LittleEndian.PutUint16(hdr[20:], 1)
	binary.LittleEndian.PutUint16(hdr[22:], uint16(channels))
	binary.LittleEndian.PutUint32(hdr[24:], uint32(rate))
	binary.LittleEndian.PutUint32(hdr[28:], uint32(rate*channels*2))
	binary.LittleEndian.PutUint16(hdr[32:], uint16(channels*2))
	binary.LittleEndian.PutUint16(hdr[34:], 16)
	copy(hdr[36:], "data")
	binary.LittleEndian.PutUint32(hdr[40:], uint32(len(data)))
	path := filepath.Join(t.TempDir(), "a.wav")
	if err := os.WriteFile(path, append(hdr, data...), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestProbeWithBinary(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not on PATH")
	}
	f := New("ffprobe")
	got, err := f.Probe(context.Background(), wav(t, 22050, 1, 2))
	want := domain.Probe{Container: "wav", Codec: "pcm_s16le", DurationMs: 2000, BitrateBps: 352800, SampleRateHz: 22050, Channels: 1}
	if err != nil || got != want {
		t.Fatalf("Probe = %+v, %v; want %+v", got, err, want)
	}

	junk := filepath.Join(t.TempDir(), "junk.flac")
	if err := os.WriteFile(junk, []byte("not audio at all"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Probe(context.Background(), junk); !errors.Is(err, application.ErrUndecodable) {
		t.Fatalf("junk: err = %v, want ErrUndecodable", err)
	}
}

func TestRedact(t *testing.T) {
	if got := redact("http://minio:9000/b/k.flac?X-Amz-Signature=abc"); got != "http://minio:9000/b/k.flac?REDACTED" {
		t.Fatalf("redact = %q", got)
	}
	if got := redact("/tmp/a.wav"); got != "/tmp/a.wav" {
		t.Fatalf("redact = %q", got)
	}
}
