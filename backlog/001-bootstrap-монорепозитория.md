# EPIC-001 — Bootstrap монорепозитория

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Priority:** P0

## Цель

Создать основу monorepo, в которой каждый deployable сервис является отдельным Go module, а корень управляется через `go.work`.

## Задачи

- [ ] TASK-001.1 Создать структуру каталогов:
  - `services/`
  - `workers/`
  - `libs/`
  - `api/proto/`
  - `docs/`
  - `deploy/`
  - `scripts/`

- [ ] TASK-001.2 Создать root `go.work`.

- [ ] TASK-001.3 Добавить первые Go modules:
  - `libs/platform`
  - `libs/contracts`
  - `services/catalog`

- [ ] TASK-001.4 Настроить единый module namespace.

- [ ] TASK-001.5 Создать root `Makefile`.

- [ ] TASK-001.6 Добавить `.gitignore`.

- [ ] TASK-001.7 Добавить `.editorconfig`.

- [ ] TASK-001.8 Создать root `README.md`.

- [ ] TASK-001.9 Добавить команды:
  - `make fmt`
  - `make vet`
  - `make test`
  - `make tidy`
  - `make build`

- [ ] TASK-001.10 Проверить `go work sync`.

## Definition of Done

- `go.work` существует.
- Все подключенные modules корректно разрешаются.
- `go work sync` завершается без ошибки.
- `make test` работает для существующих modules.
- В репозитории отсутствуют циклические зависимости между modules.

---
