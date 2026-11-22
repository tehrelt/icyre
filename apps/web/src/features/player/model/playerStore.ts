import { create } from 'zustand';

import type { CollectionKind } from '@/entities/track/api';
import type { Track } from '@/entities/track/model';
import type { RepeatMode } from '@/shared/ui';

/** What is being played, so cards and rows can show their "playing" state. */
export interface PlaybackSource {
  kind: CollectionKind | 'library';
  id: string;
}

export interface PlayerState {
  queue: Track[];
  queueIndex: number;
  currentTrack: Track | null;
  source: PlaybackSource | null;
  /** The listener's intent: should audio be playing. */
  isPlaying: boolean;
  /** Audio is stalled waiting for data. */
  buffering: boolean;
  /** Seconds. */
  position: number;
  duration: number;
  buffered: number;
  /** 0–100 */
  volume: number;
  muted: boolean;
  shuffle: boolean;
  repeatMode: RepeatMode;
  /** Confirmed active audio output device id ('default' for system default). */
  outputDeviceId: string;
  /** Output device currently being switched to, or null when idle. */
  pendingOutputDeviceId: string | null;
  /** Error from the last failed output device switch, if any. */
  outputDeviceError: string | null;
  error: string | null;
  /** Bumped whenever the current track must be (re)loaded by the engine. */
  loadId: number;
  /** Pending seek for the engine; bumped per request. */
  seekRequest: { at: number; id: number } | null;
  /** Pending output device switch for the controller; bumped per request. */
  outputDeviceRequest: { deviceId: string; id: number } | null;
}

export interface PlayerActions {
  playQueue: (tracks: Track[], startIndex?: number, source?: PlaybackSource | null) => void;
  playAt: (index: number) => void;
  removeFromQueue: (index: number) => void;
  moveInQueue: (fromIndex: number, toIndex: number) => void;
  clearUpcoming: () => void;
  clearQueue: () => void;
  enqueueNext: (track: Track) => void;
  addToQueue: (track: Track) => void;
  play: () => void;
  pause: () => void;
  togglePlay: () => void;
  next: () => void;
  previous: () => void;
  seek: (sec: number) => void;
  setVolume: (volume: number) => void;
  setMuted: (muted: boolean) => void;
  toggleShuffle: () => void;
  cycleRepeat: () => void;
  setOutputDevice: (deviceId: string) => void;
  // Feedback from the audio engine (called by the controller only).
  engineTime: (position: number) => void;
  engineDuration: (duration: number) => void;
  engineBuffered: (buffered: number) => void;
  engineBuffering: (buffering: boolean) => void;
  engineEnded: () => void;
  engineError: (message: string) => void;
  engineOutputDevice: (deviceId: string, requestId?: number) => void;
  engineOutputDeviceError: (message: string, requestId?: number, fallbackDeviceId?: string) => void;
}

export type PlayerStore = PlayerState & PlayerActions;

const RESTART_THRESHOLD_SEC = 3;

export const initialPlayerState: PlayerState = {
  queue: [],
  queueIndex: -1,
  currentTrack: null,
  source: null,
  isPlaying: false,
  buffering: false,
  position: 0,
  duration: 0,
  buffered: 0,
  volume: 70,
  muted: false,
  shuffle: false,
  repeatMode: 'off',
  outputDeviceId: 'default',
  pendingOutputDeviceId: null,
  outputDeviceError: null,
  error: null,
  loadId: 0,
  seekRequest: null,
  outputDeviceRequest: null,
};

const playable = (t: Track | undefined): t is Track => !!t && t.available;

/** Next playable index after `from` in direction `step`, or -1. */
function findPlayable(queue: Track[], from: number, step: 1 | -1, wrap: boolean): number {
  for (let i = 1; i <= queue.length; i++) {
    let idx = from + step * i;
    if (wrap) idx = (idx + queue.length) % queue.length;
    else if (idx < 0 || idx >= queue.length) return -1;
    if (playable(queue[idx])) return idx;
  }
  return -1;
}

export const usePlayerStore = create<PlayerStore>()((set, get) => {
  /** Makes `index` the current track and asks the engine to load it. */
  const startAt = (index: number, isPlaying = true) => {
    const { queue, loadId } = get();
    const track = queue[index] ?? null;
    set({
      queueIndex: index,
      currentTrack: track,
      isPlaying: isPlaying && !!track,
      position: 0,
      duration: track?.durationSec ?? 0,
      buffered: 0,
      buffering: false,
      error: null,
      loadId: loadId + 1,
      seekRequest: null,
    });
  };

  const advance = (auto: boolean) => {
    const { queue, queueIndex, shuffle, repeatMode } = get();
    if (queue.length === 0) return;
    if (auto && repeatMode === 'one') return startAt(queueIndex);

    let idx: number;
    if (shuffle && queue.length > 1) {
      const candidates = queue.map((_, i) => i).filter((i) => i !== queueIndex && playable(queue[i]));
      idx = candidates.length ? candidates[Math.floor(Math.random() * candidates.length)]! : -1;
    } else {
      idx = findPlayable(queue, queueIndex, 1, repeatMode === 'all');
    }
    if (idx === -1) {
      // End of queue: stop at the start of the last track.
      set({ isPlaying: false, position: 0, seekRequest: { at: 0, id: Date.now() } });
      return;
    }
    startAt(idx);
  };

  return {
    ...initialPlayerState,

    playQueue: (tracks, startIndex = 0, source = null) => {
      if (tracks.length === 0) return;
      set({ queue: tracks, source });
      const first = playable(tracks[startIndex]) ? startIndex : findPlayable(tracks, startIndex, 1, true);
      if (first !== -1) startAt(first);
    },

    playAt: (index) => {
      const { queue, queueIndex } = get();
      if (index < 0 || index >= queue.length) return;
      if (index === queueIndex) {
        get().togglePlay();
        return;
      }
      if (playable(queue[index])) startAt(index, true);
    },

    removeFromQueue: (index) => {
      const { queue, queueIndex, isPlaying, repeatMode, loadId } = get();
      if (index < 0 || index >= queue.length) return;
      const nextQueue = queue.filter((_, i) => i !== index);
      if (nextQueue.length === 0) {
        set({
          queue: [],
          queueIndex: -1,
          currentTrack: null,
          source: null,
          isPlaying: false,
          position: 0,
          duration: 0,
          buffered: 0,
          buffering: false,
          error: null,
          seekRequest: null,
          loadId: loadId + 1,
        });
        return;
      }
      if (index < queueIndex) {
        set({ queue: nextQueue, queueIndex: queueIndex - 1 });
        return;
      }
      if (index > queueIndex) {
        set({ queue: nextQueue });
        return;
      }
      const nextIdx = findPlayable(nextQueue, index - 1, 1, repeatMode === 'all');
      if (nextIdx !== -1) {
        set({ queue: nextQueue });
        startAt(nextIdx, isPlaying);
        return;
      }
      set({
        queue: nextQueue,
        queueIndex: -1,
        currentTrack: null,
        isPlaying: false,
        position: 0,
        duration: 0,
        buffered: 0,
        buffering: false,
        error: null,
        seekRequest: null,
        loadId: loadId + 1,
      });
    },

    moveInQueue: (fromIndex, toIndex) => {
      const { queue, queueIndex } = get();
      if (fromIndex < 0 || fromIndex >= queue.length || toIndex < 0 || toIndex >= queue.length || fromIndex === toIndex) return;
      const nextQueue = [...queue];
      const [moved] = nextQueue.splice(fromIndex, 1);
      nextQueue.splice(toIndex, 0, moved!);
      let nextIndex = queueIndex;
      if (queueIndex === fromIndex) nextIndex = toIndex;
      else if (fromIndex < queueIndex && toIndex >= queueIndex) nextIndex = queueIndex - 1;
      else if (fromIndex > queueIndex && toIndex <= queueIndex) nextIndex = queueIndex + 1;
      set({ queue: nextQueue, queueIndex: nextIndex });
    },

    clearUpcoming: () => {
      const { queue, queueIndex } = get();
      if (queueIndex < 0 || queueIndex >= queue.length - 1) return;
      set({ queue: queue.slice(0, queueIndex + 1) });
    },

    clearQueue: () => {
      const { loadId } = get();
      set({
        queue: [],
        queueIndex: -1,
        currentTrack: null,
        source: null,
        isPlaying: false,
        position: 0,
        duration: 0,
        buffered: 0,
        buffering: false,
        error: null,
        seekRequest: null,
        loadId: loadId + 1,
      });
    },

    enqueueNext: (track) => {
      if (!playable(track)) return;
      const { queue, queueIndex } = get();
      if (queue.length === 0 || queueIndex < 0) {
        get().playQueue([track], 0, null);
        return;
      }
      set({ queue: [...queue.slice(0, queueIndex + 1), track, ...queue.slice(queueIndex + 1)] });
    },

    addToQueue: (track) => {
      if (!playable(track)) return;
      const { queue, queueIndex } = get();
      if (queue.length === 0 || queueIndex < 0) {
        get().playQueue([track], 0, null);
        return;
      }
      set({ queue: [...queue, track] });
    },

    play: () => {
      if (get().currentTrack) set({ isPlaying: true, error: null });
    },
    pause: () => set({ isPlaying: false }),
    togglePlay: () => (get().isPlaying ? get().pause() : get().play()),

    next: () => advance(false),
    previous: () => {
      const { position, queue, queueIndex, repeatMode } = get();
      if (position > RESTART_THRESHOLD_SEC || queue.length === 0) return get().seek(0);
      const idx = findPlayable(queue, queueIndex, -1, repeatMode === 'all');
      if (idx === -1) get().seek(0);
      else startAt(idx);
    },

    seek: (sec) => {
      const at = Math.max(0, Math.min(sec, get().duration || sec));
      set({ position: at, seekRequest: { at, id: (get().seekRequest?.id ?? 0) + 1 } });
    },

    setVolume: (volume) => set({ volume: Math.max(0, Math.min(100, Math.round(volume))), muted: false }),
    setMuted: (muted) => set({ muted }),
    setOutputDevice: (deviceId) => {
      const target = deviceId || 'default';
      const { outputDeviceId, pendingOutputDeviceId, outputDeviceRequest } = get();
      if (target === pendingOutputDeviceId) return;
      if (target === outputDeviceId && !pendingOutputDeviceId) {
        set({ outputDeviceError: null });
        return;
      }
      set({
        pendingOutputDeviceId: target,
        outputDeviceError: null,
        outputDeviceRequest: { deviceId: target, id: (outputDeviceRequest?.id ?? 0) + 1 },
      });
    },
    toggleShuffle: () => set({ shuffle: !get().shuffle }),
    cycleRepeat: () => {
      const order: RepeatMode[] = ['off', 'all', 'one'];
      set({ repeatMode: order[(order.indexOf(get().repeatMode) + 1) % order.length]! });
    },

    engineTime: (position) => set({ position }),
    engineDuration: (duration) => {
      if (Number.isFinite(duration) && duration > 0) set({ duration });
    },
    engineBuffered: (buffered) => set({ buffered }),
    engineBuffering: (buffering) => set({ buffering }),
    engineEnded: () => advance(true),
    engineError: (message) => set({ error: message, isPlaying: false, buffering: false }),
    engineOutputDevice: (deviceId, requestId) =>
      set((s) => ({
        outputDeviceId: deviceId || 'default',
        pendingOutputDeviceId: requestId == null || s.outputDeviceRequest?.id === requestId ? null : s.pendingOutputDeviceId,
        outputDeviceError: null,
      })),
    engineOutputDeviceError: (message, requestId, fallbackDeviceId) =>
      set((s) => ({
        outputDeviceId: fallbackDeviceId !== undefined ? fallbackDeviceId : s.outputDeviceId,
        pendingOutputDeviceId: requestId == null || s.outputDeviceRequest?.id === requestId ? null : s.pendingOutputDeviceId,
        outputDeviceError: message,
      })),
  };
});

/** True when the given collection is the one currently loaded. */
export const isSourceActive = (s: PlayerState, kind: PlaybackSource['kind'], id: string) => s.source?.kind === kind && s.source.id === id;
