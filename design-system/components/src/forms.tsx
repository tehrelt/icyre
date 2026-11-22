import { React, cx, Icon, IconButton, useControlled, type Size } from './core';
import type { IconName } from './icons';

/* ---------- Field wrapper ---------- */
interface FieldShellProps {
  id: string;
  label?: string;
  hint?: string;
  error?: string;
  counter?: string;
  children: any;
}
function FieldShell({ id, label, hint, error, counter, children }: FieldShellProps) {
  return (
    <div className="ic-fieldwrap">
      {label && (
        <label className="ic-label" htmlFor={id}>
          {label}
        </label>
      )}
      {children}
      {(error || hint || counter) && (
        <div className="ic-help">
          {error ? (
            <span className="ic-help-error" id={id + '-msg'}>
              <Icon name="alert" size={14} />
              {error}
            </span>
          ) : hint ? (
            <span id={id + '-msg'}>{hint}</span>
          ) : (
            <span />
          )}
          {counter && <span className="ic-help-counter">{counter}</span>}
        </div>
      )}
    </div>
  );
}

/* ---------- Input ---------- */
export interface InputProps {
  label?: string;
  hint?: string;
  error?: string;
  iconStart?: IconName;
  size?: Size;
  forceState?: 'hover' | 'focus';
  disabled?: boolean;
  id?: string;
  [rest: string]: any;
}
export function Input({ label, hint, error, iconStart, size = 'md', forceState, disabled, id, ...rest }: InputProps) {
  const auto = React.useId();
  const fid = id ?? 'ic-in' + auto.replace(/:/g, '');
  const filled = rest.value != null ? String(rest.value).length > 0 : rest.defaultValue != null && String(rest.defaultValue).length > 0;
  return (
    <FieldShell id={fid} label={label} hint={hint} error={error}>
      <div className={cx('ic-field', 'ic-field-' + size, error && 'is-error', disabled && 'is-disabled', filled && 'is-filled', forceState && 'is-' + forceState)}>
        {iconStart && <Icon name={iconStart} size={18} className="ic-field-icon" />}
        <input id={fid} className="ic-field-input" disabled={disabled} aria-invalid={error ? true : undefined} aria-describedby={error || hint ? fid + '-msg' : undefined} {...rest} />
      </div>
    </FieldShell>
  );
}

/* ---------- SearchInput ---------- */
export interface SearchInputProps {
  value?: string;
  defaultValue?: string;
  onChange?: (v: string) => void;
  placeholder?: string;
  shortcut?: string;
  size?: 'md' | 'lg';
  forceState?: 'hover' | 'focus';
  label?: string;
}
export function SearchInput({ value, defaultValue = '', onChange, placeholder = 'Artists, albums, tracks', shortcut = '⌘K', size = 'md', forceState, label = 'Search' }: SearchInputProps) {
  const [v, setV] = useControlled<string>(value, defaultValue, onChange);
  const ref = React.useRef(null);
  return (
    <div role="search" className={cx('ic-search', 'ic-search-' + size, v && 'is-filled', forceState && 'is-' + forceState)}>
      <Icon name="search" size={18} className="ic-search-icon" />
      <input ref={ref} className="ic-search-input" type="search" aria-label={label} placeholder={placeholder} value={v} onChange={(e: any) => setV(e.target.value)} />
      {v ? (
        <IconButton
          icon="close"
          label="Clear search"
          size="sm"
          className="ic-search-clear"
          onClick={() => {
            setV('');
            (ref.current as any)?.focus();
          }}
        />
      ) : shortcut ? (
        <kbd className="ic-kbd">{shortcut}</kbd>
      ) : null}
    </div>
  );
}

/* ---------- Textarea ---------- */
export interface TextareaProps {
  label?: string;
  hint?: string;
  error?: string;
  maxLength?: number;
  rows?: number;
  value?: string;
  defaultValue?: string;
  onChange?: (v: string) => void;
  disabled?: boolean;
  placeholder?: string;
  forceState?: 'hover' | 'focus';
}
export function Textarea({ label, hint, error, maxLength, rows = 4, value, defaultValue = '', onChange, disabled, placeholder, forceState }: TextareaProps) {
  const auto = React.useId();
  const fid = 'ic-ta' + auto.replace(/:/g, '');
  const [v, setV] = useControlled<string>(value, defaultValue, onChange);
  return (
    <FieldShell id={fid} label={label} hint={hint} error={error} counter={maxLength ? v.length + '/' + maxLength : undefined}>
      <div className={cx('ic-field', 'ic-field-area', error && 'is-error', disabled && 'is-disabled', v && 'is-filled', forceState && 'is-' + forceState)}>
        <textarea id={fid} className="ic-field-input" rows={rows} maxLength={maxLength} disabled={disabled} placeholder={placeholder} value={v} onChange={(e: any) => setV(e.target.value)} aria-invalid={error ? true : undefined} aria-describedby={error || hint ? fid + '-msg' : undefined} />
      </div>
    </FieldShell>
  );
}

/* ---------- Checkbox ---------- */
export interface CheckboxProps {
  label?: string;
  description?: string;
  checked?: boolean;
  defaultChecked?: boolean;
  indeterminate?: boolean;
  disabled?: boolean;
  onChange?: (v: boolean) => void;
}
export function Checkbox({ label, description, checked, defaultChecked = false, indeterminate, disabled, onChange }: CheckboxProps) {
  const [v, setV] = useControlled<boolean>(checked, defaultChecked, onChange);
  const ref = React.useRef(null);
  React.useEffect(() => {
    if (ref.current) (ref.current as any).indeterminate = !!indeterminate;
  }, [indeterminate]);
  return (
    <label className={cx('ic-check', disabled && 'is-disabled')}>
      <input ref={ref} type="checkbox" className="ic-check-input" checked={v} disabled={disabled} onChange={(e: any) => setV(e.target.checked)} />
      <span className="ic-check-box" aria-hidden="true">
        {indeterminate ? <span className="ic-check-dash" /> : <Icon name="check" size={14} />}
      </span>
      {(label || description) && (
        <span className="ic-check-text">
          {label && <span className="ic-check-label">{label}</span>}
          {description && <span className="ic-check-desc">{description}</span>}
        </span>
      )}
    </label>
  );
}

/* ---------- Radio ---------- */
export interface RadioOption {
  value: string;
  label: string;
  description?: string;
  disabled?: boolean;
}
export interface RadioGroupProps {
  name: string;
  label?: string;
  options: RadioOption[];
  value?: string;
  defaultValue?: string;
  onChange?: (v: string) => void;
}
export function RadioGroup({ name, label, options, value, defaultValue, onChange }: RadioGroupProps) {
  const [v, setV] = useControlled<string>(value, defaultValue ?? options[0]?.value ?? '', onChange);
  return (
    <fieldset className="ic-radiogroup">
      {label && <legend className="ic-label">{label}</legend>}
      {options.map((o) => (
        <Radio key={o.value} name={name} value={o.value} label={o.label} description={o.description} disabled={o.disabled} checked={v === o.value} onChange={() => setV(o.value)} />
      ))}
    </fieldset>
  );
}
export interface RadioProps {
  name?: string;
  value?: string;
  label?: string;
  description?: string;
  checked?: boolean;
  disabled?: boolean;
  onChange?: () => void;
}
export function Radio({ name, value, label, description, checked, disabled, onChange }: RadioProps) {
  return (
    <label className={cx('ic-check', 'ic-radio', disabled && 'is-disabled')}>
      <input type="radio" className="ic-check-input" name={name} value={value} checked={!!checked} disabled={disabled} onChange={() => onChange?.()} />
      <span className="ic-check-box ic-radio-dot" aria-hidden="true" />
      {(label || description) && (
        <span className="ic-check-text">
          {label && <span className="ic-check-label">{label}</span>}
          {description && <span className="ic-check-desc">{description}</span>}
        </span>
      )}
    </label>
  );
}

/* ---------- Switch ---------- */
export interface SwitchProps {
  label?: string;
  description?: string;
  checked?: boolean;
  defaultChecked?: boolean;
  disabled?: boolean;
  onChange?: (v: boolean) => void;
}
export function Switch({ label, description, checked, defaultChecked = false, disabled, onChange }: SwitchProps) {
  const [v, setV] = useControlled<boolean>(checked, defaultChecked, onChange);
  return (
    <label className={cx('ic-switch', disabled && 'is-disabled')}>
      <input type="checkbox" role="switch" className="ic-check-input" checked={v} disabled={disabled} onChange={(e: any) => setV(e.target.checked)} />
      <span className="ic-switch-track" aria-hidden="true">
        <span className="ic-switch-thumb" />
      </span>
      {(label || description) && (
        <span className="ic-check-text">
          {label && <span className="ic-check-label">{label}</span>}
          {description && <span className="ic-check-desc">{description}</span>}
        </span>
      )}
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
export function Slider({ value, defaultValue = 0, min = 0, max = 100, step = 1, buffered, tone = 'neutral', size = 'md', label, valueText, disabled, showThumb = 'always', onChange }: SliderProps) {
  const [v, setV] = useControlled<number>(value, defaultValue, onChange);
  const pct = max > min ? ((v - min) / (max - min)) * 100 : 0;
  const bpct = buffered != null && max > min ? ((buffered - min) / (max - min)) * 100 : null;
  return (
    <div className={cx('ic-slider', 'ic-slider-' + tone, 'ic-slider-' + size, 'ic-thumb-' + showThumb, disabled && 'is-disabled')} style={{ '--pct': pct + '%' }}>
      <div className="ic-slider-track">
        {bpct != null && <div className="ic-slider-buffer" style={{ width: bpct + '%' }} />}
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
        onChange={(e: any) => setV(Number(e.target.value))}
      />
      <div className="ic-slider-thumb" aria-hidden="true" />
    </div>
  );
}
