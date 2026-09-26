# EPIC-027 — ClickHouse foundation

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P2

## Задачи

- [x] TASK-027.1 Добавить ClickHouse в Docker Compose.

- [x] TASK-027.2 Создать playback events table.

- [x] TASK-027.3 Создать daily aggregates.

- [x] TASK-027.4 Добавить retention strategy.

## Итог реализации

- Compose: `clickhouse/clickhouse-server:26.3`, база `icyre`, HTTP `:8123` (native `:9000` занят MinIO),
  volume `clickhouse-data`, healthcheck `/ping`; входит в `make up-core`.
- `libs/platform/clickhouse` — клиент на net/http: typed query parameters, JSONEachRow insert/query,
  `insert_deduplication_token`, readiness, миграции `NNNNN_name.sql` с учётом версий в `schema_migrations`.
- Схема — `libs/contracts/analytics`: `playback_events` (ReplacingMergeTree по `event_id`, обогащение `album_id`/`artist_ids`),
  `daily_track_stats`, `daily_artist_stats`, `daily_totals` (ReplacingMergeTree по `computed_at` — пересчёт дня идемпотентен).
- Retention: raw — TTL 180 дней, агрегаты — 5 лет; партиции по месяцу, `ttl_only_drop_parts`.
- `workers/analytics migrate` применяет схему (`make migrate`); consumer и jobs — EPIC-028.
- Проверено на реальном ClickHouse: повторный migrate — no-op, дедупликация batch и повторного события, дневная агрегация.

---
