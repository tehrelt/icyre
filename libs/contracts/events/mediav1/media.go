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
	// TypeMetadataExtracted: technical metadata of an uploaded master is
	// stored (Metadata Worker, specs/workers/metadata.md).
	TypeMetadataExtracted = "media.metadata_extracted"
	// TypeAudioFeaturesExtracted: tempo and loudness features of an
	// uploaded master are stored (Audio Analysis Worker,
	// specs/workers/audio-analysis.md).
	TypeAudioFeaturesExtracted = "audio.features_extracted"
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

// MetadataExtracted is the payload of media.metadata_extracted: what
// ffprobe reports about the master of one upload. BitrateBps is the audio
// stream bitrate, else the container one; 0 when neither is declared.
type MetadataExtracted struct {
	UploadID     string    `json:"uploadId"`
	TrackID      string    `json:"trackId"`
	Container    string    `json:"container"`
	Codec        string    `json:"codec"`
	DurationMs   int64     `json:"durationMs"`
	BitrateBps   int64     `json:"bitrateBps"`
	SampleRateHz int       `json:"sampleRateHz"`
	Channels     int       `json:"channels"`
	SourceSHA256 string    `json:"sourceSha256"`
	UploadedAt   time.Time `json:"uploadedAt"`
	ExtractedAt  time.Time `json:"extractedAt"`
}

// AudioFeaturesExtracted is the payload of audio.features_extracted: what
// the Audio Analysis Worker measured on the master of one upload.
// BPM is nil when the track has no detectable tempo. Loudness follows
// EBU R128: integrated loudness in LUFS, loudness range in LU, true peak
// in dBTP; levels of silence are floored at -70.
type AudioFeaturesExtracted struct {
	UploadID string   `json:"uploadId"`
	TrackID  string   `json:"trackId"`
	BPM      *float64 `json:"bpm"`
	// BPMConfidence is the tempo's periodicity strength in [0, 1].
	BPMConfidence   float64 `json:"bpmConfidence"`
	IntegratedLUFS  float64 `json:"integratedLufs"`
	LoudnessRangeLU float64 `json:"loudnessRangeLu"`
	TruePeakDBTP    float64 `json:"truePeakDbtp"`
	// SilenceRatio is the share of the track below -60 dBFS, in [0, 1].
	SilenceRatio float64 `json:"silenceRatio"`
	AnalyzedMs   int64   `json:"analyzedMs"`
	// AnalyzerVersion changes when the algorithms do; features of different
	// versions are not comparable.
	AnalyzerVersion string    `json:"analyzerVersion"`
	SourceSHA256    string    `json:"sourceSha256"`
	UploadedAt      time.Time `json:"uploadedAt"`
	AnalyzedAt      time.Time `json:"analyzedAt"`
}
