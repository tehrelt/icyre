package clickhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// migrationsTable records applied versions in the client's database.
const migrationsTable = "schema_migrations"

// Migration is one versioned schema file: NNNNN_name.sql.
type Migration struct {
	Version    int
	Name       string
	Statements []string
}

// LoadMigrations reads dir of fsys. Each file holds statements separated by
// a ';' at the end of a line; "--" comment lines are dropped. ClickHouse has
// no transactional DDL, so every statement must be idempotent
// (IF NOT EXISTS / IF EXISTS): a migration interrupted halfway is re-run.
func LoadMigrations(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: read migrations: %w", err)
	}
	var out []Migration
	seen := map[int]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		prefix, name, ok := strings.Cut(strings.TrimSuffix(e.Name(), ".sql"), "_")
		v, err := strconv.Atoi(prefix)
		if !ok || err != nil || v <= 0 {
			return nil, fmt.Errorf("clickhouse: migration %q: want NNNNN_name.sql", e.Name())
		}
		if prev, dup := seen[v]; dup {
			return nil, fmt.Errorf("clickhouse: migrations %q and %q share version %d", prev, e.Name(), v)
		}
		seen[v] = e.Name()
		raw, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		stmts := splitStatements(string(raw))
		if len(stmts) == 0 {
			return nil, fmt.Errorf("clickhouse: migration %q is empty", e.Name())
		}
		out = append(out, Migration{Version: v, Name: name, Statements: stmts})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func splitStatements(sql string) []string {
	var stmts []string
	var cur strings.Builder
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			stmts = append(stmts, s)
		}
		cur.Reset()
	}
	for _, line := range strings.Split(sql, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "--") {
			continue
		}
		if strings.HasSuffix(t, ";") {
			cur.WriteString(strings.TrimSuffix(strings.TrimRight(line, " \t\r"), ";"))
			flush()
			continue
		}
		cur.WriteString(line)
		cur.WriteString("\n")
	}
	flush()
	return stmts
}

// Migrate applies the migrations newer than the highest recorded version, in
// order, and returns how many it applied. Run it from one process at a time
// (a migrate command or job), not from every replica on startup.
func (c *Client) Migrate(ctx context.Context, migrations []Migration) (int, error) {
	err := c.Exec(ctx, `CREATE TABLE IF NOT EXISTS `+migrationsTable+` (
    version    UInt32,
    name       String,
    applied_at DateTime64(3, 'UTC') DEFAULT now64(3)
) ENGINE = MergeTree ORDER BY version`, nil)
	if err != nil {
		return 0, fmt.Errorf("clickhouse: create %s: %w", migrationsTable, err)
	}
	current, err := c.schemaVersion(ctx)
	if err != nil {
		return 0, err
	}
	applied := 0
	for _, m := range migrations {
		if m.Version <= current {
			continue
		}
		for i, stmt := range m.Statements {
			if err := c.Exec(ctx, stmt, nil); err != nil {
				return applied, fmt.Errorf("clickhouse: migration %d_%s statement %d: %w", m.Version, m.Name, i+1, err)
			}
		}
		err := c.Exec(ctx, "INSERT INTO "+migrationsTable+" (version, name) VALUES ({v:UInt32}, {n:String})",
			Params{"v": strconv.Itoa(m.Version), "n": m.Name})
		if err != nil {
			return applied, fmt.Errorf("clickhouse: record migration %d: %w", m.Version, err)
		}
		applied++
	}
	return applied, nil
}

func (c *Client) schemaVersion(ctx context.Context) (int, error) {
	var v int
	err := c.Query(ctx, "SELECT max(version) AS v FROM "+migrationsTable, nil, func(line []byte) error {
		var r struct {
			V json.Number `json:"v"`
		}
		if err := json.Unmarshal(line, &r); err != nil {
			return err
		}
		n, err := r.V.Int64()
		v = int(n)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("clickhouse: read schema version: %w", err)
	}
	return v, nil
}
