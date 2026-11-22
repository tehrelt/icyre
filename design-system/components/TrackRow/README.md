Dense track-list row: index/play, cover, title + artist, album, badge, like, duration, more.

**Provide:** `index`, `title`, `artist`, `album`, `duration` (seconds), `cover`/`art`, `badge`, `explicit`, `liked`, `state`, `showCover`, `onPlay`, `onLike`, `onMore`, `onSelect`. Pair with `TrackListHeader` at the top.

- 56px min height, `inset-row` padding — the densest rhythm in the system.
- Hover: `surface-hover`; the index becomes a play glyph; like and more fade in.
- Selected: `selected` fill + faint brand border. Playing: `iridescent-selected` ground + animated PlaybackState in place of the index + `text-brand` title (three cues, not one). Paused keeps the ground with still bars.
- Unavailable: all text `text-disabled`, greyed cover, an "Unavailable" badge, no play/like.
- Double-click plays; the row's own buttons stop propagation.
