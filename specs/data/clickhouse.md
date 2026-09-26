# ClickHouse

## Назначение
Большие объёмы append-heavy аналитических событий (прослушивания) и дневные агрегаты по ним.

## Почему не PostgreSQL
Агрегации по миллионам событий лучше выполнять в column-oriented analytics DB.

## Стенд
`clickhouse/clickhouse-server:26.3`, база `icyre`, только HTTP-интерфейс `:8123`
(native `:9000` занят MinIO). Клиент — `libs/platform/clickhouse` (net/http, typed query parameters
`{name:Type}`, JSONEachRow, insert deduplication token, миграции).

## Схема
Источник истины — `libs/contracts/analytics/migrations` (общий контракт Analytics Worker и читателей агрегатов).
Применяет `analytics migrate` (`make migrate`); версии — в `schema_migrations`. DDL в ClickHouse не транзакционный,
поэтому каждый statement идемпотентен (`IF NOT EXISTS`).

| Таблица | Engine / ключ | Содержимое |
|---|---|---|
| `playback_events` | ReplacingMergeTree(`ingested_at`), `PARTITION BY toYYYYMM(at)`, `ORDER BY (toDate(at), track_id, event_id)` | сырые события `playback.events` + обогащение (`album_id`, `artist_ids`) |
| `daily_track_stats` | ReplacingMergeTree(`computed_at`), `ORDER BY (day, track_id)` | plays, completions, skips, unique listeners, listened ms |
| `daily_artist_stats` | ReplacingMergeTree(`computed_at`), `ORDER BY (day, artist_id)` | то же по артисту (трек с несколькими артистами засчитывается каждому) |
| `daily_totals` | ReplacingMergeTree(`computed_at`), `ORDER BY day` | то же по платформе, `active_listeners` — DAL |

Определения: plays — `playback.started`, completions — `playback.finished`, skips — `playback.skipped`,
completion rate = completions / (completions + skips) — считается при чтении.

## Идемпотентность
- Повтор batch'а после сбоя: `insert_deduplication_token` (для нереплицированного MergeTree включён
  `non_replicated_deduplication_window`).
- Повторная доставка события в другом batch: одинаковый `event_id` → одна строка после merge
  ReplacingMergeTree; точные подсчёты читают `FINAL`.
- Агрегаты не материализованные view, а пересчёт дня job'ом (`INSERT … SELECT … FINAL`): новая строка
  с большим `computed_at` вытесняет старую. Поздние события и повторы не дают двойного счёта — день просто
  пересчитывается ещё раз.

## Retention
- Raw events: TTL 180 дней (`analytics.RawRetentionDays`).
- Агрегаты: TTL 5 лет (`analytics.AggregateRetentionYears`).
- Партиции по месяцу (totals — по году) и `ttl_only_drop_parts = 1`: TTL удаляет целые parts, без перезаписи.
