package domain

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/recommendation"
)

// Algorithm names the scoring in the sets it produces.
const Algorithm = "mvp-content-v1"

// Weights of the score components; they sum to 1.
const (
	WeightStyle      = 0.35
	WeightArtist     = 0.30
	WeightPopularity = 0.20
	WeightFreshness  = 0.15
)

// Taste signals: how much one action says about liking a track.
const (
	signalCompletion = 1.0
	signalLike       = 3.0
	signalAlbumLike  = 2.0
	signalFollow     = 3.0
	signalSkip       = -1.0
	// signalOpen is a play that neither finished nor was skipped (yet).
	signalOpen = 0.25
)

const (
	// soundShare of style when both the user and the track have features.
	soundShare = 0.3
	// freshnessHalfLife: a release loses half its freshness every 90 days.
	freshnessHalfLife = 90 * 24 * time.Hour
	// familiarPenalty scales tracks the user has already listened to the end.
	familiarPenalty = 0.5
	// minReason is the smallest weighted component named as a reason.
	minReason = 0.05
)

// Scored is a recommended track or artist.
type Scored struct {
	ID      uuid.UUID
	Score   float64
	Reasons []string
}

// Catalog is the candidate pool of one build, indexed for scoring.
type Catalog struct {
	tracks   []Track
	byID     map[uuid.UUID]*Track
	byAlbum  map[uuid.UUID][]*Track
	maxPlays uint64
	now      time.Time
}

// NewCatalog indexes tracks; now anchors freshness.
func NewCatalog(tracks []Track, now time.Time) *Catalog {
	c := &Catalog{tracks: tracks, byID: make(map[uuid.UUID]*Track, len(tracks)), byAlbum: map[uuid.UUID][]*Track{}, now: now}
	for i := range tracks {
		t := &c.tracks[i]
		c.byID[t.ID] = t
		c.byAlbum[t.AlbumID] = append(c.byAlbum[t.AlbumID], t)
		c.maxPlays = max(c.maxPlays, t.Plays)
	}
	return c
}

// Len is the number of candidate tracks.
func (c *Catalog) Len() int { return len(c.tracks) }

// Profile is one user's taste distilled from their signals.
type Profile struct {
	artists   map[uuid.UUID]float64
	genres    map[uuid.UUID]float64
	maxArtist float64
	genreNorm float64
	sound     *Sound
	exclude   map[uuid.UUID]bool
	familiar  map[uuid.UUID]bool
}

// Profile builds the user's taste. Tracks unknown to the catalog (deleted,
// not ready) contribute nothing.
func (c *Catalog) Profile(s Signals) Profile {
	p := Profile{artists: map[uuid.UUID]float64{}, genres: map[uuid.UUID]float64{}, exclude: map[uuid.UUID]bool{}, familiar: map[uuid.UUID]bool{}}
	sounds := map[string]*soundSum{}

	add := func(t *Track, w float64) {
		for _, a := range t.ArtistIDs {
			p.artists[a] += w
		}
		for _, g := range t.GenreIDs {
			p.genres[g] += w
		}
		if w > 0 && t.Sound != nil {
			s := sounds[t.Sound.Version]
			if s == nil {
				s = &soundSum{}
				sounds[t.Sound.Version] = s
			}
			s.add(t.Sound.Vector, w)
		}
	}

	seen := map[uuid.UUID]bool{}
	for id, in := range s.History {
		seen[id] = true
		w := float64(in.Completions)*signalCompletion + float64(in.Skips)*signalSkip +
			float64(in.Plays-min(in.Plays, in.Completions+in.Skips))*signalOpen
		if s.LikedTracks[id] {
			w += signalLike
			p.exclude[id] = true
		}
		if in.Skips > in.Completions && !s.LikedTracks[id] {
			p.exclude[id] = true
		}
		if in.Completions > 0 {
			p.familiar[id] = true
		}
		if t, ok := c.byID[id]; ok {
			add(t, w)
		}
	}
	for id := range s.LikedTracks {
		p.exclude[id] = true
		if t, ok := c.byID[id]; ok && !seen[id] {
			add(t, signalLike)
		}
	}
	for album := range s.LikedAlbums {
		tracks := c.byAlbum[album]
		for _, t := range tracks {
			p.exclude[t.ID] = true
		}
		if len(tracks) > 0 {
			// Once per album: its artists and genres, as its first track has them.
			a := *tracks[0]
			a.Sound = nil
			add(&a, signalAlbumLike)
		}
	}
	for a := range s.FollowedArtists {
		p.artists[a] += signalFollow
	}

	for _, v := range p.artists {
		p.maxArtist = max(p.maxArtist, v)
	}
	for _, v := range p.genres {
		if v > 0 {
			p.genreNorm += v * v
		}
	}
	p.genreNorm = math.Sqrt(p.genreNorm)
	var best *soundSum
	var bestVersion string
	for v, s := range sounds {
		if best == nil || s.weight > best.weight {
			best, bestVersion = s, v
		}
	}
	if best != nil {
		p.sound = &Sound{Version: bestVersion, Vector: best.mean()}
	}
	return p
}

type soundSum struct {
	sum    [4]float64
	weight float64
}

func (s *soundSum) add(v [4]float64, w float64) {
	for i := range v {
		s.sum[i] += v[i] * w
	}
	s.weight += w
}

func (s *soundSum) mean() [4]float64 {
	var m [4]float64
	for i := range s.sum {
		m[i] = s.sum[i] / s.weight
	}
	return m
}

// components of a track's score before weighting.
type components struct {
	style, artist, popularity, freshness float64
}

func (k components) score() float64 {
	return k.style*WeightStyle + k.artist*WeightArtist + k.popularity*WeightPopularity + k.freshness*WeightFreshness
}

func (k components) reasons() []string {
	parts := []struct {
		name string
		v    float64
	}{
		{recommendation.ReasonStyle, k.style * WeightStyle},
		{recommendation.ReasonArtist, k.artist * WeightArtist},
		{recommendation.ReasonPopular, k.popularity * WeightPopularity},
		{recommendation.ReasonFresh, k.freshness * WeightFreshness},
	}
	slices.SortStableFunc(parts, func(a, b struct {
		name string
		v    float64
	}) int {
		return cmp.Compare(b.v, a.v)
	})
	var out []string
	for _, p := range parts[:2] {
		if p.v >= minReason {
			out = append(out, p.name)
		}
	}
	return out
}

func (c *Catalog) components(p Profile, t *Track) components {
	var k components
	if p.maxArtist > 0 {
		var aff float64
		for _, a := range t.ArtistIDs {
			aff += max(0, p.artists[a])
		}
		k.artist = clamp01(aff / p.maxArtist)
	}
	var genre float64
	if p.genreNorm > 0 && len(t.GenreIDs) > 0 {
		var dot float64
		for _, g := range t.GenreIDs {
			dot += max(0, p.genres[g])
		}
		genre = clamp01(dot / (p.genreNorm * math.Sqrt(float64(len(t.GenreIDs)))))
	}
	k.style = genre
	if p.sound != nil && t.Sound != nil && p.sound.Version == t.Sound.Version {
		k.style = (1-soundShare)*genre + soundShare*soundSimilarity(p.sound.Vector, t.Sound.Vector)
	}
	k.popularity = c.popularity(t)
	k.freshness = c.freshness(t)
	return k
}

// soundSimilarity is 1 for identical vectors and 0 for opposite corners of
// the unit hypercube (distance 2 in four dimensions).
func soundSimilarity(a, b [4]float64) float64 {
	var d float64
	for i := range a {
		d += (a[i] - b[i]) * (a[i] - b[i])
	}
	return clamp01(1 - math.Sqrt(d)/2)
}

func (c *Catalog) popularity(t *Track) float64 {
	if c.maxPlays == 0 {
		return 0
	}
	return math.Log1p(float64(t.Plays)) / math.Log1p(float64(c.maxPlays))
}

func (c *Catalog) freshness(t *Track) float64 {
	if t.Released.IsZero() {
		return 0
	}
	age := max(c.now.Sub(t.Released), 0)
	return math.Pow(0.5, float64(age)/float64(freshnessHalfLife))
}

// Recommend ranks the catalog for one user: at most nTracks tracks and
// nArtists artists (an artist scores as its best track).
func (c *Catalog) Recommend(s Signals, nTracks, nArtists int) (tracks, artists []Scored) {
	p := c.Profile(s)
	scored := make([]Scored, 0, len(c.tracks))
	best := map[uuid.UUID]Scored{}
	for i := range c.tracks {
		t := &c.tracks[i]
		if p.exclude[t.ID] {
			continue
		}
		k := c.components(p, t)
		score := k.score()
		if p.familiar[t.ID] {
			score *= familiarPenalty
		}
		sc := Scored{ID: t.ID, Score: round(score), Reasons: k.reasons()}
		scored = append(scored, sc)
		for _, a := range t.ArtistIDs {
			if b, ok := best[a]; !ok || sc.Score > b.Score {
				best[a] = Scored{ID: a, Score: sc.Score, Reasons: sc.Reasons}
			}
		}
	}
	return top(scored, nTracks), top(mapValues(best), nArtists)
}

// Popular is the fallback for users without signals: tracks by popularity
// and freshness, artists by their tracks' plays.
func (c *Catalog) Popular(nTracks, nArtists int) (tracks, artists []Scored) {
	plays := map[uuid.UUID]uint64{}
	for i := range c.tracks {
		t := &c.tracks[i]
		k := components{popularity: c.popularity(t), freshness: c.freshness(t)}
		score := 0.7*k.popularity + 0.3*k.freshness
		reasons := []string{recommendation.ReasonPopular}
		if k.freshness*0.3 > k.popularity*0.7 {
			reasons = []string{recommendation.ReasonFresh}
		}
		tracks = append(tracks, Scored{ID: t.ID, Score: round(score), Reasons: reasons})
		for _, a := range t.ArtistIDs {
			plays[a] += t.Plays
		}
	}
	var maxPlays uint64
	for _, p := range plays {
		maxPlays = max(maxPlays, p)
	}
	for a, p := range plays {
		score := 0.0
		if maxPlays > 0 {
			score = math.Log1p(float64(p)) / math.Log1p(float64(maxPlays))
		}
		artists = append(artists, Scored{ID: a, Score: round(score), Reasons: []string{recommendation.ReasonPopular}})
	}
	return top(tracks, nTracks), top(artists, nArtists)
}

// top sorts by score (ties by ID, for stable output) and keeps n.
func top(s []Scored, n int) []Scored {
	slices.SortFunc(s, func(a, b Scored) int {
		if c := cmp.Compare(b.Score, a.Score); c != 0 {
			return c
		}
		return cmp.Compare(a.ID.String(), b.ID.String())
	})
	if len(s) > n {
		s = s[:n]
	}
	return s
}

func mapValues(m map[uuid.UUID]Scored) []Scored {
	out := make([]Scored, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func round(v float64) float64 {
	return math.Round(v*1e4) / 1e4
}
