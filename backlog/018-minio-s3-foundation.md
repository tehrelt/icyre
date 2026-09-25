# EPIC-018 — MinIO / S3 foundation

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Задачи

- [x] TASK-018.1 Добавить MinIO в Docker Compose.

- [x] TASK-018.2 Создать bucket bootstrap.

- [x] TASK-018.3 Определить object key convention.

- [x] TASK-018.4 Добавить S3 adapter.

- [x] TASK-018.5 Реализовать presigned upload.

- [x] TASK-018.6 Реализовать presigned download.

- [x] TASK-018.7 Добавить checksum validation.

## Итог реализации

- Compose: `minio` (`pgsty/minio:RELEASE.2026-08-04T00-00-00Z` — MinIO Inc. больше не публикует community-образы,
  pgsty пересобирает upstream AGPL-исходники), volume `minio-data`, healthcheck; консоль :9001.
- Bucket bootstrap: job `minio-init` (`deploy/minio/init.sh`, `mc`): приватный bucket `icyre-media`, без анонимного доступа.
- Ключи: `libs/contracts/media` — `tracks/{trackId}/audio/{64|128|256}.aac`, `original/source.{ext}`, `cover/cover.webp`.
- Adapter: `libs/platform/objectstore` (minio-go) — `PresignDownload` (Range, `response-cache-control`),
  `PresignUpload` (подписанные `Content-Type` и `x-amz-checksum-sha256`), `Put` (SHA-256 checksum), `Stat`,
  `VerifySHA256`, readiness по bucket. URL подписываются на публичный endpoint (CDN/origin), сервис ходит по внутреннему.
- Checksum validation проверена integration-тестом на реальном MinIO: тело с чужим SHA-256 отклоняется, без
  подписанного заголовка — отклоняется, после загрузки `VerifySHA256` сверяет digest; просроченный URL → 403; Range → 206.
- `make seed-media` (`scripts/seed-media.ts`, Bun + ffmpeg) генерирует варианты для треков каталога вместо
  ещё не существующего media pipeline (EPIC-019/020).

---
