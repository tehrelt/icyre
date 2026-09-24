# EPIC-020 — Transcoder Worker

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Priority:** P1

## Задачи

- [ ] TASK-020.1 Создать worker module.

- [ ] TASK-020.2 Подключить Kafka consumer.

- [ ] TASK-020.3 Реализовать download original.

- [ ] TASK-020.4 Подключить FFmpeg.

- [ ] TASK-020.5 Генерировать:
  - 64 kbps;
  - 128 kbps;
  - 256 kbps.

- [ ] TASK-020.6 Загружать variants в MinIO.

- [ ] TASK-020.7 Реализовать idempotency.

- [ ] TASK-020.8 Публиковать `track.transcoded`.

- [ ] TASK-020.9 Добавить retry.

- [ ] TASK-020.10 Добавить DLQ.

---
