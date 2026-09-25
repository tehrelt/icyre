import { create } from 'zustand';
import { createJSONStorage, persist, type StateStorage } from 'zustand/middleware';

const MAX_RECENT = 8;

interface RecentSearchesState {
  items: string[];
  add: (query: string) => void;
  remove: (query: string) => void;
  clear: () => void;
}

/** localStorage that degrades to memory when storage is blocked (private mode, sandboxed frames). */
const safeStorage: StateStorage = (() => {
  const memory = new Map<string, string>();
  const ls = (): Storage | null => {
    try {
      return typeof window !== 'undefined' ? window.localStorage : null;
    } catch {
      return null;
    }
  };
  return {
    getItem: (k) => {
      try {
        return ls()?.getItem(k) ?? memory.get(k) ?? null;
      } catch {
        return memory.get(k) ?? null;
      }
    },
    setItem: (k, v) => {
      memory.set(k, v);
      try {
        ls()?.setItem(k, v);
      } catch {
        /* keep in memory */
      }
    },
    removeItem: (k) => {
      memory.delete(k);
      try {
        ls()?.removeItem(k);
      } catch {
        /* ignore */
      }
    },
  };
})();

const normalize = (q: string) => q.trim().replace(/\s+/g, ' ').toLowerCase();

/** Recent searches of this device (client state, not server state). */
export const useRecentSearches = create<RecentSearchesState>()(
  persist(
    (set, get) => ({
      items: [],
      add: (query) => {
        const q = normalize(query);
        if (q.length < 2) return;
        set({ items: [q, ...get().items.filter((i) => i !== q)].slice(0, MAX_RECENT) });
      },
      remove: (query) => set({ items: get().items.filter((i) => i !== query) }),
      clear: () => set({ items: [] }),
    }),
    { name: 'icyre.recent-searches', storage: createJSONStorage(() => safeStorage), version: 1 },
  ),
);
