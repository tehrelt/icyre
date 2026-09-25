import type { StoreApi, UseBoundStore } from 'zustand';

import type { AudioEngine } from '../engine/AudioEngine';
import type { PlayerStore } from '../model/playerStore';

export type PlaybackEventType = 'started' | 'finished' | 'skipped';

/** One report for POST /playback/events (Playback Service). */
export interface PlaybackReport {
  type: PlaybackEventType;
  playbackId: string;
  trackId: string;
  source?: string;
  durationMs: number;
  listenedMs: number;
}

export type PlaybackReporter = (report: PlaybackReport) => Promise<unknown>;

/** Largest gap between two time updates still counted as listening; bigger jumps are seeks. */
const MAX_TICK_SEC = 2;

interface Current {
  loadId: number;
  playbackId: string;
  trackId: string;
  source?: string;
  durationMs: number;
  listenedSec: number;
  lastPosition: number | null;
  started: boolean;
  done: boolean;
}

/**
 * Reports playback telemetry: `started` when a track actually starts
 * playing, `finished` when it ends, `skipped` when another track replaces
 * it. `listenedMs` counts time really played — seeking does not add to it.
 * Reporting is best effort: a failed report never affects playback.
 */
export function bindPlaybackReporter(store: UseBoundStore<StoreApi<PlayerStore>>, engine: AudioEngine, report: PlaybackReporter): () => void {
  let current: Current | null = null;

  const send = (type: PlaybackEventType, c: Current) => {
    void report({
      type,
      playbackId: c.playbackId,
      trackId: c.trackId,
      source: c.source,
      durationMs: c.durationMs,
      listenedMs: Math.min(c.durationMs, Math.round(c.listenedSec * 1000)),
    }).catch(() => undefined);
  };

  const begin = (s: PlayerStore) => {
    if (current?.started && !current.done) send('skipped', current);
    current = s.currentTrack
      ? {
          loadId: s.loadId,
          playbackId: crypto.randomUUID(),
          trackId: s.currentTrack.id,
          source: s.source ? `${s.source.kind}:${s.source.id}` : undefined,
          durationMs: Math.max(1, Math.round(s.currentTrack.durationSec * 1000)),
          listenedSec: 0,
          lastPosition: null,
          started: false,
          done: false,
        }
      : null;
  };

  const unsubscribeEngine = engine.subscribe((e) => {
    const c = current;
    if (!c || c.loadId !== store.getState().loadId) return;
    switch (e.type) {
      case 'playing':
        if (!c.started) {
          c.started = true;
          send('started', c);
        }
        break;
      case 'time': {
        const delta = c.lastPosition == null ? 0 : e.position - c.lastPosition;
        if (c.started && store.getState().isPlaying && delta > 0 && delta <= MAX_TICK_SEC) c.listenedSec += delta;
        c.lastPosition = e.position;
        break;
      }
      case 'ended':
        if (c.started && !c.done) {
          c.done = true;
          send('finished', c);
        }
        break;
    }
  });

  const unsubscribeStore = store.subscribe((s, prev) => {
    if (s.loadId !== prev.loadId) begin(s);
  });

  return () => {
    unsubscribeStore();
    unsubscribeEngine();
  };
}
