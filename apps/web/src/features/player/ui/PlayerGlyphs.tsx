// Canvas-only glyphs (not in the Design System icon set yet), drawn to the DS
// icon grid: 24px, 1.75 stroke, round caps.
export function DeviceGlyph() {
  return (
    <svg
      className="ic-icon"
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <rect x="6" y="3.5" width="12" height="17" rx="2.5" />
      <circle cx="12" cy="14" r="3" />
      <path d="M12 7.5h.01" />
    </svg>
  );
}

export function ExpandGlyph() {
  return (
    <svg
      className="ic-icon"
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M14.5 4.5h5v5" />
      <path d="M9.5 19.5h-5v-5" />
      <path d="M19.5 4.5 14 10" />
      <path d="M4.5 19.5 10 14" />
    </svg>
  );
}
