/**
 * In-process mock of the public API (/api/v1) for development and tests.
 * It returns the same status codes and error envelope as the real gateway.
 */
import { search, searchAlbums, searchArtists, searchBrowsePage, searchPlaylists, searchTracks, type SearchType } from './searchIndex';
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
export const mockAuth = { signedIn: true, tokens: new Set<string>(), issued: 0, email: 'rin@example.com' };

interface MockAccount {
  id: string;
  email: string;
  password: string;
  username: string;
  displayName: string;
}

export const mockAccounts = new Map<string, MockAccount>([
  [
    'rin@example.com',
    {
      id: currentUser.id,
      email: 'rin@example.com',
      password: 'password1234',
      username: currentUser.username,
      displayName: currentUser.displayName,
    },
  ],
]);

/** Saved tracks, albums and followed artists of the mock listener (Library & Social Services). */
const likedFixtures = [...prismHoursPage.tracks, ...homeFeed.trending.today, ...homeFeed.trending.week].flatMap((t) => (t?.liked ? [t.id] : []));
const savedAlbumFixtures = [
  'alb-prism-hours',
  'alb-salt-lanterns',
  'alb-hollow-signal',
  'alb-winter-index',
  'alb-lowlight-atlas',
  'alb-quiet-machines',
];
const followedArtistFixtures = homeFeed.followedArtists.map((a) => a.id);

export const mockLibrary = {
  tracks: new Set<string>(likedFixtures),
  initial: new Set(likedFixtures).size,
  albums: new Set<string>(savedAlbumFixtures),
  initialAlbums: savedAlbumFixtures.length,
  followedArtists: new Set<string>(followedArtistFixtures),
};

const unauthorized = (code: string, message: string): MockResponse => ({ status: 401, json: { error: { code, message, requestId: 'mock-auth' } } });

function issueTokens(email: string, userId: string) {
  const accessToken = `mock-access-${++mockAuth.issued}`;
  mockAuth.tokens.add(accessToken);
  mockAuth.signedIn = true;
  mockAuth.email = email;
  return {
    accessToken,
    tokenType: 'Bearer' as const,
    expiresIn: 900,
    expiresAt: new Date(Date.now() + 900_000).toISOString(),
    user: { id: userId, email, roles: ['USER'] },
  };
}

function mockRefresh(): MockResponse {
  if (!mockAuth.signedIn) return unauthorized('SESSION_EXPIRED', 'Session expired, sign in again');
  const account = mockAccounts.get(mockAuth.email);
  return ok(issueTokens(account?.email ?? 'rin@example.com', account?.id ?? currentUser.id));
}

function mockLogin(body: unknown): MockResponse {
  const raw = (body ?? {}) as { email?: unknown; password?: unknown };
  const email = typeof raw.email === 'string' ? raw.email.trim().toLowerCase() : '';
  const password = typeof raw.password === 'string' ? raw.password : '';
  const account = mockAccounts.get(email);
  if (!account || account.password !== password) {
    return unauthorized('INVALID_CREDENTIALS', 'Email or password is incorrect');
  }
  return ok(issueTokens(account.email, account.id));
}

function mockRegister(body: unknown): MockResponse {
  const raw = (body ?? {}) as { email?: unknown; password?: unknown };
  const email = typeof raw.email === 'string' ? raw.email.trim().toLowerCase() : '';
  const password = typeof raw.password === 'string' ? raw.password : '';
  const fields: Record<string, string> = {};
  if (!email || email.length > 254 || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    fields.email = 'must be a valid email address';
  }
  const len = Array.from(password).length;
  if (len < 10) fields.password = 'must be at least 10 characters';
  else if (len > 128) fields.password = 'must be at most 128 characters';
  else if (!password.trim()) fields.password = 'must not be blank';
  if (Object.keys(fields).length > 0) {
    return { status: 422, json: { error: { code: 'VALIDATION_FAILED', message: 'Request validation failed', details: { fields } } } };
  }
  if (mockAccounts.has(email)) {
    return { status: 409, json: { error: { code: 'EMAIL_TAKEN', message: 'An account with this email already exists', requestId: 'mock-auth' } } };
  }
  const local = email.split('@')[0] || 'listener';
  const account: MockAccount = {
    id: `usr-${mockAccounts.size + 1}`,
    email,
    password,
    username: local,
    displayName: local,
  };
  mockAccounts.set(email, account);
  return { status: 201, json: issueTokens(account.email, account.id) };
}

function mockCurrentUser(): MockResponse {
  const account = mockAccounts.get(mockAuth.email);
  if (!account || account.email === 'rin@example.com') return ok(currentUser);
  return ok({ ...currentUser, id: account.id, username: account.username, displayName: account.displayName });
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

interface CatalogArtistDTO {
  id: string;
  name: string;
  listeners?: string;
  coverUrl: null;
  art: number;
}

interface CatalogAlbumDTO {
  id: string;
  title: string;
  albumType: 'ALBUM' | 'EP' | 'SINGLE' | 'COMPILATION';
  releaseDate: string;
  artistIds: string[];
  explicit: boolean;
  coverUrl: null;
  art: number;
}

interface CatalogTrackDTO {
  id: string;
  albumId: string;
  artistIds: string[];
  title: string;
  durationMs: number;
  trackNumber: number;
  discNumber: number;
  explicit: boolean;
  status: 'READY' | 'BLOCKED';
  coverUrl: null;
  art: number;
}

function buildCatalogIndex() {
  const artists = new Map<string, CatalogArtistDTO>();
  const artistByName = new Map<string, string>();
  const albums = new Map<string, CatalogAlbumDTO>();
  const tracks = new Map<string, CatalogTrackDTO>();

  const ensureArtist = (name: string, id?: string, art = 0, listeners?: string): string => {
    const existingId = id ?? artistByName.get(name);
    if (existingId && artists.has(existingId)) return existingId;
    const resolvedId = id ?? `art-${name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')}`;
    artists.set(resolvedId, { id: resolvedId, name, listeners, coverUrl: null, art });
    artistByName.set(name, resolvedId);
    return resolvedId;
  };

  [...homeFeed.followedArtists, ...searchArtists].forEach((a) => {
    ensureArtist(a.name, a.id, a.art, a.listeners);
  });
  ensureArtist(prismHoursPage.artist.name, prismHoursPage.artist.id, prismHoursPage.artist.art);

  const addAlbum = (a: {
    id: string;
    title: string;
    artistName: string;
    year?: number;
    releaseDate?: string;
    albumType?: string;
    explicit?: boolean;
    art: number;
  }) => {
    if (albums.has(a.id)) return;
    const artistIds = a.artistName.split(' · ').map((n) => ensureArtist(n.trim(), undefined, a.art));
    const albumType = (a.albumType === 'EP' || a.albumType === 'SINGLE' || a.albumType === 'COMPILATION' ? a.albumType : 'ALBUM') as CatalogAlbumDTO['albumType'];
    albums.set(a.id, {
      id: a.id,
      title: a.title,
      albumType,
      releaseDate: a.releaseDate ?? `${a.year ?? 2025}-01-17`,
      artistIds,
      explicit: Boolean(a.explicit),
      coverUrl: null,
      art: a.art,
    });
  };

  addAlbum({ ...prismHoursPage.album, artistName: prismHoursPage.artist.name });
  addAlbum({ ...homeFeed.albumOfTheWeek.album, releaseDate: homeFeed.albumOfTheWeek.releaseDate });
  [...homeFeed.recentlyPlayed, ...homeFeed.recommended.items, ...homeFeed.newReleases.items, ...prismHoursPage.moreByArtist, ...searchAlbums].forEach((item) => {
    if (item.kind === 'album') addAlbum(item);
  });

  const addTrack = (
    t: { id: string; title: string; artistName: string; albumId: string; albumTitle: string; durationSec: number; explicit?: boolean; available?: boolean; art: number },
    index: number,
  ) => {
    if (tracks.has(t.id)) return;
    if (!albums.has(t.albumId)) {
      addAlbum({ id: t.albumId, title: t.albumTitle, artistName: t.artistName, art: t.art });
    }
    const artistIds = t.artistName.split(' · ').map((n) => ensureArtist(n.trim(), undefined, t.art));
    tracks.set(t.id, {
      id: t.id,
      albumId: t.albumId,
      artistIds,
      title: t.title,
      durationMs: t.durationSec * 1000,
      trackNumber: index + 1,
      discNumber: 1,
      explicit: Boolean(t.explicit),
      status: t.available === false ? 'BLOCKED' : 'READY',
      coverUrl: null,
      art: t.art,
    });
  };

  [...prismHoursPage.tracks, ...homeFeed.trending.today, ...homeFeed.trending.week, ...searchTracks].forEach((t, i) => {
    if (t) addTrack(t, i);
  });

  return { artists, albums, tracks };
}

const catalogIndex = buildCatalogIndex();

function paginateMock<T>(items: T[], query: URLSearchParams) {
  const limit = Math.min(100, Math.max(1, Number(query.get('limit') ?? 20) || 20));
  const offset = Math.max(0, Number(query.get('cursor') ?? 0) || 0);
  const slice = items.slice(offset, offset + limit);
  const hasMore = offset + limit < items.length;
  return {
    data: slice,
    pagination: { nextCursor: hasMore ? String(offset + limit) : null, hasMore },
  };
}

function mockTimestamp(index: number): string {
  return new Date(Date.UTC(2026, 10, 20, 12, 0, 0) - index * 3_600_000).toISOString();
}

function tracksOf(kind: string, id: string): MockResponse {
  const info = collections.get(id);
  if (!info) {
    const code = kind === 'album' ? 'ALBUM_NOT_FOUND' : kind === 'artist' ? 'ARTIST_NOT_FOUND' : 'PLAYLIST_NOT_FOUND';
    return notFound(code, `${kind[0]?.toUpperCase()}${kind.slice(1)} not found`);
  }
  return ok({ data: generatedTracks(id, info.title, info.artistName, info.art, kind === 'artist' ? 5 : 8) });
}

/** BFF playlist page (GET /pages/playlists/:id): header and tracks in play order. */
function playlistPage(id: string): MockResponse {
  const info = collections.get(id);
  if (!info || !id.startsWith('pl-')) return notFound('PLAYLIST_NOT_FOUND', 'Playlist not found');
  const tracks = generatedTracks(id, info.title, info.artistName, info.art, 8);
  return ok({
    playlist: {
      id,
      title: info.title,
      ownerId: currentUser.id,
      owner: info.artistName,
      trackCount: tracks.length,
      durationSec: tracks.reduce((sum, t) => sum + t.durationSec, 0),
      updatedAt: '2026-09-26T10:00:00Z',
      coverUrl: null,
      art: info.art,
    },
    tracks,
  });
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
  ['POST', '/auth/login', (_p, _q, ctx) => mockLogin(ctx.body)],
  ['POST', '/auth/register', (_p, _q, ctx) => mockRegister(ctx.body)],
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
  ['GET', '/users/me', authed(() => mockCurrentUser())],
  [
    'GET',
    '/me/library/summary',
    // Canvas counters, moved by likes and saved albums made during the session.
    authed(() =>
      ok({
        tracks: librarySummary.likedTracksCount + mockLibrary.tracks.size - mockLibrary.initial,
        albums: librarySummary.savedCount + mockLibrary.albums.size - mockLibrary.initialAlbums,
      }),
    ),
  ],
  [
    'GET',
    '/me/library/tracks',
    authed((_p, q) => {
      const items = Array.from(mockLibrary.tracks)
        .reverse()
        .map((trackId, i) => ({ trackId, savedAt: mockTimestamp(i) }));
      return ok(paginateMock(items, q));
    }),
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
  [
    'GET',
    '/me/library/albums',
    authed((_p, q) => {
      const items = Array.from(mockLibrary.albums)
        .reverse()
        .map((albumId, i) => ({ albumId, savedAt: mockTimestamp(i) }));
      return ok(paginateMock(items, q));
    }),
  ],
  ['GET', '/me/library/albums/contains', authed((_p, q) => ok({ data: (q.get('ids') ?? '').split(',').filter((id) => mockLibrary.albums.has(id)) }))],
  [
    'PUT',
    '/me/library/albums/:id',
    authed((p) => {
      mockLibrary.albums.add(p.id ?? '');
      return { status: 204, json: null };
    }),
  ],
  [
    'DELETE',
    '/me/library/albums/:id',
    authed((p) => {
      mockLibrary.albums.delete(p.id ?? '');
      return { status: 204, json: null };
    }),
  ],
  [
    'GET',
    '/users/:id/following',
    (_p, q) => {
      if (q.get('type') !== 'artist') return ok(paginateMock([], q));
      const items = Array.from(mockLibrary.followedArtists)
        .reverse()
        .map((artistId, i) => ({ artistId, followedAt: mockTimestamp(i) }));
      return ok(paginateMock(items, q));
    },
  ],
  ['GET', '/me/following/contains', authed((_p, q) => ok({ data: (q.get('ids') ?? '').split(',').filter((id) => mockLibrary.followedArtists.has(id)) }))],
  [
    'PUT',
    '/artists/:id/follow',
    authed((p) => {
      mockLibrary.followedArtists.add(p.id ?? '');
      return { status: 204, json: null };
    }),
  ],
  [
    'DELETE',
    '/artists/:id/follow',
    authed((p) => {
      mockLibrary.followedArtists.delete(p.id ?? '');
      return { status: 204, json: null };
    }),
  ],
  [
    'GET',
    '/albums/:id',
    (p) => {
      const album = catalogIndex.albums.get(p.id ?? '');
      return album ? ok(album) : notFound('ALBUM_NOT_FOUND', 'Album not found');
    },
  ],
  [
    'GET',
    '/tracks',
    (_p, q) => {
      const ids = (q.get('ids') ?? '').split(',').filter(Boolean);
      return ok({ data: ids.map((id) => catalogIndex.tracks.get(id)).filter(Boolean) });
    },
  ],
  [
    'GET',
    '/artists',
    (_p, q) => {
      const ids = (q.get('ids') ?? '').split(',').filter(Boolean);
      return ok({ data: ids.map((id) => catalogIndex.artists.get(id)).filter(Boolean) });
    },
  ],
  ['POST', '/playback/events', authed(() => ({ status: 202, json: null }))],
  [
    'GET',
    '/me/playlists',
    authed(() =>
      ok({
        data: librarySummary.playlists.map((p, i) => ({
          ...p,
          ownerId: currentUser.id,
          updatedAt: mockTimestamp(i * 24),
        })),
      }),
    ),
  ],
  ['GET', '/pages/home', () => ok(homeFeed)],
  ['GET', '/pages/albums/:id', (p) => albumPage(p.id ?? '')],
  ['GET', '/pages/search', () => ok(searchBrowsePage)],
  ['GET', '/search', (_p, q) => searchRoute(q)],
  ['GET', '/pages/playlists/:id', (p) => playlistPage(p.id ?? '')],
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
