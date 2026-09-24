// Ported from the ICYRE Design System (components/src/media.tsx).
import type { MouseEvent } from 'react';

import { cx } from '@/shared/lib/cx';
import { formatTime } from '@/shared/lib/formatTime';

import { Artwork, Icon, IconButton } from './core';
import { Badge, ExplicitMark, Skeleton, type BadgeTone } from './display';
import { PlaybackState } from './player';

export type CardState = 'default' | 'hover' | 'selected' | 'playing' | 'disabled' | 'loading';

/* ---------- MediaCard (base primitive) ---------- */
export interface MediaCardProps {
  title: string;
  subtitle?: string;
  cover?: string | null;
  art?: number;
  shape?: 'square' | 'circle';
  kind?: 'album' | 'artist' | 'playlist' | 'media';
  badge?: BadgeTone;
  explicit?: boolean;
  state?: CardState;
  stacked?: boolean;
  width?: number | string;
  onOpen?: () => void;
  onPlay?: () => void;
}

export function MediaCard({
  title,
  subtitle,
  cover,
  art = 0,
  shape = 'square',
  kind = 'media',
  badge,
  explicit,
  state = 'default',
  stacked,
  width,
  onOpen,
  onPlay,
}: MediaCardProps) {
  if (state === 'loading') {
    return (
      <div className={cx('ic-card', `ic-card-${kind}`, 'is-loading')} style={{ width }} aria-busy="true">
        <div className={cx('ic-card-media', shape === 'circle' && 'is-circle')}>
          <Skeleton shape={shape === 'circle' ? 'circle' : 'rect'} width="100%" height="100%" radius="md" />
        </div>
        <div className="ic-card-meta">
          <Skeleton width="72%" />
          <Skeleton width="48%" />
        </div>
      </div>
    );
  }
  const disabled = state === 'disabled';
  const playing = state === 'playing';
  return (
    <div
      className={cx('ic-card', `ic-card-${kind}`, state !== 'default' && `is-${state}`, stacked && 'is-stacked')}
      style={{ width }}
      aria-disabled={disabled || undefined}
      aria-current={playing ? 'true' : undefined}
    >
      <div className={cx('ic-card-media', shape === 'circle' && 'is-circle')}>
        {stacked && (
          <>
            <span className="ic-stack ic-stack-2" aria-hidden="true" />
            <span className="ic-stack ic-stack-1" aria-hidden="true" />
          </>
        )}
        <Artwork src={cover} art={art} shape={shape} alt="" />
        {playing && (
          <span className="ic-card-eq">
            <PlaybackState size="sm" />
          </span>
        )}
        {!disabled && onPlay && (
          <span className="ic-card-play">
            <IconButton icon={playing ? 'pause' : 'play'} variant="play" size="md" label={`${playing ? 'Pause' : 'Play'} ${title}`} onClick={onPlay} />
          </span>
        )}
      </div>
      <button type="button" className="ic-card-meta ic-card-link" disabled={disabled} onClick={onOpen}>
        <span className="ic-card-title">{title}</span>
        <span className="ic-card-sub">
          {explicit && <ExplicitMark />}
          {subtitle}
        </span>
      </button>
      {badge && (
        <span className="ic-card-badge">
          <Badge tone={badge} />
        </span>
      )}
    </div>
  );
}

export interface AlbumCardProps extends Omit<MediaCardProps, 'kind' | 'shape' | 'subtitle'> {
  artist: string;
  year?: number;
}

export function AlbumCard({ artist, year, ...rest }: AlbumCardProps) {
  return <MediaCard {...rest} kind="album" subtitle={year ? `${artist} · ${year}` : artist} />;
}

export interface ArtistCardProps extends Omit<MediaCardProps, 'kind' | 'shape' | 'title' | 'subtitle'> {
  name: string;
  listeners?: string;
}

export function ArtistCard({ name, listeners, ...rest }: ArtistCardProps) {
  return <MediaCard {...rest} kind="artist" shape="circle" title={name} subtitle={listeners ? `${listeners} listeners` : 'Artist'} />;
}

export interface PlaylistCardProps extends Omit<MediaCardProps, 'kind' | 'shape' | 'subtitle' | 'stacked'> {
  owner?: string;
  trackCount?: number;
}

export function PlaylistCard({ owner, trackCount, ...rest }: PlaylistCardProps) {
  const sub = [owner, trackCount != null ? `${trackCount} tracks` : null].filter(Boolean).join(' · ');
  return <MediaCard {...rest} kind="playlist" stacked subtitle={sub || 'Playlist'} />;
}

/* ---------- FeaturedCard (collectible) ---------- */
export interface FeaturedCardProps {
  title: string;
  artist: string;
  category?: string;
  serial?: string;
  cover?: string | null;
  art?: number;
  meta?: string[];
  badges?: BadgeTone[];
  state?: 'default' | 'hover' | 'playing' | 'loading';
  width?: number | string;
  onPlay?: () => void;
  onOpen?: () => void;
}

export function FeaturedCard({
  title,
  artist,
  category = 'Album',
  serial,
  cover,
  art = 0,
  meta = [],
  badges = ['featured'],
  state = 'default',
  width = 280,
  onPlay,
  onOpen,
}: FeaturedCardProps) {
  if (state === 'loading') {
    return (
      <div className="ic-featured is-loading" style={{ width }} aria-busy="true">
        <div className="ic-featured-frame">
          <div className="ic-featured-strip">
            <Skeleton width={96} />
          </div>
          <div className="ic-featured-media">
            <Skeleton shape="rect" width="100%" height="100%" radius="md" />
          </div>
          <div className="ic-featured-body">
            <Skeleton width="70%" height={18} />
            <Skeleton width="44%" />
          </div>
        </div>
      </div>
    );
  }
  const playing = state === 'playing';
  return (
    <article className={cx('ic-featured', state !== 'default' && `is-${state}`)} style={{ width }}>
      <div className="ic-featured-frame">
        <div className="ic-featured-strip">
          <span className="ic-overline">Featured · {category}</span>
          {serial && <span className="ic-meta ic-featured-serial">{serial}</span>}
        </div>
        <div className="ic-featured-media">
          <Artwork src={cover} art={art} alt="" />
          <span className="ic-featured-glint" aria-hidden="true" />
          {onPlay && (
            <span className="ic-featured-play">
              <IconButton icon={playing ? 'pause' : 'play'} variant="play" size="lg" label={`${playing ? 'Pause' : 'Play'} ${title}`} onClick={onPlay} />
            </span>
          )}
        </div>
        <button type="button" className="ic-featured-body ic-card-link" onClick={onOpen}>
          <span className="ic-featured-title">{title}</span>
          <span className="ic-featured-artist">{artist}</span>
        </button>
        <div className="ic-featured-foot">
          <span className="ic-meta ic-featured-meta">
            {playing && <PlaybackState size="sm" />}
            {meta.join(' · ')}
          </span>
          <span className="ic-featured-badges">
            {badges.map((b) => (
              <Badge key={b} tone={b} />
            ))}
          </span>
        </div>
      </div>
    </article>
  );
}

/* ---------- TrackRow ---------- */
export type TrackRowState = 'default' | 'hover' | 'selected' | 'playing' | 'paused' | 'unavailable' | 'loading';

export interface TrackRowProps {
  index: number;
  title: string;
  artist: string;
  album?: string;
  /** Seconds. */
  duration: number;
  art?: number;
  cover?: string | null;
  badge?: BadgeTone;
  explicit?: boolean;
  liked?: boolean;
  state?: TrackRowState;
  showCover?: boolean;
  onPlay?: () => void;
  onLike?: () => void;
  onMore?: () => void;
  onSelect?: () => void;
}

const stop = (fn?: () => void) => (e: MouseEvent) => {
  e.stopPropagation();
  fn?.();
};

export function TrackRow({
  index,
  title,
  artist,
  album,
  duration,
  art = 0,
  cover,
  badge,
  explicit,
  liked,
  state = 'default',
  showCover = true,
  onPlay,
  onLike,
  onMore,
  onSelect,
}: TrackRowProps) {
  if (state === 'loading') {
    return (
      <div className={cx('ic-track', 'is-loading', !showCover && 'no-cover')} aria-busy="true">
        <span className="ic-track-index">
          <Skeleton width={12} />
        </span>
        {showCover && <Skeleton shape="rect" width={40} height={40} radius="xs" />}
        <span className="ic-track-main">
          <Skeleton width="46%" />
          <Skeleton width="28%" />
        </span>
        <span className="ic-track-album">
          <Skeleton width="60%" />
        </span>
        <span />
        <span />
        <Skeleton width={32} />
        <span />
      </div>
    );
  }
  const unavailable = state === 'unavailable';
  const current = state === 'playing' || state === 'paused';
  return (
    <div
      role="row"
      className={cx('ic-track', state !== 'default' && `is-${state}`, current && 'is-current', !showCover && 'no-cover')}
      aria-selected={state === 'selected' || undefined}
      aria-disabled={unavailable || undefined}
      aria-current={current ? 'true' : undefined}
      onClick={onSelect}
      onDoubleClick={unavailable ? undefined : onPlay}
    >
      <span className="ic-track-index" role="cell">
        {current ? (
          <span className="ic-track-eq">
            <PlaybackState size="sm" state={state === 'paused' ? 'paused' : 'playing'} />
          </span>
        ) : (
          <span className="ic-track-num">{index}</span>
        )}
        {!unavailable && (
          <button type="button" className="ic-track-play" aria-label={`${state === 'playing' ? 'Pause' : 'Play'} ${title}`} onClick={stop(onPlay)}>
            <Icon name={state === 'playing' ? 'pause' : 'play'} size={16} filled />
          </button>
        )}
      </span>
      {showCover && <Artwork src={cover} art={art} className="ic-track-cover" />}
      <span className="ic-track-main" role="cell">
        <span className="ic-track-title">{title}</span>
        <span className="ic-track-artist">
          {explicit && <ExplicitMark />}
          {artist}
        </span>
      </span>
      <span className="ic-track-album" role="cell">
        {album}
      </span>
      <span className="ic-track-badge">{unavailable ? <Badge tone="neutral">Unavailable</Badge> : badge ? <Badge tone={badge} /> : null}</span>
      <span className={cx('ic-track-like', liked && 'is-liked')}>
        {!unavailable && (
          <IconButton
            icon="heart"
            size="sm"
            variant={liked ? 'active' : 'neutral'}
            pressed={!!liked}
            label={liked ? 'Remove from Liked' : 'Add to Liked'}
            onClick={stop(onLike)}
          />
        )}
      </span>
      <span className="ic-time ic-time-tertiary ic-track-dur" role="cell">
        {formatTime(duration)}
      </span>
      <span className="ic-track-more">
        <IconButton icon="more" size="sm" label={`More options for ${title}`} onClick={stop(onMore)} />
      </span>
    </div>
  );
}
