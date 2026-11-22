Shimmering placeholder that mirrors the final layout while content loads.

**Provide:** `shape` (`text` | `rect` | `circle`), `width`, `height`, `radius`.

- Match the real component's geometry exactly (cards and rows expose a `loading` state that does this for you).
- Shimmer uses `surface-pressed`; it stops under reduced motion.
