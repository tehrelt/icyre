/**
 * In-process mock of the public API (/api/v1) for development and tests.
 * It returns the same status codes and error envelope as the real gateway.
 */
import { search, searchAlbums, searchArtists, searchBrowsePage, searchPlaylists, type SearchType } from './searchIndex';
import { currentUser, generatedAlbumPage, generatedTracks, homeFeed, librarySummary, prismHoursPage } from './fixtures';

interface MockResponse {
  status: number;
  json: unknown;
}

interface RequestContext {
  token: string | null;
  body: unknown;
}

type Handler = (params: Record<string, string>, query: URLSearchParams, ctx: RequestContext) => MockResponse;

/**
 * Mock session: `signedIn` stands for the HttpOnly refresh cookie, `tokens`
 * for access tokens the mock Auth still accepts. Tests flip these to model
 * anonymous visitors and expired access tokens.
 */
export const mockAuth = { signedIn: true, tokens: new Set<string>(), issued: 0 };

/** Saved tracks of the mock listener (Library Service), seeded from the fixtures' liked flags. */
const likedFixtures = [...prismHoursPage.tracks, ...homeFeed.trending.today, ...homeFeed.trending.week].flatMap((t) => (t?.liked ? [t.id] : []));
export const mockLibrary = { tracks: new Set<string>(likedFixtures), initial: new Set(likedFixtures).size };

const unauthorized = (code: string, message: string): MockResponse => ({ status: 401, json: { error: { code, message, requestId: 'mock-auth' } } });

function mockRefresh(): MockResponse {
  if (!mockAuth.signedIn) return unauthorized('SESSION_EXPIRED', 'Session expired, sign in again');
  const accessToken = `mock-access-${++mockAuth.issued}`;
  mockAuth.tokens.add(accessToken);
  return ok({
    accessToken,
    tokenType: 'Bearer',
    expiresIn: 900,
    expiresAt: new Date(Date.now() + 900_000).toISOString(),
    user: { id: currentUser.id, email: 'rin@example.com', roles: ['USER'] },
  });
}

function authed(handler: Handler): Handler {
  return (params, query, ctx) =>
    ctx.token && mockAuth.tokens.has(ctx.token) ? handler(params, query, ctx) : unauthorized('UNAUTHENTICATED', 'Authentication required');
}

/**
 * Route patterns that fail with 503 — lets tests and local demos exercise
 * error states (e.g. `mockFaults.add('/search')`).
 */
export const mockFaults = new Set<string>();

/** Per-route response overrides for tests (e.g. an empty BFF home page). */
export const mockOverrides = new Map<string, () => { status: number; json: unknown }>();

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
  ['POST', '/auth/refresh', () => mockRefresh()],
  [
    'POST',
    '/auth/logout',
    () => {
      mockAuth.signedIn = false;
      mockAuth.tokens.clear();
      return { status: 204, json: null };
    },
  ],
  ['GET', '/users/me', authed(() => ok(currentUser))],
  [
    'GET',
    '/me/library/summary',
    // Canvas counters, moved by likes made during the session.
    authed(() => ok({ tracks: librarySummary.likedTracksCount + mockLibrary.tracks.size - mockLibrary.initial, albums: librarySummary.savedCount })),
  ],
  ['GET', '/me/library/tracks/contains', authed((_p, q) => ok({ data: (q.get('ids') ?? '').split(',').filter((id) => mockLibrary.tracks.has(id)) }))],
  [
    'PUT',
    '/me/library/tracks/:id',
    authed((p) => {
      mockLibrary.tracks.add(p.id ?? '');
      return { status: 204, json: null };
    }),
  ],
  [
    'DELETE',
    '/me/library/tracks/:id',
    authed((p) => {
      mockLibrary.tracks.delete(p.id ?? '');
      return { status: 204, json: null };
    }),
  ],
  ['GET', '/me/playlists', authed(() => ok({ data: librarySummary.playlists }))],
  ['GET', '/pages/home', () => ok(homeFeed)],
  ['GET', '/pages/albums/:id', (p) => albumPage(p.id ?? '')],
  ['GET', '/pages/search', () => ok(searchBrowsePage)],
  ['GET', '/search', (_p, q) => searchRoute(q)],
  ['GET', '/playlists/:id/tracks', (p) => tracksOf('playlist', p.id ?? '')],
  ['GET', '/artists/:id/top-tracks', (p) => tracksOf('artist', p.id ?? '')],
  // Stream authorization. Mock mode plays through the simulated engine, so
  // the URL is never fetched.
  [
    'POST',
    '/stream/authorize',
    authed((_p, _q, { body }) => {
      const trackId = (body as { trackId?: string } | undefined)?.trackId ?? '';
      return ok({ trackId, quality: '256', url: `mock-media://tracks/${trackId}/audio/256.aac`, expiresAt: new Date(Date.now() + 300_000).toISOString() });
    }),
  ],
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

export async function mockRequest(method: string, path: string, body: unknown, signal?: AbortSignal, token: string | null = null): Promise<MockResponse> {
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
    const override = mockOverrides.get(pattern);
    if (override) return structuredClone(override());
    if (mockFaults.has(pattern)) {
      return { status: 503, json: { error: { code: 'SERVICE_UNAVAILABLE', message: 'Service is temporarily unavailable', requestId: 'mock-fault' } } };
    }
    return structuredClone(handler(params, query, { token, body }));
  }
  return notFound('NOT_FOUND', `No mock for ${method} ${path}`);
}
