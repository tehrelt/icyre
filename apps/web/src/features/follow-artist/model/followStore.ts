import { create } from 'zustand';

interface FollowState {
  overrides: Record<string, boolean>;
  set: (artistId: string, followed: boolean) => void;
}

export const useFollowStore = create<FollowState>((set) => ({
  overrides: {},
  set: (artistId, followed) => set((s) => ({ overrides: { ...s.overrides, [artistId]: followed } })),
}));
