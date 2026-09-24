import { z } from 'zod';

import { AlbumSummarySchema } from '@/entities/album/model';
import { ArtistSummarySchema } from '@/entities/artist/model';
import { PlaylistSummarySchema } from '@/entities/playlist/model';

/** Any card-shaped catalog item a page can show in a grid: album, playlist or artist. */
export const CollectionSchema = z.discriminatedUnion('kind', [AlbumSummarySchema, PlaylistSummarySchema, ArtistSummarySchema]);
export type Collection = z.infer<typeof CollectionSchema>;
