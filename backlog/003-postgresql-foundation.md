# EPIC-003 — PostgreSQL foundation

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P0

## Цель

Подключить основное транзакционное хранилище.

## Задачи

- [x] TASK-003.1 Добавить PostgreSQL в Docker Compose.

- [x] TASK-003.2 Настроить persistent volume.

- [x] TASK-003.3 Добавить healthcheck.

- [x] TASK-003.4 Создать `libs/platform/postgres`.

- [x] TASK-003.5 Использовать `pgx`.

- [x] TASK-003.6 Настроить connection pool.

- [x] TASK-003.7 Добавить configurable timeouts.

- [x] TASK-003.8 Выбрать migration tool.

- [x] TASK-003.9 Добавить команду запуска миграций.

- [x] TASK-003.10 Создать первую schema `catalog`.

## Definition of Done

- PostgreSQL поднимается через Docker Compose.
- Приложение подключается через `pgxpool`.
- Миграции запускаются одной командой.
- Readiness падает при невозможности использовать БД.

## Итог реализации

- Migration tool: **goose v3** (Provider API, embedded SQL), version table `<schema>.schema_migrations` на каждую schema домена.
- Команды: `catalog migrate`, `make migrate`, job `catalog-migrate` в compose.
- Readiness: при остановленном PostgreSQL `/health/ready` → 503, `/health/live` → 200 (проверено).
- `libs/platform/postgres`: pgxpool, connect/statement timeouts, `db_query_duration_seconds{operation,result}`, `db_pool_*`.

---
