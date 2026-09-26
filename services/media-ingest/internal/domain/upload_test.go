package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRequestValidate(t *testing.T) {
	good := Request{TrackID: uuid.New(), ContentType: "audio/flac", SizeBytes: 10, SHA256Hex: strings.Repeat("ab", 32)}
	f, sum, err := good.Validate(100)
	if err != nil || f.Ext != "flac" || len(sum) != 32 {
		t.Fatal(f, sum, err)
	}
	var ve *ValidationError
	_, _, err = Request{ContentType: "video/mp4", SizeBytes: 101, SHA256Hex: "zz"}.Validate(100)
	if !errors.As(err, &ve) || len(ve.Fields) != 4 {
		t.Fatalf("%v %+v", err, ve)
	}
}

func TestFormatSniffing(t *testing.T) {
	cases := []struct {
		ct   string
		head string
		ok   bool
	}{
		{"audio/flac", "fLaC\x00\x00\x00\x22", true},
		{"audio/flac", "RIFF\x00\x00\x00\x00WAVE", false},
		{"audio/wav", "RIFF\x24\x08\x00\x00WAVE", true},
		{"audio/x-wav", "RIFF\x24\x08\x00\x00AVI ", false},
		{"audio/mpeg", "ID3\x04\x00", true},
		{"audio/mpeg", "\xff\xfb\x90\x00", true},
		{"audio/mpeg", "<html>", false},
	}
	for _, c := range cases {
		f, ok := FormatOf(c.ct)
		if !ok || f.Matches([]byte(c.head)) != c.ok {
			t.Errorf("%s %q: want %v", c.ct, c.head, c.ok)
		}
	}
}

func TestSettle(t *testing.T) {
	at := time.Now()
	u := Upload{Status: StatusPending}
	if d := u.Settle("", at); d.Status != StatusCompleted || d.CompletedAt == nil || u.Status != StatusPending {
		t.Fatal(d, u)
	}
	if f := u.Settle("SIZE_MISMATCH", at); f.Status != StatusFailed || f.FailureReason != "SIZE_MISMATCH" {
		t.Fatal(f)
	}
}
