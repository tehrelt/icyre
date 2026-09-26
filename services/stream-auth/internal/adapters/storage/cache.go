package storage

import (
	"context"
	"log/slog"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/libs/platform/redis"
)

// Variants is the uncached variant lookup.
type Variants interface {
	Available(ctx context.Context, trackID string) ([]media.Quality, error)
}

// CachedVariants remembers complete variant sets. The transcoder writes the
// variants one by one, so an incomplete set (mid-transcode) is not cached:
// the track gets every quality as soon as the transcoder finishes. A
// re-transcode replaces variants in place and never shrinks a complete set.
type CachedVariants struct {
	next  Variants
	cache *redis.Cache[[]media.Quality]
	log   *slog.Logger
}

// NewCachedVariants wraps next with cache.
func NewCachedVariants(next Variants, cache *redis.Cache[[]media.Quality], log *slog.Logger) *CachedVariants {
	return &CachedVariants{next: next, cache: cache, log: log}
}

// Available implements application.Variants.
func (c *CachedVariants) Available(ctx context.Context, trackID string) ([]media.Quality, error) {
	if v, ok, err := c.cache.Get(ctx, trackID); err == nil && ok {
		return v, nil
	} else if err != nil {
		c.log.WarnContext(ctx, "variant cache read failed, using storage", "error", err)
	}
	v, err := c.next.Available(ctx, trackID)
	if err != nil || len(v) < len(media.Qualities) {
		return v, err
	}
	if err := c.cache.Set(ctx, trackID, v); err != nil {
		c.log.WarnContext(ctx, "variant cache write failed", "error", err)
	}
	return v, nil
}
