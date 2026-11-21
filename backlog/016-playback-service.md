# EPIC-016 — Playback Service

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Цель

Управлять playback session и генерировать события прослушивания.

## Задачи

- [x] TASK-016.1 Создать module.

- [x] TASK-016.2 Создать playback session model.

- [x] TASK-016.3 Хранить transient state в Redis.

- [x] TASK-016.4 Реализовать queue.

- [x] TASK-016.5 Реализовать shuffle.

- [x] TASK-016.6 Реализовать repeat mode.

- [x] TASK-016.7 Реализовать:
  - `POST /playback/sessions`
  - `GET /playback/session`
  - `PUT /playback/queue`
  - `PATCH /playback/state`

- [x] TASK-016.8 Публиковать:
  - `playback.started`
  - `playback.finished`
  - `playback.skipped`

- [x] TASK-016.9 Ограничить частоту progress events.

## Итог реализации

- `services/playback`: доменная модель `Session` и `Queue` (`domain/session.go`), детерминированный `Order` для shuffle и режимы `repeatMode` (`off`, `all`, `one`).
- Redis (`internal/adapters/redis`): ключи `playback:{userId}` и `queue:{userId}` с TTL (`PLAYBACK_SESSION_TTL`, по умолчанию `24h`), throttle-маркер `playback:progress:{userId}:{playbackId}` (`SetNX` с `PLAYBACK_PROGRESS_INTERVAL`, по умолчанию `10s`).
- HTTP API (`/api/v1/playback/*`, проксируется через `deploy/gateway/nginx.conf`): `POST /sessions`, `GET /session`, `PUT /queue`, `PATCH /state`, `POST /events` (включая `progress`).
- События `playback.events` (`libs/contracts/events/playbackv1`): `playback.started`, `playback.progress`, `playback.finished`, `playback.skipped`.

---
