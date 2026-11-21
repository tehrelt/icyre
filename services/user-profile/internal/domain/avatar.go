package domain

import (
	"bytes"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Avatar upload errors.
var (
	ErrAvatarUploadNotFound = errors.New("avatar upload not found")
	ErrAvatarNotUploaded    = errors.New("avatar has not been uploaded yet")
	ErrStorageUnavailable   = errors.New("avatar storage is unavailable")
)

// AvatarRejectedError means the uploaded avatar failed verification.
type AvatarRejectedError struct{ Reason string }

func (e *AvatarRejectedError) Error() string { return "avatar upload rejected: " + e.Reason }

// Rejection reasons for avatar uploads.
const (
	ReasonExpired             = "EXPIRED"
	ReasonSizeMismatch        = "SIZE_MISMATCH"
	ReasonContentTypeMismatch = "CONTENT_TYPE_MISMATCH"
	ReasonChecksumMismatch    = "CHECKSUM_MISMATCH"
	ReasonUnrecognizedFormat  = "UNRECOGNIZED_FORMAT"
)

// AvatarSniffLen is how many leading bytes avatar format detection needs.
const AvatarSniffLen = 12

// AvatarUploadStatus is the lifecycle of an avatar upload session.
type AvatarUploadStatus string

// Avatar upload statuses. PENDING -> COMPLETED | FAILED; both ends are final.
const (
	AvatarStatusPending   AvatarUploadStatus = "PENDING"
	AvatarStatusCompleted AvatarUploadStatus = "COMPLETED"
	AvatarStatusFailed    AvatarUploadStatus = "FAILED"
)

// AvatarFormat is an accepted avatar image format.
type AvatarFormat struct {
	ContentType string
	Ext         string
	sniff       func(head []byte) bool
}

var avatarFormats = []AvatarFormat{
	{ContentType: "image/jpeg", Ext: "jpg", sniff: isJPEG},
	{ContentType: "image/png", Ext: "png", sniff: isPNG},
	{ContentType: "image/webp", Ext: "webp", sniff: isWebP},
}

func isJPEG(b []byte) bool {
	return len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF
}

func isPNG(b []byte) bool {
	return bytes.HasPrefix(b, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
}

func isWebP(b []byte) bool {
	return len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP"
}

// AvatarFormatOf returns the format for a declared content type.
func AvatarFormatOf(contentType string) (AvatarFormat, bool) {
	for _, f := range avatarFormats {
		if f.ContentType == contentType {
			return f, true
		}
	}
	return AvatarFormat{}, false
}

// Matches reports whether the object's head matches this format's magic bytes.
func (f AvatarFormat) Matches(head []byte) bool {
	if f.sniff == nil {
		return false
	}
	return f.sniff(head)
}

// AvatarUpload is one avatar upload session.
type AvatarUpload struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	ContentType   string
	SizeBytes     int64
	SHA256        []byte
	Key           string
	Status        AvatarUploadStatus
	FailureReason string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	CompletedAt   *time.Time
}

// AvatarUploadRequest is what a client declares before uploading an avatar.
type AvatarUploadRequest struct {
	ContentType string
	SizeBytes   int64
	SHA256Hex   string
}

// Validate checks an avatar upload request against the size limit and accepted
// formats, returning the format and decoded SHA-256 digest.
func (r AvatarUploadRequest) Validate(maxSize int64) (AvatarFormat, []byte, error) {
	fields := map[string]string{}
	f, ok := AvatarFormatOf(r.ContentType)
	if !ok {
		fields["contentType"] = "must be image/jpeg, image/png or image/webp"
	}
	if r.SizeBytes <= 0 || r.SizeBytes > maxSize {
		fields["sizeBytes"] = "must be between 1 and the upload limit"
	}
	sum, err := hex.DecodeString(r.SHA256Hex)
	if err != nil || len(sum) != 32 {
		fields["sha256"] = "must be a hex SHA-256 digest"
	}
	if len(fields) > 0 {
		return AvatarFormat{}, nil, &ValidationError{Fields: fields}
	}
	return f, sum, nil
}

// Settle returns a settled copy of the upload: COMPLETED when reason is empty,
// FAILED otherwise.
func (u AvatarUpload) Settle(reason string, at time.Time) AvatarUpload {
	t := stamp(at)
	u.CompletedAt = &t
	u.Status, u.FailureReason = AvatarStatusCompleted, ""
	if reason != "" {
		u.Status, u.FailureReason = AvatarStatusFailed, reason
	}
	return u
}
