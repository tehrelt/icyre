import { React, cx, Icon, useControlled } from './core';
import type { IconName } from './icons';

/* ---------- Avatar ---------- */
export interface AvatarProps {
  name: string;
  src?: string;
  size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl';
  ring?: boolean;
  art?: number;
}
export function Avatar({ name, src, size = 'md', ring, art }: AvatarProps) {
  const initials = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]!.toUpperCase())
    .join('');
  const hash = art ?? [...name].reduce((a, c) => a + c.charCodeAt(0), 0);
  return (
    <span className={cx('ic-avatar', 'ic-avatar-' + size, ring && 'ic-avatar-ring')} title={name}>
      {src ? (
        <img src={src} alt={name} />
      ) : (
        <span className={cx('ic-avatar-fallback', 'ic-art-soft-' + (hash % 4))} role="img" aria-label={name}>
          {initials}
        </span>
      )}
    </span>
  );
}

/* ---------- Badge ---------- */
export type BadgeTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger' | 'featured' | 'explicit' | 'new' | 'trending';
export interface BadgeProps {
  tone?: BadgeTone;
  icon?: IconName;
  children?: any;
}
const BADGE_ICON: Partial<Record<BadgeTone, IconName>> = { featured: 'spark', trending: 'trending' };
export function Badge({ tone = 'neutral', icon, children }: BadgeProps) {
  if (tone === 'explicit')
    return (
      <span className="ic-badge ic-badge-explicit" role="img" aria-label="Explicit" title="Explicit">
        E
      </span>
    );
  const ic = icon ?? BADGE_ICON[tone];
  const label = children ?? (tone === 'new' ? 'New' : tone === 'featured' ? 'Featured' : tone === 'trending' ? 'Trending' : null);
  return (
    <span className={cx('ic-badge', 'ic-badge-' + tone)}>
      {ic && <Icon name={ic} size={12} filled={ic === 'spark'} />}
      {label}
    </span>
  );
}

/* ---------- Tag ---------- */
export interface TagProps {
  children: any;
  onRemove?: () => void;
}
export function Tag({ children, onRemove }: TagProps) {
  return (
    <span className="ic-tag">
      {children}
      {onRemove && (
        <button type="button" className="ic-tag-remove" aria-label={'Remove ' + String(children)} onClick={onRemove}>
          <Icon name="close" size={12} />
        </button>
      )}
    </span>
  );
}

/* ---------- Chip ---------- */
export interface ChipProps {
  children: any;
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

/* ---------- Divider ---------- */
export interface DividerProps {
  orientation?: 'horizontal' | 'vertical';
  label?: string;
  strength?: 'subtle' | 'default';
}
export function Divider({ orientation = 'horizontal', label, strength = 'subtle' }: DividerProps) {
  if (label)
    return (
      <div className={cx('ic-divider-labeled', 'ic-divider-' + strength)} role="separator">
        <span>{label}</span>
      </div>
    );
  return <hr className={cx('ic-divider', 'ic-divider-' + orientation, 'ic-divider-' + strength)} aria-orientation={orientation} />;
}

/* ---------- Skeleton ---------- */
export interface SkeletonProps {
  shape?: 'text' | 'rect' | 'circle';
  width?: number | string;
  height?: number | string;
  radius?: 'xs' | 'sm' | 'md' | 'card';
}
export function Skeleton({ shape = 'text', width, height, radius }: SkeletonProps) {
  return <span className={cx('ic-skeleton', 'ic-skeleton-' + shape, radius && 'ic-r-' + radius)} style={{ width, height }} aria-hidden="true" />;
}

/* ---------- Progress ---------- */
export interface ProgressProps {
  value?: number;
  max?: number;
  tone?: 'neutral' | 'iridescent';
  size?: 'sm' | 'md';
  label: string;
  showValue?: boolean;
}
export function Progress({ value, max = 100, tone = 'neutral', size = 'md', label, showValue }: ProgressProps) {
  const indeterminate = value == null;
  const pct = indeterminate ? 0 : Math.max(0, Math.min(100, (value! / max) * 100));
  return (
    <div className="ic-progress-wrap">
      <div className="ic-progress-head">
        <span className="ic-label">{label}</span>
        {showValue && !indeterminate && <span className="ic-meta">{Math.round(pct)}%</span>}
      </div>
      <div
        className={cx('ic-progress', 'ic-progress-' + tone, 'ic-progress-' + size, indeterminate && 'is-indeterminate')}
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={max}
        aria-valuenow={indeterminate ? undefined : value}
      >
        <div className="ic-progress-bar" style={indeterminate ? undefined : { width: pct + '%' }} />
      </div>
    </div>
  );
}
