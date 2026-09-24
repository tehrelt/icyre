# EPIC-047 — Catalog Screens (Album)

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Цель

Экран альбома из canvas (`Album.dc.html`).

Design System: https://claude.ai/artifact/RA7b9Qphfgy2MmAVzsxcDT · Canvas: https://claude.ai/artifact/5GypRL4RmbSQUxEGMDkwfM

## Задачи

- [x] TASK-047.1 Hero: обложка в рамке, overline+serial, display-заголовок, артист, мета, теги.

- [x] TASK-047.2 Действия: Play/Pause (iridescent), Shuffle, Save, More, переключатель вида списка.

- [x] TASK-047.3 Tracklist (TrackListHeader, TrackRow без обложки, unavailable-состояние), подвал релиза.

- [x] TASK-047.4 More by artist.

- [x] TASK-047.5 Контракт `GET /api/v1/pages/albums/{id}` (zod) + mock fixtures из canvas; состояния loading / not found / error.

- [x] TASK-047.6 Реальные данные — BFF album page aggregation (EPIC-033, TASK-033.3).

## Definition of Done

Соответствует canvas, playing-состояние синхронизировано с плеером.

---
