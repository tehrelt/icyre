# EPIC-004 — Redis foundation

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Priority:** P1

## Цель

Подготовить Redis для cache, sessions и transient state.

## Задачи

- [ ] TASK-004.1 Добавить Redis в Docker Compose.

- [ ] TASK-004.2 Добавить Redis healthcheck.

- [ ] TASK-004.3 Создать `libs/platform/redis`.

- [ ] TASK-004.4 Настроить timeout.

- [ ] TASK-004.5 Настроить connection pool.

- [ ] TASK-004.6 Определить naming convention ключей.

- [ ] TASK-004.7 Добавить helper для TTL cache.

## Definition of Done

- Redis доступен из сервисов.
- Cache API не содержит бизнес-типов.
- Timeout и ошибки корректно обрабатываются.

---
