// Package application implements the Admin Service use cases.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/requestid"
	"github.com/tehrelt/icyre/services/admin/internal/domain"
	"github.com/tehrelt/icyre/services/admin/internal/ports"
)

// Pagination bounds.
const (
	DefaultPageSize = 50
	MaxPageSize     = 100
)

// Deps wires the application service dependencies.
type Deps struct {
	Ops         ports.OperationRepository
	Audit       ports.AuditRepository
	Jobs        ports.TranscodeJobRepository
	Catalog     ports.CatalogClient
	Auth        ports.AuthClient
	MediaIngest ports.MediaIngestClient
	Tx          ports.Transactor
	Log         *slog.Logger
	Now         func() time.Time
}

// Service implements Admin Service use cases.
type Service struct {
	ops         ports.OperationRepository
	audit       ports.AuditRepository
	jobs        ports.TranscodeJobRepository
	catalog     ports.CatalogClient
	auth        ports.AuthClient
	mediaIngest ports.MediaIngestClient
	tx          ports.Transactor
	log         *slog.Logger
	now         func() time.Time
}

// New constructs a Service from deps.
func New(deps Deps) *Service {
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	log := deps.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		ops:         deps.Ops,
		audit:       deps.Audit,
		jobs:        deps.Jobs,
		catalog:     deps.Catalog,
		auth:        deps.Auth,
		mediaIngest: deps.MediaIngest,
		tx:          deps.Tx,
		log:         log,
		now:         now,
	}
}

// ModerateTrack blocks or unblocks a track via Catalog with idempotency and audit logging.
func (s *Service) ModerateTrack(ctx context.Context, actor domain.Actor, idempotencyKey string, trackID uuid.UUID, blocked bool, reason string) (domain.Operation, error) {
	if !actor.CanModerateTracks() {
		return domain.Operation{}, domain.ErrForbidden
	}
	action := domain.ActionTrackBlock
	if !blocked {
		action = domain.ActionTrackUnblock
	}
	key, trimmedReason, err := domain.ValidateModerationInput(idempotencyKey, trackID, reason)
	if err != nil {
		return domain.Operation{}, err
	}
	return s.moderate(ctx, actor, key, action, domain.TargetTrack, trackID, blocked, trimmedReason)
}

// ModerateUser blocks or unblocks a user via Auth with idempotency and audit logging.
func (s *Service) ModerateUser(ctx context.Context, actor domain.Actor, idempotencyKey string, userID uuid.UUID, blocked bool, reason string) (domain.Operation, error) {
	if !actor.CanModerateUsers() {
		return domain.Operation{}, domain.ErrForbidden
	}
	if actor.ID == userID {
		return domain.Operation{}, domain.ErrCannotBlockSelf
	}
	action := domain.ActionUserBlock
	if !blocked {
		action = domain.ActionUserUnblock
	}
	key, trimmedReason, err := domain.ValidateModerationInput(idempotencyKey, userID, reason)
	if err != nil {
		return domain.Operation{}, err
	}
	return s.moderate(ctx, actor, key, action, domain.TargetUser, userID, blocked, trimmedReason)
}

func (s *Service) moderate(ctx context.Context, actor domain.Actor, key string, action domain.Action, targetType domain.TargetType, targetID uuid.UUID, blocked bool, reason string) (domain.Operation, error) {
	hash := domain.ComputeRequestHash(action, targetType, targetID, blocked, reason)
	opID, err := uuid.NewV7()
	if err != nil {
		return domain.Operation{}, fmt.Errorf("operation id: %w", err)
	}
	auditID, err := uuid.NewV7()
	if err != nil {
		return domain.Operation{}, fmt.Errorf("audit id: %w", err)
	}
	now := s.now().UTC()
	reqID := requestid.From(ctx)

	candidate := domain.Operation{
		ID:             opID,
		ActorID:        actor.ID,
		IdempotencyKey: key,
		RequestHash:    hash,
		Action:         action,
		TargetType:     targetType,
		TargetID:       targetID,
		DesiredBlocked: blocked,
		Reason:         reason,
		Status:         domain.StatusPending,
		BeforeState:    map[string]any{},
		AfterState:     map[string]any{},
		RequestID:      reqID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	var op domain.Operation
	err = s.tx.InTx(ctx, func(txCtx context.Context) error {
		var created bool
		var txErr error
		op, created, txErr = s.ops.FindOrCreateOperation(txCtx, candidate)
		if txErr != nil {
			return txErr
		}
		if !created {
			if op.RequestHash != hash {
				return domain.ErrIdempotencyConflict
			}
			return nil
		}
		return s.audit.AppendAuditLog(txCtx, domain.AuditEntry{
			ID:          auditID,
			OperationID: op.ID,
			ActorID:     actor.ID,
			Action:      action,
			TargetType:  targetType,
			TargetID:    targetID,
			Phase:       domain.PhaseRequested,
			Reason:      reason,
			Metadata:    map[string]any{"desiredBlocked": blocked},
			RequestID:   reqID,
			CreatedAt:   now,
		})
	})
	if err != nil {
		return domain.Operation{}, err
	}
	if op.Status.Settled() {
		return op, nil
	}
	return s.executeOperation(ctx, op)
}

func (s *Service) executeOperation(ctx context.Context, op domain.Operation) (domain.Operation, error) {
	var (
		status              domain.OperationStatus
		phase               domain.AuditPhase
		resultCode          string
		before, after, meta map[string]any
		revocationsPending  bool
		ownerErr            *domain.OwnerError
		err                 error
	)

	switch op.TargetType {
	case domain.TargetTrack:
		status, phase, resultCode, before, after, meta, err = s.callCatalog(ctx, op)
	case domain.TargetUser:
		status, phase, resultCode, before, after, meta, revocationsPending, err = s.callAuth(ctx, op)
	default:
		return domain.Operation{}, fmt.Errorf("unsupported target type %q", op.TargetType)
	}

	if err != nil {
		if errors.As(err, &ownerErr) {
			settled, settleErr := s.settleOperation(ctx, op, domain.StatusRejected, domain.PhaseRejected, ownerErr.Code,
				map[string]any{}, map[string]any{"errorCode": ownerErr.Code, "status": ownerErr.Status},
				map[string]any{"errorCode": ownerErr.Code, "status": ownerErr.Status}, false)
			if settleErr != nil {
				return domain.Operation{}, settleErr
			}
			return settled, ownerErr
		}
		s.log.WarnContext(ctx, "moderation upstream unavailable; leaving operation pending",
			"operation_id", op.ID, "target_type", op.TargetType, "target_id", op.TargetID, "error", err)
		return op, nil
	}

	return s.settleOperation(ctx, op, status, phase, resultCode, before, after, meta, revocationsPending)
}

func (s *Service) callCatalog(ctx context.Context, op domain.Operation) (domain.OperationStatus, domain.AuditPhase, string, map[string]any, map[string]any, map[string]any, error) {
	rec, err := s.catalog.ModerateTrack(ctx, op.ID, op.TargetID, op.DesiredBlocked)
	if err != nil {
		return "", "", "", nil, nil, nil, err
	}
	status, phase, code := domain.StatusNoop, domain.PhaseNoop, "NOOP"
	if rec.Changed {
		status, phase, code = domain.StatusApplied, domain.PhaseApplied, "APPLIED"
	}
	before := map[string]any{"status": rec.PreviousStatus}
	after := map[string]any{"status": rec.CurrentStatus}
	meta := map[string]any{"previousStatus": rec.PreviousStatus, "currentStatus": rec.CurrentStatus, "changed": rec.Changed}
	return status, phase, code, before, after, meta, nil
}

func (s *Service) callAuth(ctx context.Context, op domain.Operation) (domain.OperationStatus, domain.AuditPhase, string, map[string]any, map[string]any, map[string]any, bool, error) {
	rec, err := s.auth.ModerateUser(ctx, op.ID, op.TargetID, op.DesiredBlocked)
	if err != nil {
		return "", "", "", nil, nil, nil, false, err
	}
	status, phase, code := domain.StatusNoop, domain.PhaseNoop, "NOOP"
	if rec.Changed {
		status, phase, code = domain.StatusApplied, domain.PhaseApplied, "APPLIED"
	}
	prevBlocked := rec.Blocked
	if rec.Changed {
		prevBlocked = !rec.Blocked
	}
	before := map[string]any{"blocked": prevBlocked}
	after := map[string]any{"blocked": rec.Blocked, "revokedSessions": rec.RevokedSessions}
	meta := map[string]any{
		"blocked":            rec.Blocked,
		"changed":            rec.Changed,
		"revokedSessions":    rec.RevokedSessions,
		"revocationsPending": rec.RevocationsPending,
	}
	return status, phase, code, before, after, meta, rec.RevocationsPending, nil
}

func (s *Service) settleOperation(ctx context.Context, op domain.Operation, status domain.OperationStatus, phase domain.AuditPhase, resultCode string, before, after, meta map[string]any, revocationsPending bool) (domain.Operation, error) {
	auditID, err := uuid.NewV7()
	if err != nil {
		return domain.Operation{}, fmt.Errorf("audit id: %w", err)
	}
	now := s.now().UTC()
	reqID := requestid.From(ctx)
	if reqID == "" {
		reqID = op.RequestID
	}
	settled := op.Settle(status, resultCode, before, after, revocationsPending, now)

	var out domain.Operation
	err = s.tx.InTx(ctx, func(txCtx context.Context) error {
		var txErr error
		out, txErr = s.ops.FinalizeOperation(txCtx, settled)
		if txErr != nil {
			return txErr
		}
		return s.audit.AppendAuditLog(txCtx, domain.AuditEntry{
			ID:          auditID,
			OperationID: op.ID,
			ActorID:     op.ActorID,
			Action:      op.Action,
			TargetType:  op.TargetType,
			TargetID:    op.TargetID,
			Phase:       phase,
			Reason:      op.Reason,
			Metadata:    domain.CloneMap(meta),
			RequestID:   reqID,
			CreatedAt:   now,
		})
	})
	if err != nil {
		return domain.Operation{}, err
	}
	return out, nil
}

// ReconcilePending retries PENDING moderation operations against owner services.
func (s *Service) ReconcilePending(ctx context.Context, limit int) error {
	if limit <= 0 {
		limit = DefaultPageSize
	}
	pending, err := s.ops.ListPendingOperations(ctx, limit)
	if err != nil {
		return err
	}
	for _, op := range pending {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := s.executeOperation(ctx, op); err != nil {
			var ownerErr *domain.OwnerError
			if errors.As(err, &ownerErr) {
				s.log.InfoContext(ctx, "pending operation rejected during reconciliation",
					"operation_id", op.ID, "code", ownerErr.Code)
				continue
			}
			s.log.WarnContext(ctx, "reconcile pending operation failed",
				"operation_id", op.ID, "error", err)
		}
	}
	return nil
}

// GetOperation returns a moderation operation if the actor is authorized to view it.
func (s *Service) GetOperation(ctx context.Context, actor domain.Actor, id uuid.UUID) (domain.Operation, error) {
	if !actor.CanModerateTracks() {
		return domain.Operation{}, domain.ErrForbidden
	}
	op, err := s.ops.GetOperation(ctx, id)
	if err != nil {
		return domain.Operation{}, err
	}
	if !actor.CanViewOperation(op) {
		return domain.Operation{}, domain.ErrOperationNotFound
	}
	return op, nil
}

// RecordTranscodeEvent projects a media pipeline event into admin.transcode_jobs.
func (s *Service) RecordTranscodeEvent(ctx context.Context, job domain.TranscodeJob) error {
	if job.ProjectedAt.IsZero() {
		job.ProjectedAt = s.now().UTC()
	}
	return s.jobs.UpsertTranscodeJob(ctx, job)
}
