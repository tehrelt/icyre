# EPIC-003 — PostgreSQL foundation

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Priority:** P0

## Цель

Подключить основное транзакционное хранилище.

## Задачи

- [ ] TASK-003.1 Добавить PostgreSQL в Docker Compose.

- [ ] TASK-003.2 Настроить persistent volume.

- [ ] TASK-003.3 Добавить healthcheck.

- [ ] TASK-003.4 Создать `libs/platform/postgres`.

- [ ] TASK-003.5 Использовать `pgx`.

- [ ] TASK-003.6 Настроить connection pool.

- [ ] TASK-003.7 Добавить configurable timeouts.

- [ ] TASK-003.8 Выбрать migration tool.

- [ ] TASK-003.9 Добавить команду запуска миграций.

- [ ] TASK-003.10 Создать первую schema `catalog`.

## Definition of Done

- PostgreSQL поднимается через Docker Compose.
- Приложение подключается через `pgxpool`.
- Миграции запускаются одной командой.
- Readiness падает при невозможности использовать БД.

---
