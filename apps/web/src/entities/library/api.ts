import { infiniteQueryOptions, queryOptions, useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { AlbumSummarySchema, AlbumTypeSchema, type AlbumSummary } from '@/entities/album/model';
import { ArtistSummarySchema, type ArtistSummary } from '@/entities/artist/model';
import { TrackSchema, type Track } from '@/entities/track/model';
import { apiGet, apiRequest } from '@/shared/api/client';

/** GET /me/library/summary (Library Service): sidebar counters. */
export const LibrarySummarySchema = z
  .object({ tracks: z.number().int().nonnegative(), albums: z.number().int().nonnegative() })
  .transform((s) => ({ savedCount: s.albums, likedTracksCount: s.tracks }));

export type LibrarySummary = z.infer<typeof LibrarySummarySchema>;

export const librarySummaryQuery = queryOptions({
  queryKey: ['library', 'summary'],
  queryFn: ({ signal }) => apiGet('/me/library/summary', LibrarySummarySchema, signal),
  staleTime: 60_000,
});

export const useLibrarySummary = () => useQuery(librarySummaryQuery);

/** Deterministic fallback cover index (0–7), matching BFF FNV-1a hash. */
export function artIndex(id: string): number {
  let hash = 0x811c9dc5;
  for (let i = 0; i < id.length; i++) {
    hash ^= id.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193);
  }
  return (hash >>> 0) % 8;
}

const PaginationSchema = z.object({
  nextCursor: z.string().nullable().default(null),
  hasMore: z.boolean().default(false),
});

const SavedAlbumsPageSchema = z.object({
  data: z.array(z.object({ albumId: z.string(), savedAt: z.string() })),
  pagination: PaginationSchema,
});

const SavedTracksPageSchema = z.object({
  data: z.array(z.object({ trackId: z.string(), savedAt: z.string() })),
  pagination: PaginationSchema,
});

const FollowedArtistsPageSchema = z.object({
  data: z.array(z.object({ artistId: z.string(), followedAt: z.string() })),
  pagination: PaginationSchema,
});

const ContainsSchema = z.object({ data: z.array(z.string()) });

const CatalogArtistSchema = z.object({
  id: z.string(),
  name: z.string(),
  listeners: z.string().optional(),
  coverUrl: z.string().nullable().optional(),
  art: z.number().int().optional(),
});

const CatalogAlbumSchema = z.object({
  id: z.string(),
  title: z.string(),
  albumType: AlbumTypeSchema.optional(),
  releaseDate: z.string(),
  artistIds: z.array(z.string()).default([]),
  explicit: z.boolean().optional(),
  coverUrl: z.string().nullable().optional(),
  art: z.number().int().optional(),
});

const CatalogTrackSchema = z.object({
  id: z.string(),
  albumId: z.string(),
  artistIds: z.array(z.string()).default([]),
  title: z.string(),
  durationMs: z.number().nonnegative(),
  explicit: z.boolean().default(false),
  status: z.string().default('READY'),
  coverUrl: z.string().nullable().optional(),
  art: z.number().int().optional(),
});

const CatalogArtistsListSchema = z.object({ data: z.array(CatalogArtistSchema) });
const CatalogTracksListSchema = z.object({ data: z.array(CatalogTrackSchema) });

export type SavedAlbumItem = AlbumSummary & { savedAt: string };
export type LikedTrackItem = Track & { savedAt: string };
export type FollowedArtistItem = ArtistSummary & { followedAt: string };

export interface LibraryPageSlice<T> {
  items: T[];
  nextCursor: string | null;
  hasMore: boolean;
}

const PAGE_LIMIT = 24;

function dedupe(ids: string[]): string[] {
  return Array.from(new Set(ids.filter(Boolean)));
}

async function fetchArtistsMap(ids: string[], signal?: AbortSignal): Promise<Map<string, z.infer<typeof CatalogArtistSchema>>> {
  const unique = dedupe(ids);
  const map = new Map<string, z.infer<typeof CatalogArtistSchema>>();
  if (unique.length === 0) return map;
  const res = await apiGet(`/artists?ids=${unique.map(encodeURIComponent).join(',')}`, CatalogArtistsListSchema, signal);
  for (const artist of res.data) {
    map.set(artist.id, artist);
  }
  return map;
}

async function fetchAlbumsMap(ids: string[], signal?: AbortSignal): Promise<Map<string, z.infer<typeof CatalogAlbumSchema>>> {
  const unique = dedupe(ids);
  const map = new Map<string, z.infer<typeof CatalogAlbumSchema>>();
  if (unique.length === 0) return map;
  const settled = await Promise.allSettled(
    unique.map((id) => apiGet(`/albums/${encodeURIComponent(id)}`, CatalogAlbumSchema, signal)),
  );
  for (const result of settled) {
    if (result.status === 'fulfilled') {
      map.set(result.value.id, result.value);
    }
  }
  return map;
}

function joinArtistNames(artistIds: string[], artists: Map<string, z.infer<typeof CatalogArtistSchema>>): string {
  const names = artistIds.map((id) => artists.get(id)?.name).filter((n): n is string => Boolean(n));
  return names.length > 0 ? names.join(' · ') : 'Unknown artist';
}

function parseYear(releaseDate: string): number | undefined {
  const year = Number.parseInt(releaseDate.slice(0, 4), 10);
  return Number.isFinite(year) ? year : undefined;
}

export async function fetchSavedAlbumsPage(cursor?: string, signal?: AbortSignal): Promise<LibraryPageSlice<SavedAlbumItem>> {
  const qs = new URLSearchParams({ limit: String(PAGE_LIMIT) });
  if (cursor) qs.set('cursor', cursor);
  const page = await apiGet(`/me/library/albums?${qs.toString()}`, SavedAlbumsPageSchema, signal);
  if (page.data.length === 0) {
    return { items: [], nextCursor: page.pagination.nextCursor, hasMore: page.pagination.hasMore };
  }

  const albums = await fetchAlbumsMap(
    page.data.map((entry) => entry.albumId),
    signal,
  );
  const artistIds = Array.from(albums.values()).flatMap((a) => a.artistIds);
  const artists = await fetchArtistsMap(artistIds, signal).catch(() => new Map<string, z.infer<typeof CatalogArtistSchema>>());

  const items: SavedAlbumItem[] = [];
  for (const entry of page.data) {
    const album = albums.get(entry.albumId);
    if (!album) continue;
    const summary = AlbumSummarySchema.parse({
      kind: 'album',
      id: album.id,
      title: album.title,
      artistName: joinArtistNames(album.artistIds, artists),
      year: parseYear(album.releaseDate),
      explicit: album.explicit ?? false,
      coverUrl: album.coverUrl ?? null,
      art: album.art ?? artIndex(album.id),
      albumType: album.albumType,
    });
    items.push({ ...summary, savedAt: entry.savedAt });
  }

  return { items, nextCursor: page.pagination.nextCursor, hasMore: page.pagination.hasMore };
}

export async function fetchLikedTracksPage(cursor?: string, signal?: AbortSignal): Promise<LibraryPageSlice<LikedTrackItem>> {
  const qs = new URLSearchParams({ limit: String(PAGE_LIMIT) });
  if (cursor) qs.set('cursor', cursor);
  const page = await apiGet(`/me/library/tracks?${qs.toString()}`, SavedTracksPageSchema, signal);
  if (page.data.length === 0) {
    return { items: [], nextCursor: page.pagination.nextCursor, hasMore: page.pagination.hasMore };
  }

  const trackIds = dedupe(page.data.map((entry) => entry.trackId));
  const tracksRes = await apiGet(`/tracks?ids=${trackIds.map(encodeURIComponent).join(',')}`, CatalogTracksListSchema, signal);
  const tracksById = new Map(tracksRes.data.map((t) => [t.id, t]));

  const artistIds = tracksRes.data.flatMap((t) => t.artistIds);
  const albumIds = tracksRes.data.map((t) => t.albumId);
  const [artists, albums] = await Promise.all([
    fetchArtistsMap(artistIds, signal).catch(() => new Map<string, z.infer<typeof CatalogArtistSchema>>()),
    fetchAlbumsMap(albumIds, signal).catch(() => new Map<string, z.infer<typeof CatalogAlbumSchema>>()),
  ]);

  const items: LikedTrackItem[] = [];
  for (const entry of page.data) {
    const t = tracksById.get(entry.trackId);
    if (!t) continue;
    const album = albums.get(t.albumId);
    const parsed = TrackSchema.parse({
      id: t.id,
      title: t.title,
      artistName: joinArtistNames(t.artistIds, artists),
      albumId: t.albumId,
      albumTitle: album?.title ?? 'Unknown album',
      durationSec: Math.round(t.durationMs / 1000),
      explicit: t.explicit,
      coverUrl: t.coverUrl ?? album?.coverUrl ?? null,
      art: t.art ?? album?.art ?? artIndex(t.albumId),
      liked: true,
      available: t.status === 'READY',
    });
    items.push({ ...parsed, savedAt: entry.savedAt });
  }

  return { items, nextCursor: page.pagination.nextCursor, hasMore: page.pagination.hasMore };
}

export async function fetchFollowedArtistsPage(
  userId: string,
  cursor?: string,
  signal?: AbortSignal,
): Promise<LibraryPageSlice<FollowedArtistItem>> {
  const qs = new URLSearchParams({ type: 'artist', limit: String(PAGE_LIMIT) });
  if (cursor) qs.set('cursor', cursor);
  const page = await apiGet(`/users/${encodeURIComponent(userId)}/following?${qs.toString()}`, FollowedArtistsPageSchema, signal);
  if (page.data.length === 0) {
    return { items: [], nextCursor: page.pagination.nextCursor, hasMore: page.pagination.hasMore };
  }

  const artists = await fetchArtistsMap(
    page.data.map((entry) => entry.artistId),
    signal,
  );

  const items: FollowedArtistItem[] = [];
  for (const entry of page.data) {
    const artist = artists.get(entry.artistId);
    if (!artist) continue;
    const summary = ArtistSummarySchema.parse({
      kind: 'artist',
      id: artist.id,
      name: artist.name,
      listeners: artist.listeners,
      coverUrl: artist.coverUrl ?? null,
      art: artist.art ?? artIndex(artist.id),
    });
    items.push({ ...summary, followedAt: entry.followedAt });
  }

  return { items, nextCursor: page.pagination.nextCursor, hasMore: page.pagination.hasMore };
}

export const savedAlbumsInfiniteQuery = (enabled: boolean) =>
  infiniteQueryOptions({
    queryKey: ['library', 'albums'],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => fetchSavedAlbumsPage(pageParam, signal),
    getNextPageParam: (lastPage) => (lastPage.hasMore ? (lastPage.nextCursor ?? undefined) : undefined),
    enabled,
    staleTime: 60_000,
  });

export const likedTracksInfiniteQuery = (enabled: boolean) =>
  infiniteQueryOptions({
    queryKey: ['library', 'tracks'],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => fetchLikedTracksPage(pageParam, signal),
    getNextPageParam: (lastPage) => (lastPage.hasMore ? (lastPage.nextCursor ?? undefined) : undefined),
    enabled,
    staleTime: 60_000,
  });

export const followedArtistsInfiniteQuery = (userId: string | undefined) =>
  infiniteQueryOptions({
    queryKey: ['library', 'artists', userId ?? ''],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam, signal }) => fetchFollowedArtistsPage(userId!, pageParam, signal),
    getNextPageParam: (lastPage) => (lastPage.hasMore ? (lastPage.nextCursor ?? undefined) : undefined),
    enabled: Boolean(userId),
    staleTime: 60_000,
  });

export const useSavedAlbums = (enabled = true) => useInfiniteQuery(savedAlbumsInfiniteQuery(enabled));
export const useLikedTracks = (enabled = true) => useInfiniteQuery(likedTracksInfiniteQuery(enabled));
export const useFollowedArtists = (userId: string | undefined) => useInfiniteQuery(followedArtistsInfiniteQuery(userId));

/** PUT/DELETE /me/library/albums/{id} — idempotent, 204 (Library Service). */
export function setAlbumSaved(albumId: string, saved: boolean): Promise<null> {
  return apiRequest(`/me/library/albums/${encodeURIComponent(albumId)}`, z.null(), { method: saved ? 'PUT' : 'DELETE' });
}

export const savedAlbumsContainsQuery = (ids: string[]) =>
  queryOptions({
    queryKey: ['library', 'contains', 'albums', ids],
    queryFn: async ({ signal }) => {
      const res = await apiGet(`/me/library/albums/contains?ids=${ids.map(encodeURIComponent).join(',')}`, ContainsSchema, signal);
      return new Set(res.data);
    },
    enabled: ids.length > 0,
    staleTime: 60_000,
  });

export const useSavedAlbumsContains = (ids: string[]) => useQuery(savedAlbumsContainsQuery(ids.slice(0, 100)));

/** PUT/DELETE /artists/{id}/follow — idempotent, 204 (Social Service). */
export function setArtistFollowed(artistId: string, followed: boolean): Promise<null> {
  return apiRequest(`/artists/${encodeURIComponent(artistId)}/follow`, z.null(), { method: followed ? 'PUT' : 'DELETE' });
}
