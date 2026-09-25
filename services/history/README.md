# Listening History Service

История прослушиваний слушателя (EPIC-026, `specs/workers/listening-history.md`): consumer `playback.events`
и API истории в одном сервисе. Схема `history`, таблица `listens`.

## Засчитанное прослушивание

`playback.finished` / `playback.skipped` становится listen, если проиграно не меньше `HISTORY_MIN_LISTEN` (30s)
или не меньше `HISTORY_MIN_PERCENT` (50) % трека. Пороги — бизнес-конфигурация. `playback.started` не записывается.

Idempotency: `playback_id` — первичный ключ, повторная доставка события ничего не вставляет.
Неразбираемые события — сразу в `playback.events.dlq`; сбой БД — retries, затем DLQ.

## API (`/api/v1/me/history`, Bearer, только GET, `Cache-Control: private, no-store`)

| Путь | |
|---|---|
| `GET /tracks?limit=&cursor=` | прослушивания, новые первыми: `{data: [{trackId, source, listenedMs, playedAt}], pagination}` (limit ≤ 100, keyset) |
| `GET /sources?limit=` | источники (`album:<id>`, `playlist:<id>`…) по последнему прослушиванию, без повторов (≤ 50) — для «Recently played» |

## Конфигурация

`DATABASE_URL` (обязателен), `REDIS_ADDR`, `KAFKA_BROKERS`, `KAFKA_ENABLED`, `AUTH_JWKS_URL`,
`HISTORY_MIN_LISTEN`, `HISTORY_MIN_PERCENT`.
