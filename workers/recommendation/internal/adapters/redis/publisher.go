// Package redis publishes ready recommendation sets
// (libs/contracts/recommendation) for the Recommendation Service.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/recommendation"
	platformredis "github.com/tehrelt/icyre/libs/platform/redis"
)

// pipelineSize bounds the commands sent in one round trip.
const pipelineSize = 500

// Publisher implements application.Publisher.
type Publisher struct {
	rdb *platformredis.Client
	ttl time.Duration
}

// New returns a Publisher. Sets expire after ttl: a user who stops
// generating signals falls back to the popular set.
func New(rdb *platformredis.Client, ttl time.Duration) *Publisher {
	return &Publisher{rdb: rdb, ttl: ttl}
}

// Publish writes sets by key, pipelined.
func (p *Publisher) Publish(ctx context.Context, sets map[string]recommendation.Set) error {
	pipe := p.rdb.Pipeline()
	n := 0
	flush := func() error {
		if n == 0 {
			return nil
		}
		_, err := pipe.Exec(ctx)
		n = 0
		if err != nil {
			return fmt.Errorf("publish recommendations: %w", err)
		}
		return nil
	}
	for key, set := range sets {
		raw, err := json.Marshal(set)
		if err != nil {
			return fmt.Errorf("encode %s: %w", key, err)
		}
		pipe.Set(ctx, key, raw, p.ttl)
		if n++; n == pipelineSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}
