# EPIC-013 — Playlist Service

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [-] IN PROGRESS

**Priority:** P1

## Задачи

- [x] TASK-013.1 Создать Playlist aggregate.

- [x] TASK-013.2 Создать PlaylistTrack.

- [x] TASK-013.3 Реализовать position ordering.

- [ ] TASK-013.4 Реализовать CRUD плейлистов.

- [x] TASK-013.5 Реализовать add track.

- [x] TASK-013.6 Реализовать remove track.

- [ ] TASK-013.7 Реализовать reorder.

- [ ] TASK-013.8 Публиковать playlist events.

## Прогресс

- Срез 1 (`services/playlist`): aggregate и PlaylistTrack, position ordering (append под `FOR UPDATE`,
  integration-тест на параллельные добавления), create/get/`/me/playlists`, add/remove track (идемпотентно, проверка
  трека в Catalog, только владелец). Сайдбар читает `/me/playlists`.
- Осталось: TASK-013.4 (PATCH/DELETE плейлиста), TASK-013.7 reorder, TASK-013.8 события.

---
