# EPIC-017 — Stream Authorization Service

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Задачи

- [x] TASK-017.1 Создать module.

- [x] TASK-017.2 Реализовать проверку Track status.

- [x] TASK-017.3 Реализовать lookup media variant.

- [x] TASK-017.4 Реализовать signed URL.

- [x] TASK-017.5 Добавить TTL signed URL.

- [x] TASK-017.6 Реализовать:
  - `POST /stream/authorize`

- [x] TASK-017.7 Добавить audit/security logs.

## Definition of Done

- Backend возвращает URL.
- Audio body не проксируется через сервис.
- URL автоматически истекает.

## Итог реализации

`services/stream-auth`, `POST /api/v1/stream/authorize` (через gateway). Подробности — `services/stream-auth/README.md`.

- Track status из Catalog (READY → ok; BLOCKED → 403 `TRACK_UNAVAILABLE`; DRAFT/PROCESSING → 409 `TRACK_NOT_READY`;
  DELETED/нет → 404), кеш статуса 10 с.
- Media variant: `HEAD` вариантов в MinIO, выбор запрошенного/лучшего ниже/ближайшего выше, кеш непустого набора.
- Signed URL: presigned GET, TTL 5 мин (конфиг, 1–10 мин), без пользовательских данных, scoped на один объект.
- Audit: лог-строка на каждое решение (без URL) + `stream_authorizations_total{result,reason}`.
- Frontend: плеер вызывает `POST /stream/authorize`, играет URL через `HTMLAudioElement`; при истечении URL
  переавторизуется и продолжает с той же позиции.
- Проверено e2e: 401 без токена; 200 + URL; Range → 206; `quality=64` → 64; BLOCKED → 403; нет трека → 404;
  подменённый путь в URL → 403; прямой доступ к объекту без подписи → 403; воспроизведение в браузере идёт напрямую из MinIO.
- Нет в модели: блокировки аккаунтов, региональные ограничения, подписки — проверки появятся вместе с ними.
  Playback Service (EPIC-016) пока не существует, поэтому клиент вызывает Stream Authorization напрямую.

---
