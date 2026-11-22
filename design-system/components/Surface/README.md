The three ICYRE materials: Neutral, Elevated and Collectible — every container is one of them.

**Provide:** `material` (`neutral` | `elevated` | `collectible`), `padding` (`none`|`sm`|`md`|`lg`), children.

- **Neutral** (`surface` + `border-subtle`, no shadow): lists, panels, forms. The default; 80–90% of a screen.
- **Elevated** (`surface-raised` + `shadow-floating`): menus, popovers, the player bar, toasts.
- **Collectible** (`iridescent-foil` 1.5px frame, `space-1` gutter, nested `radius-md` inner panel, `shadow-collectible`): one featured object per view. Never two collectibles side by side unless they're a deliberate set (e.g. a "featured shelf" of ≤ 3).
- Don't nest collectibles, don't put a collectible inside an elevated panel.
