// Ported from the ICYRE Design System (components/src/player.tsx).
import type { CSSProperties } from 'react';

import { useControlled } from '@/shared/hooks/useControlled';
import { cx } from '@/shared/lib/cx';
import { formatTime } from '@/shared/lib/formatTime';

import { Icon, IconButton } from './core';
import { Slider } from './forms';

/* ---------- PlaybackState ---------- */
export interface PlaybackStateProps {
  state?: 'playing' | 'paused' | 'buffering';
  size?: 'sm' | 'md';
}

export function PlaybackState({ state = 'playing', size = 'md' }: PlaybackStateProps) {
  const label = state === 'playing' ? 'Now playing' : state === 'paused' ? 'Paused' : 'Buffering';
  return (
    <span className={cx('ic-eq', `ic-eq-${state}`, `ic-eq-${size}`)} role="img" aria-label={label} title={label}>
      <i />
      <i />
      <i />
    </span>
  );
}

/* ---------- TimeLabel ---------- */
export interface TimeLabelProps {
  seconds: number;
  remaining?: boolean;
  tone?: 'secondary' | 'tertiary';
}

export function TimeLabel({ seconds, remaining, tone = 'tertiary' }: TimeLabelProps) {
  return <span className={cx('ic-time', `ic-time-${tone}`)}>{(remaining ? '−' : '') + formatTime(seconds)}</span>;
}

/* ---------- PlayerButton ---------- */
export type RepeatMode = 'off' | 'all' | 'one';

export interface PlayerButtonProps {
  kind: 'play' | 'previous' | 'next' | 'shuffle' | 'repeat';
  playing?: boolean;
  active?: boolean;
  repeatMode?: RepeatMode;
  size?: 'md' | 'lg';
  disabled?: boolean;
  loading?: boolean;
  onClick?: () => void;
}

export function PlayerButton({ kind, playing, active, repeatMode = 'off', size = 'md', disabled, loading, onClick }: PlayerButtonProps) {
  if (kind === 'play') {
    return (
      <button
        type="button"
        className={cx('ic-playbtn', `ic-playbtn-${size}`, playing && 'is-playing', loading && 'is-loading')}
        aria-label={playing ? 'Pause' : 'Play'}
        disabled={disabled}
        onClick={onClick}
      >
        {loading ? (
          <span className="ic-spinner ic-spinner-sm ic-spinner-neutral" role="status" aria-label="Buffering" />
        ) : (
          <Icon name={playing ? 'pause' : 'play'} size={size === 'lg' ? 26 : 20} filled />
        )}
      </button>
    );
  }
  const on = kind === 'repeat' ? repeatMode !== 'off' : !!active;
  const label =
    kind === 'previous'
      ? 'Previous'
      : kind === 'next'
        ? 'Next'
        : kind === 'shuffle'
          ? on
            ? 'Shuffle on'
            : 'Shuffle off'
          : repeatMode === 'one'
            ? 'Repeat one'
            : repeatMode === 'all'
              ? 'Repeat all'
              : 'Repeat off';
  const toggle = kind === 'shuffle' || kind === 'repeat';
  return (
    <button
      type="button"
      className={cx('ic-playerctl', on && 'is-on')}
      aria-label={label}
      title={label}
      aria-pressed={toggle ? on : undefined}
      disabled={disabled}
      onClick={onClick}
    >
      <Icon name={kind} size={20} filled={kind === 'previous' || kind === 'next'} />
      {kind === 'repeat' && repeatMode === 'one' && <span className="ic-playerctl-one">1</span>}
      {toggle && on && <span className="ic-playerctl-dot" aria-hidden="true" />}
    </button>
  );
}

/* ---------- ProgressBar (seek) ---------- */
export interface ProgressBarProps {
  position?: number;
  defaultPosition?: number;
  duration: number;
  buffered?: number;
  showTimes?: boolean;
  disabled?: boolean;
  onSeek?: (sec: number) => void;
}

export function ProgressBar({ position, defaultPosition = 0, duration, buffered, showTimes = true, disabled, onSeek }: ProgressBarProps) {
  const [p, setP] = useControlled<number>(position, defaultPosition, onSeek);
  return (
    <div className="ic-seek">
      {showTimes && <TimeLabel seconds={p} />}
      <div className="ic-seek-bar">
        <Slider
          label="Seek"
          tone="iridescent"
          size="sm"
          showThumb="hover"
          min={0}
          max={Math.max(duration, 1)}
          value={p}
          buffered={buffered}
          disabled={disabled}
          valueText={`${formatTime(p)} of ${formatTime(duration)}`}
          onChange={setP}
        />
      </div>
      {showTimes && <TimeLabel seconds={duration} />}
    </div>
  );
}

/* ---------- VolumeSlider ---------- */
export interface VolumeSliderProps {
  value?: number;
  defaultValue?: number;
  muted?: boolean;
  defaultMuted?: boolean;
  onChange?: (v: number) => void;
  onMutedChange?: (m: boolean) => void;
  width?: number;
}

export function VolumeSlider({ value, defaultValue = 70, muted, defaultMuted = false, onChange, onMutedChange, width = 112 }: VolumeSliderProps) {
  const [v, setV] = useControlled<number>(value, defaultValue, onChange);
  const [m, setM] = useControlled<boolean>(muted, defaultMuted, onMutedChange);
  const shown = m ? 0 : v;
  return (
    <div className="ic-volume" style={{ '--vol-w': `${width}px` } as CSSProperties}>
      <IconButton icon={m || v === 0 ? 'mute' : 'volume'} label={m ? 'Unmute' : 'Mute'} size="sm" pressed={m} onClick={() => setM(!m)} />
      <div className="ic-volume-bar">
        <Slider
          label="Volume"
          size="sm"
          showThumb="hover"
          value={shown}
          valueText={`${shown}%`}
          onChange={(n) => {
            if (m) setM(false);
            setV(n);
          }}
        />
      </div>
    </div>
  );
}
