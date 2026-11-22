import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useState } from 'react';

import { setArtistFollowed } from '@/entities/library/api';

import { useFollowStore } from './model/followStore';

export function useArtistFollow(artistId: string | null, known?: boolean) {
  const override = useFollowStore((s) => (artistId ? s.overrides[artistId] : undefined));
  const setOverride = useFollowStore((s) => s.set);
  const queryClient = useQueryClient();
  const [pending, setPending] = useState(false);
  const followed = override ?? known ?? false;

  const toggle = useCallback(async () => {
    if (!artistId || pending) return;
    const next = !followed;
    setOverride(artistId, next);
    setPending(true);
    try {
      await setArtistFollowed(artistId, next);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['library'] }),
        queryClient.invalidateQueries({ queryKey: ['pages', 'home'] }),
      ]);
    } catch {
      setOverride(artistId, !next);
    } finally {
      setPending(false);
    }
  }, [artistId, pending, followed, setOverride, queryClient]);

  return { followed, toggle, pending };
}
