import { Link } from 'react-router';
import { useShallow } from 'zustand/react/shallow';

import { useTrackLike } from '@/features/like-track';
import { usePlayerStore } from '@/features/player';
import { routes } from '@/shared/config/routes';
import { cx } from '@/shared/lib/cx';
import { Artwork, Icon, IconButton, PlayerButton, ProgressBar, VolumeSlider } from '@/shared/ui';

import styles from './PlayerBar.module.css';

// Canvas-only glyphs (not in the Design System icon set yet), drawn to the DS
// icon grid: 24px, 1.75 stroke, round caps.
function DeviceGlyph() {
  return (
    <svg className="ic-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="6" y="3.5" width="12" height="17" rx="2.5" />
      <circle cx="12" cy="14" r="3" />
      <path d="M12 7.5h.01" />
    </svg>
  );
}

function ExpandGlyph() {
  return (
    <svg className="ic-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M14.5 4.5h5v5" />
      <path d="M9.5 19.5h-5v-5" />
      <path d="M19.5 4.5 14 10" />
      <path d="M4.5 19.5 10 14" />
    </svg>
  );
}

const LATER = 'Available in a later iteration';

/** Shell / Global player from the product canvas (CPlayerBar). Never unmounts. */
export function PlayerBar() {
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
            <div className={styles.meta}>
              <span className={styles.title}>{p.track.title}</span>
              <span className={styles.artist}>{p.error ?? p.track.artistName}</span>
            </div>
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
        <IconButton icon="queue" label="Queue" size="sm" disabled title={LATER} />
        <button type="button" className="ic-iconbtn ic-iconbtn-neutral ic-iconbtn-sm" aria-label="Connect to a device" title={LATER} disabled>
          <DeviceGlyph />
        </button>
        <div className={styles.volume}>
          <VolumeSlider value={p.volume} muted={p.muted} onChange={actions.setVolume} onMutedChange={actions.setMuted} width={96} />
        </div>
        <button type="button" className="ic-iconbtn ic-iconbtn-neutral ic-iconbtn-sm" aria-label="Open expanded player" title={LATER} disabled>
          <ExpandGlyph />
        </button>
      </div>

      {/* Mobile mini-player progress line. */}
      <div className={styles.miniProgress} aria-hidden="true">
        <span style={{ width: `${progress}%` }} />
      </div>
    </div>
  );
}
