# libs/platform

Переиспользуемая инфраструктура Go-сервисов ICYRE. **Без доменных типов** (Track, Album, User…).

| Пакет | Назначение |
|---|---|
| `config` | Чтение конфигурации из ENV, валидация обязательных переменных |
| `logger` | `log/slog` JSON-логгер: `service`, `request_id`, `trace_id` из контекста |
| `httpserver` | HTTP-сервер, middleware (request ID, access log, recovery, timeout, tracing, metrics), JSON и error model |
| `health` | `GET /health/live`, `GET /health/ready` с readiness-проверками |
| `shutdown` | SIGINT/SIGTERM и упорядоченное (LIFO) закрытие ресурсов |
| `postgres` | `pgxpool`, таймауты, метрики/трейсинг запросов, миграции (goose) в schema домена; unit of work (`InTx`, `Conn`, `Transactor`) |
| `outbox` | transactional outbox: `Sink` вместо Kafka producer пишет сообщения в `<schema>.outbox` в транзакции изменения, `Relay` публирует их в Kafka (at-least-once, порядок вставки, один relay на таблицу) |
| `redis` | go-redis клиент с таймаутами и pool, `Key(...)` (`<ns>:<entity>:<id>`), generic TTL `Cache[T]` с fallback на источник |
| `objectstore` | S3-совместимое хранилище (minio-go): presigned download (Range, `Cache-Control`) и upload с `x-amz-checksum-sha256`, `Stat`, `VerifySHA256`, readiness по bucket. Подпись идёт на публичный endpoint (CDN/origin) |
| `opensearch` | Тонкий клиент OpenSearch на net/http: JSON-запросы, bulk с external versioning (`external_gte`), шаблоны, индексы, атомарная смена алиасов, readiness |
| `clickhouse` | Тонкий клиент ClickHouse на net/http: statements с typed parameters `{name:Type}`, JSONEachRow insert (deduplication token) и query, версионные миграции (`schema_migrations`), readiness |
| `authn` | Проверка access token (EdDSA JWT, `iss`/`aud`/`exp`), JWKS cache, denylist отозванных сессий в Redis, middleware `Required`/`Optional` |
| `kafka` | Producer и consumer helper (franz-go): retries, DLQ `<topic>.dlq`, trace propagation, метрики; `BatchConsumer` — batch по размеру/linger, commit после flush (для ClickHouse) |
| `telemetry` | OpenTelemetry (OTLP → Jaeger), Prometheus registry и `/metrics` |
| `httpclient` | Клиент межсервисных вызовов: trace propagation, `X-Request-ID`, таймаут |

## Правила

- Никаких бизнес-моделей и доменной логики.
- Метрики — только low-cardinality labels (`method`, `route`, `status`, `topic`, `group`, `operation`, `result`).
  Никаких `user_id`, `track_id`, `request_id` в labels.
