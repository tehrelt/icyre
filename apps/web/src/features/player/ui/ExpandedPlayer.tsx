import { useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Link } from 'react-router';
import { useShallow } from 'zustand/react/shallow';

import { useTrackLike } from '@/features/like-track';
import { routes } from '@/shared/config/routes';
import { useDialogLifecycle } from '@/shared/hooks/useDialogLifecycle';
import { cx } from '@/shared/lib/cx';
import { Artwork, ExplicitMark, IconButton, PlayerButton, ProgressBar, VolumeSlider } from '@/shared/ui';

import { usePlayerStore, type PlaybackSource } from '../model/playerStore';

import { DevicePicker } from './DevicePicker';
import styles from './ExpandedPlayer.module.css';
import { QueueList } from './QueueList';

export interface ExpandedPlayerProps {
  open: boolean;
  onClose: () => void;
}

function sourceLabel(source: PlaybackSource | null): string {
  if (!source) return 'Now playing';
  switch (source.kind) {
    case 'album':
      return 'Playing from album';
    case 'playlist':
      return 'Playing from playlist';
    case 'artist':
      return 'Playing from artist';
    case 'library':
      return 'Playing from library';
  }
}

/** Full-viewport listening view with artwork, transport, like, device picker and queue. */
export function ExpandedPlayer({ open, onClose }: ExpandedPlayerProps) {
  const ref = useRef<HTMLDivElement>(null);
  const [showQueueMobile, setShowQueueMobile] = useState(false);
  useDialogLifecycle(open, onClose, ref);

  const p = usePlayerStore(
    useShallow((s) => ({
      track: s.currentTrack,
      source: s.source,
      isPlaying: s.isPlaying,
      buffering: s.buffering,
      position: s.position,
      duration: s.duration,
      buffered: s.buffered,
      volume: s.volume,
      muted: s.muted,
      shuffle: s.shuffle,
      repeatMode: s.repeatMode,
      error: s.error,
    })),
  );
  const actions = usePlayerStore(
    useShallow((s) => ({
      togglePlay: s.togglePlay,
      next: s.next,
      previous: s.previous,
      seek: s.seek,
      setVolume: s.setVolume,
      setMuted: s.setMuted,
      toggleShuffle: s.toggleShuffle,
      cycleRepeat: s.cycleRepeat,
    })),
  );

  const like = useTrackLike(p.track);

  if (!open || !p.track || typeof document === 'undefined') return null;

  return createPortal(
    <div ref={ref} className={styles.overlay} role="dialog" aria-modal="true" aria-label="Now playing">
      <header className={styles.header}>
        <div className={styles.source}>
          <span className="ic-overline">{sourceLabel(p.source)}</span>
          <span className="ic-meta">{p.track.albumTitle}</span>
        </div>
        <IconButton icon="chevron-down" label="Collapse player" onClick={onClose} />
      </header>

      <div className={cx(styles.stage, showQueueMobile && styles.showQueue)}>
        <div className={styles.main}>
          <div className={styles.artFrame}>
            <div className={styles.artInner}>
              <Artwork art={p.track.art} src={p.track.coverUrl} alt={`${p.track.albumTitle} cover`} />
            </div>
          </div>

          <div className={styles.trackHead}>
            <div className={styles.trackMeta}>
              <div className={styles.titleRow}>
                <h2 className={styles.title}>{p.track.title}</h2>
                {p.track.explicit ? <ExplicitMark /> : null}
              </div>
              <span className={styles.sub}>
                {p.track.artistName} ·{' '}
                <Link to={routes.album(p.track.albumId)} className={styles.albumLink} onClick={onClose}>
                  {p.track.albumTitle}
                </Link>
              </span>
            </div>
            <IconButton
              icon="heart"
              size="md"
              variant={like.liked ? 'active' : 'neutral'}
              pressed={like.liked}
              label={like.liked ? 'Remove from Liked tracks' : 'Save to Liked tracks'}
              onClick={() => void like.toggle()}
            />
          </div>

          {p.error ? (
            <p className={styles.error} role="alert">
              {p.error}
            </p>
          ) : null}

          <div className={styles.seek}>
            <ProgressBar duration={p.duration} position={p.position} buffered={p.buffered} onSeek={actions.seek} />
          </div>

          <div className={styles.controls}>
            <PlayerButton kind="shuffle" active={p.shuffle} onClick={actions.toggleShuffle} />
            <PlayerButton kind="previous" onClick={actions.previous} />
            <PlayerButton kind="play" size="lg" playing={p.isPlaying} loading={p.buffering && p.isPlaying} onClick={actions.togglePlay} />
            <PlayerButton kind="next" onClick={actions.next} />
            <PlayerButton kind="repeat" repeatMode={p.repeatMode} onClick={actions.cycleRepeat} />
          </div>

          <div className={styles.utility}>
            <VolumeSlider value={p.volume} muted={p.muted} onChange={actions.setVolume} onMutedChange={actions.setMuted} width={120} />
            <div className={styles.utilityActions}>
              <DevicePicker placement="top" align="end" size="md" />
              <IconButton
                icon="queue"
                label={showQueueMobile ? 'Show artwork' : 'Queue'}
                size="md"
                variant={showQueueMobile ? 'active' : 'neutral'}
                pressed={showQueueMobile}
                className={styles.queueToggle}
                onClick={() => setShowQueueMobile((v) => !v)}
              />
            </div>
          </div>
        </div>

        <aside className={styles.queuePanel} aria-label="Up next">
          <QueueList />
        </aside>
      </div>
    </div>,
    document.body,
  );
}
