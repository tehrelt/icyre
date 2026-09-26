// Package domain holds upload sessions: a presigned PUT of a track master
// that is verified before the media pipeline sees it.
package domain

import (
	"bytes"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Status is the lifecycle of an upload session.
type Status string

// Statuses. PENDING → COMPLETED | FAILED; both ends are final.
const (
	StatusPending   Status = "PENDING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
)

// Errors.
var (
	ErrNotFound      = errors.New("upload not found")
	ErrTrackNotFound = errors.New("track not found")
	ErrForbidden     = errors.New("only artists and admins can upload media")
	// ErrNotUploaded: complete was called before the object arrived.
	ErrNotUploaded = errors.New("object has not been uploaded yet")
)

// ValidationError lists invalid request fields.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "invalid upload request" }

// RejectedError means the stored object failed verification.
type RejectedError struct{ Reason string }

func (e *RejectedError) Error() string { return "upload rejected: " + e.Reason }

// Format is an accepted master format.
type Format struct {
	ContentType string
	Ext         string
	// sniff recognises the format by its first bytes.
	sniff func(head []byte) bool
}

// SniffLen is how many leading bytes format detection needs.
const SniffLen = 12

var formats = []Format{
	{ContentType: "audio/flac", Ext: "flac", sniff: func(b []byte) bool { return bytes.HasPrefix(b, []byte("fLaC")) }},
	{ContentType: "audio/wav", Ext: "wav", sniff: isWAV},
	{ContentType: "audio/x-wav", Ext: "wav", sniff: isWAV},
	{ContentType: "audio/mpeg", Ext: "mp3", sniff: isMP3},
}

func isWAV(b []byte) bool {
	return len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WAVE"
}

// isMP3 accepts an ID3v2 tag or a bare MPEG audio frame sync.
func isMP3(b []byte) bool {
	return bytes.HasPrefix(b, []byte("ID3")) || (len(b) >= 2 && b[0] == 0xFF && b[1]&0xE0 == 0xE0)
}

// FormatOf returns the format for a declared content type.
func FormatOf(contentType string) (Format, bool) {
	for _, f := range formats {
		if f.ContentType == contentType {
			return f, true
		}
	}
	return Format{}, false
}

// Matches reports whether the object's head is really this format.
func (f Format) Matches(head []byte) bool { return f.sniff(head) }

// Upload is one upload session.
type Upload struct {
	ID          uuid.UUID
	TrackID     uuid.UUID
	UploaderID  uuid.UUID
	ContentType string
	SizeBytes   int64
	SHA256      []byte
	Key         string
	Status      Status
	// FailureReason is set for FAILED (mediav1.Reason*).
	FailureReason string
	CreatedAt     time.Time
	// ExpiresAt is when the upload URL stops working.
	ExpiresAt   time.Time
	CompletedAt *time.Time
}

// Request is what a client declares before uploading.
type Request struct {
	TrackID     uuid.UUID
	ContentType string
	SizeBytes   int64
	SHA256Hex   string
}

// Validate checks a request against the size limit and accepted formats,
// returning the format and decoded digest.
func (r Request) Validate(maxSize int64) (Format, []byte, error) {
	fields := map[string]string{}
	if r.TrackID == uuid.Nil {
		fields["trackId"] = "is required"
	}
	f, ok := FormatOf(r.ContentType)
	if !ok {
		fields["contentType"] = "must be audio/flac, audio/wav, audio/x-wav or audio/mpeg"
	}
	if r.SizeBytes <= 0 || r.SizeBytes > maxSize {
		fields["sizeBytes"] = "must be between 1 and the upload limit"
	}
	sum, err := hex.DecodeString(r.SHA256Hex)
	if err != nil || len(sum) != 32 {
		fields["sha256"] = "must be a hex SHA-256 digest"
	}
	if len(fields) > 0 {
		return Format{}, nil, &ValidationError{Fields: fields}
	}
	return f, sum, nil
}

// Settle ends a pending upload: completed when reason is empty, failed
// otherwise.
func (u Upload) Settle(reason string, at time.Time) Upload {
	u.CompletedAt = &at
	u.Status, u.FailureReason = StatusCompleted, ""
	if reason != "" {
		u.Status, u.FailureReason = StatusFailed, reason
	}
	return u
}
