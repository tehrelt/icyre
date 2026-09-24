# Catalog Service

Каталог исполнителей, альбомов, треков и жанров ([spec](../../specs/services/catalog.md)).
**Эталонная реализация** Go-сервиса ICYRE: новые сервисы повторяют эту структуру.

## Слои

```text
cmd/catalog/main.go            composition root: config → logger → OTel → connections → adapters → application → handlers → server → graceful shutdown
internal/domain/               сущности, инварианты, state machine трека, domain errors и events (без pgx/Kafka/HTTP)
internal/application/          use cases; зависят только от ports
internal/ports/                интерфейсы репозиториев и EventPublisher
internal/adapters/http/        REST: DTO, валидация формы запроса, маппинг ошибок (без бизнес-логики)
internal/adapters/postgres/    pgx-репозитории, embedded goose-миграции (schema `catalog`)
internal/adapters/kafka/       domain events → contracts/events/catalogv1 → Kafka
internal/config/               ENV-конфигурация
```

Правила: domain не импортирует инфраструктуру; handlers только транслируют; HTTP DTO ≠ domain;
FK/unique violations БД маппятся в domain errors (БД — последний страж инвариантов).

## API (`/api/v1`)

| Метод | Путь | |
|---|---|---|
| POST | `/artists` | 201 + Location |
| GET | `/artists/{id}` | |
| GET | `/artists/{id}/albums?limit&cursor` | keyset cursor pagination |
| POST | `/albums` | `albumType`: ALBUM, EP, SINGLE, COMPILATION; `releaseDate` YYYY-MM-DD |
| GET | `/albums/{id}` | |
| GET | `/albums/{id}/tracks` | |
| POST | `/tracks` | новый трек в статусе DRAFT; `artistIds` по умолчанию — артисты альбома |
| GET | `/tracks/{id}` | |
| PATCH | `/tracks/{id}` | `title`, `explicit`, `status` (DRAFT→PROCESSING→READY⇄BLOCKED, →DELETED) |
| GET | `/genres` | |

Ошибки — `{"error":{"code","message","requestId","details"}}`: 400 BAD_REQUEST, 404 *_NOT_FOUND,
409 TRACK_POSITION_TAKEN, 422 VALIDATION_FAILED / *_NOT_FOUND (ссылка в теле) / INVALID_STATUS_TRANSITION.

Служебные: `GET /health/live`, `GET /health/ready` (postgres, kafka), `GET /metrics`.

## События (`catalog.events`, key = aggregate ID, eventVersion 1)

`artist.created`, `album.created`, `track.created`, `track.updated` — payloads в `libs/contracts/events/catalogv1`.
Публикация после commit, best effort (ошибка логируется). Следующий шаг — transactional outbox.

## Конфигурация

| ENV | По умолчанию |
|---|---|
| `DATABASE_URL` | **обязательно** |
| `HTTP_ADDR` | `:8080` |
| `HTTP_REQUEST_TIMEOUT` / `SHUTDOWN_TIMEOUT` | `10s` / `15s` |
| `DATABASE_MAX_CONNS`, `DATABASE_STATEMENT_TIMEOUT`, … | `10`, `5s` |
| `MIGRATE_ON_START` | `false` |
| `KAFKA_ENABLED` / `KAFKA_BROKERS` | `true` / `localhost:9094` |
| `OTEL_ENABLED` / `OTEL_EXPORTER_OTLP_ENDPOINT` | `false` / `localhost:4318` |
| `LOG_LEVEL` / `LOG_FORMAT` | `info` / `json` |

## Запуск и тесты

```bash
make up-core && make migrate && make run-catalog
go test ./...                                                         # unit
CATALOG_TEST_DATABASE_DSN=postgres://icyre:icyre@localhost:5432/icyre?sslmode=disable \
  go test -tags integration ./...                                     # на реальном PostgreSQL
```
