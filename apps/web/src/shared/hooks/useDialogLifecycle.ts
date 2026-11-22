import { useEffect, type RefObject } from 'react';

const FOCUSABLE =
  'button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])';

/** Focus trap, Escape dismissal, body scroll lock, and focus restoration for modal dialogs. */
export function useDialogLifecycle(open: boolean, onClose: () => void, ref: RefObject<HTMLElement | null>) {
  useEffect(() => {
    if (!open) return;
    const previous = document.activeElement as HTMLElement | null;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    queueMicrotask(() => ref.current?.querySelector<HTMLElement>(FOCUSABLE)?.focus());
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') {
        if (event.defaultPrevented) return;
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
