# EPIC-010 — Catalog events

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Задачи

- [x] TASK-010.1 Добавить event:
  - `track.created`

- [x] TASK-010.2 Добавить:
  - `track.updated`
  - `album.created`
  - `artist.created`

- [x] TASK-010.3 Публиковать событие после успешной операции.

- [x] TASK-010.4 Добавить contract version.

- [x] TASK-010.5 Добавить tests event mapping.

## Definition of Done

- После создания Track Kafka получает событие.
- Event payload не зависит от HTTP DTO.
- Event version указан явно.

## Итог реализации

`libs/contracts/events/catalogv1` (Version = 1): `track.created`, `track.updated`, `album.created`, `artist.created`.
Публикация после успешной записи, ключ = aggregate ID, topic `catalog.events`. Проверено consumer'ом в compose.
Transactional outbox: события пишутся в `catalog.outbox` в транзакции изменения, relay (`libs/platform/outbox`)
публикует их в Kafka — at-least-once end to end, порядок вставки, один relay на таблицу (advisory lock).

---
