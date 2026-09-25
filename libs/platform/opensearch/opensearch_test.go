package opensearch

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBulkEncodesVersionsAndParsesItems(t *testing.T) {
	var lines []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_bulk" || r.URL.Query().Get("refresh") != "wait_for" || r.Header.Get("Content-Type") != "application/x-ndjson" {
			t.Errorf("request %s %s %s", r.URL, r.Header.Get("Content-Type"), r.Method)
		}
		sc := bufio.NewScanner(r.Body)
		for sc.Scan() {
			var m map[string]any
			_ = json.Unmarshal(sc.Bytes(), &m)
			lines = append(lines, m)
		}
		_, _ = io.WriteString(w, `{"errors":true,"items":[
			{"index":{"_id":"a","status":201}},
			{"index":{"_id":"b","status":409,"error":{"type":"version_conflict_engine_exception","reason":"stale"}}},
			{"delete":{"_id":"c","status":404}}]}`)
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL}, srv.Client())
	items, err := c.Bulk(context.Background(), []BulkOp{
		{Action: ActionIndex, Index: "tracks", ID: "a", Version: 10, Doc: map[string]string{"title": "x"}},
		{Action: ActionIndex, Index: "tracks", ID: "b", Version: 5, Doc: map[string]string{"title": "y"}},
		{Action: ActionDelete, Index: "tracks", ID: "c"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 5 {
		t.Fatalf("ndjson lines = %d", len(lines))
	}
	meta := lines[0]["index"].(map[string]any)
	if meta["version"].(float64) != 10 || meta["version_type"] != "external_gte" || meta["_id"] != "a" {
		t.Fatalf("meta %v", meta)
	}
	if _, versioned := lines[4]["delete"].(map[string]any)["version"]; versioned {
		t.Fatal("unversioned delete got a version")
	}
	if len(items) != 3 || items[0].Status != 201 || !items[1].Conflict() || items[1].Error.Type != "version_conflict_engine_exception" || items[2].Status != 404 {
		t.Fatalf("items %+v", items)
	}
}

func TestErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"type":"index_not_found_exception","reason":"no such index [x]"},"status":404}`)
	}))
	defer srv.Close()
	c := New(Config{URL: srv.URL}, srv.Client())
	err := c.Do(context.Background(), http.MethodGet, "/x/_search", nil, nil)
	if !IsNotFound(err) || !strings.Contains(err.Error(), "index_not_found_exception") {
		t.Fatal(err)
	}
	if got, err := c.AliasIndices(context.Background(), "missing"); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
}
