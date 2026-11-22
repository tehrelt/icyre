The app-wide pill search field with keyboard shortcut hint and clear button.

**Provide:** `value`/`onChange` (or `defaultValue`), `placeholder`, `shortcut` (default ⌘K; empty to hide), `size`.

- Rests on `background-subtle` without a border so the header stays quiet; hover adds `border-interactive`, focus lifts to `surface-raised` with the focus ring.
- Placeholder names what can be found: "Artists, albums, tracks".
- One per screen, top of the main column (desktop) or the Search tab (mobile).
