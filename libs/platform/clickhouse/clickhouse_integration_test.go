//go:build integration

package clickhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// Runs against CLICKHOUSE_URL (make up-core): a throwaway database per run.
func TestMigrateInsertQuery(t *testing.T) {
	url := os.Getenv("CLICKHOUSE_URL")
	if url == "" {
		t.Skip("CLICKHOUSE_URL not set")
	}
	ctx := context.Background()
	cfg := Config{URL: url, Username: os.Getenv("CLICKHOUSE_USERNAME"), Password: os.Getenv("CLICKHOUSE_PASSWORD")}
	db := fmt.Sprintf("it_%d", time.Now().UnixNano())
	if err := New(cfg, nil).Exec(ctx, "CREATE DATABASE "+db, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = New(cfg, nil).Exec(context.Background(), "DROP DATABASE IF EXISTS "+db, nil) })
	cfg.Database = db
	c := New(cfg, nil)
	if err := c.Check(ctx); err != nil {
		t.Fatal(err)
	}

	ms := []Migration{{Version: 1, Name: "events", Statements: []string{
		"CREATE TABLE IF NOT EXISTS events (id UInt32, name String) ENGINE = MergeTree ORDER BY id SETTINGS non_replicated_deduplication_window = 100",
	}}}
	for run, want := range []int{1, 0} { // the second run is a no-op
		n, err := c.Migrate(ctx, ms)
		if err != nil || n != want {
			t.Fatalf("run %d: applied %d err %v", run, n, err)
		}
	}

	rows := []any{map[string]any{"id": 1, "name": "a'; DROP TABLE events; --"}, map[string]any{"id": 2, "name": "b"}}
	for range 2 { // same token: the retry is deduplicated
		if err := c.Insert(ctx, "events", rows, "batch-1"); err != nil {
			t.Fatal(err)
		}
	}

	var got []string
	err := c.Query(ctx, "SELECT name FROM events WHERE id >= {min:UInt32} ORDER BY id", Params{"min": "1"}, func(line []byte) error {
		var r struct{ Name string }
		err := json.Unmarshal(line, &r)
		got = append(got, r.Name)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "a'; DROP TABLE events; --" {
		t.Fatalf("rows %q", got)
	}
}
