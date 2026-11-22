# EPIC-034 — gRPC contracts

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P2

## Задачи

- [x] TASK-034.1 Настроить protobuf toolchain.

- [x] TASK-034.2 Создать `catalog.v1`.

- [x] TASK-034.3 Создать `users.v1`.

- [x] TASK-034.4 Создать `playback.v1`.

- [x] TASK-034.5 Добавить generation command.

- [x] TASK-034.6 Добавить backward compatibility policy.

## Итог реализации

- Toolchain и конфигурация Buf v2: `buf.yaml` (модуль `api/proto`, правила `STANDARD` lint и `FILE` breaking) и `buf.gen.yaml` (`protoc-gen-go`, `protoc-gen-go-grpc` с `paths=source_relative` в `libs/contracts/gen`).
- Контракты `v1`:
  - `api/proto/catalog/v1/catalog.proto` — `catalog.v1.CatalogService` (артисты, альбомы, треки, жанры, batch- и cursor-выборки);
  - `api/proto/users/v1/users.proto` — `users.v1.UsersService` (публичный и собственный профиль пользователя);
  - `api/proto/playback/v1/playback.proto` — `playback.v1.PlaybackService` (сессия воспроизведения, очередь, состояние, события).
- Сгенерированные Go/gRPC-стабы размещены в `libs/contracts/gen/{catalog,users,playback}/v1`.
- Команды в `Makefile`: `proto-tools` (установка зафиксированных `buf`, `protoc-gen-go`, `protoc-gen-go-grpc` в `.tools/bin`), `proto` (форматирование и кодогенерация), `proto-lint`, `proto-breaking`, `proto-check`.
- В `.github/workflows/ci.yml` добавлен job `proto` (`make proto-tools`, `make proto-lint`, `make proto-check`, `make proto-breaking`).
- Политика обратной совместимости `v1` и правила эволюции до `v2` зафиксированы в `api/proto/README.md`.

---
