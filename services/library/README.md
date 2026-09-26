# Library Service

Сохранённые треки («Liked tracks») и альбомы слушателя (EPIC-014, `specs/services/library.md`).
Схема `library`, таблица `items (user_id, kind, entity_id, saved_at)`; Catalog-данные — только по ID.

## API (`/api/v1/me/library`, Bearer, `Cache-Control: private, no-store`)

| Метод | Путь | |
|---|---|---|
| PUT | `/tracks/{id}`, `/albums/{id}` | сохранить — **идемпотентно**, 204; повтор сохраняет исходный `savedAt`; неизвестный в Catalog (или DELETED) → 404 `TRACK_NOT_FOUND` / `ALBUM_NOT_FOUND` |
| DELETE | `/tracks/{id}`, `/albums/{id}` | убрать — идемпотентно, 204 |
| GET | `/tracks`, `/albums` `?limit=&cursor=` | новые первыми: `{data: [{trackId\|albumId, savedAt}], pagination: {nextCursor, hasMore}}`, limit ≤ 100 |
| GET | `/tracks/contains?ids=`, `/albums/contains?ids=` | какие из ≤ 100 ID сохранены: `{data: [id…]}` — для отметок «liked» на страницах |
| GET | `/summary` | `{tracks, albums}` — счётчики сайдбара |

Пагинация — keyset по `(saved_at, entity_id)` (индекс `items_recent_idx`), курсор непрозрачный.

## События (`library.events`, key = user ID)

`library.track_saved`, `library.track_removed`, `library.album_saved`, `library.album_removed`
(`libs/contracts/events/libraryv1`) — только при реальном изменении: повторный PUT/DELETE событий не порождает.
Transactional outbox (`libs/platform/outbox`): событие пишется в `library.outbox` в транзакции изменения, relay
публикует его в Kafka — at-least-once; сбой записи события откатывает изменение.

## Web BFF

BFF пробрасывает Bearer пользователя (user context propagation) в `/tracks/contains` и проставляет `liked`
на страницах; анонимно — без обращения к Library, при сбое Library страница отдаётся с `unavailable: ["liked"]`.

## Конфигурация

`DATABASE_URL` (обязателен), `REDIS_ADDR`, `KAFKA_BROKERS`/`KAFKA_ENABLED`, `CATALOG_URL`, `CATALOG_TIMEOUT` (800ms),
`AUTH_JWKS_URL`; локально — `make run-library` (:8088).
