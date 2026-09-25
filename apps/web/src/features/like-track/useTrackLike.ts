import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useState } from 'react';

import { setTrackLiked } from './api/likes';
import { useLikeStore } from './model/likeStore';

interface Likeable {
  id: string;
  liked?: boolean;
}

/**
 * Like state and toggle for one track. The heart flips at once (optimistic)
 * and flips back if Library rejects the change, e.g. for a signed-out visitor.
 */
export function useTrackLike(track: Likeable | null, known?: boolean) {
  const override = useLikeStore((s) => (track ? s.overrides[track.id] : undefined));
  const setOverride = useLikeStore((s) => s.set);
  const queryClient = useQueryClient();
  const [pending, setPending] = useState(false);
  const liked = override ?? known ?? track?.liked ?? false;

  const toggle = useCallback(async () => {
    if (!track || pending) return;
    const next = !liked;
    setOverride(track.id, next);
    setPending(true);
    try {
      await setTrackLiked(track.id, next);
      void queryClient.invalidateQueries({ queryKey: ['library'] });
    } catch {
      setOverride(track.id, !next);
    } finally {
      setPending(false);
    }
  }, [track, pending, liked, setOverride, queryClient]);

  return { liked, toggle, pending };
}
