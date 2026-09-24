// Ported from the ICYRE Design System (components/src/forms.tsx) — the
// controls needed so far. Input, Checkbox, Switch… arrive with their screens.
import type { CSSProperties } from 'react';

import { useControlled } from '@/shared/hooks/useControlled';
import { cx } from '@/shared/lib/cx';

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
