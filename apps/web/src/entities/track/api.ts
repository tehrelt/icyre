import { queryOptions } from '@tanstack/react-query';

import { z } from 'zod';

import { apiGet } from '@/shared/api/client';

import { TrackListSchema, TrackSchema, type Track } from './model';

export type CollectionKind = 'album' | 'playlist' | 'artist';

/** Album and playlist tracks come from their BFF pages, already enriched for playback. */
const PageTracksSchema = z.object({ tracks: z.array(TrackSchema) });

const fetchers: Record<CollectionKind, (id: string, signal: AbortSignal) => Promise<Track[]>> = {
  album: async (id, signal) => (await apiGet(`/pages/albums/${encodeURIComponent(id)}`, PageTracksSchema, signal)).tracks,
  playlist: async (id, signal) => (await apiGet(`/pages/playlists/${encodeURIComponent(id)}`, PageTracksSchema, signal)).tracks,
  artist: async (id, signal) => (await apiGet(`/artists/${encodeURIComponent(id)}/top-tracks`, TrackListSchema, signal)).data,
};

/** Tracks of a collection in play order — what "Play" on a card enqueues. */
export const collectionTracksQuery = (kind: CollectionKind, id: string) =>
  queryOptions({
    queryKey: ['tracks', kind, id],
    queryFn: ({ signal }) => fetchers[kind](id, signal),
    staleTime: 5 * 60_000,
  });
