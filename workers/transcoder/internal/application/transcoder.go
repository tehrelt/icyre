// Package application turns an uploaded master into stream-ready audio
// variants (specs/workers/transcoder.md).
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	"github.com/tehrelt/icyre/libs/contracts/media"
)

// Codec is the codec of every variant.
const Codec = "aac"

// Errors adapters return for conditions a retry cannot fix.
var (
	// ErrSourceMissing: the master object does not exist.
	ErrSourceMissing = errors.New("source object missing")
	// ErrUndecodable: the master is not audio FFmpeg can decode.
	ErrUndecodable = errors.New("source is not decodable audio")
	// ErrInvalidJob: the event cannot be processed as given.
	ErrInvalidJob = errors.New("invalid job")
)

// Provenance ties a variant to the master it was made from. It is stored
// with every variant object and makes processing idempotent.
type Provenance struct {
	UploadID     string
	SourceSHA256 string
	UploadedAt   time.Time
	DurationMs   int64
}

// Stored is a variant already in object storage.
type Stored struct {
	Provenance
	Quality   media.Quality
	Key       string
	SizeBytes int64
}

// Storage reads masters and writes variants.
type Storage interface {
	// Download saves the object to path and returns its hex SHA-256, or
	// ErrSourceMissing.
	Download(ctx context.Context, bucket, key, path string) (string, error)
	// Variant returns a stored variant; ok is false when there is none.
	Variant(ctx context.Context, trackID string, q media.Quality) (v Stored, ok bool, err error)
	// PutVariant uploads the file at path as the variant.
	PutVariant(ctx context.Context, trackID string, q media.Quality, path string, p Provenance) (Stored, error)
	// Bucket is where variants live.
	Bucket() string
}

// Encoder runs FFmpeg.
type Encoder interface {
	// Probe checks that src holds decodable audio and returns its
	// duration, or ErrUndecodable.
	Probe(ctx context.Context, src string) (durationMs int64, err error)
	// Encode writes every requested variant in one pass.
	Encode(ctx context.Context, src string, dst map[media.Quality]string) error
}

// Publisher announces the outcome on media.events.
type Publisher interface {
	Transcoded(ctx context.Context, e mediav1.TrackTranscoded) error
	Failed(ctx context.Context, e mediav1.TranscodeFailed) error
}

// Outcome is what Handle did with a job.
type Outcome string

// Outcomes (the result label of media_transcode_duration_seconds).
const (
	// OutcomeTranscoded: variants were produced and announced.
	OutcomeTranscoded Outcome = "transcoded"
	// OutcomeDuplicate: variants of this master already existed (redelivery);
	// track.transcoded was announced again.
	OutcomeDuplicate Outcome = "duplicate"
	// OutcomeStale: a newer upload of the track was already transcoded.
	OutcomeStale Outcome = "stale"
	// OutcomeFailed: the master is unusable; media.transcode.failed sent.
	OutcomeFailed Outcome = "failed"
)

// Transcoder is the use case.
type Transcoder struct {
	store   Storage
	enc     Encoder
	pub     Publisher
	workDir string
	now     func() time.Time
	log     *slog.Logger
}

// New returns a Transcoder; workDir "" means the OS temp directory.
func New(store Storage, enc Encoder, pub Publisher, workDir string, log *slog.Logger) *Transcoder {
	return &Transcoder{store: store, enc: enc, pub: pub, workDir: workDir, now: time.Now, log: log}
}

// Handle processes track.uploaded. It is idempotent: variants carry the
// provenance of their master, so a redelivered job re-announces the stored
// result and a stale one (older than what is stored) is dropped. An error
// is transient and the job should be retried.
func (t *Transcoder) Handle(ctx context.Context, job mediav1.TrackUploaded) (Outcome, error) {
	if job.TrackID == "" || job.Key == "" || job.Bucket == "" || job.SHA256 == "" {
		return "", fmt.Errorf("%w: incomplete job", ErrInvalidJob)
	}
	existing, err := t.existing(ctx, job.TrackID)
	if err != nil {
		return "", err
	}
	if outcome, done, err := t.alreadyHandled(ctx, job, existing); done || err != nil {
		return outcome, err
	}

	dir, err := os.MkdirTemp(t.workDir, "transcode-*")
	if err != nil {
		return "", fmt.Errorf("work dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	src := filepath.Join(dir, "source")
	sum, err := t.store.Download(ctx, job.Bucket, job.Key, src)
	switch {
	case errors.Is(err, ErrSourceMissing):
		return t.fail(ctx, job, mediav1.ReasonSourceMissing)
	case err != nil:
		return "", err
	case sum != job.SHA256:
		return t.fail(ctx, job, mediav1.ReasonSourceCorrupted)
	}

	duration, err := t.enc.Probe(ctx, src)
	switch {
	case errors.Is(err, ErrUndecodable):
		return t.fail(ctx, job, mediav1.ReasonUndecodable)
	case err != nil:
		return "", err
	}
	dst := make(map[media.Quality]string, len(media.Qualities))
	for _, q := range media.Qualities {
		dst[q] = filepath.Join(dir, q.String()+".aac")
	}
	if err := t.enc.Encode(ctx, src, dst); err != nil {
		return "", fmt.Errorf("encode %s: %w", job.TrackID, err)
	}

	p := Provenance{UploadID: job.UploadID, SourceSHA256: job.SHA256, UploadedAt: job.UploadedAt, DurationMs: duration}
	stored := make([]Stored, 0, len(media.Qualities))
	for _, q := range media.Qualities {
		v, err := t.store.PutVariant(ctx, job.TrackID, q, dst[q], p)
		if err != nil {
			return "", err
		}
		stored = append(stored, v)
	}
	if err := t.pub.Transcoded(ctx, t.transcoded(job, duration, stored)); err != nil {
		return "", err
	}
	return OutcomeTranscoded, nil
}

// existing returns the stored variants of the track, lowest first.
func (t *Transcoder) existing(ctx context.Context, trackID string) ([]Stored, error) {
	var out []Stored
	for _, q := range media.Qualities {
		v, ok, err := t.store.Variant(ctx, trackID, q)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, v)
		}
	}
	return out, nil
}

// alreadyHandled reports whether stored variants make the job redundant.
func (t *Transcoder) alreadyHandled(ctx context.Context, job mediav1.TrackUploaded, existing []Stored) (Outcome, bool, error) {
	for _, v := range existing {
		if v.UploadedAt.After(job.UploadedAt) {
			t.log.InfoContext(ctx, "newer upload already transcoded, skipping",
				"track_id", job.TrackID, "upload_id", job.UploadID, "stored_upload_id", v.UploadID)
			return OutcomeStale, true, nil
		}
	}
	if len(existing) != len(media.Qualities) {
		return "", false, nil // nothing yet, or a partial run: redo all
	}
	for _, v := range existing {
		if v.SourceSHA256 != job.SHA256 {
			return "", false, nil
		}
	}
	if err := t.pub.Transcoded(ctx, t.transcoded(job, existing[0].DurationMs, existing)); err != nil {
		return "", true, err
	}
	return OutcomeDuplicate, true, nil
}

func (t *Transcoder) transcoded(job mediav1.TrackUploaded, durationMs int64, stored []Stored) mediav1.TrackTranscoded {
	variants := make([]mediav1.Variant, len(stored))
	for i, v := range stored {
		variants[i] = mediav1.Variant{Quality: int(v.Quality), Key: v.Key, ContentType: media.AudioContentType, SizeBytes: v.SizeBytes}
	}
	return mediav1.TrackTranscoded{
		UploadID: job.UploadID, TrackID: job.TrackID, Bucket: t.store.Bucket(), Codec: Codec,
		DurationMs: durationMs, Variants: variants, SourceSHA256: job.SHA256, TranscodedAt: t.now().UTC(),
	}
}

func (t *Transcoder) fail(ctx context.Context, job mediav1.TrackUploaded, reason string) (Outcome, error) {
	t.log.WarnContext(ctx, "transcode failed", "track_id", job.TrackID, "upload_id", job.UploadID, "reason", reason)
	err := t.pub.Failed(ctx, mediav1.TranscodeFailed{UploadID: job.UploadID, TrackID: job.TrackID, Reason: reason, FailedAt: t.now().UTC()})
	if err != nil {
		return "", err
	}
	return OutcomeFailed, nil
}
