import { z } from 'zod';

export const ArtistSummarySchema = z.object({
  kind: z.literal('artist'),
  id: z.string(),
  name: z.string(),
  /** Pre-formatted listener count, e.g. "1.2M". */
  listeners: z.string().optional(),
  coverUrl: z.string().nullable().default(null),
  art: z.number().int().default(0),
});

export type ArtistSummary = z.infer<typeof ArtistSummarySchema>;
