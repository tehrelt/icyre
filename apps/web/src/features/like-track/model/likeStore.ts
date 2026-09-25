import { create } from 'zustand';

/**
 * The listener's like/unlike actions in this tab, by track ID. They win over
 * whatever a page payload said (`track.liked`), so every row, the player bar
 * and search agree the moment the heart is clicked.
 */
interface LikeState {
  overrides: Record<string, boolean>;
  set: (trackId: string, liked: boolean) => void;
}

export const useLikeStore = create<LikeState>((set) => ({
  overrides: {},
  set: (trackId, liked) => set((s) => ({ overrides: { ...s.overrides, [trackId]: liked } })),
}));
