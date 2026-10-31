import type { StoreApi, UseBoundStore } from 'zustand';

import type { Track } from '@/entities/track/model';

import type { PlayerStore } from '../model/playerStore';

const PREFERENCES_KEY = 'icyre.player.preferences';
const SEEK_STEP_SECONDS = 5;

type PlayerStoreHook = UseBoundStore<StoreApi<PlayerStore>>;

interface PlayerPreferences {
  version: 1;
  volume: number;
  muted: boolean;
}

function browserStorage(): Storage | null {
  try {
    return typeof window === 'undefined' ? null : window.localStorage;
  } catch {
    return null;
  }
}

function parsePreferences(value: string | null): PlayerPreferences | null {
  if (!value) return null;
  try {
    const parsed = JSON.parse(value) as Partial<PlayerPreferences>;
    if (parsed.version !== 1 || typeof parsed.volume !== 'number' || !Number.isFinite(parsed.volume) || typeof parsed.muted !== 'boolean') {
      return null;
    }
    return { version: 1, volume: Math.max(0, Math.min(100, Math.round(parsed.volume))), muted: parsed.muted };
  } catch {
    return null;
  }
}

/** Restores volume before the audio engine starts, then persists changes. */
export function bindPlayerPreferences(store: PlayerStoreHook, storage: Storage | null = browserStorage()): () => void {
  if (!storage) return () => undefined;

  try {
    const saved = parsePreferences(storage.getItem(PREFERENCES_KEY));
    if (saved) store.setState({ volume: saved.volume, muted: saved.muted });
  } catch {
    // Storage may be blocked in private or embedded contexts.
  }

  return store.subscribe((state, previous) => {
    if (state.volume === previous.volume && state.muted === previous.muted) return;
    const value: PlayerPreferences = { version: 1, volume: state.volume, muted: state.muted };
    try {
      storage.setItem(PREFERENCES_KEY, JSON.stringify(value));
    } catch {
      // Playback must keep working when persistence fails.
    }
  });
}

function isInteractiveTarget(target: EventTarget | null): boolean {
  return target instanceof Element && !!target.closest('input, textarea, select, button, a, [contenteditable="true"], [role="slider"]');
}

/** Global shortcuts: Space/K play, arrows seek, M mutes. */
export function bindPlayerHotkeys(store: PlayerStoreHook, target: EventTarget = window): () => void {
  const onKeyDown = (event: Event) => {
    if (!(event instanceof KeyboardEvent) || event.defaultPrevented || event.repeat || event.ctrlKey || event.metaKey || event.altKey) return;
    if (isInteractiveTarget(event.target)) return;

    const state = store.getState();
    switch (event.key.toLowerCase()) {
      case ' ':
      case 'k':
        if (!state.currentTrack) return;
        event.preventDefault();
        state.togglePlay();
        break;
      case 'arrowleft':
        if (!state.currentTrack) return;
        event.preventDefault();
        state.seek(state.position - SEEK_STEP_SECONDS);
        break;
      case 'arrowright':
        if (!state.currentTrack) return;
        event.preventDefault();
        state.seek(state.position + SEEK_STEP_SECONDS);
        break;
      case 'm':
        event.preventDefault();
        state.setMuted(!state.muted);
        break;
    }
  };

  target.addEventListener('keydown', onKeyDown);
  return () => target.removeEventListener('keydown', onKeyDown);
}

function defaultMetadata(track: Track): MediaMetadata {
  let artwork: MediaImage[] = [];
  if (track.coverUrl) {
    try {
      const baseUrl = typeof document === 'undefined' ? 'https://localhost/' : document.baseURI;
      const url = new URL(track.coverUrl, baseUrl);
      if (['http:', 'https:', 'blob:', 'data:'].includes(url.protocol)) artwork = [{ src: url.href }];
    } catch {
      // Invalid artwork must not interrupt playback.
    }
  }
  return new MediaMetadata({
    title: track.title,
    artist: track.artistName,
    album: track.albumTitle,
    artwork,
  });
}

function browserMediaSession(): MediaSession | null {
  return typeof navigator !== 'undefined' && 'mediaSession' in navigator ? navigator.mediaSession : null;
}

/** Connects lock-screen and hardware media controls to the player. */
export function bindMediaSession(
  store: PlayerStoreHook,
  session: MediaSession | null = browserMediaSession(),
  createMetadata: (track: Track) => MediaMetadata = defaultMetadata,
): () => void {
  if (!session) return () => undefined;

  const actions: Array<[MediaSessionAction, MediaSessionActionHandler]> = [
    ['play', () => store.getState().play()],
    ['pause', () => store.getState().pause()],
    ['nexttrack', () => store.getState().next()],
    ['previoustrack', () => store.getState().previous()],
    ['seekbackward', (details) => {
      const state = store.getState();
      state.seek(state.position - (details.seekOffset ?? SEEK_STEP_SECONDS));
    }],
    ['seekforward', (details) => {
      const state = store.getState();
      state.seek(state.position + (details.seekOffset ?? SEEK_STEP_SECONDS));
    }],
    ['seekto', (details) => {
      if (details.seekTime !== undefined) store.getState().seek(details.seekTime);
    }],
  ];

  for (const [action, handler] of actions) {
    try {
      session.setActionHandler(action, handler);
    } catch {
      // Browsers expose different action subsets.
    }
  }

  let previousTrack: Track | null = null;
  let previousSecond = -1;
  const sync = (state: PlayerStore) => {
    if (state.currentTrack !== previousTrack) {
      previousTrack = state.currentTrack;
      previousSecond = -1;
      try {
        session.metadata = state.currentTrack ? createMetadata(state.currentTrack) : null;
      } catch {
        session.metadata = null;
      }
    }
    session.playbackState = state.currentTrack ? (state.isPlaying ? 'playing' : 'paused') : 'none';

    const second = Math.floor(state.position);
    if (state.currentTrack && state.duration > 0 && second !== previousSecond) {
      try {
        session.setPositionState({ duration: state.duration, playbackRate: 1, position: Math.min(state.position, state.duration) });
        previousSecond = second;
      } catch {
        // Some browsers reject position updates during loading.
      }
    }
  };

  sync(store.getState());
  const unsubscribe = store.subscribe(sync);

  return () => {
    unsubscribe();
    for (const [action] of actions) {
      try {
        session.setActionHandler(action, null);
      } catch {
        // Ignore unsupported actions during cleanup.
      }
    }
    session.metadata = null;
    session.playbackState = 'none';
  };
}
