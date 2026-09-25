# EPIC-024 — Search Indexer

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Задачи

- [x] TASK-024.1 Создать worker.

- [x] TASK-024.2 Подписаться на catalog events.

- [x] TASK-024.3 Реализовать upsert.

- [x] TASK-024.4 Реализовать delete.

- [x] TASK-024.5 Сделать consumer idempotent.

- [x] TASK-024.6 Добавить rebuild/reindex command.

## Итог реализации

`workers/search-indexer` (подробности — README воркера).

- Consumer `catalog.events` (group `search-indexer`, retries + DLQ): `artist.created`, `album.created`,
  `track.created/updated`; denormalization имён альбома/артистов через Catalog API.
- Upsert/delete: READY/BLOCKED → upsert (`available`), остальные статусы → delete.
- Idempotency: ID документа = ID сущности, версия = `occurredAt` (`external_gte`) — replay и reordering безопасны
  (integration-тест: устаревшая запись не перезаписывает документ).
- `reindex`: из Catalog (альбомы → треки → артисты) в новые индексы, refresh, атомарная смена алиасов.
- Проверено e2e: артист/альбом/трек через Catalog → поиск; DRAFT не виден, READY виден, BLOCKED — `available=false`,
  переименование подхватывается, DELETED удаляется; lag consumer 0.
- Spec перечисляет также `track.deleted`, `album.updated`, `artist.updated`, `playlist.updated` — Catalog их пока не
  публикует (удаление — через статус DELETED); подписка появится вместе с событиями.

---
