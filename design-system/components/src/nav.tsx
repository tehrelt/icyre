import { React, cx, Icon, useControlled } from './core';
import type { IconName } from './icons';

/* ---------- Tabs ---------- */
export interface TabItem {
  id: string;
  label: string;
  count?: number;
  disabled?: boolean;
}
export interface TabsProps {
  items: TabItem[];
  value?: string;
  defaultValue?: string;
  onChange?: (id: string) => void;
  size?: 'md' | 'lg';
  label?: string;
}
export function Tabs({ items, value, defaultValue, onChange, size = 'md', label = 'Sections' }: TabsProps) {
  const [v, setV] = useControlled<string>(value, defaultValue ?? items[0]?.id ?? '', onChange);
  const onKey = (e: any) => {
    const enabled = items.filter((i) => !i.disabled);
    const idx = enabled.findIndex((i) => i.id === v);
    if (e.key === 'ArrowRight') setV(enabled[(idx + 1) % enabled.length]!.id);
    if (e.key === 'ArrowLeft') setV(enabled[(idx - 1 + enabled.length) % enabled.length]!.id);
  };
  return (
    <div className={cx('ic-tabs', 'ic-tabs-' + size)} role="tablist" aria-label={label} onKeyDown={onKey}>
      {items.map((t) => (
        <button key={t.id} type="button" role="tab" aria-selected={v === t.id} tabIndex={v === t.id ? 0 : -1} disabled={t.disabled} className={cx('ic-tab', v === t.id && 'is-active')} onClick={() => setV(t.id)}>
          {t.label}
          {t.count != null && <span className="ic-tab-count">{t.count}</span>}
        </button>
      ))}
    </div>
  );
}

/* ---------- SegmentedControl ---------- */
export interface SegmentOption {
  value: string;
  label?: string;
  icon?: IconName;
}
export interface SegmentedControlProps {
  options: SegmentOption[];
  value?: string;
  defaultValue?: string;
  onChange?: (v: string) => void;
  size?: 'sm' | 'md';
  label?: string;
}
export function SegmentedControl({ options, value, defaultValue, onChange, size = 'md', label = 'View' }: SegmentedControlProps) {
  const [v, setV] = useControlled<string>(value, defaultValue ?? options[0]?.value ?? '', onChange);
  return (
    <div className={cx('ic-segmented', 'ic-segmented-' + size)} role="radiogroup" aria-label={label}>
      {options.map((o) => (
        <button key={o.value} type="button" role="radio" aria-checked={v === o.value} aria-label={o.label ? undefined : o.value} className={cx('ic-segment', v === o.value && 'is-active')} onClick={() => setV(o.value)}>
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
  art?: any;
  href?: string;
  forceState?: 'hover';
  onClick?: () => void;
}
export function SidebarItem({ icon, label, active, disabled, count, art, href, forceState, onClick }: SidebarItemProps) {
  const Tag: any = href && !disabled ? 'a' : 'button';
  return (
    <Tag
      href={href}
      type={Tag === 'button' ? 'button' : undefined}
      disabled={Tag === 'button' ? disabled : undefined}
      aria-disabled={disabled || undefined}
      aria-current={active ? 'page' : undefined}
      className={cx('ic-sideitem', active && 'is-active', disabled && 'is-disabled', forceState && 'is-' + forceState)}
      onClick={onClick}
    >
      {art ?? (icon && <Icon name={icon} size={20} filled={active && icon === 'heart'} />)}
      <span className="ic-sideitem-label">{label}</span>
      {count != null && <span className="ic-sideitem-count">{count}</span>}
      {active && <span className="ic-gem" aria-hidden="true" />}
    </Tag>
  );
}

/* ---------- TopNavItem ---------- */
export interface TopNavItemProps {
  label: string;
  active?: boolean;
  disabled?: boolean;
  href?: string;
  forceState?: 'hover';
  onClick?: () => void;
}
export function TopNavItem({ label, active, disabled, href, forceState, onClick }: TopNavItemProps) {
  const Tag: any = href && !disabled ? 'a' : 'button';
  return (
    <Tag href={href} type={Tag === 'button' ? 'button' : undefined} disabled={Tag === 'button' ? disabled : undefined} aria-current={active ? 'page' : undefined} className={cx('ic-topnav', active && 'is-active', disabled && 'is-disabled', forceState && 'is-' + forceState)} onClick={onClick}>
      {label}
    </Tag>
  );
}

/* ---------- MobileNavItem ---------- */
export interface MobileNavItemProps {
  icon: IconName;
  label: string;
  active?: boolean;
  disabled?: boolean;
  badge?: boolean;
  onClick?: () => void;
}
export function MobileNavItem({ icon, label, active, disabled, badge, onClick }: MobileNavItemProps) {
  return (
    <button type="button" disabled={disabled} aria-current={active ? 'page' : undefined} className={cx('ic-mobnav', active && 'is-active')} onClick={onClick}>
      <span className="ic-mobnav-icon">
        <Icon name={icon} size={22} filled={active && icon === 'heart'} />
        {badge && <span className="ic-mobnav-dot" aria-label="New activity" />}
      </span>
      <span className="ic-mobnav-label">{label}</span>
    </button>
  );
}

/* ---------- Breadcrumb ---------- */
export interface BreadcrumbProps {
  items: Array<{ label: string; href?: string }>;
}
export function Breadcrumb({ items }: BreadcrumbProps) {
  return (
    <nav aria-label="Breadcrumb" className="ic-breadcrumb">
      <ol>
        {items.map((it, i) => {
          const last = i === items.length - 1;
          return (
            <li key={i}>
              {last ? (
                <span aria-current="page" className="ic-crumb-current">
                  {it.label}
                </span>
              ) : (
                <a className="ic-crumb" href={it.href ?? '#'}>
                  {it.label}
                </a>
              )}
              {!last && <Icon name="chevron" size={14} className="ic-crumb-sep" />}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
