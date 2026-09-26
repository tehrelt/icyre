# EPIC-029 — Recommendation Service

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P2

## Задачи

- [x] TASK-029.1 Создать recommendation worker.

- [x] TASK-029.2 Использовать:
  - history;
  - likes;
  - follows;
  - popularity;
  - audio features.

- [x] TASK-029.3 Реализовать MVP scoring.

- [x] TASK-029.4 Сохранять готовые рекомендации в Redis.

- [x] TASK-029.5 Создать Recommendation API.

- [x] TASK-029.6 Добавить fallback popular tracks.

## Итог реализации

- `workers/recommendation`: history и popularity из ClickHouse, likes из `library.events`, audio features из
  `audio.features_extracted` (своя схема Postgres), каталог/жанры/релизы из Catalog.
- Follows: в модели и скоринге учтены (`FollowedArtists`, +3 к artist affinity), источника пока нет — Social Service
  (EPIC-015) не реализован.
- MVP scoring: `style·0.35 + artist·0.30 + popularity·0.20 + freshness·0.15`, style — жанры + звучание;
  исключение лайкнутого и пропускаемого, штраф дослушанному, причины рекомендаций.
- Redis: `recommendations:user:{id}` и `recommendations:popular` (контракт `libs/contracts/recommendation`), TTL 24 ч,
  сборка раз в 10 минут и командой `build`.
- `services/recommendation`: `GET /api/v1/recommendations/home|tracks|artists`, токен необязателен, fallback
  `popular`; gateway, compose, Prometheus.
- Проверено на стенде: гость → `popular`; после лайка → `personal`, лайкнутый трек исключён, трек того же артиста
  первым с причиной `artist`.

---
