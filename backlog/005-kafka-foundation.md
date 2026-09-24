# EPIC-005 — Kafka foundation

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P0

## Цель

Создать основу event-driven взаимодействия.

## Задачи

- [x] TASK-005.1 Добавить Kafka в Docker Compose.

- [x] TASK-005.2 Создать `libs/platform/kafka`.

- [x] TASK-005.3 Реализовать producer.

- [x] TASK-005.4 Реализовать consumer helper.

- [x] TASK-005.5 Добавить graceful shutdown consumer.

- [x] TASK-005.6 Создать `libs/contracts/events`.

- [x] TASK-005.7 Добавить `EventEnvelope`.

- [x] TASK-005.8 Поддержать поля:
  - `eventId`
  - `eventType`
  - `eventVersion`
  - `occurredAt`
  - `producer`
  - `traceId`
  - `payload`

- [x] TASK-005.9 Определить initial topics.

- [x] TASK-005.10 Добавить DLQ convention.

- [x] TASK-005.11 Добавить idempotency recommendations в документацию.

## Definition of Done

- Сервис публикует тестовое событие.
- Consumer читает его.
- Shutdown consumer проходит корректно.
- Envelope единообразен для всех событий.

## Итог реализации

- franz-go: `Producer` (acks=all, sync), `Consumer` (manual commit после обработки, retries, `ErrPermanent`, DLQ `<topic>.dlq`, graceful stop).
- `libs/contracts/events.Envelope` (`eventId` UUIDv7, `eventType`, `eventVersion`, `occurredAt`, `producer`, `traceId`, `payload`).
- Initial topics: `catalog.events` + `catalog.events.dlq` (`deploy/kafka/create-topics.sh`, job `kafka-init`).
- Idempotency и эволюция схем: `libs/contracts/README.md`.
- Integration tests (реальный Kafka): publish → consume → shutdown; failed message → DLQ.

---
