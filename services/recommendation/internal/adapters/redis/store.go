// Package redis reads recommendation sets written by the worker.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/tehrelt/icyre/libs/contracts/recommendation"
	platformredis "github.com/tehrelt/icyre/libs/platform/redis"
)

// Store implements application.Store.
type Store struct{ rdb *platformredis.Client }

// New returns a Store.
func New(rdb *platformredis.Client) *Store { return &Store{rdb: rdb} }

// Get reads one set.
func (s *Store) Get(ctx context.Context, key string) (recommendation.Set, bool, error) {
	raw, err := s.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return recommendation.Set{}, false, nil
	}
	if err != nil {
		return recommendation.Set{}, false, fmt.Errorf("read recommendations: %w", err)
	}
	var set recommendation.Set
	if err := json.Unmarshal(raw, &set); err != nil {
		return recommendation.Set{}, false, fmt.Errorf("decode recommendations: %w", err)
	}
	return set, true, nil
}
