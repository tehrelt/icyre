package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/requestid"
	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

type memTx struct{}

func (memTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type memRepo struct {
	ops   map[uuid.UUID]domain.Operation
	byKey map[string]uuid.UUID
	audit []domain.AuditEntry
	jobs  map[uuid.UUID]domain.TranscodeJob
}

func newMemRepo() *memRepo {
	return &memRepo{
		ops:   map[uuid.UUID]domain.Operation{},
		byKey: map[string]uuid.UUID{},
		jobs:  map[uuid.UUID]domain.TranscodeJob{},
	}
}

func (r *memRepo) FindOrCreateOperation(_ context.Context, op domain.Operation) (domain.Operation, bool, error) {
	k := op.ActorID.String() + ":" + op.IdempotencyKey
	if id, ok := r.byKey[k]; ok {
		return r.ops[id], false, nil
	}
	r.ops[op.ID] = op
	r.byKey[k] = op.ID
	return op, true, nil
}

func (r *memRepo) GetOperation(_ context.Context, id uuid.UUID) (domain.Operation, error) {
	op, ok := r.ops[id]
	if !ok {
		return domain.Operation{}, domain.ErrOperationNotFound
	}
	return op, nil
}

func (r *memRepo) FinalizeOperation(_ context.Context, op domain.Operation) (domain.Operation, error) {
	cur, ok := r.ops[op.ID]
	if !ok {
		return domain.Operation{}, domain.ErrOperationNotFound
	}
	if cur.Status != domain.StatusPending {
		return cur, nil
	}
	r.ops[op.ID] = op
	return op, nil
}

func (r *memRepo) ListPendingOperations(_ context.Context, limit int) ([]domain.Operation, error) {
	var out []domain.Operation
	for _, op := range r.ops {
		if op.Status == domain.StatusPending {
			out = append(out, op)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (r *memRepo) AppendAuditLog(_ context.Context, entry domain.AuditEntry) error {
	for _, existing := range r.audit {
		if existing.OperationID == entry.OperationID && existing.Phase == entry.Phase {
			return nil
		}
	}
	r.audit = append(r.audit, entry)
	return nil
}

func (r *memRepo) ListAuditLogs(_ context.Context, filter domain.AuditFilter) ([]domain.AuditEntry, error) {
	out := make([]domain.AuditEntry, 0, len(r.audit))
	for i := len(r.audit) - 1; i >= 0; i-- {
		e := r.audit[i]
		if filter.ActorID != nil && e.ActorID != *filter.ActorID {
			continue
		}
		out = append(out, e)
		if len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (r *memRepo) UpsertTranscodeJob(_ context.Context, job domain.TranscodeJob) error {
	r.jobs[job.UploadID] = job
	return nil
}

func (r *memRepo) ListTranscodeJobs(_ context.Context, filter domain.TranscodeJobFilter) ([]domain.TranscodeJob, error) {
	out := make([]domain.TranscodeJob, 0, len(r.jobs))
	for _, j := range r.jobs {
		out = append(out, j)
		if len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

type stubCatalog struct {
	calls   int
	receipt domain.TrackModerationReceipt
	err     error
	page    domain.Page[domain.TrackSummary]
}

func (s *stubCatalog) ListTracks(context.Context, domain.TrackListQuery) (domain.Page[domain.TrackSummary], error) {
	return s.page, s.err
}

func (s *stubCatalog) ModerateTrack(_ context.Context, opID, trackID uuid.UUID, _ bool) (domain.TrackModerationReceipt, error) {
	s.calls++
	if s.err != nil {
		return domain.TrackModerationReceipt{}, s.err
	}
	rec := s.receipt
	rec.OperationID = opID
	rec.TrackID = trackID
	return rec, nil
}

type stubAuth struct {
	calls   int
	receipt domain.UserModerationReceipt
	err     error
	page    domain.Page[domain.UserSummary]
}

func (s *stubAuth) ListUsers(context.Context, domain.UserListQuery) (domain.Page[domain.UserSummary], error) {
	return s.page, s.err
}

func (s *stubAuth) ModerateUser(_ context.Context, opID, userID uuid.UUID, _ bool) (domain.UserModerationReceipt, error) {
	s.calls++
	if s.err != nil {
		return domain.UserModerationReceipt{}, s.err
	}
	rec := s.receipt
	rec.OperationID = opID
	rec.UserID = userID
	return rec, nil
}

type stubIngest struct {
	page domain.Page[domain.IngestUpload]
	err  error
}

func (s *stubIngest) ListUploads(context.Context, domain.IngestListQuery) (domain.Page[domain.IngestUpload], error) {
	return s.page, s.err
}

func newTestService(repo *memRepo, cat *stubCatalog, auth *stubAuth, ingest *stubIngest) *Service {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	return New(Deps{
		Ops:         repo,
		Audit:       repo,
		Jobs:        repo,
		Catalog:     cat,
		Auth:        auth,
		MediaIngest: ingest,
		Tx:          memTx{},
		Log:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:         func() time.Time { return now },
	})
}

func TestRBACAndSelfBlock(t *testing.T) {
	repo := newMemRepo()
	svc := newTestService(repo, &stubCatalog{}, &stubAuth{}, &stubIngest{})
	ctx := context.Background()
	plain := domain.Actor{ID: uuid.New()}
	mod := domain.Actor{ID: uuid.New(), Moderator: true}
	admin := domain.Actor{ID: uuid.New(), Admin: true}
	target := uuid.New()

	if _, err := svc.ModerateTrack(ctx, plain, "k1", target, true, "spam"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for plain user, got %v", err)
	}
	if _, err := svc.ModerateUser(ctx, mod, "k1", target, true, "spam"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for moderator on user, got %v", err)
	}
	if _, err := svc.ModerateUser(ctx, admin, "k1", admin.ID, true, "self"); !errors.Is(err, domain.ErrCannotBlockSelf) {
		t.Fatalf("expected ErrCannotBlockSelf, got %v", err)
	}
	if _, err := svc.ListUsers(ctx, mod, domain.UserListQuery{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for moderator ListUsers, got %v", err)
	}
	if _, err := svc.ListAuditLogs(ctx, mod, domain.AuditFilter{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for moderator ListAuditLogs, got %v", err)
	}
	if _, err := svc.ListTracks(ctx, plain, domain.TrackListQuery{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for plain ListTracks, got %v", err)
	}
	if _, err := svc.ListIngestJobs(ctx, plain, domain.IngestListQuery{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for plain ListIngestJobs, got %v", err)
	}
	if _, err := svc.ListTranscodeJobs(ctx, plain, domain.TranscodeJobFilter{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for plain ListTranscodeJobs, got %v", err)
	}
	if _, err := svc.GetOperation(ctx, plain, uuid.New()); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for plain GetOperation, got %v", err)
	}
}

func TestModerateTrackAppliedNoopReplayAndConflict(t *testing.T) {
	repo := newMemRepo()
	cat := &stubCatalog{receipt: domain.TrackModerationReceipt{PreviousStatus: "READY", CurrentStatus: "BLOCKED", Changed: true}}
	svc := newTestService(repo, cat, &stubAuth{}, &stubIngest{})
	ctx := requestid.With(context.Background(), "req-123")
	mod := domain.Actor{ID: uuid.New(), Moderator: true}
	trackID := uuid.New()

	op, err := svc.ModerateTrack(ctx, mod, "idem-1", trackID, true, "copyright")
	if err != nil {
		t.Fatalf("ModerateTrack failed: %v", err)
	}
	if op.Status != domain.StatusApplied || op.ResultCode != "APPLIED" || op.RequestID != "req-123" {
		t.Fatalf("unexpected op: %+v", op)
	}
	if len(repo.audit) != 2 || repo.audit[0].Phase != domain.PhaseRequested || repo.audit[1].Phase != domain.PhaseApplied {
		t.Fatalf("unexpected audit entries: %+v", repo.audit)
	}

	// Replay with same key and params must not call Catalog again.
	replayed, err := svc.ModerateTrack(ctx, mod, "idem-1", trackID, true, "copyright")
	if err != nil || replayed.ID != op.ID || cat.calls != 1 {
		t.Fatalf("replay mismatch: op=%+v calls=%d err=%v", replayed, cat.calls, err)
	}

	// Conflict with same key and different reason.
	if _, err := svc.ModerateTrack(ctx, mod, "idem-1", trackID, true, "different reason"); !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("expected ErrIdempotencyConflict, got %v", err)
	}

	// NOOP case.
	cat.receipt = domain.TrackModerationReceipt{PreviousStatus: "BLOCKED", CurrentStatus: "BLOCKED", Changed: false}
	noopOp, err := svc.ModerateTrack(ctx, mod, "idem-2", trackID, true, "already blocked")
	if err != nil || noopOp.Status != domain.StatusNoop || noopOp.ResultCode != "NOOP" {
		t.Fatalf("expected NOOP, got %+v err=%v", noopOp, err)
	}
}

func TestModerateUserPendingReconcileAndOwnerRejection(t *testing.T) {
	repo := newMemRepo()
	auth := &stubAuth{err: domain.ErrUpstreamUnavailable}
	svc := newTestService(repo, &stubCatalog{}, auth, &stubIngest{})
	ctx := context.Background()
	admin := domain.Actor{ID: uuid.New(), Admin: true}
	userID := uuid.New()

	// Transient error leaves operation PENDING and returns (op, nil) for 202 Accepted.
	op, err := svc.ModerateUser(ctx, admin, "u-1", userID, true, "abuse")
	if err != nil {
		t.Fatalf("expected nil error on transient upstream failure, got %v", err)
	}
	if op.Status != domain.StatusPending {
		t.Fatalf("expected PENDING, got %s", op.Status)
	}

	// ReconcilePending retries with the same operation ID and settles APPLIED.
	auth.err = nil
	auth.receipt = domain.UserModerationReceipt{Blocked: true, Changed: true, RevokedSessions: 3, RevocationsPending: true}
	if err := svc.ReconcilePending(ctx, 10); err != nil {
		t.Fatalf("ReconcilePending failed: %v", err)
	}
	settled, err := svc.GetOperation(ctx, admin, op.ID)
	if err != nil || settled.Status != domain.StatusApplied || !settled.RevocationsPending {
		t.Fatalf("expected settled APPLIED with revocationsPending, got %+v err=%v", settled, err)
	}

	// Owner 422/404/409 rejection settles REJECTED and returns *domain.OwnerError.
	auth.err = &domain.OwnerError{Status: 422, Code: "CANNOT_BLOCK_ADMIN", Message: "Cannot block an admin account"}
	rejOp, err := svc.ModerateUser(ctx, admin, "u-2", uuid.New(), true, "test")
	var oe *domain.OwnerError
	if !errors.As(err, &oe) || oe.Code != "CANNOT_BLOCK_ADMIN" {
		t.Fatalf("expected OwnerError, got %v", err)
	}
	if rejOp.Status != domain.StatusRejected || rejOp.ResultCode != "CANNOT_BLOCK_ADMIN" {
		t.Fatalf("expected REJECTED op, got %+v", rejOp)
	}
}

func TestGetOperationOwnershipAndQueriesPagination(t *testing.T) {
	repo := newMemRepo()
	cat := &stubCatalog{receipt: domain.TrackModerationReceipt{PreviousStatus: "READY", CurrentStatus: "BLOCKED", Changed: true}}
	svc := newTestService(repo, cat, &stubAuth{}, &stubIngest{})
	ctx := context.Background()
	mod1 := domain.Actor{ID: uuid.New(), Moderator: true}
	mod2 := domain.Actor{ID: uuid.New(), Moderator: true}
	admin := domain.Actor{ID: uuid.New(), Admin: true}

	op, err := svc.ModerateTrack(ctx, mod1, "k-own", uuid.New(), true, "spam")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetOperation(ctx, mod1, op.ID); err != nil {
		t.Fatalf("mod1 should view own op: %v", err)
	}
	if _, err := svc.GetOperation(ctx, mod2, op.ID); !errors.Is(err, domain.ErrOperationNotFound) {
		t.Fatalf("mod2 should get ErrOperationNotFound for mod1 op, got %v", err)
	}
	if _, err := svc.GetOperation(ctx, admin, op.ID); err != nil {
		t.Fatalf("admin should view any op: %v", err)
	}
	if _, err := svc.GetOperation(ctx, admin, uuid.New()); !errors.Is(err, domain.ErrOperationNotFound) {
		t.Fatalf("expected ErrOperationNotFound, got %v", err)
	}

	// Transcode jobs + audit pagination.
	for i := 0; i < 3; i++ {
		if err := svc.RecordTranscodeEvent(ctx, domain.TranscodeJob{
			UploadID: uuid.New(), TrackID: uuid.New(), Status: domain.TranscodeSucceeded, EventAt: time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	jobsPage, err := svc.ListTranscodeJobs(ctx, mod1, domain.TranscodeJobFilter{Limit: 2})
	if err != nil || len(jobsPage.Items) != 2 || !jobsPage.HasMore || jobsPage.NextCursor == nil {
		t.Fatalf("unexpected jobs page: %+v err=%v", jobsPage, err)
	}
	if _, _, err := DecodeTimeIDCursor(*jobsPage.NextCursor); err != nil {
		t.Fatalf("invalid cursor: %v", err)
	}

	auditPage, err := svc.ListAuditLogs(ctx, admin, domain.AuditFilter{Limit: 1})
	if err != nil || len(auditPage.Items) != 1 || !auditPage.HasMore || auditPage.NextCursor == nil {
		t.Fatalf("unexpected audit page: %+v err=%v", auditPage, err)
	}
}
