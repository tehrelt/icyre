package search

// analysis is shared by every index:
//   - icyre_text: words, lowercased, accents folded ("Café" matches "cafe");
//   - icyre_autocomplete: the same plus edge n-grams, used at index time
//     only, so "pri" finds "Prism Hours";
//   - icyre_keyword: normalizer for exact and prefix matches on whole titles.
var analysis = map[string]any{
	"normalizer": map[string]any{
		"icyre_keyword": map[string]any{"type": "custom", "filter": []string{"lowercase", "asciifolding"}},
	},
	"filter": map[string]any{
		"icyre_edge": map[string]any{"type": "edge_ngram", "min_gram": 1, "max_gram": 20},
	},
	"analyzer": map[string]any{
		"icyre_text":         map[string]any{"tokenizer": "standard", "filter": []string{"lowercase", "asciifolding"}},
		"icyre_autocomplete": map[string]any{"tokenizer": "standard", "filter": []string{"lowercase", "asciifolding", "icyre_edge"}},
	},
}

// text is a searchable display field: analyzed text, a normalized keyword
// (exact/prefix boosts) and an autocomplete sub-field. Everything is also
// copied into "suggest", the source for "did you mean".
func text() map[string]any {
	return map[string]any{
		"type": "text", "analyzer": "icyre_text", "copy_to": "suggest",
		"fields": map[string]any{
			"keyword":      map[string]any{"type": "keyword", "normalizer": "icyre_keyword", "ignore_above": 256},
			"autocomplete": map[string]any{"type": "text", "analyzer": "icyre_autocomplete", "search_analyzer": "icyre_text"},
		},
	}
}

var (
	keyword  = map[string]any{"type": "keyword"}
	boolean  = map[string]any{"type": "boolean"}
	long     = map[string]any{"type": "long"}
	integer  = map[string]any{"type": "integer"}
	float    = map[string]any{"type": "float"}
	date     = map[string]any{"type": "date"}
	day      = map[string]any{"type": "date", "format": "yyyy-MM-dd"}
	suggestF = map[string]any{"type": "text", "analyzer": "icyre_text"}
)

// mappings per alias. dynamic=strict: a field the contract does not know is
// a bug, not something to index silently.
var mappings = map[string]map[string]any{
	AliasTracks: {
		"id": keyword, "title": text(), "artistIds": keyword, "artistNames": text(),
		"albumId": keyword, "albumTitle": text(), "releaseDate": day, "durationMs": long,
		"explicit": boolean, "available": boolean, "popularity": float, "updatedAt": date, "suggest": suggestF,
	},
	AliasAlbums: {
		"id": keyword, "title": text(), "albumType": keyword, "artistIds": keyword, "artistNames": text(),
		"releaseDate": day, "popularity": float, "updatedAt": date, "suggest": suggestF,
	},
	AliasArtists: {
		"id": keyword, "name": text(), "verified": boolean, "popularity": float, "updatedAt": date, "suggest": suggestF,
	},
	AliasPlaylists: {
		"id": keyword, "title": text(), "ownerName": text(), "trackCount": integer, "popularity": float, "updatedAt": date, "suggest": suggestF,
	},
}

// Template returns the composable index template for alias (pattern
// <alias>-v*): analyzers and a strict mapping. Readers rely on the
// sub-fields it defines (<field>.keyword, <field>.autocomplete, suggest).
func Template(alias string, shards, replicas int) map[string]any {
	return map[string]any{
		"index_patterns": []string{alias + "-v*"},
		"template": map[string]any{
			"settings": map[string]any{
				"number_of_shards": shards, "number_of_replicas": replicas,
				"analysis": analysis,
			},
			"mappings": map[string]any{"dynamic": "strict", "properties": mappings[alias]},
		},
	}
}
