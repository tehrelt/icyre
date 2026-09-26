# Analytics Worker

Владелец аналитического хранилища ClickHouse (`specs/data/clickhouse.md`, `specs/workers/analytics.md`).

## Команды

```text
analytics migrate   применить схему libs/contracts/analytics к ClickHouse (идемпотентно)
```

Consumer `playback.events` и aggregation jobs — EPIC-028.

## Конфигурация

| ENV | По умолчанию |
|---|---|
| `CLICKHOUSE_URL` | `http://localhost:8123` |
| `CLICKHOUSE_USERNAME` / `CLICKHOUSE_PASSWORD` | `icyre` / `icyre` |
| `CLICKHOUSE_DATABASE` | `icyre` |
| `CLICKHOUSE_TIMEOUT` | `30s` |

## Проверка

```sh
make up-core && make migrate
make test-integration   # схема на временной базе: дедупликация событий, дневная агрегация
```
