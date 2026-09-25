// Package opensearch is a thin OpenSearch client over net/http: JSON
// requests, bulk writes, and index/alias administration. Queries are built
// by the caller as plain JSON — the package holds no search logic.
package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config configures a Client.
type Config struct {
	// URL of the cluster, e.g. http://opensearch:9200.
	URL      string
	Username string
	Password string
	// Timeout bounds each request.
	Timeout time.Duration
}

// Client talks to one cluster.
type Client struct {
	base string
	cfg  Config
	hc   *http.Client
}

// New returns a Client. hc may carry tracing; nil uses a default client.
func New(cfg Config, hc *http.Client) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if hc == nil {
		hc = &http.Client{}
	}
	return &Client{base: strings.TrimRight(cfg.URL, "/"), cfg: cfg, hc: hc}
}

// Error is a non-2xx response.
type Error struct {
	Status int
	Type   string
	Reason string
}

func (e *Error) Error() string {
	return fmt.Sprintf("opensearch: %d %s: %s", e.Status, e.Type, e.Reason)
}

// IsNotFound reports a 404 (missing index, alias or document).
func IsNotFound(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == http.StatusNotFound
}

// Do sends body (JSON-encoded unless it is []byte) and decodes a 2xx
// response into out when out is not nil.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	var r io.Reader
	contentType := "application/json"
	switch b := body.(type) {
	case nil:
	case []byte:
		r = bytes.NewReader(b)
		contentType = "application/x-ndjson"
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			return fmt.Errorf("opensearch: encode: %w", err)
		}
		r = bytes.NewReader(raw)
	}
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, r)
	if err != nil {
		return err
	}
	if r != nil {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}
	res, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("opensearch: %s %s: %w", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("opensearch: read: %w", err)
	}
	if res.StatusCode >= 300 {
		return decodeError(res.StatusCode, raw)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("opensearch: decode %s: %w", path, err)
	}
	return nil
}

func decodeError(status int, raw []byte) error {
	var body struct {
		Error json.RawMessage `json:"error"`
	}
	e := &Error{Status: status}
	if json.Unmarshal(raw, &body) == nil && len(body.Error) > 0 {
		var detail struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		}
		if json.Unmarshal(body.Error, &detail) == nil && detail.Type != "" {
			e.Type, e.Reason = detail.Type, detail.Reason
		} else {
			e.Reason = strings.Trim(string(body.Error), `"`)
		}
	}
	if e.Reason == "" {
		e.Reason = http.StatusText(status)
	}
	return e
}

// Check is a readiness check: the cluster answers and is not red.
func (c *Client) Check(ctx context.Context) error {
	var h struct {
		Status string `json:"status"`
	}
	if err := c.Do(ctx, http.MethodGet, "/_cluster/health", nil, &h); err != nil {
		return err
	}
	if h.Status == "red" {
		return errors.New("opensearch cluster is red")
	}
	return nil
}

// --- bulk ---------------------------------------------------------------

// Bulk actions.
const (
	ActionIndex  = "index"
	ActionDelete = "delete"
)

// BulkOp is one document write. With Version > 0 the write uses external
// versioning (version_type=external_gte): an older version never
// overwrites a newer one, which makes replays and reordering harmless.
type BulkOp struct {
	Action  string
	Index   string
	ID      string
	Version int64
	Doc     any
}

// BulkItem is the outcome of one op.
type BulkItem struct {
	ID     string
	Status int
	Error  *Error
}

// Conflict reports a rejected stale write (version conflict).
func (i BulkItem) Conflict() bool { return i.Status == http.StatusConflict }

// Bulk sends ops in one request and returns per-op results in order.
func (c *Client) Bulk(ctx context.Context, ops []BulkOp, refresh bool) ([]BulkItem, error) {
	if len(ops) == 0 {
		return nil, nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, op := range ops {
		meta := map[string]any{"_index": op.Index, "_id": op.ID}
		if op.Version > 0 {
			meta["version"], meta["version_type"] = op.Version, "external_gte"
		}
		if err := enc.Encode(map[string]any{op.Action: meta}); err != nil {
			return nil, err
		}
		if op.Action == ActionIndex {
			if err := enc.Encode(op.Doc); err != nil {
				return nil, err
			}
		}
	}
	path := "/_bulk"
	if refresh {
		path += "?refresh=wait_for"
	}
	var res struct {
		Items []map[string]struct {
			ID     string `json:"_id"`
			Status int    `json:"status"`
			Error  *struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
			} `json:"error"`
		} `json:"items"`
	}
	if err := c.Do(ctx, http.MethodPost, path, buf.Bytes(), &res); err != nil {
		return nil, err
	}
	out := make([]BulkItem, 0, len(res.Items))
	for _, item := range res.Items {
		for _, r := range item {
			bi := BulkItem{ID: r.ID, Status: r.Status}
			if r.Error != nil {
				bi.Error = &Error{Status: r.Status, Type: r.Error.Type, Reason: r.Error.Reason}
			}
			out = append(out, bi)
		}
	}
	return out, nil
}

// --- indices and aliases -------------------------------------------------

// PutIndexTemplate creates or replaces a composable index template.
func (c *Client) PutIndexTemplate(ctx context.Context, name string, body any) error {
	return c.Do(ctx, http.MethodPut, "/_index_template/"+url.PathEscape(name), body, nil)
}

// CreateIndex creates an index (settings/mappings come from templates when body is nil).
func (c *Client) CreateIndex(ctx context.Context, name string, body any) error {
	return c.Do(ctx, http.MethodPut, "/"+url.PathEscape(name), body, nil)
}

// DeleteIndex removes an index.
func (c *Client) DeleteIndex(ctx context.Context, name string) error {
	return c.Do(ctx, http.MethodDelete, "/"+url.PathEscape(name), nil, nil)
}

// AliasIndices returns the indices an alias points to (empty when missing).
func (c *Client) AliasIndices(ctx context.Context, alias string) ([]string, error) {
	var res map[string]json.RawMessage
	err := c.Do(ctx, http.MethodGet, "/_alias/"+url.PathEscape(alias), nil, &res)
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(res))
	for index := range res {
		out = append(out, index)
	}
	return out, nil
}

// SwapAlias points alias at index, removing it from every other index, in
// one atomic request — readers never see an empty or doubled alias.
func (c *Client) SwapAlias(ctx context.Context, alias, index string, from []string) error {
	actions := []map[string]any{}
	for _, old := range from {
		if old != index {
			actions = append(actions, map[string]any{"remove": map[string]any{"index": old, "alias": alias}})
		}
	}
	actions = append(actions, map[string]any{"add": map[string]any{"index": index, "alias": alias, "is_write_index": true}})
	return c.Do(ctx, http.MethodPost, "/_aliases", map[string]any{"actions": actions}, nil)
}

// Refresh makes recent writes searchable (tests, end of a reindex).
func (c *Client) Refresh(ctx context.Context, index string) error {
	return c.Do(ctx, http.MethodPost, "/"+url.PathEscape(index)+"/_refresh", nil, nil)
}
