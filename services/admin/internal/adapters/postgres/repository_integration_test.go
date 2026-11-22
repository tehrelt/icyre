//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	platformpg "github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ADMIN_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("ADMIN_TEST_DATABASE_DSN is not set")
	}
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("admin_it_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	})

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := platformpg.Migrate(ctx, pool, Schema, Migrations(), log); err != nil {
		t.Fatal(err)
	}
	if err := platformpg.Migrate(ctx, pool, Schema, Migrations(), log); err != nil {
		t.Fatal(err)
	}
	return pool
}

func newID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

func TestOperationsAndAuditImmutability(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

	actorID := newID()
	targetID := newID()
	op := domain.Operation{
		ID:             newID(),
		ActorID:        actorID,
		IdempotencyKey: "idem-1",
		RequestHash:    "hash-1",
		Action:         domain.ActionTrackBlock,
		TargetType:     domain.TargetTrack,
		TargetID:       targetID,
		DesiredBlocked: true,
		Reason:         "copyright",
		Status:         domain.StatusPending,
		BeforeState:    map[string]any{},
		AfterState:     map[string]any{},
		RequestID:      "req-1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	got, created, err := repo.FindOrCreateOperation(ctx, op)
	if err != nil || !created || got.ID != op.ID {
		t.Fatalf("FindOrCreateOperation create: got=%+v created=%v err=%v", got, created, err)
	}

	// Duplicate (actor_id, idempotency_key) returns existing operation.
	dup := op
	dup.ID = newID()
	dup.RequestHash = "hash-2"
	got2, created2, err := repo.FindOrCreateOperation(ctx, dup)
	if err != nil || created2 || got2.ID != op.ID || got2.RequestHash != "hash-1" {
		t.Fatalf("FindOrCreateOperation dup: got=%+v created=%v err=%v", got2, created2, err)
	}

	pending, err := repo.ListPendingOperations(ctx, 10)
	if err != nil || len(pending) != 1 || pending[0].ID != op.ID {
		t.Fatalf("ListPendingOperations: %+v err=%v", pending, err)
	}

	settled := op.Settle(domain.StatusApplied, "APPLIED", map[string]any{"status": "READY"}, map[string]any{"status": "BLOCKED"}, false, now.Add(time.Second))
	finalized, err := repo.FinalizeOperation(ctx, settled)
	if err != nil || finalized.Status != domain.StatusApplied || finalized.AfterState["status"] != "BLOCKED" {
		t.Fatalf("FinalizeOperation: %+v err=%v", finalized, err)
	}

	// Audit log insert + idempotency + immutability trigger.
	entry1 := domain.AuditEntry{
		ID: newID(), OperationID: op.ID, ActorID: actorID, Action: domain.ActionTrackBlock,
		TargetType: domain.TargetTrack, TargetID: targetID, Phase: domain.PhaseRequested,
		Reason: "copyright", Metadata: map[string]any{"desiredBlocked": true}, RequestID: "req-1", CreatedAt: now,
	}
	entry2 := domain.AuditEntry{
		ID: newID(), OperationID: op.ID, ActorID: actorID, Action: domain.ActionTrackBlock,
		TargetType: domain.TargetTrack, TargetID: targetID, Phase: domain.PhaseApplied,
		Reason: "copyright", Metadata: map[string]any{"changed": true}, RequestID: "req-1", CreatedAt: now.Add(time.Second),
	}
	if err := repo.AppendAuditLog(ctx, entry1); err != nil {
		t.Fatal(err)
	}
	if err := repo.AppendAuditLog(ctx, entry1); err != nil {
		t.Fatalf("duplicate audit phase should be idempotent: %v", err)
	}
	if err := repo.AppendAuditLog(ctx, entry2); err != nil {
		t.Fatal(err)
	}

	// UPDATE, DELETE, and TRUNCATE on admin.audit_logs must fail via trigger.
	if _, err := pool.Exec(ctx, `UPDATE admin.audit_logs SET reason = 'tampered' WHERE id = $1`, entry1.ID); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutability error on UPDATE, got %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM admin.audit_logs WHERE id = $1`, entry1.ID); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutability error on DELETE, got %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE admin.audit_logs`); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutability error on TRUNCATE, got %v", err)
	}

	// Keyset pagination on audit logs.
	page1, err := repo.ListAuditLogs(ctx, domain.AuditFilter{ActorID: &actorID, Limit: 1})
	if err != nil || len(page1) != 1 || page1[0].ID != entry2.ID {
		t.Fatalf("audit page1 = %+v err=%v", page1, err)
	}
	page2, err := repo.ListAuditLogs(ctx, domain.AuditFilter{ActorID: &actorID, AfterCreatedAt: &page1[0].CreatedAt, AfterID: &page1[0].ID, Limit: 1})
	if err != nil || len(page2) != 1 || page2[0].ID != entry1.ID {
		t.Fatalf("audit page2 = %+v err=%v", page2, err)
	}

	if _, err := repo.GetOperation(ctx, newID()); !errors.Is(err, domain.ErrOperationNotFound) {
		t.Fatalf("expected ErrOperationNotFound, got %v", err)
	}
}

func TestTranscodeJobsIdempotencyAndPagination(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool)
	t0 := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

	u1, tr1 := newID(), newID()
	u2, tr2 := newID(), newID()

	// QUEUED -> SUCCEEDED -> stale QUEUED must NOT regress.
	if err := repo.UpsertTranscodeJob(ctx, domain.TranscodeJob{
		UploadID: u1, TrackID: tr1, Status: domain.TranscodeQueued, EventAt: t0, ProjectedAt: t0,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertTranscodeJob(ctx, domain.TranscodeJob{
		UploadID: u1, TrackID: tr1, Status: domain.TranscodeSucceeded, EventAt: t0.Add(time.Minute), ProjectedAt: t0.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertTranscodeJob(ctx, domain.TranscodeJob{
		UploadID: u1, TrackID: tr1, Status: domain.TranscodeQueued, EventAt: t0.Add(2 * time.Minute), ProjectedAt: t0.Add(2 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	// Older terminal event must NOT overwrite newer terminal event.
	if err := repo.UpsertTranscodeJob(ctx, domain.TranscodeJob{
		UploadID: u1, TrackID: tr1, Status: domain.TranscodeFailed, FailureReason: "OLD", EventAt: t0.Add(30 * time.Second), ProjectedAt: t0.Add(3 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	// Second job u2 FAILED.
	if err := repo.UpsertTranscodeJob(ctx, domain.TranscodeJob{
		UploadID: u2, TrackID: tr2, Status: domain.TranscodeFailed, FailureReason: "UNDECODABLE", EventAt: t0.Add(5 * time.Minute), ProjectedAt: t0.Add(5 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	page1, err := repo.ListTranscodeJobs(ctx, domain.TranscodeJobFilter{Limit: 1})
	if err != nil || len(page1) != 1 || page1[0].UploadID != u2 || page1[0].Status != domain.TranscodeFailed {
		t.Fatalf("jobs page1 = %+v err=%v", page1, err)
	}
	page2, err := repo.ListTranscodeJobs(ctx, domain.TranscodeJobFilter{AfterEventAt: &page1[0].EventAt, AfterUploadID: &page1[0].UploadID, Limit: 1})
	if err != nil || len(page2) != 1 || page2[0].UploadID != u1 || page2[0].Status != domain.TranscodeSucceeded {
		t.Fatalf("jobs page2 = %+v err=%v", page2, err)
	}
}
