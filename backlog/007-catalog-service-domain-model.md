# EPIC-007 — Catalog Service: domain model

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P0

## Цель

Создать первую полноценную вертикаль и использовать её как эталон для остальных сервисов.

## Задачи

- [x] TASK-007.1 Создать module `services/catalog`.

- [x] TASK-007.2 Создать domain entity `Artist`.

- [x] TASK-007.3 Создать domain entity `Album`.

- [x] TASK-007.4 Создать domain entity `Track`.

- [x] TASK-007.5 Создать domain entity `Genre`.

- [x] TASK-007.6 Определить Track Status:
  - DRAFT
  - PROCESSING
  - READY
  - BLOCKED
  - DELETED

- [x] TASK-007.7 Добавить application commands/use cases.

- [x] TASK-007.8 Определить repository ports.

- [x] TASK-007.9 Определить event publisher port.

- [x] TASK-007.10 Добавить unit tests domain/application.

## Definition of Done

- Domain не импортирует pgx, Kafka, HTTP.
- Use cases тестируются без инфраструктуры.
- Ошибки домена представлены Go errors.

## Итог реализации

`services/catalog/internal/domain`: Artist, Album (AlbumType), Track (state machine статусов), Genre, ValidationError,
ReferenceError, not-found и business errors; domain events. `application` — use cases, `ports` — repository и EventPublisher.
Unit tests на in-memory fakes, без инфраструктуры.

---
