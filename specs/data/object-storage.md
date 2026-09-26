# S3 / MinIO

## Назначение
Хранение бинарных media assets.

## Layout
```text
tracks/{trackId}/original/{uploadId}.{flac|wav|mp3}
tracks/{trackId}/audio/64.aac
tracks/{trackId}/audio/128.aac
tracks/{trackId}/audio/256.aac
tracks/{trackId}/cover/cover.webp
```

## Properties
- immutable variants;
- checksums;
- metadata;
- lifecycle policies;
- versioning для критичных bucket при необходимости.

## Upload
Использовать pre-signed URLs.

## Download
Через CDN.
