// Ported verbatim from the ICYRE Design System (components/src/icons.ts).
// ICYRE icon set — 24×24 grid, 1.75 stroke, round caps and joins, 2px safe area.
// Every glyph is drawn to the same optical weight; `filled` is allowed only for play, pause, heart.
export const ICONS = {
  play: '<path d="M7.5 5.8v12.4c0 .8.9 1.3 1.6.9l9.9-6.2c.6-.4.6-1.3 0-1.7L9.1 4.9c-.7-.4-1.6.1-1.6.9z"/>',
  pause: '<rect x="6.5" y="5" width="3.5" height="14" rx="1.2"/><rect x="14" y="5" width="3.5" height="14" rx="1.2"/>',
  next: '<path d="M6 6.2v11.6c0 .7.8 1.1 1.4.7l8.1-5.8c.5-.4.5-1.1 0-1.4L7.4 5.5C6.8 5.1 6 5.5 6 6.2z"/><path d="M18.5 5.5v13"/>',
  previous: '<path d="M18 6.2v11.6c0 .7-.8 1.1-1.4.7l-8.1-5.8c-.5-.4-.5-1.1 0-1.4l8.1-5.8c.6-.4 1.4 0 1.4.7z"/><path d="M5.5 5.5v13"/>',
  shuffle: '<path d="M3.5 7h3.2c1.6 0 3 .8 3.9 2.1l2.8 5.8c.9 1.3 2.3 2.1 3.9 2.1h3.2"/><path d="M3.5 17h3.2c1.2 0 2.3-.5 3.1-1.3"/><path d="M13.7 8.3c.8-.8 1.9-1.3 3.1-1.3h3.7"/><path d="M18 4.5 20.5 7 18 9.5"/><path d="m18 14.5 2.5 2.5-2.5 2.5"/>',
  repeat: '<path d="M4 11V9.5A3.5 3.5 0 0 1 7.5 6H19"/><path d="M16.5 3.5 19 6l-2.5 2.5"/><path d="M20 13v1.5a3.5 3.5 0 0 1-3.5 3.5H5"/><path d="M7.5 20.5 5 18l2.5-2.5"/>',
  volume: '<path d="M4 9.5v5c0 .6.4 1 1 1h2.5l4.3 3.6c.6.5 1.7.1 1.7-.8V5.7c0-.9-1-1.3-1.7-.8L7.5 8.5H5c-.6 0-1 .4-1 1z"/><path d="M16.5 9a4.2 4.2 0 0 1 0 6"/><path d="M19 6.5a7.8 7.8 0 0 1 0 11"/>',
  mute: '<path d="M4 9.5v5c0 .6.4 1 1 1h2.5l4.3 3.6c.6.5 1.7.1 1.7-.8V5.7c0-.9-1-1.3-1.7-.8L7.5 8.5H5c-.6 0-1 .4-1 1z"/><path d="m16.5 9.5 5 5m0-5-5 5"/>',
  search: '<circle cx="11" cy="11" r="6.5"/><path d="m16 16 4.5 4.5"/>',
  home: '<path d="M4 10.2 12 4l8 6.2V19a1 1 0 0 1-1 1h-4.5v-5.5h-5V20H5a1 1 0 0 1-1-1z"/>',
  library: '<rect x="4" y="7" width="11" height="13" rx="2"/><path d="M8 4h9a3 3 0 0 1 3 3v10"/>',
  heart: '<path d="M12 19.5s-7.5-4.4-7.5-10A4.2 4.2 0 0 1 12 7a4.2 4.2 0 0 1 7.5 2.5c0 5.6-7.5 10-7.5 10z"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  more: '<circle cx="5.5" cy="12" r="1.4" fill="currentColor" stroke="none"/><circle cx="12" cy="12" r="1.4" fill="currentColor" stroke="none"/><circle cx="18.5" cy="12" r="1.4" fill="currentColor" stroke="none"/>',
  queue: '<path d="M4 6.5h16M4 12h10M4 17.5h7"/><path d="M16 14.5v6l4.5-3z"/>',
  user: '<circle cx="12" cy="8.5" r="3.8"/><path d="M4.5 20c1-3.6 4-5.5 7.5-5.5s6.5 1.9 7.5 5.5"/>',
  settings: '<path d="M5 4v6M5 14v6M12 4v3M12 11v9M19 4v9M19 17v3"/><circle cx="5" cy="12" r="2"/><circle cx="12" cy="9" r="2"/><circle cx="19" cy="15" r="2"/>',
  chevron: '<path d="m9.5 6 6 6-6 6"/>',
  'chevron-down': '<path d="m6 9.5 6 6 6-6"/>',
  close: '<path d="m6.5 6.5 11 11m0-11-11 11"/>',
  check: '<path d="m5.5 12.5 4 4 9-9"/>',
  spark: '<path d="M12 3.5c.5 4.3 2.2 6 6.5 6.5-4.3.5-6 2.2-6.5 6.5-.5-4.3-2.2-6-6.5-6.5 4.3-.5 6-2.2 6.5-6.5z"/>',
  trending: '<path d="m4 16.5 5-5 3.5 3.5L20 7.5"/><path d="M15 7.5h5v5"/>',
  alert: '<circle cx="12" cy="12" r="8.5"/><path d="M12 8v4.5"/><circle cx="12" cy="16" r=".9" fill="currentColor" stroke="none"/>',
  'check-circle': '<circle cx="12" cy="12" r="8.5"/><path d="m8.5 12.2 2.4 2.4 4.6-4.8"/>',
  info: '<circle cx="12" cy="12" r="8.5"/><path d="M12 11v5"/><circle cx="12" cy="8" r=".9" fill="currentColor" stroke="none"/>',
  clock: '<circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3 2"/>',
  grip: '<circle cx="9" cy="7" r="1.1" fill="currentColor" stroke="none"/><circle cx="15" cy="7" r="1.1" fill="currentColor" stroke="none"/><circle cx="9" cy="12" r="1.1" fill="currentColor" stroke="none"/><circle cx="15" cy="12" r="1.1" fill="currentColor" stroke="none"/><circle cx="9" cy="17" r="1.1" fill="currentColor" stroke="none"/><circle cx="15" cy="17" r="1.1" fill="currentColor" stroke="none"/>',
  menu: '<path d="M4 7h16M4 12h16M4 17h16"/>',
} as const;

export type IconName = keyof typeof ICONS;
export const FILLABLE: ReadonlySet<IconName> = new Set<IconName>(['play', 'pause', 'heart', 'spark']);
