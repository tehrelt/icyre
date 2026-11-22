import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useState } from 'react';

import { setAlbumSaved } from '@/entities/library/api';

import { useAlbumSaveStore } from './model/albumSaveStore';

export function useAlbumSave(albumId: string | null, known?: boolean) {
  const override = useAlbumSaveStore((s) => (albumId ? s.overrides[albumId] : undefined));
  const setOverride = useAlbumSaveStore((s) => s.set);
  const queryClient = useQueryClient();
  const [pending, setPending] = useState(false);
  const saved = override ?? known ?? false;

  const toggle = useCallback(async () => {
    if (!albumId || pending) return;
    const next = !saved;
    setOverride(albumId, next);
    setPending(true);
    try {
      await setAlbumSaved(albumId, next);
      void queryClient.invalidateQueries({ queryKey: ['library'] });
    } catch {
      setOverride(albumId, !next);
    } finally {
      setPending(false);
    }
  }, [albumId, pending, saved, setOverride, queryClient]);

  return { saved, toggle, pending };
}
