# Playback Service

## Ответственность
Состояние проигрывания и пользовательской очереди.

## Возможности
- start session;
- pause/resume metadata;
- queue;
- shuffle;
- repeat mode;
- current position;
- playback telemetry.

## API
```http
POST /playback/sessions
GET /playback/session
PUT /playback/queue
PATCH /playback/state
POST /playback/events
```

## Redis
```text
playback:{user_id}
queue:{user_id}
```

## Events
```text
playback.started
playback.progress
playback.finished
playback.skipped
```

`playback.progress` не должен генерироваться слишком часто.
