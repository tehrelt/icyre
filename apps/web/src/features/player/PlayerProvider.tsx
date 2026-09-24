import { useEffect, type ReactNode } from 'react';

import { env } from '@/shared/config/env';

import { resolveStreamUrl } from './api/streamSource';
import { bindPlayer } from './controller/playerController';
import { HtmlAudioEngine } from './engine/HtmlAudioEngine';
import { SimulatedAudioEngine } from './engine/SimulatedAudioEngine';
import { usePlayerStore } from './model/playerStore';

/**
 * Owns the single audio engine for the app. Mounted above the router's
 * outlet, so navigating between pages never interrupts playback.
 */
export function PlayerProvider({ children }: { children: ReactNode }) {
  useEffect(() => {
    const engine = env.useMocks ? new SimulatedAudioEngine() : new HtmlAudioEngine();
    return bindPlayer(usePlayerStore, engine, resolveStreamUrl);
  }, []);

  return children;
}
