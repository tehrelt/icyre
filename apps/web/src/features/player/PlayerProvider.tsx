import { useEffect, type ReactNode } from 'react';

import { env } from '@/shared/config/env';

import { reportPlayback } from './api/playbackEvents';
import { resolveStreamUrl } from './api/streamSource';
import { bindPlaybackReporter } from './controller/playbackReporter';
import { bindPlayer } from './controller/playerController';
import { bindMediaSession, bindPlayerHotkeys, bindPlayerPreferences } from './controller/playerExperience';
import { HtmlAudioEngine } from './engine/HtmlAudioEngine';
import { SimulatedAudioEngine } from './engine/SimulatedAudioEngine';
import { usePlayerStore } from './model/playerStore';

/**
 * Owns the single audio engine for the app. Mounted above the router's
 * outlet, so navigating between pages never interrupts playback.
 */
export function PlayerProvider({ children }: { children: ReactNode }) {
  useEffect(() => {
    const unbindPreferences = bindPlayerPreferences(usePlayerStore);
    const engine = env.useMocks ? new SimulatedAudioEngine() : new HtmlAudioEngine();
    const unbindReporter = bindPlaybackReporter(usePlayerStore, engine, reportPlayback);
    const unbindPlayer = bindPlayer(usePlayerStore, engine, resolveStreamUrl);
    const unbindHotkeys = bindPlayerHotkeys(usePlayerStore);
    const unbindMediaSession = bindMediaSession(usePlayerStore);
    return () => {
      unbindMediaSession();
      unbindHotkeys();
      unbindReporter();
      unbindPlayer();
      unbindPreferences();
    };
  }, []);

  return children;
}
