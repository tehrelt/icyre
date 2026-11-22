//go:build integration

package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	platformpg "github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/services/auth/internal/domain"
)

func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("AUTH_TEST_DATABASE_DSN is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("auth_it_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	cfg, _ := pgxpool.ParseConfig(dsn)
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
	if err := platformpg.Migrate(ctx, pool, Schema, Migrations(), slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	return pool
}

func h(s string) []byte { x := sha256.Sum256([]byte(s)); return x[:] }

func TestAccountsAndSessions(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	accounts, sessions := NewAccounts(pool), NewSessions(pool)
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)

	acc := domain.NewAccount(uuid.Must(uuid.NewV7()), "rin@example.com", "$argon2id$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA", now)
	if err := accounts.Create(ctx, acc); err != nil {
		t.Fatal(err)
	}
	dup := domain.NewAccount(uuid.Must(uuid.NewV7()), "rin@example.com", acc.PasswordHash, now)
	if err := accounts.Create(ctx, dup); !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("duplicate email: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO auth.accounts (id, email, password_hash, created_at, updated_at) VALUES ($1, 'x@y.z', 'plaintext', now(), now())`, uuid.New()); err == nil {
		t.Fatal("DB accepted a non-Argon2id password hash")
	}
	got, err := accounts.ByEmail(ctx, "rin@example.com")
	if err != nil || got.ID != acc.ID || got.Roles[0] != domain.RoleUser {
		t.Fatalf("account %+v err %v", got, err)
	}

	s := domain.NewSession(uuid.Must(uuid.NewV7()), acc.ID, h("r1"), "test-agent", now, time.Hour)
	if err := sessions.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := sessions.ByRefreshHash(ctx, h("r1"))
	if err != nil || loaded.ID != s.ID {
		t.Fatalf("by hash: %+v %v", loaded, err)
	}

	// Compare-and-swap rotation: the second writer with the same old hash loses.
	a, b := loaded, loaded
	_ = a.Rotate(h("r2"), now.Add(time.Minute), time.Hour)
	_ = b.Rotate(h("r3"), now.Add(time.Minute), time.Hour)
	if err := sessions.SaveRotation(ctx, a, h("r1")); err != nil {
		t.Fatal(err)
	}
	if err := sessions.SaveRotation(ctx, b, h("r1")); !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("concurrent rotation: %v", err)
	}
	prev, err := sessions.ByPreviousHash(ctx, h("r1"))
	if err != nil || prev.ID != s.ID || !bytes.Equal(prev.RefreshHash, h("r2")) {
		t.Fatalf("previous hash lookup: %+v %v", prev, err)
	}

	active, _ := sessions.ListActive(ctx, acc.ID, now.Add(2*time.Minute))
	if len(active) != 1 {
		t.Fatalf("active = %d", len(active))
	}
	prev.Revoke(domain.RevokeLogout, now.Add(3*time.Minute))
	if err := sessions.SaveRevocation(ctx, prev); err != nil {
		t.Fatal(err)
	}
	active, _ = sessions.ListActive(ctx, acc.ID, now.Add(4*time.Minute))
	if len(active) != 0 {
		t.Fatalf("revoked session still active")
	}
	final, _ := sessions.Get(ctx, s.ID)
	if final.RevokeReason != domain.RevokeLogout || final.RevokedAt == nil {
		t.Fatalf("revocation not stored: %+v", final)
	}
}

func TestModerationRepositories(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	accounts := NewAccounts(pool)
	sessions := NewSessions(pool)
	receipts := NewModerationReceipts(pool)
	pending := NewPendingRevocations(pool)
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)

	acc1 := domain.NewAccount(uuid.Must(uuid.NewV7()), "u1@example.com", "$argon2id$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA", now)
	acc2 := domain.NewAccount(uuid.Must(uuid.NewV7()), "u2@example.com", "$argon2id$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA", now)
	if err := accounts.Create(ctx, acc1); err != nil {
		t.Fatal(err)
	}
	if err := accounts.Create(ctx, acc2); err != nil {
		t.Fatal(err)
	}

	if err := accounts.SetBlocked(ctx, acc1.ID, true, &now); err != nil {
		t.Fatal(err)
	}
	locked, err := accounts.ByIDForUpdate(ctx, acc1.ID)
	if err != nil || !locked.Blocked || locked.BlockedAt == nil {
		t.Fatalf("ByIDForUpdate = %+v err=%v", locked, err)
	}
	byEmail, err := accounts.ByEmailForUpdate(ctx, acc2.Email)
	if err != nil || byEmail.ID != acc2.ID {
		t.Fatalf("ByEmailForUpdate = %+v err=%v", byEmail, err)
	}

	onlyBlocked := true
	listed, err := accounts.ListAccounts(ctx, nil, 10, &onlyBlocked)
	if err != nil || len(listed) != 1 || listed[0].ID != acc1.ID {
		t.Fatalf("ListAccounts(blocked=true) = %+v err=%v", listed, err)
	}
	afterFirst, err := accounts.ListAccounts(ctx, &acc1.ID, 10, nil)
	if err != nil || len(afterFirst) != 1 || afterFirst[0].ID != acc2.ID {
		t.Fatalf("ListAccounts(afterID) = %+v err=%v", afterFirst, err)
	}

	s1 := domain.NewSession(uuid.Must(uuid.NewV7()), acc1.ID, h("s1"), "ua", now, time.Hour)
	s2 := domain.NewSession(uuid.Must(uuid.NewV7()), acc1.ID, h("s2"), "ua", now, time.Hour)
	_ = sessions.Create(ctx, s1)
	_ = sessions.Create(ctx, s2)
	revokedIDs, err := sessions.RevokeAllUserSessions(ctx, acc1.ID, domain.RevokeAccountBlocked, now.Add(time.Minute))
	if err != nil || len(revokedIDs) != 2 {
		t.Fatalf("RevokeAllUserSessions = %v err=%v", revokedIDs, err)
	}

	opID := uuid.Must(uuid.NewV7())
	rec := domain.ModerationReceipt{OperationID: opID, UserID: acc1.ID, Blocked: true, Changed: true, RevokedSessions: 2, CreatedAt: now}
	if err := receipts.SaveModerationReceipt(ctx, rec); err != nil {
		t.Fatal(err)
	}
	gotRec, err := receipts.GetModerationReceipt(ctx, opID)
	if err != nil || gotRec.OperationID != opID || gotRec.RevokedSessions != 2 {
		t.Fatalf("GetModerationReceipt = %+v err=%v", gotRec, err)
	}

	if err := pending.SavePendingRevocations(ctx, revokedIDs, now.Add(16*time.Minute)); err != nil {
		t.Fatal(err)
	}
	items, err := pending.ListPendingRevocations(ctx, 10)
	if err != nil || len(items) != 2 {
		t.Fatalf("ListPendingRevocations = %+v err=%v", items, err)
	}
	if err := pending.DeletePendingRevocation(ctx, revokedIDs[0]); err != nil {
		t.Fatal(err)
	}
	items, err = pending.ListPendingRevocations(ctx, 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("ListPendingRevocations after delete = %+v err=%v", items, err)
	}
}

func TestAccountBlockingAndModerationReceipts(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	accounts, sessions, receipts := NewAccounts(pool), NewSessions(pool), NewModerationReceipts(pool)
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)

	acc := domain.NewAccount(uuid.Must(uuid.NewV7()), "mod@example.com", "$argon2id$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA", now)
	if err := accounts.Create(ctx, acc); err != nil {
		t.Fatal(err)
	}
	s1 := domain.NewSession(uuid.Must(uuid.NewV7()), acc.ID, h("m1"), "ua1", now, time.Hour)
	s2 := domain.NewSession(uuid.Must(uuid.NewV7()), acc.ID, h("m2"), "ua2", now, time.Hour)
	if err := sessions.Create(ctx, s1); err != nil {
		t.Fatal(err)
	}
	if err := sessions.Create(ctx, s2); err != nil {
		t.Fatal(err)
	}

	if err := platformpg.InTx(ctx, pool, func(ctx context.Context) error {
		locked, err := accounts.ByIDForUpdate(ctx, acc.ID)
		if err != nil || locked.Blocked {
			return fmt.Errorf("ByIDForUpdate: %+v %w", locked, err)
		}
		if _, err := accounts.ByEmailForUpdate(ctx, acc.Email); err != nil {
			return err
		}
		blockedAt := now.Add(time.Minute)
		if err := accounts.SetBlocked(ctx, acc.ID, true, &blockedAt); err != nil {
			return err
		}
		revoked, err := sessions.RevokeAllUserSessions(ctx, acc.ID, domain.RevokeAccountBlocked, blockedAt)
		if err != nil || len(revoked) != 2 {
			return fmt.Errorf("RevokeAllUserSessions: %v %w", revoked, err)
		}
		return receipts.SaveModerationReceipt(ctx, domain.ModerationReceipt{
			OperationID: uuid.Must(uuid.NewV7()), UserID: acc.ID, Blocked: true, Changed: true, RevokedSessions: len(revoked), CreatedAt: blockedAt,
		})
	}); err != nil {
		t.Fatal(err)
	}

	onlyBlocked := true
	listed, err := accounts.ListAccounts(ctx, nil, 10, &onlyBlocked)
	if err != nil || len(listed) != 1 || !listed[0].Blocked || listed[0].BlockedAt == nil {
		t.Fatalf("ListAccounts: %+v %v", listed, err)
	}
}
