# EPIC-002 — Shared platform library

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P0

## Цель

Создать минимальный общий инфраструктурный слой без бизнес-логики.

## Задачи

- [x] TASK-002.1 Создать `libs/platform/config`.

- [x] TASK-002.2 Реализовать загрузку конфигурации из ENV.

- [x] TASK-002.3 Добавить validation обязательных переменных.

- [x] TASK-002.4 Создать `libs/platform/logger`.

- [x] TASK-002.5 Использовать `log/slog`.

- [x] TASK-002.6 Добавить structured JSON logging.

- [x] TASK-002.7 Создать `libs/platform/httpserver`.

- [x] TASK-002.8 Добавить middleware:
  - request ID;
  - access log;
  - panic recovery;
  - timeout.

- [x] TASK-002.9 Создать `libs/platform/health`.

- [x] TASK-002.10 Добавить:
  - `/health/live`
  - `/health/ready`

- [x] TASK-002.11 Создать graceful shutdown helper.

- [x] TASK-002.12 Добавить обработку SIGTERM/SIGINT.

## Definition of Done

- Любой новый сервис можно подключить к platform library.
- `main.go` не содержит инфраструктурного boilerplate сверх composition root.
- Structured logs работают.
- Graceful shutdown проверен вручную.

## Итог реализации

Пакеты `libs/platform`: `config`, `logger`, `requestid`, `httpserver` (RequestID, Tracing, AccessLog, Recover, Timeout,
route metrics, error model helpers), `health` (`/health/live`, `/health/ready`, drain), `shutdown` (SIGINT/SIGTERM + LIFO closers).
Graceful shutdown проверен `docker compose stop catalog`: http → kafka producer → postgres → tracing.

---
