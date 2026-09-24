# EPIC-017 — Stream Authorization Service

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Priority:** P1

## Задачи

- [ ] TASK-017.1 Создать module.

- [ ] TASK-017.2 Реализовать проверку Track status.

- [ ] TASK-017.3 Реализовать lookup media variant.

- [ ] TASK-017.4 Реализовать signed URL.

- [ ] TASK-017.5 Добавить TTL signed URL.

- [ ] TASK-017.6 Реализовать:
  - `POST /stream/authorize`

- [ ] TASK-017.7 Добавить audit/security logs.

## Definition of Done

- Backend возвращает URL.
- Audio body не проксируется через сервис.
- URL автоматически истекает.

---
