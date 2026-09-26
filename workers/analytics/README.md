# Analytics Worker

`playback.events` → ClickHouse → дневные агрегаты и отчёты
(`specs/workers/analytics.md`, схема — `specs/data/clickhouse.md`).

## Команды

```text
analytics                                   consumer + периодический пересчёт агрегатов, /health, /metrics
analytics migrate                           применить схему libs/contracts/analytics (идемпотентно)
analytics aggregate -from D -to D           пересчитать агрегаты дней [from, to] (backfill)
analytics report -from D -to D -limit N     JSON: plays, unique listeners, skips, completion rate,
                                            DAL, топ треков и артистов
```

## Поток

1. **Consumer** (`platform/kafka.BatchConsumer`, группа `analytics`): записи копятся до `ANALYTICS_BATCH_SIZE`
   или `ANALYTICS_BATCH_LINGER`, вставляются одним блоком, затем коммитятся offsets (at-least-once).
   Нераспознаваемые записи → `playback.events.dlq`; незнакомые типы событий пропускаются.
2. **Обогащение**: `album_id` и `artist_ids` трека из Catalog (`GET /api/v1/tracks?ids=`, по 100), кэш на
   `ANALYTICS_TRACK_CACHE_TTL`. Catalog недоступен → batch ретраится: события без артистов навсегда выпали бы
   из чартов артистов. После всех ретраев процесс завершается, orchestrator перезапускает его.
3. **Идемпотентность**: повтор того же batch — `insert_deduplication_token` (hash event ID); повторная доставка
   события в другом batch — ReplacingMergeTree по `event_id` + `FINAL` в агрегации.
4. **Aggregation jobs**: каждые `ANALYTICS_AGGREGATE_EVERY` пересчитываются сегодня и
   `ANALYTICS_AGGREGATE_LOOKBACK_DAYS` дней до него (поздние события). Пересчёт дня — `INSERT … SELECT … FINAL`
   в `daily_track_stats`, `daily_artist_stats`, `daily_totals`; новая строка вытесняет старую.

## Метрики

| Метрика | Смысл |
|---|---|
| plays | `playback.started` |
| completions / skips | `playback.finished` / `playback.skipped` |
| completion rate | completions / (completions + skips) |
| unique listeners | за день — из агрегатов; за период — из сырых событий (точно в пределах 180 дней) |
| DAL | среднее `active_listeners` по дням |
| popular tracks / artists | сумма plays по дням, трек с несколькими артистами засчитывается каждому |

Prometheus: `kafka_consumer_messages_total{result=ok|retry|dlq|skip|error}`,
`analytics_aggregation_runs_total{result}`, `analytics_aggregation_duration_seconds`.

## Конфигурация

| ENV | По умолчанию |
|---|---|
| `CLICKHOUSE_URL` / `CLICKHOUSE_USERNAME` / `CLICKHOUSE_PASSWORD` / `CLICKHOUSE_DATABASE` | `http://localhost:8123` / `icyre` / `icyre` / `icyre` |
| `KAFKA_BROKERS` | `localhost:9094` |
| `CATALOG_URL` / `CATALOG_TIMEOUT` | `http://localhost:8081` / `3s` |
| `ANALYTICS_BATCH_SIZE` / `ANALYTICS_BATCH_LINGER` | `5000` / `2s` |
| `ANALYTICS_TRACK_CACHE_TTL` | `10m` |
| `ANALYTICS_AGGREGATE_EVERY` / `ANALYTICS_AGGREGATE_LOOKBACK_DAYS` | `1m` / `1` |
| `CONSUMER_MAX_RETRIES` / `CONSUMER_RETRY_BACKOFF` | `8` / `1s` |

## Запуск и проверка

```sh
make up-core && make migrate
make run-analytics          # :8096, нужен Catalog на :8081
make analytics-report
make test-integration       # вставка → пересчёт → отчёт на временной базе
```

В compose: `analytics-migrate` (job) и `analytics`; backfill и отчёт —
`docker compose run --rm analytics aggregate|report …`.
