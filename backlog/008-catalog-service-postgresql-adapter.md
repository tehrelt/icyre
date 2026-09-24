# EPIC-008 — Catalog Service: PostgreSQL adapter

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P0

## Задачи

- [x] TASK-008.1 Создать миграции:
  - artists;
  - albums;
  - tracks;
  - genres;
  - track_artists;
  - album_artists.

- [x] TASK-008.2 Реализовать Artist repository.

- [x] TASK-008.3 Реализовать Album repository.

- [x] TASK-008.4 Реализовать Track repository.

- [x] TASK-008.5 Добавить индексы.

- [x] TASK-008.6 Добавить integration tests с PostgreSQL.

## Definition of Done

- CRUD базовых сущностей работает.
- Constraints проверяются БД.
- Integration tests проходят на реальном PostgreSQL.

## Итог реализации

Миграции `00001_catalog_core.sql` (constraints, FK, partial unique index позиции трека, индексы keyset-пагинации),
`00002_seed_genres.sql`. Repositories на pgx, FK/unique violations → domain errors.
Integration tests на реальном PostgreSQL (отдельная временная БД на прогон): `make test-integration`.

---
