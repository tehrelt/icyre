import { keepPreviousData, queryOptions, useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { AlbumSummarySchema } from '@/entities/album/model';
import { ArtistSummarySchema } from '@/entities/artist/model';
import { CollectionSchema } from '@/entities/collection/model';
import { PlaylistSummarySchema } from '@/entities/playlist/model';
import { TrackSchema } from '@/entities/track/model';
import { apiGet } from '@/shared/api/client';

export const SEARCH_TYPES = ['all', 'tracks', 'artists', 'albums', 'playlists'] as const;
export type SearchType = (typeof SEARCH_TYPES)[number];

/** GET /api/v1/search?q=&type=&limit= — Search Service (specs/services/search.md). */
export const SearchResultSchema = z.object({
  query: z.string(),
  type: z.enum(SEARCH_TYPES),
  counts: z.object({
    tracks: z.number().int(),
    artists: z.number().int(),
    albums: z.number().int(),
    playlists: z.number().int(),
  }),
  topResult: z
    .object({
      item: CollectionSchema,
      verified: z.boolean().default(false),
      monthlyListeners: z.string().optional(),
    })
    .nullable(),
  tracks: z.array(TrackSchema),
  artists: z.array(ArtistSummarySchema),
  albums: z.array(AlbumSummarySchema),
  playlists: z.array(PlaylistSummarySchema),
  didYouMean: z.string().nullable(),
});

export type SearchResult = z.infer<typeof SearchResultSchema>;

export const searchQuery = (q: string, type: SearchType) =>
  queryOptions({
    queryKey: ['search', type, q],
    queryFn: ({ signal }) => {
      const params = new URLSearchParams({ q, type, limit: type === 'all' ? '6' : '50' });
      return apiGet(`/search?${params}`, SearchResultSchema, signal);
    },
    staleTime: 60_000,
  });

export const useSearch = (q: string, type: SearchType) =>
  useQuery({
    ...searchQuery(q, type),
    enabled: q.length > 0,
    // Keep showing the previous results of the same tab while the next query loads.
    placeholderData: (prev, prevQuery) => (prevQuery?.queryKey[1] === type ? keepPreviousData(prev) : undefined),
  });

/** GET /api/v1/pages/search — browse content before a query (canvas SearchEmpty). */
export const SearchBrowseSchema = z.object({
  genres: z.array(z.object({ slug: z.string(), label: z.string(), releaseCount: z.number().int(), art: z.number().int().default(0) })),
  moods: z.array(z.object({ id: z.string(), label: z.string() })),
  collections: z.object({ kicker: z.string().optional(), items: z.array(PlaylistSummarySchema) }),
});

export type SearchBrowse = z.infer<typeof SearchBrowseSchema>;

export const searchBrowseQuery = queryOptions({
  queryKey: ['pages', 'search'],
  queryFn: ({ signal }) => apiGet('/pages/search', SearchBrowseSchema, signal),
  staleTime: 10 * 60_000,
});

export const useSearchBrowse = () => useQuery(searchBrowseQuery);
