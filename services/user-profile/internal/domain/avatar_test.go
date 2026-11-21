package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAvatarUploadRequestValidate(t *testing.T) {
	good := AvatarUploadRequest{
		ContentType: "image/webp",
		SizeBytes:   1024,
		SHA256Hex:   strings.Repeat("ab", 32),
	}
	f, sum, err := good.Validate(5 << 20)
	if err != nil || f.Ext != "webp" || len(sum) != 32 {
		t.Fatalf("got f=%+v len=%d err=%v", f, len(sum), err)
	}

	var ve *ValidationError
	_, _, err = AvatarUploadRequest{
		ContentType: "image/gif",
		SizeBytes:   0,
		SHA256Hex:   "not-hex",
	}.Validate(5 << 20)
	if !errors.As(err, &ve) || len(ve.Fields) != 3 {
		t.Fatalf("expected 3 validation errors, got %v (%+v)", err, ve)
	}

	_, _, err = AvatarUploadRequest{
		ContentType: "image/png",
		SizeBytes:   101,
		SHA256Hex:   strings.Repeat("cd", 32),
	}.Validate(100)
	if !errors.As(err, &ve) || ve.Fields["sizeBytes"] == "" {
		t.Fatalf("expected sizeBytes error, got %v", err)
	}
}

func TestAvatarFormatSniffing(t *testing.T) {
	cases := []struct {
		ct   string
		head []byte
		ok   bool
	}{
		{"image/jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, true},
		{"image/jpeg", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, false},
		{"image/png", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}, true},
		{"image/png", []byte("RIFF\x00\x00\x00\x00WEBP"), false},
		{"image/webp", []byte("RIFF\x24\x08\x00\x00WEBP"), true},
		{"image/webp", []byte("RIFF\x24\x08\x00\x00WAVE"), false},
		{"image/webp", []byte("RIFF\x24\x08"), false},
	}
	for _, c := range cases {
		f, ok := AvatarFormatOf(c.ct)
		if !ok || f.Matches(c.head) != c.ok {
			t.Errorf("%s %x: want %v", c.ct, c.head, c.ok)
		}
	}
	if _, ok := AvatarFormatOf("image/gif"); ok {
		t.Fatal("image/gif should not be supported")
	}
	if (AvatarFormat{}).Matches([]byte{0xFF, 0xD8, 0xFF}) {
		t.Fatal("zero AvatarFormat should not match")
	}
}

func TestAvatarUploadSettle(t *testing.T) {
	at := time.Date(2026, 11, 22, 10, 0, 0, 123456789, time.UTC)
	u := AvatarUpload{Status: AvatarStatusPending}

	done := u.Settle("", at)
	if done.Status != AvatarStatusCompleted || done.FailureReason != "" || done.CompletedAt == nil || u.Status != AvatarStatusPending {
		t.Fatalf("completed=%+v orig=%+v", done, u)
	}
	if !done.CompletedAt.Equal(at.Truncate(time.Microsecond)) {
		t.Fatalf("completedAt=%v", done.CompletedAt)
	}

	failed := u.Settle(ReasonSizeMismatch, at)
	if failed.Status != AvatarStatusFailed || failed.FailureReason != ReasonSizeMismatch || failed.CompletedAt == nil {
		t.Fatalf("failed=%+v", failed)
	}
}
