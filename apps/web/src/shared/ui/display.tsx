// Ported from the ICYRE Design System (components/src/display.tsx).
import type { ReactNode } from 'react';

import { useControlled } from '@/shared/hooks/useControlled';
import { cx } from '@/shared/lib/cx';

import { Icon } from './core';
import type { IconName } from './icons';

/* ---------- Avatar ---------- */
export interface AvatarProps {
  name: string;
  src?: string | null;
  size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl';
  ring?: boolean;
  art?: number;
}

export function Avatar({ name, src, size = 'md', ring, art }: AvatarProps) {
  const initials = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() ?? '')
    .join('');
  const hash = art ?? [...name].reduce((a, c) => a + c.charCodeAt(0), 0);
  return (
    <span className={cx('ic-avatar', `ic-avatar-${size}`, ring && 'ic-avatar-ring')} title={name}>
      {src ? (
        <img src={src} alt={name} />
      ) : (
        <span className={cx('ic-avatar-fallback', `ic-art-soft-${hash % 4}`)} role="img" aria-label={name}>
          {initials}
        </span>
      )}
    </span>
  );
}

/* ---------- Badge ---------- */
export type BadgeTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger' | 'featured' | 'explicit' | 'new' | 'trending';

const BADGE_ICON: Partial<Record<BadgeTone, IconName>> = { featured: 'spark', trending: 'trending' };
const BADGE_LABEL: Partial<Record<BadgeTone, string>> = { new: 'New', featured: 'Featured', trending: 'Trending' };

export interface BadgeProps {
  tone?: BadgeTone;
  icon?: IconName;
  children?: ReactNode;
}

export function Badge({ tone = 'neutral', icon, children }: BadgeProps) {
  if (tone === 'explicit') return <ExplicitMark />;
  const ic = icon ?? BADGE_ICON[tone];
  return (
    <span className={cx('ic-badge', `ic-badge-${tone}`)}>
      {ic && <Icon name={ic} size={12} filled={ic === 'spark'} />}
      {children ?? BADGE_LABEL[tone] ?? null}
    </span>
  );
}

/** The "E" explicit-content mark used inside card and row subtitles. */
export function ExplicitMark() {
  return (
    <span className="ic-badge ic-badge-explicit" role="img" aria-label="Explicit" title="Explicit">
      E
    </span>
  );
}

/* ---------- Tag ---------- */
export interface TagProps {
  children: ReactNode;
  onRemove?: () => void;
  /** Accessible name of the remove button; defaults to "Remove <children>". */
  removeLabel?: string;
}

export function Tag({ children, onRemove, removeLabel }: TagProps) {
  return (
    <span className="ic-tag">
      {children}
      {onRemove && (
        <button type="button" className="ic-tag-remove" aria-label={removeLabel ?? `Remove ${String(children)}`} onClick={onRemove}>
          <Icon name="close" size={12} />
        </button>
      )}
    </span>
  );
}

/* ---------- Chip ---------- */
export interface ChipProps {
  children: ReactNode;
  selected?: boolean;
  defaultSelected?: boolean;
  icon?: IconName;
  disabled?: boolean;
  onChange?: (v: boolean) => void;
}

export function Chip({ children, selected, defaultSelected = false, icon, disabled, onChange }: ChipProps) {
  const [v, setV] = useControlled<boolean>(selected, defaultSelected, onChange);
  return (
    <button type="button" className={cx('ic-chip', v && 'is-selected')} aria-pressed={v} disabled={disabled} onClick={() => setV(!v)}>
      {v ? <Icon name="check" size={14} /> : icon ? <Icon name={icon} size={14} /> : null}
      {children}
    </button>
  );
}

/* ---------- Skeleton ---------- */
export interface SkeletonProps {
  shape?: 'text' | 'rect' | 'circle';
  width?: number | string;
  height?: number | string;
  radius?: 'xs' | 'sm' | 'md' | 'card';
}

export function Skeleton({ shape = 'text', width, height, radius }: SkeletonProps) {
  return (
    <span
      className={cx('ic-skeleton', `ic-skeleton-${shape}`, radius && `ic-r-${radius}`)}
      style={{ width, height }}
      aria-hidden="true"
    />
  );
}
