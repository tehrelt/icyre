# Web BFF

Backend for Frontend веб-клиента ([spec](../../specs/services/bff.md)): отдаёт page-oriented API,
агрегируя внутренние сервисы. **Не владеет данными**: нет БД, нет Kafka.

## API

| Метод | Путь | Источники |
|---|---|---|
| GET | `/api/v1/pages/home` | Catalog (`GET /albums` → New releases, `GET /artists?ids=`) |
| GET | `/api/v1/pages/albums/{id}` | Catalog: album → (tracks ∥ more-by-artist ∥ genres) → artists batch |

Контракты ответов — `internal/views` (зеркало zod-схем `apps/web/src/pages/*/api`).
Ошибки — error model: 404 `ALBUM_NOT_FOUND`, 503 `SERVICE_UNAVAILABLE` (обязательный upstream недоступен).

## Агрегация

```text
handler → application.Pages ──(port)──► ports.Catalog ◄── adapters/catalog (HTTP client)
```

- **Параллельные вызовы**: `errgroup` внутри страницы; артисты album + tracks резолвятся одним batch-запросом.
- **Timeout budget**: вся страница ограничена `PAGE_BUDGET` (1.5s), каждый upstream-вызов — `UPSTREAM_TIMEOUT` (800ms).
- **Graceful degradation**: обязательные части (альбом, его треки) — ошибка страницы; необязательные
  (more by artist, теги, имена артистов, new releases) — пустые, перечислены в поле `unavailable`.
- Секции Home без сервиса-источника (history, editorial, recommendations, social) пока всегда пустые и тоже в `unavailable`.
- **Кеш**: список жанров in-memory (`GENRE_CACHE_TTL`, 10 мин).
- **Readiness** не проверяет upstream'ы — страницы деградируют сами, а связка readiness ↔ Catalog каскадировала бы отказ.
- Внутренний транспорт — REST через `libs/platform/httpclient` (trace propagation, `X-Request-ID`). gRPC — EPIC-034, порт не меняется.

## Наблюдаемость

`bff_upstream_request_duration_seconds{upstream,operation,result}` + стандартные HTTP-метрики; спаны BFF → Catalog → DB в одном трейсе.

## Конфигурация

| ENV | По умолчанию |
|---|---|
| `CATALOG_URL` | **обязательно** |
| `HTTP_ADDR` | `:8080` |
| `PAGE_BUDGET` / `UPSTREAM_TIMEOUT` | `1500ms` / `800ms` |
| `GENRE_CACHE_TTL` | `10m` |
| `OTEL_ENABLED` / `OTEL_EXPORTER_OTLP_ENDPOINT` | `false` / `localhost:4318` |

```bash
make run-catalog   # :8081
make run-bff       # :8082
```
