import { create } from 'zustand';

interface AlbumSaveState {
  overrides: Record<string, boolean>;
  set: (albumId: string, saved: boolean) => void;
}

export const useAlbumSaveStore = create<AlbumSaveState>((set) => ({
  overrides: {},
  set: (albumId, saved) => set((s) => ({ overrides: { ...s.overrides, [albumId]: saved } })),
}));
