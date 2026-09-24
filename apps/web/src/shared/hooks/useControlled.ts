import { useState } from 'react';

/** Controlled/uncontrolled value helper used by Design System components. */
export function useControlled<T>(value: T | undefined, defaultValue: T, onChange?: (v: T) => void): [T, (v: T) => void] {
  const [inner, setInner] = useState<T>(defaultValue);
  const isControlled = value !== undefined;
  const current = isControlled ? value : inner;
  const set = (v: T) => {
    if (!isControlled) setInner(v);
    onChange?.(v);
  };
  return [current, set];
}
