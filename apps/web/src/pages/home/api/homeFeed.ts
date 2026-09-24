import { queryOptions, useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { AlbumSummarySchema } from '@/entities/album/model';
import { ArtistSummarySchema } from '@/entities/artist/model';
import { PlaylistSummarySchema } from '@/entities/playlist/model';
import { TrackSchema } from '@/entities/track/model';
import { apiGet } from '@/shared/api/client';

const CollectionSchema = z.discriminatedUnion('kind', [AlbumSummarySchema, PlaylistSummarySchema, ArtistSummarySchema]);
export type Collection = z.infer<typeof CollectionSchema>;

/**
 * GET /api/v1/pages/home — page-oriented BFF contract (specs/services/bff.md)
 * for the Home screen of the product canvas.
 */
export const HomeFeedSchema = z.object({
  recentlyPlayed: z.array(CollectionSchema),
  albumOfTheWeek: z
    .object({
      album: AlbumSummarySchema,
      serial: z.string(),
      description: z.string(),
      label: z.string(),
      releaseDate: z.string(), // YYYY-MM-DD
      format: z.string(),
      catalogNumber: z.string(),
      trackCount: z.number().int(),
      durationSec: z.number(),
      tags: z.array(z.string()),
      hiRes: z.boolean(),
    })
    .nullable(),
  recommended: z.object({ kicker: z.string().optional(), items: z.array(CollectionSchema) }),
  newReleases: z.object({ kicker: z.string().optional(), items: z.array(AlbumSummarySchema) }),
  trending: z.object({ today: z.array(TrackSchema), week: z.array(TrackSchema) }),
  madeForYou: z.object({
    featured: PlaylistSummarySchema.extend({
      serial: z.string(),
      artistsLine: z.string(),
      durationSec: z.number(),
    }).nullable(),
    playlists: z.array(PlaylistSummarySchema),
  }),
  followedArtists: z.array(ArtistSummarySchema),
});

export type HomeFeed = z.infer<typeof HomeFeedSchema>;
export type AlbumOfTheWeek = NonNullable<HomeFeed['albumOfTheWeek']>;
export type DailyMix = NonNullable<HomeFeed['madeForYou']['featured']>;

export const homeFeedQuery = queryOptions({
  queryKey: ['pages', 'home'],
  queryFn: ({ signal }) => apiGet('/pages/home', HomeFeedSchema, signal),
  staleTime: 60_000,
});

export const useHomeFeed = () => useQuery(homeFeedQuery);
