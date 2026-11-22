Square icon-only control with an accessible label and toggle support.

**Provide:** `icon`, `label` (required — becomes `aria-label` and the tooltip title), `variant`, `size`, `pressed` for toggles.

- `neutral`: toolbar actions (more, close, queue). `active`: an on-state shown by `text-brand` color *and* a filled glyph (liked heart). `selected`: a latched panel toggle (queue open) on `selected` fill. `destructive`: remove/delete.
- `play`: the round iridescent play affordance on cards and heroes. For the player transport use PlayerButton.
- Always pass `pressed` for toggles so screen readers announce state; color is never the only signal.
