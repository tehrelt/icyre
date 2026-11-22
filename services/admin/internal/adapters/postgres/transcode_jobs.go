package postgres

import (
	"context"
	"fmt"

	platformpg "github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

// UpsertTranscodeJob idempotently projects a transcode job event without
// regressing terminal states (SUCCEEDED/FAILED) back to QUEUED.
func (r *Repository) UpsertTranscodeJob(ctx context.Context, job domain.TranscodeJob) error {
	_, err := platformpg.Conn(ctx, r.pool).Exec(ctx, `
		INSERT INTO admin.transcode_jobs (upload_id, track_id, status, failure_reason, event_at, projected_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (upload_id) DO UPDATE SET
			track_id = EXCLUDED.track_id,
			status = EXCLUDED.status,
			failure_reason = EXCLUDED.failure_reason,
			event_at = EXCLUDED.event_at,
			projected_at = EXCLUDED.projected_at
		WHERE (
			admin.transcode_jobs.status = 'QUEUED'
			AND EXCLUDED.status = 'QUEUED'
			AND EXCLUDED.event_at >= admin.transcode_jobs.event_at
		) OR (
			EXCLUDED.status IN ('SUCCEEDED', 'FAILED')
			AND (admin.transcode_jobs.status = 'QUEUED' OR EXCLUDED.event_at >= admin.transcode_jobs.event_at)
		)`,
		job.UploadID, job.TrackID, string(job.Status), job.FailureReason, job.EventAt.UTC(), job.ProjectedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert transcode job: %w", err)
	}
	return nil
}

// ListTranscodeJobs returns projected transcode jobs matching filter using keyset pagination.
func (r *Repository) ListTranscodeJobs(ctx context.Context, filter domain.TranscodeJobFilter) ([]domain.TranscodeJob, error) {
	var status *string
	if filter.Status != nil {
		s := string(*filter.Status)
		status = &s
	}

	rows, err := platformpg.Conn(ctx, r.pool).Query(ctx, `
		SELECT upload_id, track_id, status, failure_reason, event_at, projected_at
		FROM admin.transcode_jobs
		WHERE ($1::uuid IS NULL OR track_id = $1::uuid)
		  AND ($2::text IS NULL OR status = $2::text)
		  AND ($3::timestamptz IS NULL OR $4::uuid IS NULL OR (event_at, upload_id) < ($3::timestamptz, $4::uuid))
		ORDER BY event_at DESC, upload_id DESC
		LIMIT $5`,
		filter.TrackID, status, filter.AfterEventAt, filter.AfterUploadID, filter.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select transcode jobs: %w", err)
	}
	defer rows.Close()

	var out []domain.TranscodeJob
	for rows.Next() {
		var (
			job domain.TranscodeJob
			st  string
		)
		if err := rows.Scan(&job.UploadID, &job.TrackID, &st, &job.FailureReason, &job.EventAt, &job.ProjectedAt); err != nil {
			return nil, fmt.Errorf("scan transcode job: %w", err)
		}
		job.Status = domain.TranscodeStatus(st)
		out = append(out, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transcode jobs: %w", err)
	}
	return out, nil
}
