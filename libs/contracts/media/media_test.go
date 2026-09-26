package media

import "testing"

func TestKeys(t *testing.T) {
	if got := TrackAudioKey("t1", Quality128); got != "tracks/t1/audio/128.aac" {
		t.Fatal(got)
	}
	if got := TrackOriginalKey("t1", "u1", "flac"); got != "tracks/t1/original/u1.flac" {
		t.Fatal(got)
	}
	if got := TrackCoverKey("t1"); got != "tracks/t1/cover/cover.webp" {
		t.Fatal(got)
	}
}

func TestParseQuality(t *testing.T) {
	if q, err := ParseQuality("256"); err != nil || q != Quality256 {
		t.Fatal(q, err)
	}
	for _, bad := range []string{"", "96", "abc", "-64"} {
		if _, err := ParseQuality(bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}
