import { Link } from 'react-router';

import { cx } from '@/shared/lib/cx';
import { Artwork, IconButton, PlaybackState } from '@/shared/ui';

import styles from './QuickTile.module.css';

export interface QuickTileProps {
  title: string;
  subtitle: string;
  to: string;
  art?: number;
  coverUrl?: string | null;
  /** 'playing' | 'paused' when this collection is loaded in the player. */
  playState?: 'playing' | 'paused' | null;
  onPlay?: () => void;
}

/** Media / Quick tile from the product canvas (CQuickTile): the "Recently played" grid. */
export function QuickTile({ title, subtitle, to, art = 0, coverUrl, playState, onPlay }: QuickTileProps) {
  const playing = playState === 'playing';
  return (
    <div className={cx(styles.tile, playState && styles.current)}>
      <Link to={to} aria-label={`Open ${title}`} className={styles.art} tabIndex={-1}>
        <Artwork art={art} src={coverUrl} />
      </Link>
      <Link to={to} className={styles.text}>
        <span className={styles.title}>{title}</span>
        <span className={styles.subtitle}>{subtitle}</span>
      </Link>
      {playing && (
        <span className={styles.eq}>
          <PlaybackState state="playing" size="sm" />
        </span>
      )}
      {onPlay && (
        <span className={styles.play}>
          <IconButton icon={playing ? 'pause' : 'play'} label={`${playing ? 'Pause' : 'Play'} ${title}`} variant="play" size="sm" onClick={onPlay} />
        </span>
      )}
    </div>
  );
}
