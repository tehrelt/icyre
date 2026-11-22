import { React, cx, Icon, IconButton } from './core';
import type { IconName } from './icons';

/* ---------- Tooltip ---------- */
export interface TooltipProps {
  content: string;
  placement?: 'top' | 'bottom';
  open?: boolean;
  shortcut?: string;
  children: any;
}
export function Tooltip({ content, placement = 'top', open, shortcut, children }: TooltipProps) {
  const [hover, setHover] = React.useState(false);
  const id = 'ic-tt' + React.useId().replace(/:/g, '');
  const shown = open ?? hover;
  return (
    <span
      className="ic-tooltip-anchor"
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      onFocus={() => setHover(true)}
      onBlur={() => setHover(false)}
      aria-describedby={shown ? id : undefined}
    >
      {children}
      <span id={id} role="tooltip" className={cx('ic-tooltip', 'ic-tooltip-' + placement, shown && 'is-open')}>
        {content}
        {shortcut && <kbd className="ic-kbd ic-kbd-inverse">{shortcut}</kbd>}
      </span>
    </span>
  );
}

/* ---------- Menu (shared by Dropdown and ContextMenu) ---------- */
export interface MenuItem {
  id?: string;
  label?: string;
  icon?: IconName;
  shortcut?: string;
  danger?: boolean;
  disabled?: boolean;
  selected?: boolean;
  divider?: boolean;
  heading?: string;
}
export interface MenuProps {
  items: MenuItem[];
  onSelect?: (item: MenuItem) => void;
  width?: number;
  label?: string;
}
export function Menu({ items, onSelect, width = 232, label = 'Menu' }: MenuProps) {
  return (
    <div className="ic-menu" role="menu" aria-label={label} style={{ width }}>
      {items.map((it, i) =>
        it.divider ? (
          <div key={i} className="ic-menu-divider" role="separator" />
        ) : it.heading ? (
          <div key={i} className="ic-menu-heading">
            {it.heading}
          </div>
        ) : (
          <button
            key={it.id ?? i}
            type="button"
            role={it.selected != null ? 'menuitemcheckbox' : 'menuitem'}
            aria-checked={it.selected != null ? it.selected : undefined}
            disabled={it.disabled}
            className={cx('ic-menu-item', it.danger && 'is-danger', it.selected && 'is-selected')}
            onClick={() => onSelect?.(it)}
          >
            {it.icon ? <Icon name={it.icon} size={18} /> : <span className="ic-menu-gap" />}
            <span className="ic-menu-label">{it.label}</span>
            {it.selected ? <Icon name="check" size={16} className="ic-menu-check" /> : it.shortcut ? <kbd className="ic-kbd">{it.shortcut}</kbd> : null}
          </button>
        ),
      )}
    </div>
  );
}

/* ---------- Dropdown ---------- */
export interface DropdownProps {
  trigger: any;
  items: MenuItem[];
  open?: boolean;
  align?: 'start' | 'end';
  onSelect?: (item: MenuItem) => void;
}
export function Dropdown({ trigger, items, open, align = 'start', onSelect }: DropdownProps) {
  const [o, setO] = React.useState(false);
  const shown = open ?? o;
  const ref = React.useRef(null);
  React.useEffect(() => {
    if (!o) return;
    const close = (e: any) => {
      if (ref.current && !(ref.current as any).contains(e.target)) setO(false);
    };
    const esc = (e: any) => e.key === 'Escape' && setO(false);
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', esc);
    return () => {
      document.removeEventListener('mousedown', close);
      document.removeEventListener('keydown', esc);
    };
  }, [o]);
  return (
    <div className="ic-dropdown" ref={ref}>
      <span onClick={() => setO(!o)} aria-haspopup="menu" aria-expanded={shown}>
        {trigger}
      </span>
      {shown && (
        <div className={cx('ic-dropdown-panel', 'ic-align-' + align)}>
          <Menu
            items={items}
            onSelect={(it) => {
              setO(false);
              onSelect?.(it);
            }}
          />
        </div>
      )}
    </div>
  );
}

/* ---------- ContextMenu ---------- */
export interface ContextMenuProps {
  items: MenuItem[];
  children: any;
  defaultPosition?: { x: number; y: number };
  onSelect?: (item: MenuItem) => void;
}
export function ContextMenu({ items, children, defaultPosition, onSelect }: ContextMenuProps) {
  const [pos, setPos] = React.useState(defaultPosition ?? null);
  const ref = React.useRef(null);
  React.useEffect(() => {
    if (!pos) return;
    const close = (e: any) => {
      if (e.type === 'keydown' && e.key !== 'Escape') return;
      if (e.type === 'mousedown' && (e.target as any).closest?.('.ic-menu')) return;
      setPos(null);
    };
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', close);
    return () => {
      document.removeEventListener('mousedown', close);
      document.removeEventListener('keydown', close);
    };
  }, [pos]);
  return (
    <div
      ref={ref}
      className="ic-context"
      onContextMenu={(e: any) => {
        e.preventDefault();
        const r = (ref.current as any).getBoundingClientRect();
        setPos({ x: e.clientX - r.left, y: e.clientY - r.top });
      }}
    >
      {children}
      {pos && (
        <div className="ic-context-panel" style={{ left: pos.x, top: pos.y }}>
          <Menu
            items={items}
            label="Context menu"
            onSelect={(it) => {
              setPos(null);
              onSelect?.(it);
            }}
          />
        </div>
      )}
    </div>
  );
}

/* ---------- Popover ---------- */
export interface PopoverProps {
  title?: string;
  children: any;
  footer?: any;
  width?: number;
  onClose?: () => void;
}
export function Popover({ title, children, footer, width = 300, onClose }: PopoverProps) {
  return (
    <div className="ic-popover" role="dialog" aria-label={title} style={{ width }}>
      {(title || onClose) && (
        <div className="ic-popover-head">
          {title && <div className="ic-popover-title">{title}</div>}
          {onClose && <IconButton icon="close" label="Close" size="sm" onClick={onClose} />}
        </div>
      )}
      <div className="ic-popover-body">{children}</div>
      {footer && <div className="ic-popover-foot">{footer}</div>}
    </div>
  );
}

/* ---------- Modal ---------- */
export interface ModalProps {
  open: boolean;
  title: string;
  description?: string;
  children?: any;
  footer?: any;
  size?: 'sm' | 'md' | 'lg';
  inline?: boolean;
  onClose?: () => void;
}
export function Modal({ open, title, description, children, footer, size = 'md', inline, onClose }: ModalProps) {
  const id = 'ic-md' + React.useId().replace(/:/g, '');
  React.useEffect(() => {
    if (!open || !onClose) return;
    const esc = (e: any) => e.key === 'Escape' && onClose();
    document.addEventListener('keydown', esc);
    return () => document.removeEventListener('keydown', esc);
  }, [open, onClose]);
  if (!open) return null;
  return (
    <div className={cx('ic-scrim', inline && 'is-inline')} onMouseDown={(e: any) => e.target === e.currentTarget && onClose?.()}>
      <div className={cx('ic-modal', 'ic-modal-' + size)} role="dialog" aria-modal="true" aria-labelledby={id}>
        <div className="ic-modal-head">
          <div>
            <h2 className="ic-modal-title" id={id}>
              {title}
            </h2>
            {description && <p className="ic-modal-desc">{description}</p>}
          </div>
          {onClose && <IconButton icon="close" label="Close" size="sm" onClick={onClose} />}
        </div>
        {children && <div className="ic-modal-body">{children}</div>}
        {footer && <div className="ic-modal-foot">{footer}</div>}
      </div>
    </div>
  );
}

/* ---------- Drawer ---------- */
export interface DrawerProps {
  open: boolean;
  title: string;
  side?: 'right' | 'bottom';
  children?: any;
  footer?: any;
  inline?: boolean;
  onClose?: () => void;
}
export function Drawer({ open, title, side = 'right', children, footer, inline, onClose }: DrawerProps) {
  if (!open) return null;
  return (
    <div className={cx('ic-scrim', 'ic-scrim-drawer', inline && 'is-inline')} onMouseDown={(e: any) => e.target === e.currentTarget && onClose?.()}>
      <aside className={cx('ic-drawer', 'ic-drawer-' + side)} role="dialog" aria-modal="true" aria-label={title}>
        {side === 'bottom' && <span className="ic-drawer-grabber" aria-hidden="true" />}
        <div className="ic-drawer-head">
          <h2 className="ic-drawer-title">{title}</h2>
          {onClose && <IconButton icon="close" label="Close" size="sm" onClick={onClose} />}
        </div>
        <div className="ic-drawer-body">{children}</div>
        {footer && <div className="ic-drawer-foot">{footer}</div>}
      </aside>
    </div>
  );
}

/* ---------- Toast ---------- */
export interface ToastProps {
  tone?: 'neutral' | 'success' | 'warning' | 'danger' | 'info';
  title: string;
  description?: string;
  action?: { label: string; onClick?: () => void };
  art?: number;
  onClose?: () => void;
}
const TOAST_ICON: Record<string, IconName> = { success: 'check-circle', warning: 'alert', danger: 'alert', info: 'info', neutral: 'check-circle' };
export function Toast({ tone = 'neutral', title, description, action, onClose }: ToastProps) {
  return (
    <div className={cx('ic-toast', 'ic-toast-' + tone)} role={tone === 'danger' ? 'alert' : 'status'}>
      <Icon name={TOAST_ICON[tone]!} size={20} className="ic-toast-icon" />
      <div className="ic-toast-text">
        <div className="ic-toast-title">{title}</div>
        {description && <div className="ic-toast-desc">{description}</div>}
      </div>
      {action && (
        <button type="button" className="ic-toast-action" onClick={action.onClick}>
          {action.label}
        </button>
      )}
      {onClose && <IconButton icon="close" label="Dismiss" size="sm" onClick={onClose} />}
    </div>
  );
}
