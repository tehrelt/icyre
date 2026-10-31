// Ported from the ICYRE Design System (components/src/forms.tsx) — the
// controls needed so far. Input, Checkbox, Switch… arrive with their screens.

import {
  forwardRef,
  useEffect,
  useId,
  useImperativeHandle,
  useRef,
  type CSSProperties,
  type InputHTMLAttributes,
  type KeyboardEvent,
  type ReactNode,
} from 'react';

import { useControlled } from '@/shared/hooks/useControlled';
import { cx } from '@/shared/lib/cx';

import { Icon, IconButton } from './core';

interface ChoiceProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'children' | 'onChange' | 'type'> {
  label: ReactNode;
  description?: ReactNode;
  onChange?: (checked: boolean) => void;
}

function ChoiceText({
  label,
  description,
  labelId,
  descriptionId,
}: Pick<ChoiceProps, 'label' | 'description'> & { labelId: string; descriptionId: string }) {
  return (
    <span className="ic-check-text">
      <span className="ic-check-label" id={labelId}>
        {label}
      </span>
      {description != null ? (
        <span className="ic-check-desc" id={descriptionId}>
          {description}
        </span>
      ) : null}
    </span>
  );
}

/* ---------- Checkbox ---------- */
export interface CheckboxProps extends ChoiceProps {
  indeterminate?: boolean;
}

export function Checkbox({ label, description, indeterminate = false, disabled, className, onChange, ...inputProps }: CheckboxProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const baseId = useId();
  const labelId = `${baseId}-label`;
  const descriptionId = `${baseId}-description`;

  useEffect(() => {
    if (inputRef.current) inputRef.current.indeterminate = indeterminate;
  }, [indeterminate]);

  return (
    <label className={cx('ic-check', disabled && 'is-disabled', className)}>
      <input
        {...inputProps}
        ref={inputRef}
        className="ic-check-input"
        type="checkbox"
        disabled={disabled}
        aria-labelledby={labelId}
        aria-checked={indeterminate ? 'mixed' : undefined}
        aria-describedby={description != null ? descriptionId : undefined}
        onChange={(event) => onChange?.(event.target.checked)}
      />
      <span className="ic-check-box" aria-hidden="true">
        {indeterminate ? <span className="ic-check-dash" /> : <Icon name="check" size={13} />}
      </span>
      <ChoiceText label={label} description={description} labelId={labelId} descriptionId={descriptionId} />
    </label>
  );
}

/* ---------- Radio ---------- */
export type RadioProps = ChoiceProps;

export function Radio({ label, description, disabled, className, onChange, ...inputProps }: RadioProps) {
  const baseId = useId();
  const labelId = `${baseId}-label`;
  const descriptionId = `${baseId}-description`;
  return (
    <label className={cx('ic-check', 'ic-radio', disabled && 'is-disabled', className)}>
      <input
        {...inputProps}
        className="ic-check-input"
        type="radio"
        disabled={disabled}
        aria-labelledby={labelId}
        aria-describedby={description != null ? descriptionId : undefined}
        onChange={(event) => onChange?.(event.target.checked)}
      />
      <span className="ic-check-box ic-radio-dot" aria-hidden="true" />
      <ChoiceText label={label} description={description} labelId={labelId} descriptionId={descriptionId} />
    </label>
  );
}

/* ---------- Switch ---------- */
export type SwitchProps = ChoiceProps;

export function Switch({ label, description, disabled, className, onChange, ...inputProps }: SwitchProps) {
  const baseId = useId();
  const labelId = `${baseId}-label`;
  const descriptionId = `${baseId}-description`;
  return (
    <label className={cx('ic-switch', disabled && 'is-disabled', className)}>
      <input
        {...inputProps}
        className="ic-check-input"
        type="checkbox"
        role="switch"
        disabled={disabled}
        aria-labelledby={labelId}
        aria-describedby={description != null ? descriptionId : undefined}
        onChange={(event) => onChange?.(event.target.checked)}
      />
      <span className="ic-switch-track" aria-hidden="true">
        <span className="ic-switch-thumb" />
      </span>
      <ChoiceText label={label} description={description} labelId={labelId} descriptionId={descriptionId} />
    </label>
  );
}

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
