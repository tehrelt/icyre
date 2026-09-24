import { z } from 'zod';

import type { Track } from '@/entities/track/model';
import { apiGet } from '@/shared/api/client';

const StreamGrantSchema = z.object({
  trackId: z.string(),
  url: z.string().nullable(),
  expiresAt: z.string().nullable(),
});

/**
 * Asks Playback / Stream Authorization for a signed CDN URL. The response
 * holds only the URL; the audio itself is fetched from the CDN.
 */
export async function resolveStreamUrl(track: Track): Promise<string | null> {
  const grant = await apiGet(`/playback/tracks/${encodeURIComponent(track.id)}/stream`, StreamGrantSchema);
  return grant.url;
}
