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
