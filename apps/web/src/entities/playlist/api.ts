import { queryOptions, useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { apiGet } from '@/shared/api/client';

import { PlaylistSummarySchema } from './model';

/** GET /me/playlists — the listener's playlists (Playlist Service, EPIC-013). */
export const MyPlaylistsSchema = z.object({ data: z.array(PlaylistSummarySchema) });

export const myPlaylistsQuery = queryOptions({
  queryKey: ['playlists', 'mine'],
  queryFn: async ({ signal }) => (await apiGet('/me/playlists', MyPlaylistsSchema, signal)).data,
  staleTime: 60_000,
});

export const useMyPlaylists = () => useQuery(myPlaylistsQuery);
