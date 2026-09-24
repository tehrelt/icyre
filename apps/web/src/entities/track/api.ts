import { queryOptions } from '@tanstack/react-query';

import { apiGet } from '@/shared/api/client';

import { TrackListSchema } from './model';

export type CollectionKind = 'album' | 'playlist' | 'artist';

const collectionPath: Record<CollectionKind, (id: string) => string> = {
  album: (id) => `/albums/${id}/tracks`,
  playlist: (id) => `/playlists/${id}/tracks`,
  artist: (id) => `/artists/${id}/top-tracks`,
};

/** Tracks of a collection in play order — what "Play" on a card enqueues. */
export const collectionTracksQuery = (kind: CollectionKind, id: string) =>
  queryOptions({
    queryKey: ['tracks', kind, id],
    queryFn: async ({ signal }) => (await apiGet(collectionPath[kind](id), TrackListSchema, signal)).data,
    staleTime: 5 * 60_000,
  });
