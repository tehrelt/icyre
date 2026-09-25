# EPIC-016 — Playback Service

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [-] IN PROGRESS

**Priority:** P1

## Цель

Управлять playback session и генерировать события прослушивания.

## Задачи

- [x] TASK-016.1 Создать module.

- [ ] TASK-016.2 Создать playback session model.

- [ ] TASK-016.3 Хранить transient state в Redis.

- [ ] TASK-016.4 Реализовать queue.

- [ ] TASK-016.5 Реализовать shuffle.

- [ ] TASK-016.6 Реализовать repeat mode.

- [ ] TASK-016.7 Реализовать:
  - `POST /playback/sessions`
  - `GET /playback/session`
  - `PUT /playback/queue`
  - `PATCH /playback/state`

- [ ] TASK-016.8 Публиковать:
  - `playback.started`
  - `playback.finished`
  - `playback.skipped`

- [ ] TASK-016.9 Ограничить частоту progress events.

## Прогресс

- Срез 1: `POST /api/v1/playback/events` → `playback.started/finished/skipped` в `playback.events`
  (TASK-016.8 частично: публикация есть, источник — телеметрия плеера). Сессии, очередь, shuffle/repeat,
  Redis-состояние и `playback.progress` — впереди.

---
