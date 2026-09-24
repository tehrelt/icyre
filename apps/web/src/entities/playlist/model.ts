import { z } from 'zod';

export const PlaylistSummarySchema = z.object({
  kind: z.literal('playlist'),
  id: z.string(),
  title: z.string(),
  owner: z.string().optional(),
  trackCount: z.number().int().nonnegative().optional(),
  coverUrl: z.string().nullable().default(null),
  art: z.number().int().default(0),
});

export type PlaylistSummary = z.infer<typeof PlaylistSummarySchema>;
