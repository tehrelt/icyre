//go:build integration

package postgres

import (
	"context"
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
	"github.com/tehrelt/icyre/services/user-profile/internal/domain"
)

func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("PROFILE_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("PROFILE_TEST_DATABASE_DSN is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("profile_it_%d", time.Now().UnixNano())
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

func TestRepository(t *testing.T) {
	repo := New(newTestDB(t))
	ctx := context.Background()
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)

	a := domain.FromRegistration(uuid.New(), "rin@example.com", now)
	if created, err := repo.CreateIfAbsent(ctx, a); err != nil || !created {
		t.Fatalf("create: %v %v", created, err)
	}
	if created, err := repo.CreateIfAbsent(ctx, a); err != nil || created {
		t.Fatalf("second create must be a no-op: %v %v", created, err)
	}
	b := domain.FromRegistration(uuid.New(), "rin@other.com", now)
	if _, err := repo.CreateIfAbsent(ctx, b); !errors.Is(err, domain.ErrUsernameTaken) {
		t.Fatalf("username clash: %v", err)
	}
	b.Username = "rin2"
	if _, err := repo.CreateIfAbsent(ctx, b); err != nil {
		t.Fatal(err)
	}

	name, country := "Rin Aoki", "JP"
	if _, err := a.Apply(domain.Changes{DisplayName: &name, Country: &country}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, a); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, a.UserID)
	if err != nil || got.DisplayName != "Rin Aoki" || got.Country != "JP" || !got.UpdatedAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("get %+v %v", got, err)
	}
	b.Username = "rin"
	if err := repo.Update(ctx, b); !errors.Is(err, domain.ErrUsernameTaken) {
		t.Fatalf("update clash: %v", err)
	}
	if _, err := repo.Get(ctx, uuid.New()); !errors.Is(err, domain.ErrProfileNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if err := repo.Update(ctx, domain.Profile{UserID: uuid.New(), Username: "ghost", DisplayName: "G"}); !errors.Is(err, domain.ErrProfileNotFound) {
		t.Fatalf("update missing: %v", err)
	}
}
