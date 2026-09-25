# EPIC-014 — Library Service

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Задачи

- [x] TASK-014.1 Создать module.

- [x] TASK-014.2 Реализовать сохранённые tracks.

- [x] TASK-014.3 Реализовать сохранённые albums.

- [x] TASK-014.4 Реализовать pagination.

- [x] TASK-014.5 Реализовать idempotent save/remove.

- [x] TASK-014.6 Публиковать:
  - `library.track_saved`
  - `library.track_removed`

## Итог реализации

`services/library` (подробности — README сервиса).

- Сохранённые tracks и albums: `PUT/DELETE/GET /api/v1/me/library/{tracks|albums}` (+ `contains`, `summary`);
  сохранить можно только существующий в Catalog (не DELETED) элемент.
- Pagination: keyset по `(saved_at, entity_id)`, непрозрачный курсор, новые первыми (integration-тест с совпадающими `saved_at`).
- Idempotent save/remove: `ON CONFLICT DO NOTHING` / `DELETE`, 204 в обоих случаях, повтор не меняет `savedAt` и не публикует событий.
- События `library.events` (key = user): `track_saved/removed`, `album_saved/removed` — только при реальном изменении.
- BFF: user context propagation — `liked` на странице альбома из Library; деградация до `unavailable: ["liked"]`.
- Frontend: сердечко в строках треков (альбом, поиск, trending) и в плеере — оптимистично с откатом;
  счётчики сайдбара из `/me/library/summary`; в поиске отметки через `contains`.
- Проверено e2e через gateway и в браузере: лайк → счётчик 1, после перезагрузки лайк на месте (страница BFF), в поиске тоже;
  4 PUT → 3 `track_saved`, 2 DELETE → 1 `track_removed`; неизвестный трек → 404; без токена → 401.
- Экран Library (EPIC-049) и «Liked tracks» — нет макетов, BLOCKED; плейлисты сайдбара — `GET /me/playlists` (EPIC-013).

---
