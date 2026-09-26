// Package clickhouse is a thin ClickHouse client over the HTTP interface:
// statements, JSONEachRow inserts and queries, and versioned schema
// migrations. Values reach SQL only as typed query parameters ({name:Type}),
// never by string concatenation.
package clickhouse

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config configures a Client.
type Config struct {
	// URL of the HTTP interface, e.g. http://clickhouse:8123.
	URL      string
	Username string
	Password string
	// Database is the default database of every statement.
	Database string
	// Timeout bounds each request.
	Timeout time.Duration
}

// Client talks to one server (or a load balancer in front of replicas).
type Client struct {
	base string
	cfg  Config
	hc   *http.Client
}

// New returns a Client. hc may carry tracing; nil uses a default client.
func New(cfg Config, hc *http.Client) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if hc == nil {
		hc = &http.Client{}
	}
	return &Client{base: strings.TrimRight(cfg.URL, "/"), cfg: cfg, hc: hc}
}

// Error is a non-2xx response; Code is the ClickHouse exception code when
// the server reported one.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("clickhouse: %d (code %s): %s", e.Status, e.Code, e.Message)
}

// Params are query parameters referenced in SQL as {name:Type}.
type Params map[string]string

// Exec runs one statement that returns no rows (DDL, INSERT ... SELECT).
func (c *Client) Exec(ctx context.Context, query string, params Params) error {
	_, err := c.do(ctx, query, params, nil, "")
	return err
}

// Query runs a SELECT and decodes each JSONEachRow line with scan. The query
// must not carry its own FORMAT clause.
func (c *Client) Query(ctx context.Context, query string, params Params, scan func(line []byte) error) error {
	raw, err := c.do(ctx, query+"\nFORMAT JSONEachRow", params, nil, "")
	if err != nil {
		return err
	}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		if err := scan(sc.Bytes()); err != nil {
			return fmt.Errorf("clickhouse: scan: %w", err)
		}
	}
	return sc.Err()
}

// Insert writes rows (JSON-encoded structs or maps) into table as one block.
// A non-empty dedupToken makes a retried insert of the same batch a no-op on
// replicated and non-replicated MergeTree tables (insert deduplication).
func (c *Client) Insert(ctx context.Context, table string, rows []any, dedupToken string) error {
	if len(rows) == 0 {
		return nil
	}
	var body bytes.Buffer
	enc := json.NewEncoder(&body)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("clickhouse: encode row: %w", err)
		}
	}
	_, err := c.do(ctx, "INSERT INTO "+table+" FORMAT JSONEachRow", nil, &body, dedupToken)
	return err
}

// Check is a readiness probe: the server answers and the database exists.
func (c *Client) Check(ctx context.Context) error {
	var n int
	err := c.Query(ctx, "SELECT count() AS n FROM system.databases WHERE name = {db:String}",
		Params{"db": c.database()}, func(line []byte) error {
			var r struct {
				N json.Number `json:"n"`
			}
			if err := json.Unmarshal(line, &r); err != nil {
				return err
			}
			v, err := r.N.Int64()
			n = int(v)
			return err
		})
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("clickhouse: database %q does not exist", c.database())
	}
	return nil
}

func (c *Client) database() string {
	if c.cfg.Database == "" {
		return "default"
	}
	return c.cfg.Database
}

// do sends the query in the URL (or, with a body, the INSERT head in the URL
// and the rows as the body) and returns the raw 2xx response.
func (c *Client) do(ctx context.Context, query string, params Params, body io.Reader, dedupToken string) ([]byte, error) {
	q := url.Values{}
	q.Set("database", c.database())
	for k, v := range params {
		q.Set("param_"+k, v)
	}
	if dedupToken != "" {
		q.Set("insert_deduplication_token", dedupToken)
	}
	if body == nil {
		body = strings.NewReader(query)
	} else {
		q.Set("query", query)
	}
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/?"+q.Encode(), body)
	if err != nil {
		return nil, err
	}
	if c.cfg.Username != "" {
		req.Header.Set("X-ClickHouse-User", c.cfg.Username)
		req.Header.Set("X-ClickHouse-Key", c.cfg.Password)
	}
	res, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: read: %w", err)
	}
	if res.StatusCode >= 300 {
		return nil, &Error{
			Status:  res.StatusCode,
			Code:    res.Header.Get("X-ClickHouse-Exception-Code"),
			Message: strings.TrimSpace(string(raw)),
		}
	}
	return raw, nil
}
