// Ported from the ICYRE Design System (components/src/forms.tsx) — the
// controls needed so far. Input, Checkbox, Switch… arrive with their screens.

import { forwardRef, useImperativeHandle, useRef, type CSSProperties, type KeyboardEvent } from 'react';

import { useControlled } from '@/shared/hooks/useControlled';
import { cx } from '@/shared/lib/cx';

import { Icon, IconButton } from './core';

/* ---------- Slider ---------- */
export interface SliderProps {
  value?: number;
  defaultValue?: number;
  min?: number;
  max?: number;
  step?: number;
  buffered?: number;
  tone?: 'neutral' | 'iridescent';
  size?: 'sm' | 'md';
  label: string;
  valueText?: string;
  disabled?: boolean;
  showThumb?: 'always' | 'hover';
  onChange?: (v: number) => void;
}

export function Slider({
  value,
  defaultValue = 0,
  min = 0,
  max = 100,
  step = 1,
  buffered,
  tone = 'neutral',
  size = 'md',
  label,
  valueText,
  disabled,
  showThumb = 'always',
  onChange,
}: SliderProps) {
  const [v, setV] = useControlled<number>(value, defaultValue, onChange);
  const pct = max > min ? Math.min(100, Math.max(0, ((v - min) / (max - min)) * 100)) : 0;
  const bpct = buffered != null && max > min ? Math.min(100, ((buffered - min) / (max - min)) * 100) : null;
  return (
    <div
      className={cx('ic-slider', `ic-slider-${tone}`, `ic-slider-${size}`, `ic-thumb-${showThumb}`, disabled && 'is-disabled')}
      style={{ '--pct': `${pct}%` } as CSSProperties}
    >
      <div className="ic-slider-track">
        {bpct != null && <div className="ic-slider-buffer" style={{ width: `${bpct}%` }} />}
        <div className="ic-slider-fill" />
      </div>
      <input
        className="ic-slider-input"
        type="range"
        min={min}
        max={max}
        step={step}
        value={v}
        disabled={disabled}
        aria-label={label}
        aria-valuetext={valueText}
        onChange={(e) => setV(Number(e.target.value))}
      />
      <div className="ic-slider-thumb" aria-hidden="true" />
    </div>
  );
}

/* ---------- SearchInput ---------- */
export interface SearchInputProps {
  value?: string;
  defaultValue?: string;
  onChange?: (v: string) => void;
  /** Enter pressed. */
  onSubmit?: (v: string) => void;
  placeholder?: string;
  shortcut?: string;
  size?: 'md' | 'lg';
  forceState?: 'hover' | 'focus';
  label?: string;
  autoFocus?: boolean;
}

export const SearchInput = forwardRef<HTMLInputElement, SearchInputProps>(function SearchInput(
  { value, defaultValue = '', onChange, onSubmit, placeholder = 'Artists, albums, tracks', shortcut = '⌘K', size = 'md', forceState, label = 'Search', autoFocus },
  forwarded,
) {
  const [v, setV] = useControlled<string>(value, defaultValue, onChange);
  const ref = useRef<HTMLInputElement>(null);
  useImperativeHandle(forwarded, () => ref.current as HTMLInputElement);
  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') onSubmit?.(v);
    if (e.key === 'Escape' && v) {
      e.preventDefault();
      setV('');
    }
  };
  return (
    <div role="search" className={cx('ic-search', `ic-search-${size}`, v && 'is-filled', forceState && `is-${forceState}`)}>
      <Icon name="search" size={18} className="ic-search-icon" />
      <input
        ref={ref}
        className="ic-search-input"
        type="search"
        aria-label={label}
        placeholder={placeholder}
        value={v}
        autoFocus={autoFocus}
        onChange={(e) => setV(e.target.value)}
        onKeyDown={onKeyDown}
      />
      {v ? (
        <IconButton
          icon="close"
          label="Clear search"
          size="sm"
          className="ic-search-clear"
          onClick={() => {
            setV('');
            ref.current?.focus();
          }}
        />
      ) : shortcut ? (
        <kbd className="ic-kbd">{shortcut}</kbd>
      ) : null}
    </div>
  );
});
