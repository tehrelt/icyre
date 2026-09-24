# EPIC-007 — Catalog Service: domain model

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Priority:** P0

## Цель

Создать первую полноценную вертикаль и использовать её как эталон для остальных сервисов.

## Задачи

- [ ] TASK-007.1 Создать module `services/catalog`.

- [ ] TASK-007.2 Создать domain entity `Artist`.

- [ ] TASK-007.3 Создать domain entity `Album`.

- [ ] TASK-007.4 Создать domain entity `Track`.

- [ ] TASK-007.5 Создать domain entity `Genre`.

- [ ] TASK-007.6 Определить Track Status:
  - DRAFT
  - PROCESSING
  - READY
  - BLOCKED
  - DELETED

- [ ] TASK-007.7 Добавить application commands/use cases.

- [ ] TASK-007.8 Определить repository ports.

- [ ] TASK-007.9 Определить event publisher port.

- [ ] TASK-007.10 Добавить unit tests domain/application.

## Definition of Done

- Domain не импортирует pgx, Kafka, HTTP.
- Use cases тестируются без инфраструктуры.
- Ошибки домена представлены Go errors.

---
