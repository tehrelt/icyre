import { queryOptions, useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { AlbumSummarySchema, AlbumTypeSchema } from '@/entities/album/model';
import { TrackSchema } from '@/entities/track/model';
import { apiGet } from '@/shared/api/client';

/** GET /api/v1/pages/albums/{id} — page-oriented BFF contract for the Album screen (canvas Album.dc.html). */
export const AlbumPageSchema = z.object({
  album: z.object({
    id: z.string(),
    title: z.string(),
    albumType: AlbumTypeSchema,
    serial: z.string().optional(),
    year: z.number().int(),
    releaseDate: z.string(), // YYYY-MM-DD
    trackCount: z.number().int(),
    durationSec: z.number(),
    tags: z.array(z.string()),
    hiRes: z.boolean(),
    copyright: z.string().optional(),
    coverUrl: z.string().nullable().default(null),
    art: z.number().int().default(0),
  }),
  artist: z.object({
    id: z.string(),
    name: z.string(),
    avatarUrl: z.string().nullable().default(null),
    art: z.number().int().default(0),
  }),
  tracks: z.array(TrackSchema),
  moreByArtist: z.array(AlbumSummarySchema),
});

export type AlbumPage = z.infer<typeof AlbumPageSchema>;

export const albumPageQuery = (id: string) =>
  queryOptions({
    queryKey: ['pages', 'album', id],
    queryFn: ({ signal }) => apiGet(`/pages/albums/${encodeURIComponent(id)}`, AlbumPageSchema, signal),
    staleTime: 5 * 60_000,
  });

export const useAlbumPage = (id: string) => useQuery(albumPageQuery(id));
