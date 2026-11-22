import { useState } from 'react';
import { Link } from 'react-router';
import { useShallow } from 'zustand/react/shallow';

import { useTrackLike } from '@/features/like-track';
import { DevicePicker, ExpandedPlayer, ExpandGlyph, QueueDrawer, usePlayerStore } from '@/features/player';
import { routes } from '@/shared/config/routes';
import { cx } from '@/shared/lib/cx';
import { Artwork, Icon, IconButton, PlayerButton, ProgressBar, VolumeSlider } from '@/shared/ui';

import styles from './PlayerBar.module.css';

/** Shell / Global player from the product canvas (CPlayerBar). Never unmounts. */
export function PlayerBar() {
  const [queueOpen, setQueueOpen] = useState(false);
  const [expandedOpen, setExpandedOpen] = useState(false);

  const p = usePlayerStore(
    useShallow((s) => ({
      track: s.currentTrack,
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
  const idle = !p.track;
  const progress = p.duration > 0 ? Math.min(100, (p.position / p.duration) * 100) : 0;

  return (
    <div role="region" aria-label="Player" className={cx(styles.bar, idle && styles.idle)}>
      <div className={styles.now}>
        {p.track ? (
          <>
            <Link to={routes.album(p.track.albumId)} aria-label={`Open ${p.track.albumTitle}`} className={styles.cover}>
              <Artwork art={p.track.art} src={p.track.coverUrl} />
            </Link>
            <button
              type="button"
              className={styles.metaBtn}
              aria-label={`Open expanded player for ${p.track.title}`}
              onClick={() => setExpandedOpen(true)}
            >
              <span className={styles.title}>{p.track.title}</span>
              <span className={styles.artist}>{p.error ?? p.track.artistName}</span>
            </button>
            <span className={styles.like}>
              <IconButton
                icon="heart"
                size="sm"
                variant={like.liked ? 'active' : 'neutral'}
                pressed={like.liked}
                label={like.liked ? 'Remove from Liked tracks' : 'Save to Liked tracks'}
                onClick={() => void like.toggle()}
              />
            </span>
          </>
        ) : (
          <>
            <div className={styles.placeholder}>
              <Icon name="queue" size={20} />
            </div>
            <div className={styles.meta}>
              <span className={cx(styles.title, styles.muted)}>Nothing playing</span>
              <span className={cx(styles.artist, styles.faint)}>Pick a record to start listening</span>
            </div>
          </>
        )}
      </div>

      <div className={styles.center}>
        <div className={styles.controls}>
          <span className={styles.secondaryCtl}>
            <PlayerButton kind="shuffle" active={p.shuffle} disabled={idle} onClick={actions.toggleShuffle} />
          </span>
          <span className={styles.secondaryCtl}>
            <PlayerButton kind="previous" disabled={idle} onClick={actions.previous} />
          </span>
          <PlayerButton kind="play" playing={p.isPlaying} loading={p.buffering && p.isPlaying} disabled={idle} onClick={actions.togglePlay} />
          <span className={styles.secondaryCtl}>
            <PlayerButton kind="next" disabled={idle} onClick={actions.next} />
          </span>
          <span className={styles.secondaryCtl}>
            <PlayerButton kind="repeat" repeatMode={p.repeatMode} disabled={idle} onClick={actions.cycleRepeat} />
          </span>
        </div>
        <div className={styles.seek}>
          <ProgressBar duration={idle ? 1 : p.duration} position={p.position} buffered={p.buffered} disabled={idle} onSeek={actions.seek} />
        </div>
      </div>

      <div className={styles.side}>
        <IconButton
          icon="queue"
          label="Queue"
          size="sm"
          variant={queueOpen ? 'active' : 'neutral'}
          pressed={queueOpen}
          onClick={() => setQueueOpen((v) => !v)}
        />
        <DevicePicker placement="top" align="end" size="sm" />
        <div className={styles.volume}>
          <VolumeSlider value={p.volume} muted={p.muted} onChange={actions.setVolume} onMutedChange={actions.setMuted} width={96} />
        </div>
        <button
          type="button"
          className="ic-iconbtn ic-iconbtn-neutral ic-iconbtn-sm"
          aria-label="Open expanded player"
          title="Open expanded player"
          disabled={idle}
          onClick={() => setExpandedOpen(true)}
        >
          <ExpandGlyph />
        </button>
      </div>

      {/* Mobile mini-player progress line. */}
      <div className={styles.miniProgress} aria-hidden="true">
        <span style={{ width: `${progress}%` }} />
      </div>

      <QueueDrawer open={queueOpen} onClose={() => setQueueOpen(false)} />
      <ExpandedPlayer open={expandedOpen && !idle} onClose={() => setExpandedOpen(false)} />
    </div>
  );
}
