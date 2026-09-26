# EPIC-028 — Analytics Worker

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P2

## Задачи

- [x] TASK-028.1 Читать playback events.

- [x] TASK-028.2 Писать raw events в ClickHouse.

- [x] TASK-028.3 Создать aggregation jobs.

- [x] TASK-028.4 Считать:
  - plays;
  - unique listeners;
  - skips;
  - completion rate;
  - popular tracks;
  - popular artists.

## Итог реализации

- `libs/platform/kafka.BatchConsumer`: batch до N записей или linger, commit offsets после flush, retries всего batch,
  DLQ для нераспознаваемых записей, `ErrSkip` для незнакомых типов.
- `workers/analytics`: `playback.events` → обогащение `album_id`/`artist_ids` из Catalog (кэш) → `playback_events`
  одним блоком (`insert_deduplication_token`); повторная доставка схлопывается по `event_id`.
- Aggregation jobs: пересчёт сегодня + lookback дней раз в минуту (`INSERT … SELECT … FINAL`), `aggregate` для backfill.
- `report`: plays, unique listeners, skips, completion rate, DAL, топ треков и артистов (JSON).
- Compose (`analytics-migrate`, `analytics`), Prometheus scrape, `make run-analytics`, `make analytics-report`.
- Проверено на стенде: события из Kafka → ClickHouse с артистами, мусор → DLQ, незнакомый тип пропущен,
  агрегаты и отчёт; интеграционные тесты batch consumer и store.

---
