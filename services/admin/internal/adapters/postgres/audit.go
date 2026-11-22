package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	platformpg "github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

// AppendAuditLog inserts an immutable audit log entry (idempotent on operation_id + phase).
func (r *Repository) AppendAuditLog(ctx context.Context, entry domain.AuditEntry) error {
	metaJSON, err := marshalJSONObject(entry.Metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}
	_, err = platformpg.Conn(ctx, r.pool).Exec(ctx, `
		INSERT INTO admin.audit_logs (
			id, operation_id, actor_id, action, target_type, target_id,
			phase, reason, metadata, request_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (operation_id, phase) DO NOTHING`,
		entry.ID, entry.OperationID, entry.ActorID, string(entry.Action), string(entry.TargetType), entry.TargetID,
		string(entry.Phase), entry.Reason, metaJSON, entry.RequestID, entry.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

// ListAuditLogs returns audit log entries matching filter using keyset pagination.
func (r *Repository) ListAuditLogs(ctx context.Context, filter domain.AuditFilter) ([]domain.AuditEntry, error) {
	var targetType, action, phase *string
	if filter.TargetType != nil {
		s := string(*filter.TargetType)
		targetType = &s
	}
	if filter.Action != nil {
		s := string(*filter.Action)
		action = &s
	}
	if filter.Phase != nil {
		s := string(*filter.Phase)
		phase = &s
	}

	rows, err := platformpg.Conn(ctx, r.pool).Query(ctx, `
		SELECT id, operation_id, actor_id, action, target_type, target_id,
		       phase, reason, metadata, request_id, created_at
		FROM admin.audit_logs
		WHERE ($1::uuid IS NULL OR actor_id = $1::uuid)
		  AND ($2::text IS NULL OR target_type = $2::text)
		  AND ($3::uuid IS NULL OR target_id = $3::uuid)
		  AND ($4::text IS NULL OR action = $4::text)
		  AND ($5::text IS NULL OR phase = $5::text)
		  AND ($6::timestamptz IS NULL OR $7::uuid IS NULL OR (created_at, id) < ($6::timestamptz, $7::uuid))
		ORDER BY created_at DESC, id DESC
		LIMIT $8`,
		filter.ActorID, targetType, filter.TargetID, action, phase,
		filter.AfterCreatedAt, filter.AfterID, filter.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select audit logs: %w", err)
	}
	defer rows.Close()

	var out []domain.AuditEntry
	for rows.Next() {
		entry, err := scanAuditEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit logs: %w", err)
	}
	return out, nil
}

func scanAuditEntry(row pgx.Row) (domain.AuditEntry, error) {
	var (
		entry      domain.AuditEntry
		action     string
		targetType string
		phase      string
		metaRaw    []byte
	)
	err := row.Scan(
		&entry.ID, &entry.OperationID, &entry.ActorID, &action, &targetType, &entry.TargetID,
		&phase, &entry.Reason, &metaRaw, &entry.RequestID, &entry.CreatedAt,
	)
	if err != nil {
		return domain.AuditEntry{}, err
	}
	entry.Action = domain.Action(action)
	entry.TargetType = domain.TargetType(targetType)
	entry.Phase = domain.AuditPhase(phase)
	if entry.Metadata, err = unmarshalJSONObject(metaRaw); err != nil {
		return domain.AuditEntry{}, fmt.Errorf("decode audit metadata: %w", err)
	}
	return entry, nil
}
