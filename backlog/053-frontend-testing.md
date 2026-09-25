# EPIC-053 — Frontend testing

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [-] IN PROGRESS

**Priority:** P1

## Цель

Тестовая пирамида frontend.

Design System: https://claude.ai/artifact/RA7b9Qphfgy2MmAVzsxcDT · Canvas: https://claude.ai/artifact/5GypRL4RmbSQUxEGMDkwfM

## Задачи

- [x] TASK-053.1 Vitest + React Testing Library + jsdom.

- [x] TASK-053.2 Unit: player store, controller, engine, API client, greeting.

- [x] TASK-053.3 Компоненты DS: поведение и a11y-роли.

- [x] TASK-053.4 Интеграция через memory router: Home, shell, Album, Search (все состояния, recent searches, шорткат).

- [ ] TASK-053.5 Playwright e2e (сценарий: открыть Home → Play album → перейти на другую страницу).

- [ ] TASK-053.6 Visual regression против canvas.

## Definition of Done

`bun run test` в CI (EPIC-036).

---
