# Stream Authorization Service

Проверяет, что трек можно слушать, и выдаёт short-lived signed URL на аудио-вариант (EPIC-017,
`specs/services/stream-auth.md`, `specs/architecture/media-flow.md`). Аудио через сервис не проходит:
браузер берёт байты напрямую из media origin (MinIO локально, CDN в production) с HTTP Range.

## API

`POST /api/v1/stream/authorize` (Bearer):

```json
{ "trackId": "0192…", "quality": "256" }          // quality необязательно: 64 | 128 | 256
```

```json
{ "trackId": "0192…", "quality": "128", "url": "http://localhost:9000/icyre-media/tracks/…/audio/128.aac?X-Amz-…", "expiresAt": "…" }
```

`Cache-Control: no-store`. Ошибки: 401 `UNAUTHENTICATED`, 422 `VALIDATION_FAILED`, 404 `TRACK_NOT_FOUND`
(в том числе DELETED), 403 `TRACK_UNAVAILABLE` (BLOCKED), 409 `TRACK_NOT_READY` (DRAFT/PROCESSING или ещё нет
аудио), 503 — Catalog или хранилище недоступны.

## Правила

- Играет только трек в статусе `READY` (статус — из Catalog, кешируется в Redis на `STREAM_STATUS_CACHE_TTL`, 10 с).
- Вариант: запрошенный, иначе лучший ниже, иначе ближайший выше. По умолчанию 256 kbps.
  Наличие вариантов проверяется `HEAD` в хранилище; непустой набор кешируется на час (варианты immutable).
- URL: presigned GET на один объект, TTL `STREAM_URL_TTL` (5 мин, допустимо 1–10), без пользовательских данных,
  `response-cache-control: private, max-age=<ttl>`. Клиент при истечении URL запрашивает новый и продолжает с той же позиции.
- Пользователь: валидный access token и неотозванная сессия (`libs/platform/authn`). Блокировок аккаунтов,
  регионов и подписок в модели пока нет — проверки добавятся вместе с ними.

## Аудит и метрики

Каждое решение — строка лога `audit=stream_authorization` с `result` (granted/denied/error), `reason`,
`user_id`, `session_id`, `track_id`, `quality_requested`, `quality`, `request_id`, `trace_id`. Signed URL не логируется.
Метрика `stream_authorizations_total{result,reason}`.

## Конфигурация

| ENV | По умолчанию | |
|---|---|---|
| `S3_ENDPOINT` / `S3_SECURE` | `localhost:9000` / `false` | адрес хранилища для сервиса |
| `S3_PUBLIC_ENDPOINT` / `S3_PUBLIC_SECURE` | = `S3_ENDPOINT` | на него подписываются URL (CDN/origin) |
| `S3_ACCESS_KEY`, `S3_SECRET_KEY` | — | обязательны |
| `S3_MEDIA_BUCKET` | `icyre-media` | |
| `CATALOG_URL` / `CATALOG_TIMEOUT` | `http://localhost:8081` / `800ms` | |
| `AUTH_JWKS_URL` | `http://localhost:8083/api/v1/auth/.well-known/jwks.json` | |
| `REDIS_ADDR` | `localhost:6379` | |
| `STREAM_URL_TTL` | `5m` | |

## Локально

```bash
make up-core && make run-catalog   # + make run-auth
make seed && make seed-media       # каталог и аудио (ffmpeg)
make run-stream-auth               # :8085
```
