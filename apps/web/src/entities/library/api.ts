import { queryOptions, useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { PlaylistSummarySchema } from '@/entities/playlist/model';
import { apiGet } from '@/shared/api/client';

/** Sidebar data: library counters and the listener's playlists. */
export const LibrarySummarySchema = z.object({
  savedCount: z.number().int().nonnegative(),
  likedTracksCount: z.number().int().nonnegative(),
  playlists: z.array(PlaylistSummarySchema),
});

export type LibrarySummary = z.infer<typeof LibrarySummarySchema>;

export const librarySummaryQuery = queryOptions({
  queryKey: ['library', 'summary'],
  queryFn: ({ signal }) => apiGet('/library/summary', LibrarySummarySchema, signal),
  staleTime: 60_000,
});

export const useLibrarySummary = () => useQuery(librarySummaryQuery);
