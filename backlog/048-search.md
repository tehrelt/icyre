# EPIC-048 — Search

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Цель

Экраны Search из canvas: before query, results (All), loading, no results, error.

Design System: https://claude.ai/artifact/RA7b9Qphfgy2MmAVzsxcDT · Canvas: https://claude.ai/artifact/5GypRL4RmbSQUxEGMDkwfM

## Задачи

- [x] TASK-048.1 SearchInput, Tabs (EPIC-043.7), GenreTile.

- [x] TASK-048.2 Search — before query: недавние запросы (Zustand + localStorage, client state), жанры, настроения, подборки (`GET /api/v1/pages/search`).

- [x] TASK-048.3 Results (All): top result, треки, артисты, альбомы, плейлисты; табы по типу; URL `?q=&type=`, debounce, шорткат `/`.

- [x] TASK-048.4 Состояния: loading (skeleton), no results (+ «did you mean»), error (+ retry, недавние запросы).

- [ ] TASK-048.5 Интеграция с Search Service (EPIC-025): контракт `GET /api/v1/search?q=&type=&limit=` уже зафиксирован zod-схемой и mock.

## Definition of Done

Все пять состояний canvas реализованы и покрыты тестами.

---
