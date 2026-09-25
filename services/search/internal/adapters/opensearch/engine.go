// Package opensearch runs search queries: relevance, exact-prefix boosts,
// popularity and recency, in one multi-search round trip.
package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/tehrelt/icyre/libs/contracts/search"
	"github.com/tehrelt/icyre/libs/platform/opensearch"
	"github.com/tehrelt/icyre/services/search/internal/application"
)

// Client is the part of the OpenSearch client used here.
type Client interface {
	Do(ctx context.Context, method, path string, body, out any) error
}

// Engine implements application.Engine.
type Engine struct {
	c      Client
	prefix string
}

// New returns an Engine over the standard aliases.
func New(c Client) *Engine { return &Engine{c: c} }

// NewWithPrefix reads <prefix><alias> instead (isolated test indices).
func NewWithPrefix(c Client, prefix string) *Engine { return &Engine{c: c, prefix: prefix} }

// schema describes how one index is searched.
type schema struct {
	alias     string
	primary   string   // title or name
	secondary []string // artist/album names
	recency   string   // date field for the recency decay
}

var (
	tracks    = schema{search.AliasTracks, "title", []string{"artistNames", "albumTitle"}, "releaseDate"}
	artists   = schema{search.AliasArtists, "name", nil, "updatedAt"}
	albums    = schema{search.AliasAlbums, "title", []string{"artistNames"}, "releaseDate"}
	playlists = schema{search.AliasPlaylists, "title", []string{"ownerName"}, "updatedAt"}
)

// query ranks by text relevance (exact > prefix > words > fuzzy), then
// multiplies by popularity and recency:
//
//	score = relevance × (log10(2 + popularity) + 0.25 × recency)
//
// recency decays from 1 to 0.5 over a year after a 30-day grace period.
func (s schema) query(q string) map[string]any {
	fields := []string{s.primary + "^3"}
	auto := []string{s.primary + ".autocomplete^2"}
	for _, f := range s.secondary {
		fields = append(fields, f)
		auto = append(auto, f+".autocomplete")
	}
	lower := strings.ToLower(q)
	text := map[string]any{"bool": map[string]any{
		"should": []any{
			// Whole words, typos forgiven (the first letter must be right).
			map[string]any{"multi_match": map[string]any{"query": q, "fields": fields, "type": "cross_fields", "operator": "and"}},
			map[string]any{"multi_match": map[string]any{"query": q, "fields": fields, "fuzziness": "AUTO", "prefix_length": 1, "operator": "and"}},
			// Search as you type: "pri" finds "Prism Hours".
			map[string]any{"multi_match": map[string]any{"query": q, "fields": auto, "type": "cross_fields", "operator": "and", "boost": 0.5}},
		},
		"minimum_should_match": 1,
	}}
	boosts := []any{
		map[string]any{"term": map[string]any{s.primary + ".keyword": map[string]any{"value": lower, "boost": 10}}},
		map[string]any{"prefix": map[string]any{s.primary + ".keyword": map[string]any{"value": lower, "boost": 4}}},
		map[string]any{"match_phrase": map[string]any{s.primary: map[string]any{"query": q, "boost": 2}}},
	}
	return map[string]any{"function_score": map[string]any{
		"query": map[string]any{"bool": map[string]any{"must": text, "should": boosts}},
		"functions": []any{
			map[string]any{"field_value_factor": map[string]any{"field": "popularity", "modifier": "log2p", "missing": 0}},
			map[string]any{"gauss": map[string]any{s.recency: map[string]any{"origin": "now", "offset": "30d", "scale": "365d", "decay": 0.5}}, "weight": 0.25},
		},
		"score_mode": "sum",
		"boost_mode": "multiply",
	}}
}

type hits struct {
	Total struct {
		Value int `json:"value"`
	} `json:"total"`
	Hits []struct {
		ID     string          `json:"_id"`
		Score  float64         `json:"_score"`
		Source json.RawMessage `json:"_source"`
	} `json:"hits"`
}

type msearchResponse struct {
	Responses []struct {
		Hits  hits `json:"hits"`
		Error *struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"error"`
	} `json:"responses"`
}

// msearch sends one request per schema and returns the hits in order.
func (e *Engine) msearch(ctx context.Context, bodies []map[string]any, targets []schema) ([]hits, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for i, s := range targets {
		if err := enc.Encode(map[string]any{"index": e.prefix + s.alias}); err != nil {
			return nil, err
		}
		if err := enc.Encode(bodies[i]); err != nil {
			return nil, err
		}
	}
	var res msearchResponse
	if err := e.c.Do(ctx, http.MethodPost, "/_msearch", buf.Bytes(), &res); err != nil {
		return nil, err
	}
	if len(res.Responses) != len(targets) {
		return nil, fmt.Errorf("msearch: %d responses for %d queries", len(res.Responses), len(targets))
	}
	out := make([]hits, len(targets))
	for i, r := range res.Responses {
		if r.Error != nil {
			return nil, &opensearch.Error{Status: http.StatusInternalServerError, Type: r.Error.Type, Reason: r.Error.Reason}
		}
		out[i] = r.Hits
	}
	return out, nil
}

func decode[T any](h hits) ([]application.Hit[T], error) {
	out := make([]application.Hit[T], 0, len(h.Hits))
	for _, x := range h.Hits {
		var doc T
		if err := json.Unmarshal(x.Source, &doc); err != nil {
			return nil, fmt.Errorf("decode %s: %w", x.ID, err)
		}
		out = append(out, application.Hit[T]{Doc: doc, Score: x.Score})
	}
	return out, nil
}

// Search implements application.Engine.
func (e *Engine) Search(ctx context.Context, q string, sizes application.Sizes) (application.Page, error) {
	targets := []schema{tracks, artists, albums, playlists}
	counts := []int{sizes.Tracks, sizes.Artists, sizes.Albums, sizes.Playlists}
	bodies := make([]map[string]any, len(targets))
	for i, s := range targets {
		bodies[i] = map[string]any{"query": s.query(q), "size": counts[i], "track_total_hits": true}
	}
	res, err := e.msearch(ctx, bodies, targets)
	if err != nil {
		return application.Page{}, err
	}
	var p application.Page
	if p.Tracks, err = decode[search.Track](res[0]); err != nil {
		return p, err
	}
	if p.Artists, err = decode[search.Artist](res[1]); err != nil {
		return p, err
	}
	if p.Albums, err = decode[search.Album](res[2]); err != nil {
		return p, err
	}
	if p.Playlists, err = decode[search.Playlist](res[3]); err != nil {
		return p, err
	}
	p.Counts = application.Counts{Tracks: res[0].Total.Value, Artists: res[1].Total.Value, Albums: res[2].Total.Value, Playlists: res[3].Total.Value}
	return p, nil
}

// Suggest implements application.Engine: prefix matches on names and
// titles, merged across entity types by score.
func (e *Engine) Suggest(ctx context.Context, q string, limit int) ([]application.Suggestion, error) {
	targets := []schema{artists, albums, tracks, playlists}
	lower := strings.ToLower(q)
	bodies := make([]map[string]any, len(targets))
	for i, s := range targets {
		bodies[i] = map[string]any{
			"size": limit,
			"query": map[string]any{"function_score": map[string]any{
				"query": map[string]any{"bool": map[string]any{
					"must":   map[string]any{"match": map[string]any{s.primary + ".autocomplete": map[string]any{"query": q, "operator": "and"}}},
					"should": map[string]any{"prefix": map[string]any{s.primary + ".keyword": map[string]any{"value": lower, "boost": 3}}},
				}},
				"field_value_factor": map[string]any{"field": "popularity", "modifier": "log2p", "missing": 0},
				"boost_mode":         "multiply",
			}},
		}
	}
	res, err := e.msearch(ctx, bodies, targets)
	if err != nil {
		return nil, err
	}
	var out []application.Suggestion
	artistHits, err := decode[search.Artist](res[0])
	if err != nil {
		return nil, err
	}
	for _, h := range artistHits {
		out = append(out, application.Suggestion{Kind: "artist", ID: h.Doc.ID, Text: h.Doc.Name, Subtitle: "Artist", Score: h.Score})
	}
	albumHits, err := decode[search.Album](res[1])
	if err != nil {
		return nil, err
	}
	for _, h := range albumHits {
		out = append(out, application.Suggestion{Kind: "album", ID: h.Doc.ID, Text: h.Doc.Title, Subtitle: strings.Join(h.Doc.ArtistNames, ", "), Score: h.Score})
	}
	trackHits, err := decode[search.Track](res[2])
	if err != nil {
		return nil, err
	}
	for _, h := range trackHits {
		out = append(out, application.Suggestion{Kind: "track", ID: h.Doc.ID, Text: h.Doc.Title, Subtitle: strings.Join(h.Doc.ArtistNames, ", "), Score: h.Score})
	}
	playlistHits, err := decode[search.Playlist](res[3])
	if err != nil {
		return nil, err
	}
	for _, h := range playlistHits {
		out = append(out, application.Suggestion{Kind: "playlist", ID: h.Doc.ID, Text: h.Doc.Title, Subtitle: h.Doc.OwnerName, Score: h.Score})
	}
	slices.SortStableFunc(out, func(a, b application.Suggestion) int {
		switch {
		case a.Score > b.Score:
			return -1
		case a.Score < b.Score:
			return 1
		}
		return 0
	})
	return out[:min(limit, len(out))], nil
}

// Correct implements application.Engine with the term suggester over the
// "suggest" field every index copies its names and titles into.
func (e *Engine) Correct(ctx context.Context, q string) (string, error) {
	var res struct {
		Suggest map[string][]struct {
			Text    string `json:"text"`
			Offset  int    `json:"offset"`
			Length  int    `json:"length"`
			Options []struct {
				Text  string  `json:"text"`
				Score float64 `json:"score"`
				Freq  int     `json:"freq"`
			} `json:"options"`
		} `json:"suggest"`
	}
	body := map[string]any{
		"size": 0,
		"suggest": map[string]any{
			"text": q,
			"fix":  map[string]any{"term": map[string]any{"field": "suggest", "suggest_mode": "missing", "min_word_length": 3, "sort": "score"}},
		},
	}
	names := make([]string, len(search.Aliases))
	for i, a := range search.Aliases {
		names[i] = e.prefix + a
	}
	path := "/" + strings.Join(names, ",") + "/_search"
	if err := e.c.Do(ctx, http.MethodPost, path, body, &res); err != nil {
		return "", err
	}
	runes := []rune(q)
	var b strings.Builder
	pos, changed := 0, false
	for _, entry := range res.Suggest["fix"] {
		if entry.Offset < pos || entry.Offset+entry.Length > len(runes) {
			continue
		}
		b.WriteString(string(runes[pos:entry.Offset]))
		word := string(runes[entry.Offset : entry.Offset+entry.Length])
		if len(entry.Options) > 0 {
			word, changed = entry.Options[0].Text, true
		}
		b.WriteString(word)
		pos = entry.Offset + entry.Length
	}
	if !changed {
		return "", nil
	}
	b.WriteString(string(runes[pos:]))
	return b.String(), nil
}
