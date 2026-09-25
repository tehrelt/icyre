package indices

import (
	"context"
	"slices"
	"testing"

	"github.com/tehrelt/icyre/libs/contracts/search"
)

type fakeAdmin struct {
	templates map[string]any
	indices   map[string]bool
	aliases   map[string][]string
}

func newFake() *fakeAdmin {
	return &fakeAdmin{templates: map[string]any{}, indices: map[string]bool{}, aliases: map[string][]string{}}
}

func (f *fakeAdmin) PutIndexTemplate(_ context.Context, name string, body any) error {
	f.templates[name] = body
	return nil
}
func (f *fakeAdmin) CreateIndex(_ context.Context, name string, _ any) error {
	f.indices[name] = true
	return nil
}
func (f *fakeAdmin) DeleteIndex(_ context.Context, name string) error {
	delete(f.indices, name)
	return nil
}
func (f *fakeAdmin) AliasIndices(_ context.Context, alias string) ([]string, error) {
	return f.aliases[alias], nil
}
func (f *fakeAdmin) SwapAlias(_ context.Context, alias, index string, _ []string) error {
	f.aliases[alias] = []string{index}
	return nil
}

func TestEnsureIsIdempotent(t *testing.T) {
	a := newFake()
	ctx := context.Background()
	for range 2 {
		if err := Ensure(ctx, a, Options{}); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.templates) != 4 || len(a.indices) != 4 {
		t.Fatalf("templates=%d indices=%d", len(a.templates), len(a.indices))
	}
	if got := a.aliases[search.AliasTracks]; !slices.Equal(got, []string{"tracks-v1"}) {
		t.Fatal(got)
	}
}

func TestGenerationPromote(t *testing.T) {
	a := newFake()
	ctx := context.Background()
	_ = Ensure(ctx, a, Options{})
	g, err := NewGeneration(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if g.Index[search.AliasAlbums] != "albums-v2" || !a.indices["albums-v1"] {
		t.Fatalf("generation %+v", g.Index)
	}
	if err := g.Promote(ctx, a); err != nil {
		t.Fatal(err)
	}
	if a.indices["albums-v1"] || !a.indices["albums-v2"] || a.aliases[search.AliasAlbums][0] != "albums-v2" {
		t.Fatalf("after promote: %v %v", a.indices, a.aliases)
	}

	g3, _ := NewGeneration(ctx, a)
	g3.Abandon(ctx, a)
	if a.indices["albums-v3"] || a.aliases[search.AliasAlbums][0] != "albums-v2" {
		t.Fatal("abandon touched live indices")
	}
}

func TestTemplateCoversContract(t *testing.T) {
	tpl := Template(search.AliasTracks, Options{Replicas: 1})
	props := tpl["template"].(map[string]any)["mappings"].(map[string]any)["properties"].(map[string]any)
	for _, f := range []string{"id", "title", "artistNames", "albumTitle", "available", "popularity", "releaseDate", "suggest"} {
		if _, ok := props[f]; !ok {
			t.Errorf("tracks mapping lacks %s", f)
		}
	}
	if version("tracks", "tracks-v12") != 12 || version("tracks", "albums-v1") != 0 {
		t.Fatal("version parsing")
	}
}
