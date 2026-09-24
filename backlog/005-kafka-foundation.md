# EPIC-005 — Kafka foundation

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Priority:** P0

## Цель

Создать основу event-driven взаимодействия.

## Задачи

- [ ] TASK-005.1 Добавить Kafka в Docker Compose.

- [ ] TASK-005.2 Создать `libs/platform/kafka`.

- [ ] TASK-005.3 Реализовать producer.

- [ ] TASK-005.4 Реализовать consumer helper.

- [ ] TASK-005.5 Добавить graceful shutdown consumer.

- [ ] TASK-005.6 Создать `libs/contracts/events`.

- [ ] TASK-005.7 Добавить `EventEnvelope`.

- [ ] TASK-005.8 Поддержать поля:
  - `eventId`
  - `eventType`
  - `eventVersion`
  - `occurredAt`
  - `producer`
  - `traceId`
  - `payload`

- [ ] TASK-005.9 Определить initial topics.

- [ ] TASK-005.10 Добавить DLQ convention.

- [ ] TASK-005.11 Добавить idempotency recommendations в документацию.

## Definition of Done

- Сервис публикует тестовое событие.
- Consumer читает его.
- Shutdown consumer проходит корректно.
- Envelope единообразен для всех событий.

---
