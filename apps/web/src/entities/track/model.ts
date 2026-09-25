import { z } from 'zod';

/** A playable track as returned by page-oriented (BFF) endpoints. */
export const TrackSchema = z.object({
  id: z.string(),
  title: z.string(),
  artistName: z.string(),
  albumId: z.string(),
  albumTitle: z.string(),
  durationSec: z.number().nonnegative(),
  explicit: z.boolean().default(false),
  coverUrl: z.string().nullable().default(null),
  /** Generative fallback cover index (Design System Artwork) when there is no cover. */
  art: z.number().int().default(0),
  liked: z.boolean().default(false),
  badge: z.enum(['new', 'trending']).optional(),
  available: z.boolean().default(true),
  /** Lifetime play count (album pages). */
  plays: z.number().int().nonnegative().optional(),
});

export type Track = z.infer<typeof TrackSchema>;

export const TrackListSchema = z.object({ data: z.array(TrackSchema) });
