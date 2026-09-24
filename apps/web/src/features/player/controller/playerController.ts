import type { StoreApi, UseBoundStore } from 'zustand';

import type { Track } from '@/entities/track/model';

import type { AudioEngine } from '../engine/AudioEngine';
import type { PlayerStore } from '../model/playerStore';

/** Resolves a short-lived signed stream URL for a track (Stream Authorization). */
export type StreamResolver = (track: Track) => Promise<string | null>;

/**
 * Binds the player store to an audio engine:
 *
 *   Player UI → store (intent) → controller → engine → HTMLAudioElement
 *   engine events → controller → store (feedback) → UI
 *
 * Returns a disposer. Created once at the app root so playback survives navigation.
 */
export function bindPlayer(store: UseBoundStore<StoreApi<PlayerStore>>, engine: AudioEngine, resolveStream: StreamResolver): () => void {
  let disposed = false;

  const unsubscribeEngine = engine.subscribe((e) => {
    const s = store.getState();
    switch (e.type) {
      case 'time':
        s.engineTime(e.position);
        if (s.buffering) s.engineBuffering(false);
        break;
      case 'duration':
        s.engineDuration(e.duration);
        break;
      case 'buffered':
        s.engineBuffered(e.buffered);
        break;
      case 'waiting':
        s.engineBuffering(true);
        break;
      case 'playing':
        s.engineBuffering(false);
        break;
      case 'ended':
        s.engineEnded();
        break;
      case 'error':
        s.engineError(e.message);
        break;
    }
  });

  const load = async (track: Track, loadId: number) => {
    store.getState().engineBuffering(true);
    let src: string | null;
    try {
      src = await resolveStream(track);
    } catch {
      if (!disposed && store.getState().loadId === loadId) store.getState().engineError('Could not authorize the stream');
      return;
    }
    // A newer track may have been requested while the URL was resolving.
    if (disposed || store.getState().loadId !== loadId) return;
    engine.load(src, track.durationSec);
    store.getState().engineBuffering(false);
    if (store.getState().isPlaying) await engine.play();
  };

  const initial = store.getState();
  engine.setVolume(initial.volume / 100, initial.muted);

  const unsubscribeStore = store.subscribe((s, prev) => {
    if (s.loadId !== prev.loadId && s.currentTrack) {
      void load(s.currentTrack, s.loadId);
      return; // load() applies isPlaying itself
    }
    if (s.isPlaying !== prev.isPlaying) {
      if (s.isPlaying) void engine.play();
      else engine.pause();
    }
    if (s.seekRequest && s.seekRequest !== prev.seekRequest) engine.seek(s.seekRequest.at);
    if (s.volume !== prev.volume || s.muted !== prev.muted) engine.setVolume(s.volume / 100, s.muted);
  });

  return () => {
    disposed = true;
    unsubscribeStore();
    unsubscribeEngine();
    engine.destroy();
  };
}
