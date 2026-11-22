Single-line text field with label, hint, error and optional leading icon.

**Provide:** `label`, `placeholder`, `hint`, `error` (string switches the error state), `iconStart`, `size`, native input props (`value`/`onChange` or `defaultValue`).

- Rest border is `border-interactive` (3:1). Focus: `focus` border + 3px 18% focus halo, and the field turns `surface-raised`.
- Errors pair the `danger` border with an icon and message — never color alone. Messages say how to fix: "Name must be at least 2 characters".
- Disabled fields use `disabled` fill and `text-disabled`.
