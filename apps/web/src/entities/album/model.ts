import { z } from 'zod';

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
});

export type AlbumSummary = z.infer<typeof AlbumSummarySchema>;
