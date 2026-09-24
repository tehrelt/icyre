import { useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';

import { collectionTracksQuery, type CollectionKind } from '@/entities/track/api';
import type { Track } from '@/entities/track/model';

import { isSourceActive, usePlayerStore, type PlaybackSource } from './model/playerStore';

/**
 * Play/pause for a collection card: toggles when the collection is already
 * loaded, otherwise fetches its tracks and starts it from the top.
 */
export function usePlayCollection() {
  const queryClient = useQueryClient();
  return useCallback(
    async (kind: CollectionKind, id: string) => {
      const state = usePlayerStore.getState();
      if (isSourceActive(state, kind, id)) {
        state.togglePlay();
        return;
      }
      const tracks = await queryClient.fetchQuery(collectionTracksQuery(kind, id));
      usePlayerStore.getState().playQueue(tracks, 0, { kind, id });
    },
    [queryClient],
  );
}

/** Play state of a collection for cards: 'playing' | 'paused' | null. */
export function useCollectionPlayState(kind: CollectionKind, id: string): 'playing' | 'paused' | null {
  return usePlayerStore((s) => (isSourceActive(s, kind, id) ? (s.isPlaying ? 'playing' : 'paused') : null));
}

/** Plays a list of tracks starting at `track` (track rows), or toggles it if current. */
export function usePlayTrackList() {
  return useCallback((tracks: Track[], track: Track) => {
    const state = usePlayerStore.getState();
    if (state.currentTrack?.id === track.id) {
      state.togglePlay();
      return;
    }
    state.playQueue(tracks, Math.max(0, tracks.findIndex((t) => t.id === track.id)), null);
  }, []);
}

/**
 * Playback for a page that already holds its tracks (album, playlist):
 * play/pause the whole collection, shuffle it, or start at a given track.
 */
export function useSourcePlayback(source: PlaybackSource, tracks: Track[]) {
  const active = usePlayerStore((s) => isSourceActive(s, source.kind, source.id));
  const isPlaying = usePlayerStore((s) => s.isPlaying);
  const state: 'playing' | 'paused' | null = active ? (isPlaying ? 'playing' : 'paused') : null;

  const playAll = useCallback(() => {
    const s = usePlayerStore.getState();
    if (isSourceActive(s, source.kind, source.id)) s.togglePlay();
    else s.playQueue(tracks, 0, source);
  }, [source, tracks]);

  const shuffleAll = useCallback(() => {
    const s = usePlayerStore.getState();
    if (!s.shuffle) s.toggleShuffle();
    const playable = tracks.map((t, i) => (t.available ? i : -1)).filter((i) => i >= 0);
    const start = playable[Math.floor(Math.random() * playable.length)] ?? 0;
    usePlayerStore.getState().playQueue(tracks, start, source);
  }, [source, tracks]);

  const playTrack = useCallback(
    (track: Track) => {
      const s = usePlayerStore.getState();
      if (s.currentTrack?.id === track.id) s.togglePlay();
      else s.playQueue(tracks, Math.max(0, tracks.findIndex((t) => t.id === track.id)), source);
    },
    [source, tracks],
  );

  return { state, playAll, shuffleAll, playTrack };
}
