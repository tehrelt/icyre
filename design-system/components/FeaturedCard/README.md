The collectible: ICYRE's premium presentation for one featured release.

**Provide:** `title`, `artist`, `category` (Album, Single, Live…), `serial` (edition/catalog number, e.g. "No. 014"), `cover`/`art`, `meta` (strings joined with ·), `badges` (default `['featured']`), `state`, `width`, `onPlay`, `onOpen`.

- Anatomy: `iridescent-foil` frame (1.5px) → `space-1` gutter → `surface-raised` inner card with hairline → category strip (overline in `brand-secondary` + mono serial) → framed artwork → title (`h4`) and artist → dashed hairline → metadata strip (mono) + badges.
- Hover: lifts 2px and sweeps a single `iridescent-highlight` glint across the artwork over `duration-holo`. Never loop it.
- **Rarity rule:** one FeaturedCard per viewport (a curated shelf may hold three). Never use it for ordinary grid items.
- No game metaphors: no HP, stats, attack blocks or energy symbols — the serial and category strip carry the collectible feel.
