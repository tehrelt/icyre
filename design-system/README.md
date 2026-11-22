ICYRE is a light, premium music interface: pearl-white surfaces, a cool neutral palette, and a thin layer of iridescent "foil" that marks what is precious — the thing playing, the thing featured, the brand itself. The collectible-card influence lives in framing, layering, edition labels and borders, never in game mechanics.

**The formula:** light premium UI + cool neutrals + subtle iridescent material + collectible framing + modern music UX.

## Principles

1. **Neutral first, foil rarely.** 80–90% of any screen is `background`, `surface`, `border-subtle` and ink text. Iridescence is 10–20% at most and has to *mean* something.
2. **Frames, not fills.** Structure comes from hairline borders and tonal steps between surfaces; shadows are short and cool.
3. **Objects feel collectible.** Media sits in a thin frame (`space-1` gutter, nested radii), featured releases get an edition strip and a foil border.
4. **Music, not enterprise.** Editorial type for titles, mono for time and catalog data, generous hero space, dense track lists.
5. **State is never color alone.** Every status pairs color with a glyph, weight, position or label.

## Content fundamentals

- Voice: calm, precise, a little poetic in editorial copy ("slows winter down to a crawl"), plain and short in UI.
- Sentence case everywhere — buttons, titles, menus. UPPERCASE only through the `overline` style (category strips, section kickers).
- Address the listener as "you"; the product never says "I" or "we" in UI chrome.
- Buttons are verbs + object: "Play album", "Add to library", "Delete playlist". Toasts state the outcome: "Added to Late night drives".
- Times and counts are mono: `3:47`, `No. 014`, `11 tracks`. Use "·" (middle dot) to join metadata.
- No emoji in UI. No exclamation marks outside onboarding.

## Color

Primitives (`ice-*`, `electric-*`, `violet-*`, `holo-*`) are never used directly by product code — always the semantic role.

- Page: `background` (#f5f7fa, icy pearl). Sidebar and wells: `background-subtle`. Cards, inputs, lists: `surface`. Menus, popovers, modals, player: `surface-raised` (the only pure white).
- Interaction fills: `surface-hover` → `surface-pressed`. Selection: `selected` fill + `border-selected`.
- Text: `text-primary` for titles and track names, `text-secondary` for artists and descriptions, `text-tertiary` for metadata and placeholders, `text-disabled` only for disabled controls.
- Primary action is **ink** (`action-primary`), not blue — premium and neutral. Electric blue (`brand-primary`) marks *selection and navigation*: checked controls, the switch, links, the now-playing title (`text-brand`).
- `brand-secondary` (cool violet) appears only in featured labels and the trending badge.
- Semantic: `success`, `warning`, `danger`, `info`, each with a `-soft` tint for badges and toasts. `danger` is also the destructive fill.
- Focus: `focus` (#5b3df5) — see Focus.

## Iridescent material

A spectral ramp of icy cyan → electric blue → cool violet → soft magenta, with a sliver of pearl green. It should read like a CD surface or thin-film foil — never a rainbow, never orange or yellow.

| Token | Where it is allowed |
|---|---|
| `iridescent-primary` | Fill of the Play button (PlayerButton, IconButton `play`) and the `iridescent` Button. Ink text on it. |
| `iridescent-border` | 1.5px ring on those buttons, playing cards, PlaybackState bars, Avatar `ring`, sidebar active gem, brand symbol. |
| `iridescent-foil` | FeaturedCard / collectible Surface frame only. |
| `iridescent-soft` | Ground inside a collectible frame; playing card fill; featured badge. |
| `iridescent-selected` | Now-playing TrackRow and QueueItem; active MobileNavItem pill. |
| `iridescent-progress` | ProgressBar fill; active TopNavItem underline. |
| `iridescent-highlight` | One slow glint across a featured image on hover. |

Rules: at most **one** iridescent Button and **one** FeaturedCard per viewport; never as a page, panel or section background; never as the only carrier of meaning; no looping holo animation.

## Typography

Two families: **Manrope** (`sans`, Google Fonts, weights 400/500/600) for everything, **JetBrains Mono** (`mono`, 400/500) for metadata. Three weights is the ceiling.

- `display` 56/60 — hero and splash, once per screen. `h1` 40/46 page titles. `h2` 30/36 sections. `h3` 22/28 shelf and modal titles. `h4` 18/24 featured card titles. Headings are 600 with negative tracking that tightens as size grows.
- `body-lg` 17/26 editorial copy and bios; `body` 15/22 default UI (track titles at 500); `body-sm` 13/18 artist lines and subtitles; `caption` 12/16 helper text.
- `label` 13/16/500 forms and nav; `button` 14/20/600; `overline` 11/14/600 +0.12em uppercase.
- `metadata` 12/16/500 mono with tabular numerals for durations, time labels, serials, bitrate.

## Spacing and density

4px base: `space-1` 4 → `space-20` 80. Three rhythms, never one spacing for everything:

- **Dense** — track lists, queue, menus: `inset-row` (8px vertical), 56px rows, `space-3` column gaps.
- **Medium** — card grids and panels: `inset-card` 12, `gap-grid` 24 (20 tablet, 16 mobile), `inset-panel` 24 for popovers/modals.
- **Spacious** — heroes and splash: `inset-hero` 80, `gap-section` 48, `space-10` between header and content.

## Radius

Six steps only: `radius-xs` 4 (badges, checkbox, row covers) · `radius-sm` 6 (small buttons, tooltips, menu items) · `radius-md` 10 (buttons, inputs, menus, artwork inside a frame) · `radius-card` 14 (every card and the collectible frame) · `radius-lg` 20 (modals, drawers, hero panels) · `radius-pill` (chips, switch, avatars, play button, search).
Nesting rule: outer = inner + padding — `radius-card` 14 = `radius-md` 10 + `space-1` 4. Medium radii keep objects card-like; nothing is blob-round except pills.

## Borders

Borders carry ICYRE's structure.

- `border-subtle` — dividers, card edges, row separators (decorative).
- `border` — menus, modals, secondary button, tags.
- `border-strong` — hovered containers.
- `border-interactive` — resting edge of inputs, checkbox, radio, switch (3:1 on every ground).
- `border-selected` — selected card, chip, checked control, at `border-thick` 1.5px.
- `iridescent-border` / `iridescent-foil` — the holo borders, drawn with the `padding-box / border-box` two-background technique at 1.5px.
- Nested borders (frame → gutter → inner hairline) are reserved for the Collectible material.

## Depth

`shadow-flat` (default) → `shadow-raised` (hovered cards, active sidebar item, selected segment) → `shadow-floating` (menus, popovers, toasts, player) → `shadow-modal`. `shadow-collectible` adds a faint blue lift under FeaturedCard only. Prefer a tonal step or border before reaching for a shadow.

## Materials

- **Neutral Surface** — `surface` + `border-subtle`, flat. Everyday UI.
- **Elevated Surface** — `surface-raised` + `shadow-floating`. Menus, popovers, player sections.
- **Collectible Surface** — foil frame + `space-1` gutter + white inner card with hairline + `shadow-collectible`. Featured media only.

## Collectible card language

FeaturedCard is the canonical collectible: category strip (`overline` in `brand-secondary`, e.g. "FEATURED · ALBUM") with a mono edition serial ("No. 014"), framed artwork, `h4` title and artist, a dashed hairline, and a metadata strip with at most two badges. PlaylistCard's stacked layers and the `library` icon (a card pile) echo the same idea.
Never: HP, stat bars, attack blocks, energy symbols, rarity stars, holographic backgrounds on ordinary cards.

## Iconography

Custom line set (Icons asset group, and `Icon` in the bundle): 24px grid, 2px safe area, 1.75 stroke, round caps and joins, one visual weight. Sizes 16 (dense rows, badges), 20 (buttons, nav), 24 (standalone). Filled variants only for `play`, `pause`, `heart` (= liked) and `spark` (featured). Icons inherit `currentColor`; the SVG files in the asset group are drawn in ink `#141821`. No emoji, no icon fonts.

## Motion

- `duration-fast` 120ms — hover fills, icon swaps, press. `duration-normal` 200ms — menus, tooltips, card lift, player state. `duration-slow` 320ms — drawer, modal, toast. `duration-holo` 2400ms — the foil glint, once per hover.
- `easing-standard` for most things, `easing-emphasized` for entrances and the play/pause morph, `easing-exit` for anything leaving.
- Under `prefers-reduced-motion` the glint, shimmer and bars stop; PlaybackState holds a still pose so "playing" is still readable.

## Focus

One ring everywhere: `focus-ring-width` 2px solid `focus` at `focus-ring-offset` 2px (≥ 5.4:1 on every ground). Text fields use a `focus` border plus a 3px 18% halo instead of an offset ring. Never remove the ring; never rely on a color change alone.

## Accessibility

Checked pairs (WCAG 2.x): `text-primary` 14.2–17.3:1 on all neutral grounds · `text-secondary` 6.1–7.4:1 · `text-tertiary` 4.6–5.4:1 (incl. `surface-pressed` and `brand-soft`) · `text-brand` 5.2–6.0:1 · white on `action-primary` 17.8:1, on `brand-primary` 6.0:1, on `danger` 5.8:1 · `brand-active` on `brand-soft` 8.2:1 · ink on every `iridescent-primary` stop ≥ 10.8:1 · status text on its soft tint ≥ 4.9:1 · `border-interactive` ≥ 3.1:1 · `iridescent-progress` stops ≥ 3.3:1 on the track. `text-disabled` is intentionally exempt (disabled controls).
Playing/selected states always combine ≥ 2 cues (ground + bars + brand title; fill + border + check).

## Responsive

One system across three widths; components don't fork. Mobile < `bp-tablet` 768 ≤ tablet < `bp-desktop` 1200 ≤ desktop.
- Gutters: `gutter-desktop` 32, `gutter-tablet` 24, `gutter-mobile` 16. Max content `container-max` 1440.
- Card columns: 6 / 4 / 2 (`grid-columns-*`), shelves scroll horizontally on mobile.
- Sidebar 248px on desktop → 72px icon rail on tablet → MobileNavItem bottom bar (`mobile-nav-height` 64) on mobile. Player bar `player-height` 80 → 64px mini-player on mobile; Drawer `right` → `bottom`.
- Type steps down one level on mobile for `display`/`h1` (56→40, 40→30); everything else stays.

## Brand elements

- **Symbol — the glint card:** a portrait card outline with a four-point spark: collectible object + ice glint. Works to 16px.
- **Wordmark:** ICYRE built from 2.6-unit monoline strokes — open C, geometric Y, wide tracking. Always ink.
- **Brand accent:** the iridescent stroke on the symbol; the app icon inverts it (ink symbol on `iridescent-primary`).
- Use the Logo component or the Logos asset group; never retype the wordmark in a font.

## Naming

Components and design-tool layers follow `Component/Variant/Size`: `Button/Primary/MD`, `Button/Ghost/MD`, `Badge/Featured`, `Badge/Explicit`, `Card/Album`, `Card/Featured`, `Navigation/SidebarItem`, `Player/ProgressBar`. Tokens are primitive (`ice-500`, `electric-500`) aliased by semantic roles (`border-interactive`, `brand-primary`, `action-primary`). In a Figma file mirror this system as pages 00 Cover · 01 Foundations · 02 Tokens (Variables) · 03 Components (variants + auto layout) · 04 Patterns · 05 Examples (the Showcase).
