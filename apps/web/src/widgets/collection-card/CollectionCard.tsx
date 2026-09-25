import { useNavigate } from 'react-router';

import { albumTypeLabel } from '@/entities/album/model';
import type { Collection } from '@/entities/collection/model';
import { useCollectionPlayState, usePlayCollection } from '@/features/player';
import { routes } from '@/shared/config/routes';
import { AlbumCard, ArtistCard, PlaylistCard } from '@/shared/ui';

export interface CollectionCardProps {
  item: Collection;
  /** Album subtitle shows the release type ("EP · 2023") instead of the artist — artist pages and "More by". */
  albumSubtitle?: 'artist' | 'type';
}

/** Renders the Design System card for an album, playlist or artist and wires play/open. */
export function CollectionCard({ item, albumSubtitle = 'artist' }: CollectionCardProps) {
  const navigate = useNavigate();
  const play = usePlayCollection();
  const playState = useCollectionPlayState(item.kind, item.id);
  const state = playState === 'playing' ? 'playing' : 'default';
  const onPlay = () => void play(item.kind, item.id);

  switch (item.kind) {
    case 'album':
      return (
        <AlbumCard
          title={item.title}
          artist={albumSubtitle === 'type' && item.albumType ? albumTypeLabel[item.albumType] : item.artistName}
          year={item.year}
          art={item.art}
          cover={item.coverUrl}
          explicit={item.explicit}
          badge={item.badge === 'new' ? 'new' : undefined}
          state={state}
          onPlay={onPlay}
          onOpen={() => navigate(routes.album(item.id))}
        />
      );
    case 'playlist':
      return (
        <PlaylistCard
          title={item.title}
          owner={item.owner}
          trackCount={item.trackCount}
          art={item.art}
          cover={item.coverUrl}
          state={state}
          onPlay={onPlay}
          onOpen={() => navigate(routes.playlist(item.id))}
        />
      );
    case 'artist':
      return (
        <ArtistCard
          name={item.name}
          listeners={item.listeners}
          art={item.art}
          cover={item.coverUrl}
          state={state}
          onPlay={onPlay}
          onOpen={() => navigate(routes.artist(item.id))}
        />
      );
  }
}
