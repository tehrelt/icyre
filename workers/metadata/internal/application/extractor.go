// Package application extracts technical metadata from uploaded masters.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	"github.com/tehrelt/icyre/workers/metadata/internal/domain"
)

// Errors of the ports and the use case.
var (
	// ErrInvalidJob: the event cannot be processed, whatever the retries.
	ErrInvalidJob = errors.New("invalid job")
	// ErrSourceMissing: the master is not in storage.
	ErrSourceMissing = errors.New("source missing")
	// ErrUndecodable: ffprobe rejected the file.
	ErrUndecodable = errors.New("undecodable")
)

// Outcome of one track.uploaded.
type Outcome string

// Outcomes, also the result label of the duration metric.
const (
	Extracted   Outcome = "extracted"
	Duplicate   Outcome = "duplicate"
	Missing     Outcome = "missing"
	Undecodable Outcome = "undecodable"
)

// Source gives ffprobe read access to a master without downloading it.
type Source interface {
	// URL returns a short-lived URL of the object, or ErrSourceMissing.
	URL(ctx context.Context, bucket, key string) (string, error)
}

// Prober reads the technical metadata of a file or URL; a file it cannot
// decode is ErrUndecodable.
type Prober interface {
	Probe(ctx context.Context, src string) (domain.Probe, error)
}

// Repository stores metadata per upload.
type Repository interface {
	// Exists reports whether the upload's metadata is stored.
	Exists(ctx context.Context, uploadID uuid.UUID) (bool, error)
	// Save stores m and its media.metadata_extracted event in one
	// transaction; false when the upload is already stored.
	Save(ctx context.Context, m domain.Metadata) (bool, error)
}

// Extractor handles track.uploaded.
type Extractor struct {
	source Source
	prober Prober
	repo   Repository
	log    *slog.Logger
	now    func() time.Time
}

// New returns an Extractor.
func New(source Source, prober Prober, repo Repository, log *slog.Logger) *Extractor {
	return &Extractor{source: source, prober: prober, repo: repo, log: log, now: func() time.Time { return time.Now().UTC() }}
}

// Handle probes the uploaded master and stores its metadata. Redelivery
// of a stored upload is a no-op; a missing or undecodable master is logged
// and skipped (the Transcoder reports it as media.transcode.failed).
func (x *Extractor) Handle(ctx context.Context, job mediav1.TrackUploaded) (Outcome, error) {
	uploadID, e1 := uuid.Parse(job.UploadID)
	trackID, e2 := uuid.Parse(job.TrackID)
	if e1 != nil || e2 != nil || job.Bucket == "" || job.Key == "" {
		return "", fmt.Errorf("%w: upload %q track %q key %q", ErrInvalidJob, job.UploadID, job.TrackID, job.Key)
	}
	log := x.log.With("upload_id", job.UploadID, "track_id", job.TrackID)

	stored, err := x.repo.Exists(ctx, uploadID)
	if err != nil {
		return "", err
	}
	if stored {
		return Duplicate, nil
	}
	url, err := x.source.URL(ctx, job.Bucket, job.Key)
	if errors.Is(err, ErrSourceMissing) {
		log.WarnContext(ctx, "master missing, metadata skipped", "key", job.Key)
		return Missing, nil
	}
	if err != nil {
		return "", err
	}
	probe, err := x.prober.Probe(ctx, url)
	if err == nil {
		err = probe.Validate()
	}
	if errors.Is(err, ErrUndecodable) || errors.Is(err, domain.ErrInvalid) {
		log.WarnContext(ctx, "master undecodable, metadata skipped", "err", err)
		return Undecodable, nil
	}
	if err != nil {
		return "", err
	}

	saved, err := x.repo.Save(ctx, domain.Metadata{
		UploadID: uploadID, TrackID: trackID, SourceSHA256: job.SHA256, Probe: probe,
		UploadedAt: job.UploadedAt, ExtractedAt: x.now(),
	})
	if err != nil {
		return "", err
	}
	if !saved {
		return Duplicate, nil // a concurrent delivery stored it first
	}
	log.InfoContext(ctx, "metadata extracted", "codec", probe.Codec, "duration_ms", probe.DurationMs)
	return Extracted, nil
}
