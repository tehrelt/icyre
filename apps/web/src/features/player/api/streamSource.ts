import { z } from 'zod';

import type { Track } from '@/entities/track/model';
import { apiRequest } from '@/shared/api/client';

import type { StreamGrant } from '../controller/playerController';

/** POST /stream/authorize (Stream Authorization Service). */
const StreamGrantSchema = z.object({
  trackId: z.string(),
  quality: z.string(),
  url: z.string(),
  expiresAt: z.string(),
});

/**
 * Asks Stream Authorization for a short-lived signed URL. The response holds
 * only the URL; the audio itself is fetched from the media origin/CDN.
 * The server picks the best available variant (256 kbps by default).
 */
export async function resolveStreamUrl(track: Track): Promise<StreamGrant> {
  const grant = await apiRequest('/stream/authorize', StreamGrantSchema, { method: 'POST', body: { trackId: track.id } });
  return { url: grant.url, expiresAt: Date.parse(grant.expiresAt) };
}
