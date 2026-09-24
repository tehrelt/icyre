# EPIC-042 — Design Tokens

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P0

## Цель

Перенести токены ICYRE Design System в код как единственный источник значений.

Design System: https://claude.ai/artifact/RA7b9Qphfgy2MmAVzsxcDT · Canvas: https://claude.ai/artifact/5GypRL4RmbSQUxEGMDkwfM

## Задачи

- [x] TASK-042.1 `apps/web/design/icyre-tokens.json` — копия `project/tokens.json` Design System.

- [x] TASK-042.2 Генератор `bun run tokens` → `src/app/styles/tokens.css` (primitive + semantic colors, iridescent gradients, spacing, radius, shadow, border, opacity, motion, layout, z-index, typography).

- [x] TASK-042.3 Type scale классы (`t-display … t-caption`) и mobile step-down по правилам DS.

- [x] TASK-042.4 `canvas-tokens.css` — значения из макетов, которых нет в DS (тень плеера, кольцо обложки, rail/mini-player размеры).

- [ ] TASK-042.5 Предложить upstream в DS токены из `canvas-tokens.css`.

## Definition of Done

В компонентах нет raw HEX (кроме бренд-марки Logo и перенесённого DS-stylesheet); все значения — `var(--token)`.

---
