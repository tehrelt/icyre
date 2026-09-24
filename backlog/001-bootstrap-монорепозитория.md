# EPIC-001 — Bootstrap монорепозитория

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P0

## Цель

Создать основу monorepo, в которой каждый deployable сервис является отдельным Go module, а корень управляется через `go.work`.

## Задачи

- [x] TASK-001.1 Создать структуру каталогов:
  - `services/`
  - `workers/`
  - `libs/`
  - `api/proto/`
  - `docs/`
  - `deploy/`
  - `scripts/`

- [x] TASK-001.2 Создать root `go.work`.

- [x] TASK-001.3 Добавить первые Go modules:
  - `libs/platform`
  - `libs/contracts`
  - `services/catalog`

- [x] TASK-001.4 Настроить единый module namespace.

- [x] TASK-001.5 Создать root `Makefile`.

- [x] TASK-001.6 Добавить `.gitignore`.

- [x] TASK-001.7 Добавить `.editorconfig`.

- [x] TASK-001.8 Создать root `README.md`.

- [x] TASK-001.9 Добавить команды:
  - `make fmt`
  - `make vet`
  - `make test`
  - `make tidy`
  - `make build`

- [x] TASK-001.10 Проверить `go work sync`.

## Definition of Done

- `go.work` существует.
- Все подключенные modules корректно разрешаются.
- `go work sync` завершается без ошибки.
- `make test` работает для существующих modules.
- В репозитории отсутствуют циклические зависимости между modules.

## Итог реализации

- Module namespace: `github.com/tehrelt/icyre/<path>` (из `git remote`).
- `go.work` (go 1.26.0, toolchain go1.26.8): `libs/platform`, `libs/contracts`, `services/catalog`.
  Сервисы дополнительно держат `replace` на `libs/*` в `go.mod`, чтобы `go mod tidy` и Docker-сборка работали без workspace.
- TASK-001.1: `docs/` не создан — роль документации выполняет `specs/`; `workers/` появится вместе с первым worker (EPIC-020) —
  пустые каталоги заранее не создаются.
- Проверено: `go work sync`, `make fmt vet test`.

---
