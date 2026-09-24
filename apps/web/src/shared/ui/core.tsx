// Ported from the ICYRE Design System (components/src/core.tsx).
import { useId, type ButtonHTMLAttributes, type HTMLAttributes, type ReactNode } from 'react';

import { cx } from '@/shared/lib/cx';

import { FILLABLE, ICONS, type IconName } from './icons';

export type Size = 'sm' | 'md' | 'lg';
export type ForcedState = 'hover' | 'pressed' | 'focus';

/* ---------- Icon ---------- */
export interface IconProps {
  name: IconName;
  size?: number;
  filled?: boolean;
  label?: string;
  className?: string;
}

export function Icon({ name, size = 20, filled = false, label, className }: IconProps) {
  const fill = filled && FILLABLE.has(name) ? 'currentColor' : 'none';
  return (
    <svg
      className={cx('ic-icon', className)}
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill={fill}
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      role={label ? 'img' : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
      focusable="false"
      // Static, trusted markup from the Design System icon set.
      dangerouslySetInnerHTML={{ __html: ICONS[name] }}
    />
  );
}

/* ---------- Spinner ---------- */
export interface SpinnerProps {
  size?: 'sm' | 'md' | 'lg';
  tone?: 'neutral' | 'iridescent' | 'inverse';
  label?: string;
}

export function Spinner({ size = 'md', tone = 'neutral', label = 'Loading' }: SpinnerProps) {
  return <span className={cx('ic-spinner', `ic-spinner-${size}`, `ic-spinner-${tone}`)} role="status" aria-label={label} />;
}

/* ---------- Button ---------- */
export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'tonal' | 'destructive' | 'iridescent';

export interface ButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> {
  variant?: ButtonVariant;
  size?: Size;
  iconStart?: IconName;
  iconEnd?: IconName;
  loading?: boolean;
  fullWidth?: boolean;
  forceState?: ForcedState;
  children?: ReactNode;
}

export function Button({
  variant = 'primary',
  size = 'md',
  iconStart,
  iconEnd,
  loading,
  disabled,
  fullWidth,
  forceState,
  className,
  children,
  type = 'button',
  ...rest
}: ButtonProps) {
  const iconSize = size === 'sm' ? 16 : size === 'lg' ? 20 : 18;
  const spinnerTone = variant === 'primary' || variant === 'destructive' ? 'inverse' : 'neutral';
  return (
    <button
      type={type}
      {...rest}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cx(
        'ic-btn',
        `ic-btn-${variant}`,
        `ic-btn-${size}`,
        fullWidth && 'ic-btn-full',
        loading && 'is-loading',
        forceState && `is-${forceState}`,
        className,
      )}
    >
      {loading ? (
        <Spinner size="sm" tone={spinnerTone} label="Loading" />
      ) : iconStart ? (
        <Icon name={iconStart} size={iconSize} filled={iconStart === 'play' || iconStart === 'pause'} />
      ) : null}
      {children != null && <span className="ic-btn-label">{children}</span>}
      {iconEnd && !loading ? <Icon name={iconEnd} size={iconSize} /> : null}
    </button>
  );
}

/* ---------- IconButton ---------- */
export type IconButtonVariant = 'neutral' | 'active' | 'selected' | 'destructive' | 'play';

export interface IconButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> {
  icon: IconName;
  label: string;
  variant?: IconButtonVariant;
  size?: Size;
  pressed?: boolean;
  filled?: boolean;
  forceState?: ForcedState;
}

export function IconButton({
  icon,
  label,
  variant = 'neutral',
  size = 'md',
  pressed,
  filled,
  forceState,
  className,
  title,
  type = 'button',
  ...rest
}: IconButtonProps) {
  const iconSize =
    variant === 'play' ? (size === 'lg' ? 26 : size === 'sm' ? 16 : 20) : size === 'sm' ? 16 : size === 'lg' ? 24 : 20;
  const fillIcon = filled ?? (variant === 'play' || ((variant === 'active' || variant === 'selected') && icon === 'heart'));
  return (
    <button
      type={type}
      {...rest}
      aria-label={label}
      title={title ?? label}
      aria-pressed={pressed}
      className={cx('ic-iconbtn', `ic-iconbtn-${variant}`, `ic-iconbtn-${size}`, forceState && `is-${forceState}`, className)}
    >
      <Icon name={icon} size={iconSize} filled={fillIcon} />
    </button>
  );
}

/* ---------- Artwork (cover image or generative fallback) ---------- */
export interface ArtworkProps {
  src?: string | null;
  /** Index of the generative fallback cover (0–7) used when there is no src. */
  art?: number;
  alt?: string;
  shape?: 'square' | 'circle';
  className?: string;
}

export function Artwork({ src, art = 0, alt = '', shape = 'square', className }: ArtworkProps) {
  const cls = cx('ic-art', `ic-art-${Math.abs(art) % 8}`, shape === 'circle' && 'ic-art-circle', className);
  if (src) return <img className={cls} src={src} alt={alt} loading="lazy" />;
  return <span className={cls} role={alt ? 'img' : undefined} aria-label={alt || undefined} />;
}

/* ---------- Surface (materials) ---------- */
export interface SurfaceProps extends HTMLAttributes<HTMLDivElement> {
  material?: 'neutral' | 'elevated' | 'collectible';
  padding?: 'none' | 'sm' | 'md' | 'lg';
}

export function Surface({ material = 'neutral', padding = 'md', className, children, ...rest }: SurfaceProps) {
  return (
    <div {...rest} className={cx('ic-surface', `ic-surface-${material}`, `ic-pad-${padding}`, className)}>
      {material === 'collectible' ? <div className="ic-surface-inner">{children}</div> : children}
    </div>
  );
}

/* ---------- Logo ---------- */
// Brand artwork: the only place with literal brand colours (they are the marks themselves).
/* eslint-disable no-restricted-syntax */
const WORDMARK_PATHS =
  'M2 6v20' + 'M26.66 9.57A10 10 0 1 0 26.66 22.43' + 'M34 6l9 10 9-10M43 16v10' + 'M59 26V6h9a5.5 5.5 0 0 1 0 11h-9M67 17l7 9' + 'M94 6H81v20h13M81 16h11';
const INK = '#141821';
const HOLO_STOPS = ['#1b86a8', '#3d5cff', '#7a4cf5', '#c0459f'];
const APP_ICON_STOPS = ['#b8eef7', '#b9c8ff', '#d2c2ff', '#f5c3ec', '#d3f3e6'];
/* eslint-enable no-restricted-syntax */
const SPARK = 'M16 9.5c.4 3.6 1.9 5.1 5.5 5.5-3.6.4-5.1 1.9-5.5 5.5-.4-3.6-1.9-5.1-5.5-5.5 3.6-.4 5.1-1.9 5.5-5.5z';

export interface LogoProps {
  variant?: 'lockup' | 'symbol' | 'wordmark' | 'app-icon';
  tone?: 'holo' | 'ink';
  height?: number;
  title?: string;
}

export function Logo({ variant = 'lockup', tone = 'holo', height = 28, title = 'ICYRE' }: LogoProps) {
  const uid = useId().replace(/:/g, '');
  const gid = `ic-holo-${uid}`;
  const stroke = tone === 'holo' ? `url(#${gid})` : INK;

  const defs = (
    <defs>
      <linearGradient id={gid} x1="0" y1="0" x2="1" y2="1">
        {HOLO_STOPS.map((c, i) => (
          <stop key={c} offset={[0, 0.38, 0.7, 1][i]} stopColor={c} />
        ))}
      </linearGradient>
    </defs>
  );
  const symbol = (
    <g>
      <rect x="7" y="4.5" width="18" height="23" rx="4" fill="none" stroke={stroke} strokeWidth="2.4" />
      <path d={SPARK} fill={stroke} />
    </g>
  );
  const wordmark = (x: number) => (
    <path transform={`translate(${x} 0)`} d={WORDMARK_PATHS} fill="none" stroke={INK} strokeWidth="2.6" strokeLinejoin="miter" />
  );

  if (variant === 'app-icon') {
    const bg = `ic-appbg-${uid}`;
    return (
      <svg viewBox="0 0 64 64" height={height} role="img" aria-label={title} className="ic-logo">
        <defs>
          <linearGradient id={bg} x1="0" y1="0" x2="1" y2="1">
            {APP_ICON_STOPS.map((c, i) => (
              <stop key={c} offset={[0, 0.34, 0.62, 0.9, 1][i]} stopColor={c} />
            ))}
          </linearGradient>
        </defs>
        <rect width="64" height="64" rx="14" fill={`url(#${bg})`} />
        <g transform="translate(8 8) scale(1.5)">
          <rect x="7" y="4.5" width="18" height="23" rx="4" fill="none" stroke={INK} strokeWidth="2.2" />
          <path d={SPARK} fill={INK} />
        </g>
      </svg>
    );
  }
  if (variant === 'symbol') {
    return (
      <svg viewBox="0 0 32 32" height={height} role="img" aria-label={title} className="ic-logo">
        {defs}
        {symbol}
      </svg>
    );
  }
  if (variant === 'wordmark') {
    return (
      <svg viewBox="0 0 96 32" height={height} role="img" aria-label={title} className="ic-logo">
        {wordmark(0)}
      </svg>
    );
  }
  return (
    <svg viewBox="0 0 138 32" height={height} role="img" aria-label={title} className="ic-logo">
      {defs}
      {symbol}
      {wordmark(40)}
    </svg>
  );
}
