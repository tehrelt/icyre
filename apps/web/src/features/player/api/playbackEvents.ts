import { z } from 'zod';

import { apiRequest } from '@/shared/api/client';

import type { PlaybackReport } from '../controller/playbackReporter';

/** POST /playback/events — 202 without a body. Anonymous listeners get 401 and are not tracked. */
export const reportPlayback = (report: PlaybackReport) => apiRequest('/playback/events', z.null(), { method: 'POST', body: report });
