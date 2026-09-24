import { useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';

import { collectionTracksQuery, type CollectionKind } from '@/entities/track/api';
import type { Track } from '@/entities/track/model';

import { isSourceActive, usePlayerStore } from './model/playerStore';

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
