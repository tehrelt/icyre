import type { ComponentProps } from 'react';

import { TrackRow } from '@/shared/ui';

import { useTrackLike } from '../useTrackLike';

type Props = Omit<ComponentProps<typeof TrackRow>, 'liked' | 'onLike'> & {
  track: { id: string; liked?: boolean };
  /** Saved state known from another source (e.g. a Library lookup). */
  known?: boolean;
};

/** A TrackRow whose heart saves the track to Liked tracks. */
export function LikeableTrackRow({ track, known, ...row }: Props) {
  const like = useTrackLike(track, known);
  return <TrackRow {...row} liked={like.liked} onLike={() => void like.toggle()} />;
}
