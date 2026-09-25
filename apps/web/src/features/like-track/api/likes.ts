import { queryOptions, useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { apiGet, apiRequest } from '@/shared/api/client';

/** PUT/DELETE /me/library/tracks/{id} — idempotent, 204 (Library Service). */
export function setTrackLiked(trackId: string, liked: boolean): Promise<null> {
  return apiRequest(`/me/library/tracks/${encodeURIComponent(trackId)}`, z.null(), { method: liked ? 'PUT' : 'DELETE' });
}

const ContainsSchema = z.object({ data: z.array(z.string()) });

/**
 * Which of these tracks the listener has saved — for lists that do not come
 * from the BFF (search results). Anonymous visitors get a 401: no likes.
 */
export const savedTracksQuery = (ids: string[]) =>
  queryOptions({
    queryKey: ['library', 'contains', 'tracks', ids],
    queryFn: async ({ signal }) => {
      const res = await apiGet(`/me/library/tracks/contains?ids=${ids.map(encodeURIComponent).join(',')}`, ContainsSchema, signal);
      return new Set(res.data);
    },
    enabled: ids.length > 0,
    staleTime: 60_000,
  });

export const useSavedTracks = (ids: string[]) => useQuery(savedTracksQuery(ids.slice(0, 100)));
