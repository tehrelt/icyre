# libs/platform

Переиспользуемая инфраструктура Go-сервисов ICYRE. **Без доменных типов** (Track, Album, User…).

| Пакет | Назначение |
|---|---|
| `config` | Чтение конфигурации из ENV, валидация обязательных переменных |
| `logger` | `log/slog` JSON-логгер: `service`, `request_id`, `trace_id` из контекста |
| `httpserver` | HTTP-сервер, middleware (request ID, access log, recovery, timeout, tracing, metrics), JSON и error model |
| `health` | `GET /health/live`, `GET /health/ready` с readiness-проверками |
| `shutdown` | SIGINT/SIGTERM и упорядоченное (LIFO) закрытие ресурсов |
| `postgres` | `pgxpool`, таймауты, метрики/трейсинг запросов, миграции (goose) в schema домена |
| `kafka` | Producer и consumer helper (franz-go): retries, DLQ `<topic>.dlq`, trace propagation, метрики |
| `telemetry` | OpenTelemetry (OTLP → Jaeger), Prometheus registry и `/metrics` |

## Правила

- Никаких бизнес-моделей и доменной логики.
- Метрики — только low-cardinality labels (`method`, `route`, `status`, `topic`, `group`, `operation`, `result`).
  Никаких `user_id`, `track_id`, `request_id` в labels.
