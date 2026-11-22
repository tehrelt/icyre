import { queryOptions, useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { apiGet } from '@/shared/api/client';

import { PlaylistSummarySchema } from './model';

/** A playlist as the Playlist Service returns it, mapped to the card shape the UI uses. */
const PlaylistItemSchema = z
  .object({
    id: z.string(),
    title: z.string(),
    trackCount: z.number().int().nonnegative(),
    art: z.number().int().optional(),
    owner: z.string().optional(),
    updatedAt: z.string().optional(),
  })
  .transform((p) => ({
    ...PlaylistSummarySchema.parse({ kind: 'playlist', id: p.id, title: p.title, owner: p.owner, trackCount: p.trackCount, art: p.art ?? 0 }),
    updatedAt: p.updatedAt,
  }));

export type MyPlaylist = z.infer<typeof PlaylistItemSchema>;

/** GET /me/playlists — the listener's playlists (Playlist Service, EPIC-013). */
export const MyPlaylistsSchema = z.object({ data: z.array(PlaylistItemSchema) });

export const myPlaylistsQuery = queryOptions({
  queryKey: ['playlists', 'mine'],
  queryFn: async ({ signal }) => (await apiGet('/me/playlists', MyPlaylistsSchema, signal)).data,
  staleTime: 60_000,
});

export const useMyPlaylists = () => useQuery(myPlaylistsQuery);
