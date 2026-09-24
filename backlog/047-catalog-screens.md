# EPIC-047 — Catalog Screens (Album)

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [ ] TODO

**Priority:** P1

## Цель

Экран альбома из canvas (`Album.dc.html`).

Design System: https://claude.ai/artifact/RA7b9Qphfgy2MmAVzsxcDT · Canvas: https://claude.ai/artifact/5GypRL4RmbSQUxEGMDkwfM

## Задачи

- [ ] TASK-047.1 Hero: обложка в рамке, overline+serial, display-заголовок, артист, мета, теги.

- [ ] TASK-047.2 Действия: Play/Pause (iridescent), Shuffle, Save, More, переключатель вида списка.

- [ ] TASK-047.3 Tracklist (TrackListHeader, TrackRow без обложки, unavailable-состояние), подвал релиза.

- [ ] TASK-047.4 More by artist.

- [ ] TASK-047.5 Данные: `GET /api/v1/pages/albums/{id}` (BFF) / Catalog API.

## Definition of Done

Соответствует canvas, playing-состояние синхронизировано с плеером.

---
