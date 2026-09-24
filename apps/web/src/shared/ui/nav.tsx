// Ported from the ICYRE Design System (components/src/nav.tsx).
// Navigation items render router links (`to`) so the SPA never reloads.
import type { ReactNode } from 'react';
import { Link } from 'react-router';

import { useControlled } from '@/shared/hooks/useControlled';
import { cx } from '@/shared/lib/cx';

import { Icon } from './core';
import type { IconName } from './icons';

/* ---------- SegmentedControl ---------- */
export interface SegmentOption<V extends string = string> {
  value: V;
  label?: string;
  icon?: IconName;
}

export interface SegmentedControlProps<V extends string = string> {
  options: SegmentOption<V>[];
  value?: V;
  defaultValue?: V;
  onChange?: (v: V) => void;
  size?: 'sm' | 'md';
  label?: string;
}

export function SegmentedControl<V extends string = string>({
  options,
  value,
  defaultValue,
  onChange,
  size = 'md',
  label = 'View',
}: SegmentedControlProps<V>) {
  const [v, setV] = useControlled<V>(value, defaultValue ?? (options[0]?.value as V), onChange);
  return (
    <div className={cx('ic-segmented', `ic-segmented-${size}`)} role="radiogroup" aria-label={label}>
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={v === o.value}
          aria-label={o.label ? undefined : o.value}
          className={cx('ic-segment', v === o.value && 'is-active')}
          onClick={() => setV(o.value)}
        >
          {o.icon && <Icon name={o.icon} size={16} />}
          {o.label}
        </button>
      ))}
    </div>
  );
}

/* ---------- SidebarItem ---------- */
export interface SidebarItemProps {
  icon?: IconName;
  label: string;
  active?: boolean;
  disabled?: boolean;
  count?: number;
  /** Artwork shown instead of the icon (playlists). */
  art?: ReactNode;
  to?: string;
  forceState?: 'hover';
  onClick?: () => void;
}

export function SidebarItem({ icon, label, active, disabled, count, art, to, forceState, onClick }: SidebarItemProps) {
  const className = cx('ic-sideitem', active && 'is-active', disabled && 'is-disabled', forceState && `is-${forceState}`);
  const content = (
    <>
      {art ?? (icon && <Icon name={icon} size={20} filled={active && icon === 'heart'} />)}
      <span className="ic-sideitem-label">{label}</span>
      {count != null && <span className="ic-sideitem-count">{count}</span>}
      {active && <span className="ic-gem" aria-hidden="true" />}
    </>
  );
  if (to && !disabled) {
    return (
      <Link to={to} aria-current={active ? 'page' : undefined} className={className} onClick={onClick} title={label}>
        {content}
      </Link>
    );
  }
  return (
    <button type="button" disabled={disabled} aria-current={active ? 'page' : undefined} className={className} onClick={onClick} title={label}>
      {content}
    </button>
  );
}

/* ---------- MobileNavItem ---------- */
export interface MobileNavItemProps {
  icon: IconName;
  label: string;
  active?: boolean;
  disabled?: boolean;
  badge?: boolean;
  to: string;
}

export function MobileNavItem({ icon, label, active, disabled, badge, to }: MobileNavItemProps) {
  return (
    <Link
      to={to}
      aria-current={active ? 'page' : undefined}
      aria-disabled={disabled || undefined}
      className={cx('ic-mobnav', active && 'is-active')}
    >
      <span className="ic-mobnav-icon">
        <Icon name={icon} size={22} filled={active && icon === 'heart'} />
        {badge && <span className="ic-mobnav-dot" aria-label="New activity" />}
      </span>
      <span className="ic-mobnav-label">{label}</span>
    </Link>
  );
}

/* ---------- Breadcrumb ---------- */
export interface BreadcrumbProps {
  items: Array<{ label: string; to?: string }>;
}

export function Breadcrumb({ items }: BreadcrumbProps) {
  return (
    <nav aria-label="Breadcrumb" className="ic-breadcrumb">
      <ol>
        {items.map((it, i) => {
          const last = i === items.length - 1;
          return (
            <li key={`${it.label}-${i}`}>
              {last ? (
                <span aria-current="page" className="ic-crumb-current">
                  {it.label}
                </span>
              ) : (
                <Link className="ic-crumb" to={it.to ?? '/'}>
                  {it.label}
                </Link>
              )}
              {!last && <Icon name="chevron" size={14} className="ic-crumb-sep" />}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
