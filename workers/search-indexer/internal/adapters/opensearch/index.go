// Package opensearch applies index writes through the bulk API.
package opensearch

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tehrelt/icyre/libs/platform/opensearch"
	"github.com/tehrelt/icyre/workers/search-indexer/internal/application"
)

// Bulker is the part of the client used here.
type Bulker interface {
	Bulk(ctx context.Context, ops []opensearch.BulkOp, refresh bool) ([]opensearch.BulkItem, error)
}

// Index implements application.Index.
type Index struct {
	client  Bulker
	refresh bool
}

// New returns an Index. refresh=true makes writes searchable before
// returning (tests); production relies on the refresh interval.
func New(c Bulker, refresh bool) *Index { return &Index{client: c, refresh: refresh} }

// Apply sends writes in one bulk request. A version conflict means a newer
// version is already indexed (replayed or reordered event) and is not an
// error; neither is deleting a missing document.
func (x *Index) Apply(ctx context.Context, writes []application.Write) error {
	ops := make([]opensearch.BulkOp, len(writes))
	for i, w := range writes {
		ops[i] = opensearch.BulkOp{Action: opensearch.ActionIndex, Index: w.Index, ID: w.ID, Version: w.Version, Doc: w.Doc}
		if w.Delete {
			ops[i].Action = opensearch.ActionDelete
		}
	}
	items, err := x.client.Bulk(ctx, ops, x.refresh)
	if err != nil {
		return err
	}
	for i, item := range items {
		switch {
		case item.Status < 300, item.Conflict():
		case item.Status == http.StatusNotFound && writes[i].Delete:
		default:
			return fmt.Errorf("index %s/%s: %w", writes[i].Index, item.ID, item.Error)
		}
	}
	return nil
}
