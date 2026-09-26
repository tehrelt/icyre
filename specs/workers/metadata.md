# Metadata Worker

## Назначение
Извлечение технических метаданных аудиофайла.

## Input event
```text
track.uploaded
```

## Поля
```text
container
codec
duration
bitrate
sample_rate
channels
```

## Tools
FFprobe. Мастер не скачивается: ffprobe читает заголовок по presigned GET URL (HTTP Range).

## Result
Метаданные пишутся в БД (`media_metadata.track_metadata`, строка на upload) и публикуются событием
`media.metadata_extracted` через transactional outbox (media.events, key = trackId).

## Retry
Upload ID — ключ идемпотентности: повторная доставка не пишет ни строку, ни событие. Мастер, которого нет
или который ffprobe не декодирует, пропускается (ошибку сообщает Transcoder через `media.transcode.failed`).
Реализация — `workers/metadata/README.md`.
