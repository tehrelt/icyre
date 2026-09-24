/**
 * In-process mock of the public API (/api/v1) for development and tests.
 * It returns the same status codes and error envelope as the real gateway.
 */
import { currentUser, generatedTracks, homeFeed, librarySummary, prismHoursTracks } from './fixtures';

interface MockResponse {
  status: number;
  json: unknown;
}

type Handler = (params: Record<string, string>) => MockResponse;

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

const routes: Array<[method: string, pattern: string, handler: Handler]> = [
  ['GET', '/me', () => ok(currentUser)],
  ['GET', '/library/summary', () => ok(librarySummary)],
  ['GET', '/pages/home', () => ok(homeFeed)],
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
  for (const [m, pattern, handler] of routes) {
    if (m !== method) continue;
    const params = match(pattern, path);
    if (params) return structuredClone(handler(params));
  }
  return notFound('NOT_FOUND', `No mock for ${method} ${path}`);
}
