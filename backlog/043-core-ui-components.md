# EPIC-043 — Core UI Components

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P0

## Цель

Перенести компоненты Design System в `shared/ui` как типизированный React без изменения визуала.

Design System: `design-system/`

## Задачи

- [x] TASK-043.1 Stylesheet компонентов (`bundle.css` DS) → `shared/ui/styles/components.css`.

- [x] TASK-043.2 Icon (+ набор иконок DS), Spinner, Button, IconButton, Artwork, Surface, Logo.

- [x] TASK-043.3 Avatar, Badge, Tag, Chip, Skeleton, Slider, SegmentedControl, SidebarItem, MobileNavItem, Breadcrumb.

- [x] TASK-043.4 MediaCard / AlbumCard / ArtistCard / PlaylistCard / FeaturedCard / TrackRow.

- [x] TASK-043.5 PlayerButton, ProgressBar, VolumeSlider, TimeLabel, PlaybackState.

- [x] TASK-043.6 Patterns из canvas: InlineAlert (Search results — error), EmptyState (Search results — no results).

- [x] TASK-043.7 SearchInput (forwardRef, Enter/Escape), Tabs (roving focus, счётчики), TrackListHeader — для Search и Album.

- [x] TASK-043.10 Input, Textarea (`shared/ui/forms.tsx`).

- [x] TASK-043.8 Tooltip, Menu/Dropdown/ContextMenu, Popover, Modal, Drawer, Toast, QueueItem.

- [x] TASK-043.9 Checkbox, Radio, Switch, Divider, Progress, TopNavItem.

## Definition of Done

Каждый компонент соответствует DS (классы, варианты, состояния, a11y) и покрыт тестом поведения.

---
