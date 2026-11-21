# Playback Service

Управление playback-сессией, очередью, shuffle/repeat в Redis и публикация событий прослушивания в Kafka (EPIC-016).

## API

- `POST /api/v1/playback/sessions` (Bearer) → 201: старт сессии `{tracks, startIndex, source, durationMs, shuffle, repeatMode}`
- `GET /api/v1/playback/session` (Bearer) → 200 / 404 `SESSION_NOT_FOUND`: текущая сессия и очередь
- `PUT /api/v1/playback/queue` (Bearer) → 200: замена очереди `{tracks, startIndex, source}`
- `PATCH /api/v1/playback/state` (Bearer) → 200: частичное обновление `{isPlaying, positionMs, durationMs, shuffle, repeatMode, queueIndex, action}`
- `POST /api/v1/playback/events` (Bearer) → 202:

```json
{ "type": "started|finished|skipped", "playbackId": "uuid — одно проигрывание трека",
  "trackId": "…", "source": "album:<id>", "durationMs": 240000, "listenedMs": 238000 }
```

`listenedMs` — реально проигранное время (перемотка не считается). Валидация: известный `type`, UUID,
`durationMs` 1…7 200 000, `listenedMs` 0…duration (+5 с), `source` вида `album|playlist|artist|search|queue:<id>`;
иначе 422. Время события ставит сервер.

## События

`playback.events` (key = user ID): `playback.started`, `playback.progress`, `playback.finished`, `playback.skipped` —
`libs/contracts/events/playbackv1`. Потребители: Listening History (EPIC-026), Analytics (EPIC-028).
`playback.progress` ограничивается по частоте через Redis (`PLAYBACK_PROGRESS_INTERVAL`, по умолчанию `10s`).

## Конфигурация

`REDIS_ADDR` (сессии `playback:{userId}`, очередь `queue:{userId}`, throttle `playback:progress:{userId}:{playbackId}`, отозванные токены),
`PLAYBACK_SESSION_TTL` (`24h`), `PLAYBACK_PROGRESS_INTERVAL` (`10s`), `KAFKA_BROKERS`, `AUTH_JWKS_URL`, общие `HTTP_*`, `LOG_*`, `OTEL_*`.

`ANALYTICS_MODE` — `kafka` (по умолчанию) или `sync`. `sync` — только для эксперимента EPIC-038
(`make load-analytics`): событие обогащается из Catalog (`CATALOG_URL`) и пишется в ClickHouse
(`CLICKHOUSE_*`) прямо в запросе, в Kafka не попадает — History и рекомендации его не видят.
