The base card primitive for any media object — artwork in a thin frame plus a two-line meta block.

**Provide:** `title`, `subtitle`, `cover` (image URL) or `art` (generative fallback 0–7), `shape`, `badge`, `explicit`, `state`, `onOpen`, `onPlay`.

- Frame: `space-1` padding around `radius-md` artwork inside `radius-card` (14 = 10 + 4).
- States: `hover` lifts to `surface` + `border-subtle` + `shadow-raised` and reveals the round play button; `selected` = `selected` fill + 1.5px `border-selected`; `playing` = `iridescent-soft` fill + `iridescent-border` frame + PlaybackState bars + `text-brand` title; `disabled` = `opacity-disabled`; `loading` = skeleton.
- Use AlbumCard / ArtistCard / PlaylistCard rather than MediaCard directly.
