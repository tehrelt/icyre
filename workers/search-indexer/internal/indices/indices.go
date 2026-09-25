// Package indices defines the search indices (analyzers, mappings) and
// their lifecycle: versioned physical indices behind stable aliases.
package indices

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/tehrelt/icyre/libs/contracts/search"
)

// Admin is the index administration the lifecycle needs.
type Admin interface {
	PutIndexTemplate(ctx context.Context, name string, body any) error
	CreateIndex(ctx context.Context, name string, body any) error
	DeleteIndex(ctx context.Context, name string) error
	AliasIndices(ctx context.Context, alias string) ([]string, error)
	SwapAlias(ctx context.Context, alias, index string, from []string) error
}

// Options tune index settings per environment.
type Options struct {
	Shards   int
	Replicas int
}

func (o Options) withDefaults() Options {
	if o.Shards <= 0 {
		o.Shards = 1
	}
	return o
}

// Template returns the index template for alias (see search.Template).
func Template(alias string, o Options) map[string]any {
	o = o.withDefaults()
	return search.Template(alias, o.Shards, o.Replicas)
}

// Versioned is <alias>-v<n>.
func Versioned(alias string, n int) string { return alias + "-v" + strconv.Itoa(n) }

// version parses the n of <alias>-v<n>; 0 when it is not ours.
func version(alias, index string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(index, alias+"-v"))
	if err != nil || !strings.HasPrefix(index, alias+"-v") {
		return 0
	}
	return n
}

// Ensure installs templates and creates <alias>-v1 behind each alias that
// does not exist yet. Idempotent: run on every deploy (`search-indexer migrate`).
func Ensure(ctx context.Context, a Admin, o Options) error {
	for _, alias := range search.Aliases {
		if err := a.PutIndexTemplate(ctx, "icyre-"+alias, Template(alias, o)); err != nil {
			return fmt.Errorf("template %s: %w", alias, err)
		}
		current, err := a.AliasIndices(ctx, alias)
		if err != nil {
			return err
		}
		if len(current) > 0 {
			continue
		}
		index := Versioned(alias, 1)
		if err := a.CreateIndex(ctx, index, nil); err != nil {
			return fmt.Errorf("create %s: %w", index, err)
		}
		if err := a.SwapAlias(ctx, alias, index, nil); err != nil {
			return err
		}
	}
	return nil
}

// Generation is a set of fresh physical indices being filled by a rebuild.
type Generation struct {
	// Index maps alias → new physical index.
	Index    map[string]string
	previous map[string][]string
}

// NewGeneration creates <alias>-v<max+1> for every alias. Readers keep using
// the old indices until Promote.
func NewGeneration(ctx context.Context, a Admin) (*Generation, error) {
	g := &Generation{Index: map[string]string{}, previous: map[string][]string{}}
	for _, alias := range search.Aliases {
		current, err := a.AliasIndices(ctx, alias)
		if err != nil {
			return nil, err
		}
		next := 1
		for _, idx := range current {
			next = max(next, version(alias, idx)+1)
		}
		index := Versioned(alias, next)
		if err := a.CreateIndex(ctx, index, nil); err != nil {
			return nil, fmt.Errorf("create %s: %w", index, err)
		}
		g.Index[alias], g.previous[alias] = index, current
	}
	return g, nil
}

// Promote atomically moves every alias to the new indices and drops the
// old ones.
func (g *Generation) Promote(ctx context.Context, a Admin) error {
	aliases := make([]string, 0, len(g.Index))
	for alias := range g.Index {
		aliases = append(aliases, alias)
	}
	slices.Sort(aliases)
	for _, alias := range aliases {
		if err := a.SwapAlias(ctx, alias, g.Index[alias], g.previous[alias]); err != nil {
			return fmt.Errorf("swap %s: %w", alias, err)
		}
	}
	for _, alias := range aliases {
		for _, old := range g.previous[alias] {
			if old != g.Index[alias] {
				if err := a.DeleteIndex(ctx, old); err != nil {
					return fmt.Errorf("drop %s: %w", old, err)
				}
			}
		}
	}
	return nil
}

// Abandon drops the new indices after a failed rebuild; aliases are untouched.
func (g *Generation) Abandon(ctx context.Context, a Admin) {
	for _, index := range g.Index {
		_ = a.DeleteIndex(ctx, index)
	}
}
