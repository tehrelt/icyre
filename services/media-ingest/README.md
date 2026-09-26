# Media Ingest Service

Приём мастер-файлов треков (EPIC-019, `specs/services/media-ingest.md`). Байты файла через
backend не идут: сервис выдаёт presigned PUT, клиент грузит прямо в MinIO/S3, затем
вызывает `complete` — сервис проверяет объект и публикует `track.uploaded`.

```text
Artist/Admin ──POST /media/uploads──▶ Media Ingest ──▶ media.uploads (PENDING)
      │  ◀── presigned PUT (Content-Type + x-amz-checksum-sha256)
      ├──PUT file──▶ MinIO  tracks/{trackId}/original/{uploadId}.{ext}
      └──POST /media/uploads/{id}/complete──▶ verify ──▶ COMPLETED + track.uploaded
                                                     └─▶ FAILED + media.ingest.failed
```

## API

| Метод | Путь | Ответ |
|-------|------|-------|
| `POST` | `/api/v1/media/uploads` | `201` сессия + `upload {method,url,headers,expiresAt}` |
| `GET` | `/api/v1/media/uploads/{id}` | `200` сессия (владелец или ADMIN, иначе `404`) |
| `POST` | `/api/v1/media/uploads/{id}/complete` | `200` COMPLETED · `409 UPLOAD_INCOMPLETE` · `422 UPLOAD_REJECTED {reason}` |

Создание: `{"trackId","contentType","sizeBytes","sha256"}` (hex). Только роли `ARTIST`/`ADMIN`
(`403 FORBIDDEN`), трек должен существовать в Catalog (`404 TRACK_NOT_FOUND`).

## Проверки

- **размер** — объявленный `sizeBytes` ≤ `MEDIA_MAX_UPLOAD_BYTES` (по умолчанию 512 MiB);
  на `complete` фактический размер обязан совпасть (`SIZE_MISMATCH`);
- **MIME** — `audio/flac`, `audio/wav`, `audio/x-wav`, `audio/mpeg`; заголовок подписан, а на
  `complete` первые байты сверяются с форматом (`fLaC`, `RIFF…WAVE`, `ID3`/MPEG sync) —
  `CONTENT_TYPE_MISMATCH` / `UNRECOGNIZED_FORMAT`;
- **целостность** — SHA-256 передаётся в `x-amz-checksum-sha256`: хранилище отвергает
  несовпадающее тело, `complete` сверяет сохранённый checksum (`CHECKSUM_MISMATCH`);
- **срок** — объекта нет после истечения URL → `EXPIRED`.

Отклонённый объект удаляется. Каждая загрузка — свой неизменяемый ключ, так что повторная
загрузка не трогает мастер, который может читать транскодер.

## События (`media.events`, ключ — trackId)

`track.uploaded`, `media.ingest.failed` (`libs/contracts/events/mediav1`) пишутся в
`media.outbox` в транзакции смены статуса; relay доставляет их в Kafka. Повторный `complete`
возвращает прежний итог без новых событий; гонка двух `complete` публикует одно событие
(`UPDATE … WHERE status = 'PENDING'`).

## Запуск

```sh
make run-media-ingest     # :8092, нужны Catalog :8081, Auth :8083, MinIO :9000
make test-integration     # MEDIA_TEST_DATABASE_DSN
```
