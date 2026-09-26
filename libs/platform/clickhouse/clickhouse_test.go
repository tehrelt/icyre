package clickhouse

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestInsertSendsJSONEachRowWithDedupToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("query") != "INSERT INTO events FORMAT JSONEachRow" || q.Get("database") != "icyre" ||
			q.Get("insert_deduplication_token") != "batch-1" || r.Header.Get("X-ClickHouse-User") != "u" {
			t.Errorf("request %s %v", r.URL, r.Header)
		}
		body, _ := io.ReadAll(r.Body)
		if got := string(body); got != "{\"id\":1}\n{\"id\":2}\n" {
			t.Errorf("body %q", got)
		}
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, Username: "u", Password: "p", Database: "icyre"}, srv.Client())
	rows := []any{map[string]int{"id": 1}, map[string]int{"id": 2}}
	if err := c.Insert(context.Background(), "events", rows, "batch-1"); err != nil {
		t.Fatal(err)
	}
}

func TestQueryPassesParamsAndScansRows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.HasSuffix(string(body), "FORMAT JSONEachRow") || r.URL.Query().Get("param_day") != "2026-09-27" {
			t.Errorf("query %q params %v", body, r.URL.Query())
		}
		_, _ = io.WriteString(w, "{\"n\":1}\n{\"n\":2}\n")
	}))
	defer srv.Close()

	var sum int
	err := New(Config{URL: srv.URL}, srv.Client()).Query(context.Background(),
		"SELECT n FROM t WHERE day = {day:Date}", Params{"day": "2026-09-27"}, func(line []byte) error {
			var r struct{ N int }
			err := json.Unmarshal(line, &r)
			sum += r.N
			return err
		})
	if err != nil || sum != 3 {
		t.Fatalf("sum %d err %v", sum, err)
	}
}

func TestErrorCarriesExceptionCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-ClickHouse-Exception-Code", "60")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "Code: 60. DB::Exception: Unknown table\n")
	}))
	defer srv.Close()

	err := New(Config{URL: srv.URL}, srv.Client()).Exec(context.Background(), "SELECT 1 FROM nope", nil)
	var e *Error
	if !errors.As(err, &e) || e.Code != "60" || e.Status != http.StatusNotFound {
		t.Fatalf("err %v", err)
	}
}

func TestLoadMigrationsOrdersAndSplits(t *testing.T) {
	fsys := fstest.MapFS{
		"m/00002_second.sql": {Data: []byte("-- note; not a statement\nCREATE TABLE b (x UInt8)\nENGINE = Memory;\n\nCREATE TABLE c (x UInt8) ENGINE = Memory;\n")},
		"m/00001_first.sql":  {Data: []byte("CREATE TABLE a (x UInt8) ENGINE = Memory")},
		"m/README.md":        {Data: []byte("ignored")},
	}
	ms, err := LoadMigrations(fsys, "m")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[0].Version != 1 || ms[0].Name != "first" || ms[1].Version != 2 {
		t.Fatalf("migrations %+v", ms)
	}
	want := []string{"CREATE TABLE b (x UInt8)\nENGINE = Memory", "CREATE TABLE c (x UInt8) ENGINE = Memory"}
	if len(ms[1].Statements) != 2 || ms[1].Statements[0] != want[0] || ms[1].Statements[1] != want[1] {
		t.Fatalf("statements %q", ms[1].Statements)
	}
}

func TestLoadMigrationsRejectsBadNames(t *testing.T) {
	for name, fsys := range map[string]fstest.MapFS{
		"no version": {"m/first.sql": {Data: []byte("SELECT 1")}},
		"duplicate":  {"m/00001_a.sql": {Data: []byte("SELECT 1")}, "m/1_b.sql": {Data: []byte("SELECT 1")}},
		"empty":      {"m/00001_a.sql": {Data: []byte("-- nothing\n")}},
	} {
		if _, err := LoadMigrations(fsys, "m"); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}
