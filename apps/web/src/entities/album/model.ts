import { z } from 'zod';

/** Release type, as in the Catalog API (albumType). */
export const AlbumTypeSchema = z.enum(['ALBUM', 'EP', 'SINGLE', 'COMPILATION']);
export type AlbumType = z.infer<typeof AlbumTypeSchema>;

export const albumTypeLabel: Record<AlbumType, string> = {
  ALBUM: 'Album',
  EP: 'EP',
  SINGLE: 'Single',
  COMPILATION: 'Compilation',
};

export const AlbumSummarySchema = z.object({
  kind: z.literal('album'),
  id: z.string(),
  title: z.string(),
  artistName: z.string(),
  year: z.number().int().optional(),
  explicit: z.boolean().default(false),
  coverUrl: z.string().nullable().default(null),
  art: z.number().int().default(0),
  badge: z.enum(['new', 'featured']).optional(),
  albumType: AlbumTypeSchema.optional(),
});

export type AlbumSummary = z.infer<typeof AlbumSummarySchema>;
