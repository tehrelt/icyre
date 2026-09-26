# EPIC-037 — Integration testing

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Задачи

- [x] TASK-037.1 PostgreSQL integration tests.

- [x] TASK-037.2 Redis integration tests.

- [x] TASK-037.3 Kafka integration tests.

- [x] TASK-037.4 MinIO integration tests.

- [x] TASK-037.5 OpenSearch integration tests.

- [x] TASK-037.6 End-to-end Catalog flow.

- [x] TASK-037.7 End-to-end Media upload/transcode flow.

- [x] TASK-037.8 End-to-end Playback flow.

## Итог реализации

- Интеграционные тесты (build tag `integration`, пропускаются без env) — аудит: все 18 пакетов
  подключены к `make test-integration`:
  - PostgreSQL: репозитории catalog, auth, user-profile, library, history, playlist, media-ingest,
    workers metadata, audio-analysis, recommendation и `libs/platform/outbox` (каждый тест мигрирует
    свою одноразовую схему/БД);
  - Redis, Kafka, MinIO: `libs/platform/{redis,kafka,objectstore}`;
  - OpenSearch: `services/search`, `workers/search-indexer`;
  - ClickHouse: `libs/platform/clickhouse`, `workers/analytics`.
- `make up-test-infra` — `docker compose up --wait` только нужной инфраструктуры (healthchecks compose).
- CI: job `integration` в `.github/workflows/ci.yml` — `make up-test-infra` → `make test-integration`,
  логи стенда при падении, `docker compose down -v`.
- End-to-end (`tests/e2e`, отдельный модуль в `go.work`, build tag `e2e`, `make test-e2e`) — через
  API Gateway поднятого `make up` стенда, асинхронные шаги — polling с таймаутом (`E2E_TIMEOUT`, 2m):
  - Catalog: artist/album/track (запись — напрямую в Catalog, gateway read-only) → чтение через
    gateway → 403 на запись через gateway → artist в поиске → track PROCESSING → READY → в поиске
    как `available`;
  - Media: listener получает 403 → роль ARTIST (прямой UPDATE `auth.accounts`, admin-API ролей нет;
    нужен `E2E_DATABASE_DSN`) → upload session → 409 на ранний complete → presigned PUT WAV в MinIO →
    complete → трек READY после Transcoder;
  - Playback: register → login → 401 без токена → пустая история → started/finished → 422 на
    неверный тип → прослушивание в `/me/history/tracks`.
- `.github/workflows/e2e.yml` — полный стенд в CI (`make images`, `docker compose up`, ожидание
  сервисов за gateway, `make test-e2e`) по `workflow_dispatch` и ночью.
