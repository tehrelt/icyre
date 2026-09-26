# Transcoder Worker

## Назначение
Конвертация исходного аудио в stream-ready варианты.

## Input event
```text
track.uploaded
```

## Pipeline
1. Скачать original.
2. Проверить декодирование.
3. FFmpeg transcoding.
4. Загрузить варианты.
5. Сохранить media metadata.
6. Отправить `track.transcoded`.

## Output
```text
64 kbps
128 kbps
256 kbps
```

## Output events
```text
track.transcoded        — все варианты записаны (media.events, key = trackId)
media.transcode.failed  — SOURCE_MISSING | SOURCE_CORRUPTED | UNDECODABLE
```

## Retry
Операция должна быть идемпотентной: повторная обработка того же job не создаёт конфликтующие объекты.
Варианты хранят provenance мастера (upload ID, SHA-256, время загрузки): повтор того же мастера не транскодирует
заново, устаревший job пропускается. Реализация — `workers/transcoder/README.md`.
