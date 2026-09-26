# Metadata Worker

Извлекает технические метаданные загруженного мастера (EPIC-021, `specs/workers/metadata.md`).
Работает параллельно с Transcoder: оба читают `track.uploaded` своими consumer group.

## Поток

```text
media.events: track.uploaded
  -> stat tracks/{trackId}/original/{uploadId}.{ext}
  -> presigned GET (TTL = PROBE_TIMEOUT) -> ffprobe читает заголовок по HTTP Range, мастер не скачивается
  -> media_metadata.track_metadata + media_metadata.outbox (одна транзакция)
  -> relay -> media.events: media.metadata_extracted
```

Consumer group `metadata-worker`, `/health/*` и `/metrics` на `HTTP_ADDR`. Локально: `make run-metadata`
(нужен ffprobe в PATH, миграции применяются на старте); в compose — сервис `metadata-worker`.

## Данные

`media_metadata.track_metadata` — строка на upload: `container`, `codec`, `duration_ms`, `bitrate_bps`,
`sample_rate_hz`, `channels`, `source_sha256`, `uploaded_at`, `extracted_at`. Повторная загрузка трека —
новая строка; текущий мастер — последний `uploaded_at` (индекс `track_id, uploaded_at DESC`).

Значения — первого аудиопотока; длительность и битрейт потока, иначе контейнера (битрейт контейнера
учитывает заголовки). `bitrate_bps = 0` — ни поток, ни контейнер его не объявляют.

## События

| Событие | Когда |
|---|---|
| `media.metadata_extracted` | строка записана; payload: `uploadId`, `trackId`, `container`, `codec`, `durationMs`, `bitrateBps`, `sampleRateHz`, `channels`, `sourceSha256`, `uploadedAt`, `extractedAt` |

Ключ — track ID, тот же порядок, что у остальных `media.events`.

## Идемпотентность и ошибки

- upload уже записан — `duplicate`, без работы; гонка двух доставок — `INSERT … ON CONFLICT DO NOTHING`,
  событие пишет только вставившая строку;
- мастера нет (`missing`) или ffprobe его отверг (`undecodable`) — лог и commit: ошибку публикует Transcoder
  (`media.transcode.failed`);
- хранилище, ffprobe не запустился, Postgres — retries платформенного consumer
  (`CONSUMER_MAX_RETRIES`, `CONSUMER_RETRY_BACKOFF`), затем `media.events.dlq`;
- нераспознанный envelope/payload, неизвестная версия, неполный job — сразу в DLQ.

Метрика: `media_metadata_extract_duration_seconds{result=extracted|duplicate|missing|undecodable|error}`.

## Конфигурация

`DATABASE_URL`, `MIGRATE_ON_START` (false), `KAFKA_BROKERS`, `S3_ENDPOINT`, `S3_SECURE`, `S3_ACCESS_KEY`,
`S3_SECRET_KEY`, `S3_REGION`, `S3_TIMEOUT`, `S3_MEDIA_BUCKET` (`icyre-media`, для readiness), `FFPROBE_PATH`
(`ffprobe`), `PROBE_TIMEOUT` (1m), `CONSUMER_MAX_RETRIES` (5), `CONSUMER_RETRY_BACKOFF` (2s).
