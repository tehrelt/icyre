import {
  cloneElement,
  useEffect,
  useId,
  useRef,
  useState,
  type KeyboardEvent,
  type MouseEventHandler,
  type ReactElement,
  type ReactNode,
  type RefObject,
} from 'react';
import { createPortal } from 'react-dom';

import { useControlled } from '@/shared/hooks/useControlled';
import { cx } from '@/shared/lib/cx';

import { Icon, IconButton } from './core';
import type { IconName } from './icons';

interface TriggerProps {
  onClick?: MouseEventHandler<HTMLElement>;
  'aria-describedby'?: string;
  'aria-expanded'?: boolean;
  'aria-haspopup'?: 'menu' | 'dialog' | 'true';
}

type Trigger = ReactElement<TriggerProps>;

function useOutsideDismiss(open: boolean, ref: RefObject<HTMLElement | null>, onClose: () => void) {
  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: PointerEvent) => {
      if (event.target instanceof Node && !ref.current?.contains(event.target)) onClose();
    };
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.addEventListener('pointerdown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('pointerdown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open, onClose, ref]);
}

/* ---------- Tooltip ---------- */
export interface TooltipProps {
  content: ReactNode;
  children: Trigger;
  placement?: 'top' | 'bottom';
}

export function Tooltip({ content, children, placement = 'top' }: TooltipProps) {
  const [open, setOpen] = useState(false);
  const id = useId();
  const trigger = cloneElement(children, {
    'aria-describedby': [children.props['aria-describedby'], id].filter(Boolean).join(' '),
  });
  return (
    <span
      className="ic-tooltip-anchor"
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
      onFocusCapture={() => setOpen(true)}
      onBlurCapture={() => setOpen(false)}
    >
      {trigger}
      {open ? (
        <span id={id} role="tooltip" className={cx('ic-tooltip', `ic-tooltip-${placement}`, 'is-open')}>
          {content}
        </span>
      ) : null}
    </span>
  );
}

/* ---------- Menu ---------- */
export interface MenuProps {
  children: ReactNode;
  label?: string;
  onClose?: () => void;
  autoFocus?: boolean;
  className?: string;
}

const menuItems = (element: HTMLElement) =>
  Array.from(element.querySelectorAll<HTMLElement>('[role="menuitem"], [role="menuitemcheckbox"]')).filter(
    (item) => item.getAttribute('aria-disabled') !== 'true',
  );

export function Menu({ children, label = 'Options', onClose, autoFocus = false, className }: MenuProps) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (autoFocus) queueMicrotask(() => {
      if (ref.current) menuItems(ref.current)[0]?.focus();
    });
  }, [autoFocus]);

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const items = menuItems(event.currentTarget);
    const current = items.indexOf(document.activeElement as HTMLElement);
    let next: number | undefined;
    if (event.key === 'ArrowDown') next = (current + 1) % items.length;
    if (event.key === 'ArrowUp') next = (current - 1 + items.length) % items.length;
    if (event.key === 'Home') next = 0;
    if (event.key === 'End') next = items.length - 1;
    if (event.key === 'Escape') onClose?.();
    const nextItem = next == null ? undefined : items[next];
    if (nextItem) {
      event.preventDefault();
      nextItem.focus();
    }
  };
  return (
    <div ref={ref} role="menu" aria-label={label} className={cx('ic-menu', className)} onKeyDown={onKeyDown}>
      {children}
    </div>
  );
}

export interface MenuItemProps {
  label: string;
  icon?: IconName;
  selected?: boolean;
  danger?: boolean;
  disabled?: boolean;
  onSelect?: () => void;
}

export function MenuItem({ label, icon, selected, danger, disabled, onSelect }: MenuItemProps) {
  return (
    <button
      type="button"
      role={selected == null ? 'menuitem' : 'menuitemcheckbox'}
      aria-checked={selected == null ? undefined : selected}
      aria-disabled={disabled || undefined}
      disabled={disabled}
      className={cx('ic-menu-item', selected && 'is-selected', danger && 'is-danger')}
      onClick={onSelect}
    >
      {selected || icon ? <Icon name={selected ? 'check' : icon!} size={18} /> : <span className="ic-menu-gap" />}
      <span className="ic-menu-label">{label}</span>
    </button>
  );
}

export function MenuDivider() {
  return <div className="ic-menu-divider" role="separator" />;
}

export function MenuHeading({ children }: { children: ReactNode }) {
  return <div className="ic-menu-heading">{children}</div>;
}

/* ---------- Dropdown ---------- */
export interface DropdownProps {
  trigger: Trigger;
  children: ReactNode;
  open?: boolean;
  defaultOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
  align?: 'start' | 'end';
}

export function Dropdown({ trigger, children, open, defaultOpen = false, onOpenChange, align = 'start' }: DropdownProps) {
  const [shown, setShown] = useControlled(open, defaultOpen, onOpenChange);
  const ref = useRef<HTMLSpanElement>(null);
  const close = () => {
    setShown(false);
    ref.current?.querySelector<HTMLElement>('button, a')?.focus();
  };
  useOutsideDismiss(shown, ref, close);
  const control = cloneElement(trigger, {
    'aria-haspopup': 'menu',
    'aria-expanded': shown,
    onClick: (event) => {
      trigger.props.onClick?.(event);
      if (!event.defaultPrevented) setShown(!shown);
    },
  });
  return (
    <span className="ic-dropdown" ref={ref}>
      {control}
      {shown ? (
        <span className={cx('ic-dropdown-panel', `ic-align-${align}`)} onClick={(event) => {
          if ((event.target as Element).closest('[role="menuitem"], [role="menuitemcheckbox"]')) close();
        }}>
          {children}
        </span>
      ) : null}
    </span>
  );
}

/* ---------- ContextMenu ---------- */
export interface ContextMenuProps {
  children: ReactNode;
  menu: ReactNode;
}

export function ContextMenu({ children, menu }: ContextMenuProps) {
  const [position, setPosition] = useState<{ x: number; y: number } | null>(null);
  const ref = useRef<HTMLDivElement>(null);
  const close = () => setPosition(null);
  useOutsideDismiss(position != null, ref, close);
  return (
    <div
      className="ic-context"
      ref={ref}
      onContextMenu={(event) => {
        event.preventDefault();
        const bounds = event.currentTarget.getBoundingClientRect();
        setPosition({ x: event.clientX - bounds.left, y: event.clientY - bounds.top });
        queueMicrotask(() => ref.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus());
      }}
    >
      {children}
      {position ? (
        <div className="ic-context-panel" style={{ left: position.x, top: position.y }} onClick={close}>
          {menu}
        </div>
      ) : null}
    </div>
  );
}

/* ---------- Popover ---------- */
export interface PopoverProps {
  trigger: Trigger;
  title?: string;
  children: ReactNode;
  footer?: ReactNode;
  open?: boolean;
  defaultOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
  align?: 'start' | 'end';
}

export function Popover({ trigger, title, children, footer, open, defaultOpen = false, onOpenChange, align = 'start' }: PopoverProps) {
  const [shown, setShown] = useControlled(open, defaultOpen, onOpenChange);
  const ref = useRef<HTMLSpanElement>(null);
  const close = () => {
    setShown(false);
    queueMicrotask(() => ref.current?.querySelector<HTMLElement>('button, a')?.focus());
  };
  useOutsideDismiss(shown, ref, close);
  const control = cloneElement(trigger, {
    'aria-haspopup': 'dialog',
    'aria-expanded': shown,
    onClick: (event) => {
      trigger.props.onClick?.(event);
      if (!event.defaultPrevented) setShown(!shown);
    },
  });
  return (
    <span className="ic-dropdown" ref={ref}>
      {control}
      {shown ? (
        <div role="dialog" aria-label={title ?? 'Details'} className={cx('ic-dropdown-panel', `ic-align-${align}`, 'ic-popover')}>
          {title ? (
            <div className="ic-popover-head">
              <span className="ic-popover-title">{title}</span>
              <IconButton icon="close" label="Close" size="sm" onClick={close} />
            </div>
          ) : null}
          <div className="ic-popover-body">{children}</div>
          {footer ? <div className="ic-popover-foot">{footer}</div> : null}
        </div>
      ) : null}
    </span>
  );
}

const FOCUSABLE = 'button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])';

function useDialogLifecycle(open: boolean, onClose: () => void, ref: RefObject<HTMLElement | null>) {
  useEffect(() => {
    if (!open) return;
    const previous = document.activeElement as HTMLElement | null;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    queueMicrotask(() => ref.current?.querySelector<HTMLElement>(FOCUSABLE)?.focus());
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        onClose();
        return;
      }
      if (event.key !== 'Tab' || !ref.current) return;
      const focusable = Array.from(ref.current.querySelectorAll<HTMLElement>(FOCUSABLE));
      const first = focusable[0];
      const last = focusable.at(-1);
      if (!first || !last) return;
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      document.body.style.overflow = overflow;
      previous?.focus();
    };
  }, [onClose, open, ref]);
}

interface DialogBaseProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children?: ReactNode;
  footer?: ReactNode;
  closeLabel?: string;
}

/* ---------- Modal ---------- */
export interface ModalProps extends DialogBaseProps {
  description?: string;
  size?: 'sm' | 'md' | 'lg';
}

export function Modal({ open, onClose, title, description, children, footer, closeLabel = 'Close', size = 'md' }: ModalProps) {
  const ref = useRef<HTMLDivElement>(null);
  const titleId = useId();
  const descriptionId = useId();
  useDialogLifecycle(open, onClose, ref);
  if (!open || typeof document === 'undefined') return null;
  return createPortal(
    <div className="ic-scrim" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <div
        ref={ref}
        className={cx('ic-modal', `ic-modal-${size}`)}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descriptionId : undefined}
      >
        <div className="ic-modal-head">
          <div>
            <h2 id={titleId} className="ic-modal-title">{title}</h2>
            {description ? <p id={descriptionId} className="ic-modal-desc">{description}</p> : null}
          </div>
          <IconButton icon="close" label={closeLabel} onClick={onClose} />
        </div>
        {children ? <div className="ic-modal-body">{children}</div> : null}
        {footer ? <div className="ic-modal-foot">{footer}</div> : null}
      </div>
    </div>,
    document.body,
  );
}

/* ---------- Drawer ---------- */
export interface DrawerProps extends DialogBaseProps {
  placement?: 'right' | 'bottom';
}

export function Drawer({ open, onClose, title, children, footer, closeLabel = 'Close', placement = 'right' }: DrawerProps) {
  const ref = useRef<HTMLDivElement>(null);
  const titleId = useId();
  useDialogLifecycle(open, onClose, ref);
  if (!open || typeof document === 'undefined') return null;
  return createPortal(
    <div className="ic-scrim ic-scrim-drawer" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <aside ref={ref} className={cx('ic-drawer', `ic-drawer-${placement}`)} role="dialog" aria-modal="true" aria-labelledby={titleId}>
        {placement === 'bottom' ? <span className="ic-drawer-grabber" aria-hidden="true" /> : null}
        <div className="ic-drawer-head">
          <h2 id={titleId} className="ic-drawer-title">{title}</h2>
          <IconButton icon="close" label={closeLabel} onClick={onClose} />
        </div>
        <div className="ic-drawer-body">{children}</div>
        {footer ? <div className="ic-drawer-foot">{footer}</div> : null}
      </aside>
    </div>,
    document.body,
  );
}

/* ---------- Toast ---------- */
export type ToastTone = 'neutral' | 'success' | 'warning' | 'danger' | 'info';

const TOAST_ICON: Record<ToastTone, IconName> = {
  neutral: 'info',
  success: 'check-circle',
  warning: 'alert',
  danger: 'alert',
  info: 'info',
};

export interface ToastProps {
  title: string;
  description?: string;
  tone?: ToastTone;
  actionLabel?: string;
  onAction?: () => void;
  onDismiss?: () => void;
}

export function Toast({ title, description, tone = 'neutral', actionLabel, onAction, onDismiss }: ToastProps) {
  return (
    <div className={cx('ic-toast', `ic-toast-${tone}`)} role={tone === 'danger' ? 'alert' : 'status'}>
      <Icon name={TOAST_ICON[tone]} size={18} className="ic-toast-icon" />
      <div className="ic-toast-text">
        <div className="ic-toast-title">{title}</div>
        {description ? <div className="ic-toast-desc">{description}</div> : null}
      </div>
      {actionLabel && onAction ? <button type="button" className="ic-toast-action" onClick={onAction}>{actionLabel}</button> : null}
      {onDismiss ? <IconButton icon="close" label="Dismiss" size="sm" onClick={onDismiss} /> : null}
    </div>
  );
}
