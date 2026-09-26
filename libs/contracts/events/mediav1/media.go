// Package mediav1 holds version 1 payloads of Media Ingest events,
// published to events.TopicMediaEvents keyed by track ID (per-track order).
package mediav1

import "time"

// Version is the eventVersion of every payload in this package.
const Version = 1

// Event types (specs/services/media-ingest.md).
const (
	// TypeTrackUploaded: a verified master is in object storage and can be
	// transcoded.
	TypeTrackUploaded = "track.uploaded"
	// TypeIngestFailed: an upload was rejected or expired; nothing to process.
	TypeIngestFailed = "media.ingest.failed"
	// TypeTrackTranscoded: every audio variant of the track is stored
	// (Transcoder, specs/workers/transcoder.md).
	TypeTrackTranscoded = "track.transcoded"
	// TypeTranscodeFailed: the master cannot be transcoded; retrying will
	// not help.
	TypeTranscodeFailed = "media.transcode.failed"
)

// TrackUploaded is the payload of track.uploaded.
type TrackUploaded struct {
	UploadID    string `json:"uploadId"`
	TrackID     string `json:"trackId"`
	UploaderID  string `json:"uploaderId"`
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	// SHA256 is the hex digest the store verified on arrival.
	SHA256     string    `json:"sha256"`
	UploadedAt time.Time `json:"uploadedAt"`
}

// Failure reasons of media.ingest.failed.
const (
	ReasonExpired             = "EXPIRED"
	ReasonSizeMismatch        = "SIZE_MISMATCH"
	ReasonContentTypeMismatch = "CONTENT_TYPE_MISMATCH"
	ReasonUnrecognizedFormat  = "UNRECOGNIZED_FORMAT"
	ReasonChecksumMismatch    = "CHECKSUM_MISMATCH"
)

// IngestFailed is the payload of media.ingest.failed.
type IngestFailed struct {
	UploadID string    `json:"uploadId"`
	TrackID  string    `json:"trackId"`
	Reason   string    `json:"reason"`
	FailedAt time.Time `json:"failedAt"`
}

// Variant is one stored audio variant.
type Variant struct {
	// Quality is the bitrate in kbps (64, 128, 256).
	Quality     int    `json:"quality"`
	Key         string `json:"key"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}

// TrackTranscoded is the payload of track.transcoded. Variants are listed
// lowest quality first.
type TrackTranscoded struct {
	UploadID   string    `json:"uploadId"`
	TrackID    string    `json:"trackId"`
	Bucket     string    `json:"bucket"`
	Codec      string    `json:"codec"`
	DurationMs int64     `json:"durationMs"`
	Variants   []Variant `json:"variants"`
	// SourceSHA256 is the hex digest of the master the variants came from.
	SourceSHA256 string    `json:"sourceSha256"`
	TranscodedAt time.Time `json:"transcodedAt"`
}

// Failure reasons of media.transcode.failed.
const (
	ReasonSourceMissing   = "SOURCE_MISSING"
	ReasonSourceCorrupted = "SOURCE_CORRUPTED"
	ReasonUndecodable     = "UNDECODABLE"
)

// TranscodeFailed is the payload of media.transcode.failed.
type TranscodeFailed struct {
	UploadID string    `json:"uploadId"`
	TrackID  string    `json:"trackId"`
	Reason   string    `json:"reason"`
	FailedAt time.Time `json:"failedAt"`
}
