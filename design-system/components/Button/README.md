Text buttons for actions; six variants, three sizes, loading built in.

**Provide:** children (label), `variant`, `size` (`sm` 32 / `md` 40 / `lg` 48), optional `iconStart`/`iconEnd`, `loading`, `disabled`, `fullWidth`, native button props.

- `primary` (ink `action-primary`): the one main action per view — Save, Create, Continue.
- `secondary`: equal-weight alternatives; `ghost`: tertiary actions in toolbars; `tonal` (`brand-soft`): positive secondary actions like Follow.
- `destructive`: only inside a confirmation (Modal footer), never as a first click.
- `iridescent`: the brand action — Play on album/artist heroes, Start radio. **At most one per screen.** Ink label on `iridescent-primary`, framed by `iridescent-border`.
- Labels are sentence case verbs: "Play album", "Add to library". No ALL CAPS, no emoji.
- `loading` swaps the leading icon for a Spinner and keeps the label so the width doesn't jump.
- `forceState` exists only for documentation previews.
