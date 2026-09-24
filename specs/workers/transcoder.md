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

## Retry
Операция должна быть идемпотентной: повторная обработка того же job не создаёт конфликтующие объекты.
