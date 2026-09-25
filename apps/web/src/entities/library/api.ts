import { queryOptions, useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { apiGet } from '@/shared/api/client';

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
