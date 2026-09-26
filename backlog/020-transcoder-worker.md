# EPIC-020 — Transcoder Worker

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Priority:** P1

## Задачи

- [x] TASK-020.1 Создать worker module.

- [x] TASK-020.2 Подключить Kafka consumer.

- [x] TASK-020.3 Реализовать download original.

- [x] TASK-020.4 Подключить FFmpeg.

- [x] TASK-020.5 Генерировать:
  - 64 kbps;
  - 128 kbps;
  - 256 kbps.

- [x] TASK-020.6 Загружать variants в MinIO.

- [x] TASK-020.7 Реализовать idempotency.

- [x] TASK-020.8 Публиковать `track.transcoded`.

- [x] TASK-020.9 Добавить retry.

- [x] TASK-020.10 Добавить DLQ.

---
