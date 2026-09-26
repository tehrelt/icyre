# Transcoder

Превращает загруженный мастер в stream-ready варианты (EPIC-020, `specs/workers/transcoder.md`).
Заменяет `make seed-media` для треков, прошедших Media Ingest.

## Поток

```text
media.events: track.uploaded
  -> download tracks/{trackId}/original/{uploadId}.{ext} (SHA-256 сверяется с событием)
  -> ffprobe: есть аудиопоток и длительность
  -> ffmpeg, один проход: AAC-LC / ADTS, stereo, 44.1 kHz, 64/128/256 kbps
  -> PUT tracks/{trackId}/audio/{64,128,256}.aac (+ provenance в x-amz-meta-*)
  -> media.events: track.transcoded
```

Consumer group `transcoder`, `/health/*` и `/metrics` на `HTTP_ADDR`. Локально: `make run-transcoder`
(нужны ffmpeg/ffprobe в PATH); в compose — сервис `transcoder` (образ на alpine с ffmpeg).

## События

| Событие | Когда |
|---|---|
| `track.transcoded` | все три варианта записаны; payload: `trackId`, `uploadId`, `codec`, `durationMs`, `variants[]` (quality, key, contentType, sizeBytes), `sourceSha256` |
| `media.transcode.failed` | мастер непригоден, повтор не поможет: `SOURCE_MISSING`, `SOURCE_CORRUPTED` (SHA-256 не совпал), `UNDECODABLE` (ffprobe отверг файл) |

Публикуются в `media.events` с ключом track ID (тот же порядок, что у `track.uploaded`). Outbox нет — у
воркера нет БД: offset `track.uploaded` коммитится после публикации, повтор безопасен (см. ниже).

## Идемпотентность

Каждый вариант хранит provenance: `Upload-Id`, `Source-Sha256`, `Uploaded-At`, `Duration-Ms`.

- все три варианта от того же мастера (тот же SHA-256) — повторная доставка: без работы, `track.transcoded` отправляется снова;
- хоть один вариант от более поздней загрузки — событие устарело, пропускается;
- иначе (нет вариантов, частичный прогон, старый мастер, варианты `seed-media` без provenance) — транскодирование заново,
  варианты перезаписываются на месте.

## Retry и DLQ

Сбой хранилища, FFmpeg или Kafka — retries платформенного consumer (`CONSUMER_MAX_RETRIES`, `CONSUMER_RETRY_BACKOFF`),
затем `media.events.dlq`. Нераспознанный envelope/payload, неизвестная версия, неполный job — сразу в DLQ.
Непригодный мастер — не ошибка доставки: `media.transcode.failed` и commit.

Метрика: `media_transcode_duration_seconds{result=transcoded|duplicate|stale|failed|error}`.

## Ограничения

- Catalog пока не читает `track.transcoded`: статус трека (`PROCESSING` → `READY`) не меняется автоматически.
- Повторная загрузка перезаписывает варианты по тем же ключам: уже выданные signed URL (≤ 10 мин) могут
  получить байты нового мастера посреди Range-чтения.
- Stream Authorization кэширует только полный набор вариантов, поэтому трек в процессе записи вариантов
  не застревает с неполным набором.
- Треки обрабатываются последовательно в рамках партиции; параллелизм — числом партиций `media.events` и реплик.

## Конфигурация

`KAFKA_BROKERS`, `S3_ENDPOINT`, `S3_SECURE`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_REGION`, `S3_TIMEOUT`,
`S3_MEDIA_BUCKET` (`icyre-media`), `FFMPEG_PATH` (`ffmpeg`), `FFPROBE_PATH` (`ffprobe`), `TRANSCODE_TIMEOUT` (10m),
`TRANSCODE_WORK_DIR` (временный каталог ОС), `CONSUMER_MAX_RETRIES` (5), `CONSUMER_RETRY_BACKOFF` (2s).
