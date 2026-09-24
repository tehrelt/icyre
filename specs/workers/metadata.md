# Metadata Worker

## Назначение
Извлечение технических метаданных аудиофайла.

## Поля
```text
codec
sample_rate
channels
duration
bitrate
container
```

## Tools
FFprobe / FFmpeg.

## Result
Метаданные пишутся в БД и могут публиковаться событием `media.metadata_extracted`.
