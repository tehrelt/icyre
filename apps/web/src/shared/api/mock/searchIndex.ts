/**
 * Mock search over a small fixture catalogue (content from the Search
 * screens of the product canvas plus the Home fixtures). Mirrors the public
 * contract of GET /api/v1/search; ranking is a simple stand-in for OpenSearch.
 */
import { homeFeed, prismHoursTracks } from './fixtures';

const artist = (id: string, name: string, listeners: string, art: number, popularity: number, verified = false) => ({
  kind: 'artist' as const, id, name, listeners, art, coverUrl: null, verified, popularity,
});
const album = (id: string, title: string, artistName: string, year: number, art: number, popularity: number, explicit = false) => ({
  kind: 'album' as const, id, title, artistName, year, art, coverUrl: null, explicit, popularity,
});
const playlist = (id: string, title: string, owner: string, trackCount: number, art: number, popularity: number) => ({
  kind: 'playlist' as const, id, title, owner, trackCount, art, coverUrl: null, popularity,
});
const track = (id: string, title: string, artistName: string, albumId: string, albumTitle: string, durationSec: number, art: number, popularity: number, extra: Record<string, unknown> = {}) => ({
  id, title, artistName, albumId, albumTitle, durationSec, art, coverUrl: null, explicit: false, liked: false, available: true, popularity, ...extra,
});

export const searchArtists = [
  artist('art-nova-hale', 'Nova Hale', '1.2M', 3, 100, true),
  artist('art-novaline', 'Novaline', '41K', 5, 40),
  artist('art-casa-nova-trio', 'Casa Nova Trio', '23K', 1, 30),
  artist('art-nova-kiri', 'Nova Kiri', '118K', 7, 60),
  artist('art-terra-nova', 'Terra Nova', '9.4K', 6, 20),
  artist('art-villanova-choir', 'Villanova Choir', '5.1K', 2, 10),
  ...homeFeed.followedArtists.filter((a) => a.id !== 'art-nova-hale').map((a, i) => artist(a.id, a.name, a.listeners, a.art, 50 - i, true)),
  artist('art-aster-vale', 'Aster Vale', '96K', 1, 45),
];

export const searchAlbums = [
  album('alb-prism-hours', 'Prism Hours', 'Nova Hale', 2026, 0, 100),
  album('alb-winter-index', 'Winter Index', 'Nova Hale', 2024, 3, 70),
  album('alb-nova-lines', 'Nova Lines', 'Juno Reyes', 2025, 1, 50),
  album('alb-supernova-drift', 'Supernova Drift', 'Kai Frost', 2026, 4, 60, true),
  album('alb-bossa-nova-rooms', 'Bossa Nova Rooms', 'Casa Nova Trio', 2022, 5, 30),
  album('alb-close-weather', 'Close Weather', 'Nova Hale', 2023, 5, 55),
  ...homeFeed.newReleases.items.map((a, i) => album(a.id, a.title, a.artistName, a.year, a.art, 40 - i, a.explicit)),
  album(homeFeed.albumOfTheWeek.album.id, 'Salt Lanterns', 'Mira Solen', 2026, 2, 80),
];

export const searchPlaylists = [
  playlist('pl-this-is-nova-hale', 'This is Nova Hale', 'ICYRE', 40, 3, 90),
  playlist('pl-nova-and-friends', 'Nova & friends', 'ICYRE', 55, 0, 60),
  playlist('pl-supernova', 'Supernova', 'lea.m', 32, 4, 40),
  playlist('pl-nova-for-studying', 'nova for studying', 'theo', 87, 5, 35),
  playlist('pl-bossa-nova-at-dusk', 'Bossa nova at dusk', 'ICYRE', 60, 1, 30),
  playlist('pl-novas-and-nebulae', 'Novas & nebulae', 'kira', 24, 7, 20),
  playlist('pl-glass-grain', 'Glass & grain', 'ICYRE', 45, 2, 50),
];

export const searchTracks = [
  ...prismHoursTracks.map((t, i) => ({ ...t, popularity: ('plays' in t ? Number(t.plays) : 0) / 50_000 + (i === 1 ? 50 : 0) })),
  track('trk-supernova-drift', 'Supernova Drift', 'Kai Frost', 'alb-supernova-drift', 'Supernova Drift', 205, 4, 85, { explicit: true }),
  track('trk-nova', 'Nova', 'Aster Vale', 'alb-nova-single', 'Nova', 262, 1, 80),
  ...homeFeed.trending.today.filter((t) => t.albumId !== 'alb-prism-hours').map((t, i) => ({ ...t, popularity: 60 - i })),
];

function norm(s: string): string {
  return s.toLowerCase().normalize('NFKD').replace(/[̀-ͯ]/g, '').replace(/[^\p{L}\p{N}\s&]/gu, ' ');
}

/** Text relevance: exact > prefix > word prefix > substring; all query words must match. */
function relevance(query: string, ...fields: string[]): number {
  const q = norm(query).trim();
  const words = q.split(/\s+/).filter(Boolean);
  if (words.length === 0) return 0;
  let best = 0;
  for (const f of fields.map(norm)) {
    if (!words.every((w) => f.includes(w))) continue;
    const score = f === q ? 4 : f.startsWith(q) ? 3 : f.split(/\s+/).some((w) => w.startsWith(words[0]!)) ? 2 : 1;
    best = Math.max(best, score);
  }
  return best;
}

function rank<T extends { popularity: number }>(items: T[], score: (x: T) => number): T[] {
  return items
    .map((x) => ({ x, s: score(x) }))
    .filter((r) => r.s > 0)
    .sort((a, b) => b.s - a.s || b.x.popularity - a.x.popularity)
    .map((r) => r.x);
}

const strip = <T extends { popularity: number }>({ popularity: _p, ...rest }: T) => rest;

function levenshtein(a: string, b: string): number {
  const dp = Array.from({ length: b.length + 1 }, (_, i) => i);
  for (let i = 1; i <= a.length; i++) {
    let prev = dp[0]!;
    dp[0] = i;
    for (let j = 1; j <= b.length; j++) {
      const tmp = dp[j]!;
      dp[j] = Math.min(dp[j]! + 1, dp[j - 1]! + 1, prev + (a[i - 1] === b[j - 1] ? 0 : 1));
      prev = tmp;
    }
  }
  return dp[b.length]!;
}

const vocabulary = [...new Set([...searchArtists.map((a) => a.name), ...searchAlbums.map((a) => a.title), ...searchTracks.map((t) => t.title)].flatMap((s) => norm(s).split(/\s+/)).filter((w) => w.length > 2))];

/** "Did you mean": replaces each unknown word with the closest catalogue word. */
function suggest(query: string): string | null {
  const words = norm(query).trim().split(/\s+/).filter(Boolean);
  let changed = false;
  const fixed = words.map((w) => {
    if (vocabulary.includes(w)) return w;
    let best = w;
    let dist = Math.max(2, Math.floor(w.length / 3)) + 1;
    for (const v of vocabulary) {
      const d = levenshtein(w, v);
      if (d < dist) {
        dist = d;
        best = v;
      }
    }
    if (best !== w) changed = true;
    return best;
  });
  return changed ? fixed.join(' ') : null;
}

export type SearchType = 'all' | 'tracks' | 'artists' | 'albums' | 'playlists';

export function search(query: string, type: SearchType, limit: number) {
  const tracks = rank(searchTracks, (t) => relevance(query, t.title, t.artistName) * 2 + (relevance(query, t.albumTitle) > 0 ? 1 : 0));
  const artists = rank(searchArtists, (a) => relevance(query, a.name));
  const albums = rank(searchAlbums, (a) => relevance(query, a.title, a.artistName));
  const playlists = rank(searchPlaylists, (p) => relevance(query, p.title));

  const counts = { tracks: tracks.length, artists: artists.length, albums: albums.length, playlists: playlists.length };
  const total = counts.tracks + counts.artists + counts.albums + counts.playlists;

  // Top result: a strongly matching artist first, then album, then track.
  const topArtist = artists[0] && relevance(query, artists[0].name) >= 2 ? artists[0] : undefined;
  const topAlbum = albums[0] && relevance(query, albums[0].title) >= 3 ? albums[0] : undefined;
  let topResult: unknown = null;
  if (topArtist) {
    const { verified, ...rest } = strip(topArtist);
    topResult = { item: rest, verified, monthlyListeners: topArtist.listeners };
  } else if (topAlbum) topResult = { item: strip(topAlbum), verified: false };
  else if (artists[0]) {
    const { verified, ...rest } = strip(artists[0]);
    topResult = { item: rest, verified, monthlyListeners: artists[0].listeners };
  } else if (albums[0]) topResult = { item: strip(albums[0]), verified: false };
  else if (playlists[0]) topResult = { item: strip(playlists[0]), verified: false };

  const take = (kind: SearchType, n: number) => (type === 'all' ? n : type === kind ? limit : 0);
  return {
    query,
    type,
    counts,
    topResult: type === 'all' ? topResult : null,
    tracks: tracks.slice(0, take('tracks', 4)).map(strip),
    artists: artists.slice(0, take('artists', 6)).map(({ verified: _v, ...a }) => strip(a)),
    albums: albums.slice(0, take('albums', 6)).map(strip),
    playlists: playlists.slice(0, take('playlists', 6)).map(strip),
    didYouMean: total === 0 ? suggest(query) : null,
  };
}

/** GET /api/v1/pages/search — Search before a query (canvas SearchEmpty). */
export const searchBrowsePage = {
  genres: [
    ['ambient', 'Ambient', 2140, 2],
    ['electronic', 'Electronic', 8902, 4],
    ['indie-folk', 'Indie folk', 3417, 5],
    ['hip-hop', 'Hip-hop', 12655, 6],
    ['jazz', 'Jazz', 5038, 1],
    ['classical', 'Classical', 7221, 3],
    ['downtempo', 'Downtempo', 1876, 0],
    ['rnb', 'R&B', 6340, 7],
    ['post-rock', 'Post-rock', 1204, 5],
    ['house', 'House', 9733, 4],
    ['soundtrack', 'Soundtrack', 2988, 3],
    ['experimental', 'Experimental', 1562, 2],
  ].map(([slug, label, releaseCount, art]) => ({ slug, label, releaseCount, art })),
  moods: ['Calm', 'Focus', 'Late night', 'Uplifting', 'Melancholy', 'Workout', 'Rainy day', 'Dreamy', 'Slow morning'].map((label) => ({
    id: norm(label).trim().replace(/\s+/g, '-'),
    label,
  })),
  collections: {
    kicker: 'Curated by the editors',
    items: [
      playlist('pl-ice-age', 'Ice age', 'ICYRE', 60, 2, 0),
      playlist('pl-glass-grain', 'Glass & grain', 'ICYRE', 45, 5, 0),
      playlist('pl-fresh-frost', 'Fresh frost', 'ICYRE', 50, 4, 0),
      playlist('pl-low-light-jazz', 'Low light jazz', 'ICYRE', 72, 1, 0),
      playlist('pl-field-notes', 'Field notes', 'ICYRE', 38, 3, 0),
      playlist('pl-slow-motion', 'Slow motion', 'ICYRE', 64, 7, 0),
    ].map(strip),
  },
};
