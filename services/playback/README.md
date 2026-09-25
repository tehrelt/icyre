# Playback Service

Первый срез EPIC-016: телеметрия плеера. Сессии, серверная очередь, shuffle/repeat на сервере — следующие задачи
(сейчас очередь живёт в клиенте, `apps/web/src/features/player`).

## API

`POST /api/v1/playback/events` (Bearer) → 202:

```json
{ "type": "started|finished|skipped", "playbackId": "uuid — одно проигрывание трека",
  "trackId": "…", "source": "album:<id>", "durationMs": 240000, "listenedMs": 238000 }
```

`listenedMs` — реально проигранное время (перемотка не считается). Валидация: известный `type`, UUID,
`durationMs` 1…7 200 000, `listenedMs` 0…duration (+5 с), `source` вида `album|playlist|artist|search|queue:<id>`;
иначе 422. Время события ставит сервер.

## События

`playback.events` (key = user ID): `playback.started`, `playback.finished`, `playback.skipped` —
`libs/contracts/events/playbackv1`. Потребители: Listening History (EPIC-026), позже Analytics.
`playback.progress` пока не публикуется (TASK-016.9).

## Конфигурация

`REDIS_ADDR` (список отозванных сессий), `KAFKA_BROKERS`, `AUTH_JWKS_URL`, общие `HTTP_*`, `LOG_*`, `OTEL_*`.
