/**
 * In-process mock of the public API (/api/v1) for development and tests.
 * It returns the same status codes and error envelope as the real gateway.
 */
import { search, searchAlbums, searchArtists, searchBrowsePage, searchPlaylists, type SearchType } from './searchIndex';
import { currentUser, generatedAlbumPage, generatedTracks, homeFeed, librarySummary, prismHoursPage, prismHoursTracks } from './fixtures';

interface MockResponse {
  status: number;
  json: unknown;
}

type Handler = (params: Record<string, string>, query: URLSearchParams) => MockResponse;

/**
 * Route patterns that fail with 503 — lets tests and local demos exercise
 * error states (e.g. `mockFaults.add('/search')`).
 */
export const mockFaults = new Set<string>();

const SEARCH_TYPES: SearchType[] = ['all', 'tracks', 'artists', 'albums', 'playlists'];

function searchRoute(q: URLSearchParams): MockResponse {
  const query = (q.get('q') ?? '').trim();
  if (!query) {
    return { status: 422, json: { error: { code: 'VALIDATION_FAILED', message: 'Query must not be empty', details: { fields: { q: 'must not be empty' } } } } };
  }
  const type = (SEARCH_TYPES as string[]).includes(q.get('type') ?? '') ? (q.get('type') as SearchType) : 'all';
  const limit = Math.min(50, Math.max(1, Number(q.get('limit') ?? 20) || 20));
  return ok(search(query, type, limit));
}

const ok = (json: unknown): MockResponse => ({ status: 200, json });
const notFound = (code: string, message: string): MockResponse => ({
  status: 404,
  json: { error: { code, message, requestId: `mock-${Math.random().toString(36).slice(2, 10)}` } },
});

interface CollectionInfo {
  title: string;
  artistName: string;
  art: number;
}

/** Every collection that appears in the fixtures, by id. */
function collectionIndex(): Map<string, CollectionInfo> {
  const index = new Map<string, CollectionInfo>();
  const add = (c: { kind: string; id: string; art: number; title?: string; name?: string; artistName?: string; owner?: string }) =>
    index.set(c.id, { title: c.title ?? c.name ?? '', artistName: c.artistName ?? c.name ?? c.owner ?? 'Various artists', art: c.art });

  [...homeFeed.recentlyPlayed, ...homeFeed.recommended.items, ...homeFeed.newReleases.items, ...homeFeed.followedArtists].forEach(add);
  [...homeFeed.madeForYou.playlists, homeFeed.madeForYou.featured, ...librarySummary.playlists].forEach(add);
  add(homeFeed.albumOfTheWeek.album);
  prismHoursPage.moreByArtist.forEach(add);
  [...searchArtists, ...searchAlbums, ...searchPlaylists, ...searchBrowsePage.collections.items].forEach((c) => {
    if (!index.has(c.id)) add(c);
  });
  return index;
}

const collections = collectionIndex();

function tracksOf(kind: string, id: string): MockResponse {
  if (kind === 'album' && id === 'alb-prism-hours') return ok({ data: prismHoursTracks });
  const info = collections.get(id);
  if (!info) {
    const code = kind === 'album' ? 'ALBUM_NOT_FOUND' : kind === 'artist' ? 'ARTIST_NOT_FOUND' : 'PLAYLIST_NOT_FOUND';
    return notFound(code, `${kind[0]?.toUpperCase()}${kind.slice(1)} not found`);
  }
  return ok({ data: generatedTracks(id, info.title, info.artistName, info.art, kind === 'artist' ? 5 : 8) });
}

interface IndexedAlbum extends CollectionInfo {
  year?: number;
}

function albumPage(id: string): MockResponse {
  if (id === prismHoursPage.album.id) return ok(prismHoursPage);
  const info: IndexedAlbum | undefined = prismHoursPage.moreByArtist.find((a) => a.id === id) ?? collections.get(id);
  if (!info || !id.startsWith('alb-')) return notFound('ALBUM_NOT_FOUND', 'Album not found');
  return ok(generatedAlbumPage(id, info.title, info.artistName, info.art, info.year));
}

const routes: Array<[method: string, pattern: string, handler: Handler]> = [
  ['GET', '/me', () => ok(currentUser)],
  ['GET', '/library/summary', () => ok(librarySummary)],
  ['GET', '/pages/home', () => ok(homeFeed)],
  ['GET', '/pages/albums/:id', (p) => albumPage(p.id ?? '')],
  ['GET', '/pages/search', () => ok(searchBrowsePage)],
  ['GET', '/search', (_p, q) => searchRoute(q)],
  ['GET', '/albums/:id/tracks', (p) => tracksOf('album', p.id ?? '')],
  ['GET', '/playlists/:id/tracks', (p) => tracksOf('playlist', p.id ?? '')],
  ['GET', '/artists/:id/top-tracks', (p) => tracksOf('artist', p.id ?? '')],
  // Stream authorization: no media origin locally, so no signed URL.
  ['GET', '/playback/tracks/:id/stream', (p) => ok({ trackId: p.id, url: null, expiresAt: null })],
];

function match(pattern: string, path: string): Record<string, string> | null {
  const a = pattern.split('/');
  const b = path.split('?')[0]!.split('/');
  if (a.length !== b.length) return null;
  const params: Record<string, string> = {};
  for (let i = 0; i < a.length; i++) {
    const seg = a[i]!;
    if (seg.startsWith(':')) params[seg.slice(1)] = decodeURIComponent(b[i]!);
    else if (seg !== b[i]) return null;
  }
  return params;
}

/** Simulated network latency; zero under test. */
const latencyMs = import.meta.env.MODE === 'test' ? 0 : 250;

export async function mockRequest(method: string, path: string, _body: unknown, signal?: AbortSignal): Promise<MockResponse> {
  if (latencyMs > 0) {
    await new Promise<void>((resolve, reject) => {
      const t = setTimeout(resolve, latencyMs);
      signal?.addEventListener('abort', () => {
        clearTimeout(t);
        reject(new DOMException('Aborted', 'AbortError'));
      });
    });
  }
  const query = new URLSearchParams(path.split('?')[1] ?? '');
  for (const [m, pattern, handler] of routes) {
    if (m !== method) continue;
    const params = match(pattern, path);
    if (!params) continue;
    if (mockFaults.has(pattern)) {
      return { status: 503, json: { error: { code: 'SERVICE_UNAVAILABLE', message: 'Service is temporarily unavailable', requestId: 'mock-fault' } } };
    }
    return structuredClone(handler(params, query));
  }
  return notFound('NOT_FOUND', `No mock for ${method} ${path}`);
}
