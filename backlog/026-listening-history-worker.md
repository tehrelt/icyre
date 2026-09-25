# EPIC-026 — Listening History Worker

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P2

## Задачи

- [x] TASK-026.1 Подписаться на playback events.

- [x] TASK-026.2 Определить completed listen rule.

- [x] TASK-026.3 Сохранять listening history.

- [x] TASK-026.4 Обеспечить idempotency.

- [x] TASK-026.5 Добавить history API в отдельный сервис или профильный модуль.

## Итог реализации

`services/history` — consumer `playback.events` (group `listening-history`) и API истории в одном сервисе.

- Completed listen rule: ≥ 30 с или ≥ 50 % трека (`HISTORY_MIN_LISTEN`, `HISTORY_MIN_PERCENT`).
- Storage: PostgreSQL `history.listens`, keyset-индекс по `(user_id, played_at, playback_id)`.
- Idempotency: `playback_id` — PK, `ON CONFLICT DO NOTHING`.
- API: `GET /api/v1/me/history/tracks` (пагинация) и `/sources` (недавние альбомы/плейлисты для Home).
- Источник событий: Playback Service (срез 1) + телеметрия плеера во фронтенде.
- Проверено e2e через gateway: повторный `finished` записан один раз, пропуск на 8 с не засчитан, на 45 с — засчитан;
  источники без повторов по свежести; без токена 401.

---
