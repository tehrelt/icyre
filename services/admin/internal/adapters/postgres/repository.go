package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	platformpg "github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

// Repository implements OperationRepository, AuditRepository, and TranscodeJobRepository.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository returns a Repository backed by pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const operationColumns = `id, actor_id, idempotency_key, request_hash, action, target_type, target_id,
	desired_blocked, reason, status, result_code, before_state, after_state,
	revocations_pending, request_id, created_at, updated_at, completed_at`

// FindOrCreateOperation inserts op or returns the existing operation for (actor_id, idempotency_key).
func (r *Repository) FindOrCreateOperation(ctx context.Context, op domain.Operation) (domain.Operation, bool, error) {
	beforeJSON, err := marshalJSONObject(op.BeforeState)
	if err != nil {
		return domain.Operation{}, false, fmt.Errorf("marshal before_state: %w", err)
	}
	afterJSON, err := marshalJSONObject(op.AfterState)
	if err != nil {
		return domain.Operation{}, false, fmt.Errorf("marshal after_state: %w", err)
	}

	q := platformpg.Conn(ctx, r.pool)
	row := q.QueryRow(ctx, `
		INSERT INTO admin.moderation_operations (
			id, actor_id, idempotency_key, request_hash, action, target_type, target_id,
			desired_blocked, reason, status, result_code, before_state, after_state,
			revocations_pending, request_id, created_at, updated_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (actor_id, idempotency_key) DO NOTHING
		RETURNING `+operationColumns,
		op.ID, op.ActorID, op.IdempotencyKey, op.RequestHash, string(op.Action), string(op.TargetType), op.TargetID,
		op.DesiredBlocked, op.Reason, string(op.Status), op.ResultCode, beforeJSON, afterJSON,
		op.RevocationsPending, op.RequestID, op.CreatedAt.UTC(), op.UpdatedAt.UTC(), op.CompletedAt,
	)
	inserted, err := scanOperation(row)
	if err == nil {
		return inserted, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Operation{}, false, fmt.Errorf("insert moderation operation: %w", err)
	}

	existing, err := r.getOperationByIdempotencyKey(ctx, op.ActorID, op.IdempotencyKey)
	if err != nil {
		return domain.Operation{}, false, err
	}
	return existing, false, nil
}

func (r *Repository) getOperationByIdempotencyKey(ctx context.Context, actorID uuid.UUID, key string) (domain.Operation, error) {
	row := platformpg.Conn(ctx, r.pool).QueryRow(ctx, `
		SELECT `+operationColumns+`
		FROM admin.moderation_operations
		WHERE actor_id = $1 AND idempotency_key = $2`,
		actorID, key,
	)
	op, err := scanOperation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Operation{}, domain.ErrOperationNotFound
	}
	if err != nil {
		return domain.Operation{}, fmt.Errorf("select operation by idempotency key: %w", err)
	}
	return op, nil
}

// GetOperation loads a moderation operation by ID.
func (r *Repository) GetOperation(ctx context.Context, id uuid.UUID) (domain.Operation, error) {
	row := platformpg.Conn(ctx, r.pool).QueryRow(ctx, `
		SELECT `+operationColumns+`
		FROM admin.moderation_operations
		WHERE id = $1`,
		id,
	)
	op, err := scanOperation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Operation{}, domain.ErrOperationNotFound
	}
	if err != nil {
		return domain.Operation{}, fmt.Errorf("select moderation operation: %w", err)
	}
	return op, nil
}

// FinalizeOperation transitions a PENDING operation to a settled state.
func (r *Repository) FinalizeOperation(ctx context.Context, op domain.Operation) (domain.Operation, error) {
	beforeJSON, err := marshalJSONObject(op.BeforeState)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("marshal before_state: %w", err)
	}
	afterJSON, err := marshalJSONObject(op.AfterState)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("marshal after_state: %w", err)
	}

	row := platformpg.Conn(ctx, r.pool).QueryRow(ctx, `
		UPDATE admin.moderation_operations
		SET status = $2,
		    result_code = $3,
		    before_state = $4,
		    after_state = $5,
		    revocations_pending = $6,
		    updated_at = $7,
		    completed_at = $8
		WHERE id = $1 AND status = 'PENDING'
		RETURNING `+operationColumns,
		op.ID, string(op.Status), op.ResultCode, beforeJSON, afterJSON,
		op.RevocationsPending, op.UpdatedAt.UTC(), op.CompletedAt,
	)
	updated, err := scanOperation(row)
	if err == nil {
		return updated, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return r.GetOperation(ctx, op.ID)
	}
	return domain.Operation{}, fmt.Errorf("finalize moderation operation: %w", err)
}

// ListPendingOperations returns up to limit PENDING operations ordered oldest first.
func (r *Repository) ListPendingOperations(ctx context.Context, limit int) ([]domain.Operation, error) {
	rows, err := platformpg.Conn(ctx, r.pool).Query(ctx, `
		SELECT `+operationColumns+`
		FROM admin.moderation_operations
		WHERE status = 'PENDING'
		ORDER BY created_at ASC
		LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select pending operations: %w", err)
	}
	defer rows.Close()

	var out []domain.Operation
	for rows.Next() {
		op, err := scanOperation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan pending operation: %w", err)
		}
		out = append(out, op)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending operations: %w", err)
	}
	return out, nil
}

func scanOperation(row pgx.Row) (domain.Operation, error) {
	var (
		op         domain.Operation
		action     string
		targetType string
		status     string
		beforeRaw  []byte
		afterRaw   []byte
	)
	err := row.Scan(
		&op.ID, &op.ActorID, &op.IdempotencyKey, &op.RequestHash,
		&action, &targetType, &op.TargetID, &op.DesiredBlocked,
		&op.Reason, &status, &op.ResultCode, &beforeRaw, &afterRaw,
		&op.RevocationsPending, &op.RequestID, &op.CreatedAt, &op.UpdatedAt, &op.CompletedAt,
	)
	if err != nil {
		return domain.Operation{}, err
	}
	op.Action = domain.Action(action)
	op.TargetType = domain.TargetType(targetType)
	op.Status = domain.OperationStatus(status)
	if op.BeforeState, err = unmarshalJSONObject(beforeRaw); err != nil {
		return domain.Operation{}, fmt.Errorf("decode before_state: %w", err)
	}
	if op.AfterState, err = unmarshalJSONObject(afterRaw); err != nil {
		return domain.Operation{}, fmt.Errorf("decode after_state: %w", err)
	}
	return op, nil
}

func marshalJSONObject(m map[string]any) ([]byte, error) {
	if len(m) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

func unmarshalJSONObject(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return map[string]any{}, nil
	}
	return out, nil
}
