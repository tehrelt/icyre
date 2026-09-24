# EPIC-002 — Shared platform library

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Priority:** P0

## Цель

Создать минимальный общий инфраструктурный слой без бизнес-логики.

## Задачи

- [ ] TASK-002.1 Создать `libs/platform/config`.

- [ ] TASK-002.2 Реализовать загрузку конфигурации из ENV.

- [ ] TASK-002.3 Добавить validation обязательных переменных.

- [ ] TASK-002.4 Создать `libs/platform/logger`.

- [ ] TASK-002.5 Использовать `log/slog`.

- [ ] TASK-002.6 Добавить structured JSON logging.

- [ ] TASK-002.7 Создать `libs/platform/httpserver`.

- [ ] TASK-002.8 Добавить middleware:
  - request ID;
  - access log;
  - panic recovery;
  - timeout.

- [ ] TASK-002.9 Создать `libs/platform/health`.

- [ ] TASK-002.10 Добавить:
  - `/health/live`
  - `/health/ready`

- [ ] TASK-002.11 Создать graceful shutdown helper.

- [ ] TASK-002.12 Добавить обработку SIGTERM/SIGINT.

## Definition of Done

- Любой новый сервис можно подключить к platform library.
- `main.go` не содержит инфраструктурного boilerplate сверх composition root.
- Structured logs работают.
- Graceful shutdown проверен вручную.

---
