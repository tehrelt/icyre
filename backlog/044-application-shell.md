# EPIC-044 — Application Shell

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P0

## Цель

Каркас приложения из canvas: Sidebar, Page toolbar, область страницы, Global Player.

Design System: https://claude.ai/artifact/RA7b9Qphfgy2MmAVzsxcDT · Canvas: https://claude.ai/artifact/5GypRL4RmbSQUxEGMDkwfM

## Задачи

- [x] TASK-044.1 AppShell layout (sidebar 248 | page, player 80) — `app/layouts/AppShell`.

- [x] TASK-044.2 Sidebar (CSidebar): лого, основная навигация со счётчиками, плейлисты, профиль/настройки.

- [x] TASK-044.3 Page toolbar (CToolbar): back/forward по истории, breadcrumb, What's new, аватар.

- [x] TASK-044.4 Section header (CSectionHeader), Quick tile (CQuickTile).

- [x] TASK-044.5 Роуты экранов следующих эпиков ведут на «not built» страницу внутри shell.

- [x] TASK-044.6 Genre tile (CGenreTile).

## Definition of Done

Навигация не размонтирует плеер (тест `keeps playback running across navigation`).

---
