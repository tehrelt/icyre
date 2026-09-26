//go:build integration

package outbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/libs/platform/postgres"
)

type fakeProducer struct {
	mu   sync.Mutex
	sent []string
	fail error
}

func (f *fakeProducer) Publish(_ context.Context, msgs ...kafka.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return f.fail
	}
	for _, m := range msgs {
		f.sent = append(f.sent, string(m.Key))
	}
	return nil
}

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("OUTBOX_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("OUTBOX_TEST_DATABASE_DSN is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("outbox_it_%d", time.Now().UnixNano())
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
	if _, err := pool.Exec(ctx, `
		CREATE SCHEMA svc;
		CREATE TABLE svc.items (id int PRIMARY KEY);
		CREATE TABLE svc.outbox (
			id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, topic text NOT NULL, key bytea,
			value bytea NOT NULL, headers jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	return pool
}

func msg(key string) kafka.Message {
	return kafka.Message{Topic: "svc.events", Key: []byte(key), Value: []byte(`{}`), Headers: map[string]string{"event-type": "x"}}
}

func TestOutboxEndToEnd(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Committed change → its message is in the outbox; rolled back → neither.
	for i, fail := range []bool{false, true, false} {
		err := postgres.InTx(ctx, pool, func(ctx context.Context) error {
			q := postgres.Conn(ctx, pool)
			if _, err := q.Exec(ctx, `INSERT INTO svc.items VALUES ($1)`, i); err != nil {
				return err
			}
			if err := Write(ctx, q, "svc.outbox", msg(fmt.Sprint(i))); err != nil {
				return err
			}
			if fail {
				return errors.New("business rule")
			}
			return nil
		})
		if (err != nil) != fail {
			t.Fatal(i, err)
		}
	}

	p := &fakeProducer{fail: errors.New("broker down")}
	r, err := NewRelay(pool, p, RelayConfig{Table: "svc.outbox", Batch: 1}, quiet, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Broker down: nothing is deleted, the rows wait.
	if _, err := r.RelayOnce(ctx); err == nil {
		t.Fatal("publish failure must surface")
	}
	p.fail = nil
	for range 3 {
		if _, err := r.RelayOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if fmt.Sprint(p.sent) != "[0 2]" {
		t.Fatalf("sent %v (order and rollback)", p.sent)
	}
	var left int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM svc.outbox`).Scan(&left)
	if left != 0 {
		t.Fatalf("%d rows left", left)
	}

	// A second relay stays idle while the first holds the lock.
	_ = Write(ctx, pool, "svc.outbox", msg("3"))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('svc.outbox'))`); err != nil {
		t.Fatal(err)
	}
	if n, err := r.RelayOnce(ctx); err != nil || n != 0 {
		t.Fatalf("follower relayed %d, %v", n, err)
	}
	_ = tx.Rollback(ctx)
	if n, _ := r.RelayOnce(ctx); n != 1 {
		t.Fatalf("after unlock relayed %d", n)
	}
}
