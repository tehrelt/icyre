Line icons on a 24px grid with a 1.75 stroke, round caps and joins — one weight across the set.

**Provide:** `name`, optional `size` (16 / 20 / 24), `filled` (only `play`, `pause`, `heart`, `spark`), `label` when the icon stands alone and carries meaning.

- Icons inherit `currentColor`; color them through the parent's text token, never a raw hex.
- 16px in dense rows and badges, 20px in buttons and nav, 24px for standalone controls.
- Filled glyphs mean *state*: filled heart = liked, filled play/pause in transport controls.
- Brand-specific shapes: `library` is a stack of cards (the collectible pile), `settings` is a mixer, not a gear.
