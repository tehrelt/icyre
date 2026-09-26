// Package application measures audio features of uploaded masters.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	"github.com/tehrelt/icyre/workers/audio-analysis/internal/domain"
)

// Errors of the ports and the use case.
var (
	// ErrInvalidJob: the event cannot be processed, whatever the retries.
	ErrInvalidJob = errors.New("invalid job")
	// ErrSourceMissing: the master is not in storage.
	ErrSourceMissing = errors.New("source missing")
	// ErrUndecodable: ffmpeg rejected the file.
	ErrUndecodable = errors.New("undecodable")
)

// Outcome of one track.uploaded.
type Outcome string

// Outcomes, also the result label of the duration metric.
const (
	Analyzed    Outcome = "analyzed"
	Duplicate   Outcome = "duplicate"
	Missing     Outcome = "missing"
	Undecodable Outcome = "undecodable"
)

// Source gives ffmpeg read access to a master without a local copy.
type Source interface {
	// URL returns a short-lived URL of the object, or ErrSourceMissing.
	URL(ctx context.Context, bucket, key string) (string, error)
}

// Analyzer measures the features of a file or URL; a file it cannot
// decode is ErrUndecodable.
type Analyzer interface {
	Analyze(ctx context.Context, src string) (domain.Features, error)
}

// Repository stores features per upload.
type Repository interface {
	// Exists reports whether the upload's features are stored.
	Exists(ctx context.Context, uploadID uuid.UUID) (bool, error)
	// Save stores a and its audio.features_extracted event in one
	// transaction; false when the upload is already stored.
	Save(ctx context.Context, a domain.Analysis) (bool, error)
}

// Service handles track.uploaded.
type Service struct {
	source   Source
	analyzer Analyzer
	repo     Repository
	log      *slog.Logger
	now      func() time.Time
}

// New returns a Service.
func New(source Source, analyzer Analyzer, repo Repository, log *slog.Logger) *Service {
	return &Service{source: source, analyzer: analyzer, repo: repo, log: log, now: func() time.Time { return time.Now().UTC() }}
}

// Handle analyzes the uploaded master and stores its features. Redelivery
// of a stored upload is a no-op; a missing or undecodable master is logged
// and skipped (the Transcoder reports it as media.transcode.failed).
func (s *Service) Handle(ctx context.Context, job mediav1.TrackUploaded) (Outcome, error) {
	uploadID, e1 := uuid.Parse(job.UploadID)
	trackID, e2 := uuid.Parse(job.TrackID)
	if e1 != nil || e2 != nil || job.Bucket == "" || job.Key == "" {
		return "", fmt.Errorf("%w: upload %q track %q key %q", ErrInvalidJob, job.UploadID, job.TrackID, job.Key)
	}
	log := s.log.With("upload_id", job.UploadID, "track_id", job.TrackID)

	stored, err := s.repo.Exists(ctx, uploadID)
	if err != nil {
		return "", err
	}
	if stored {
		return Duplicate, nil
	}
	url, err := s.source.URL(ctx, job.Bucket, job.Key)
	if errors.Is(err, ErrSourceMissing) {
		log.WarnContext(ctx, "master missing, analysis skipped", "key", job.Key)
		return Missing, nil
	}
	if err != nil {
		return "", err
	}
	features, err := s.analyzer.Analyze(ctx, url)
	if err == nil {
		err = features.Validate()
	}
	if errors.Is(err, ErrUndecodable) || errors.Is(err, domain.ErrInvalid) {
		log.WarnContext(ctx, "master undecodable, analysis skipped", "err", err)
		return Undecodable, nil
	}
	if err != nil {
		return "", err
	}

	saved, err := s.repo.Save(ctx, domain.Analysis{
		UploadID: uploadID, TrackID: trackID, SourceSHA256: job.SHA256, Features: features,
		AnalyzerVersion: domain.AnalyzerVersion, UploadedAt: job.UploadedAt, AnalyzedAt: s.now(),
	})
	if err != nil {
		return "", err
	}
	if !saved {
		return Duplicate, nil // a concurrent delivery stored it first
	}
	log.InfoContext(ctx, "audio analyzed", "bpm", features.BPM, "bpm_confidence", features.BPMConfidence,
		"integrated_lufs", features.IntegratedLUFS, "silence_ratio", features.SilenceRatio)
	return Analyzed, nil
}
