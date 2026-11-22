A trigger that opens a Menu of actions or single-choice options.

**Provide:** `trigger` (usually a secondary/ghost Button), `items` (MenuItem[]: `label`, `icon`, `shortcut`, `selected`, `danger`, `disabled`, `divider`, `heading`), `onSelect`, `align`.

- Menu: `surface-raised`, `border`, `radius-md`, `shadow-floating`, items 36px tall.
- Closes on select, outside click and Escape. Destructive items go last, after a divider.
