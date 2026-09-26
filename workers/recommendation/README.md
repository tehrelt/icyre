# Recommendation Worker

Строит персональные рекомендации и публикует их в Redis — алгоритм и входы: `specs/workers/recommendation-worker.md`.

## Команды

```text
recommendation-worker          consumer library.events + media.events, сборка каждые RECOMMENDATION_BUILD_EVERY
recommendation-worker migrate  миграции схемы recommendation
recommendation-worker build    одна сборка всех наборов
```

## Устройство

- `internal/domain` — модель и скоринг (чистые функции, покрыты тестами).
- `internal/application` — сборка: каталог + популярность + история + лайки + features → наборы.
- `internal/adapters` — Catalog (REST), ClickHouse (история, популярность), Postgres (лайки, features),
  Kafka (сигналы), Redis (публикация наборов, pipeline).
- Consumer идемпотентен: лайки сравнивают время события (устаревшее удаление не стирает новый лайк),
  features хранят последний master.

## Конфигурация

| ENV | По умолчанию |
|---|---|
| `DATABASE_URL` | — (обязательно) |
| `REDIS_ADDR` | `localhost:6379` |
| `CLICKHOUSE_URL` / `CLICKHOUSE_USERNAME` / `CLICKHOUSE_PASSWORD` / `CLICKHOUSE_DATABASE` | `http://localhost:8123` / `icyre` / `icyre` / `icyre` |
| `KAFKA_BROKERS` | `localhost:9094` |
| `CATALOG_URL` | `http://localhost:8081` |
| `RECOMMENDATION_BUILD_EVERY` / `RECOMMENDATION_SET_TTL` | `10m` / `24h` |
| `RECOMMENDATION_HISTORY_DAYS` / `RECOMMENDATION_POPULARITY_DAYS` | `90` / `30` |
| `RECOMMENDATION_TRACKS` / `RECOMMENDATION_ARTISTS` | `50` / `20` |

Prometheus: `recommendation_build_runs_total{result}`, `recommendation_build_duration_seconds`,
`recommendation_personal_sets`, `kafka_consumer_messages_total`.

## Запуск

```sh
make up-core && make migrate
make run-recommendation-worker   # :8098; нужен Catalog :8081
make recommendations-build       # разовая сборка
```
