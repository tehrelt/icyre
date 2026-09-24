# EPIC-046 — Home

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Цель

Экран Home из canvas (`Main.dc.html`).

Design System: https://claude.ai/artifact/RA7b9Qphfgy2MmAVzsxcDT · Canvas: https://claude.ai/artifact/5GypRL4RmbSQUxEGMDkwfM

## Задачи

- [x] TASK-046.1 Приветствие по времени суток + фильтр-чипы (All/Albums/Playlists/Artists).

- [x] TASK-046.2 Recently played, Album of the week (FeaturedCard + факты), Recommended, New releases.

- [x] TASK-046.3 Trending (Today/This week) с TrackRow, Made for you (Daily mix + плейлисты), Artists you follow.

- [x] TASK-046.4 Loading skeleton, error InlineAlert с retry.

- [x] TASK-046.5 Контракт BFF `GET /api/v1/pages/home` (zod) + mock fixtures.

- [x] TASK-046.6 Подключить реальный BFF (EPIC-033) вместо mock: `VITE_API_MOCKS=false`; пустые секции скрываются.

## Definition of Done

Интеграционные тесты: секции, воспроизведение, фильтры.

---
