# Social Service

Подписки слушателя на пользователей и артистов (EPIC-015, `specs/services/social.md`).
Схема `social`: `follows (follower_id, target_type, target_id, followed_at)` и `counters`; пользователи и артисты —
только по ID (User Profile, Catalog), без cross-schema foreign keys.

## API (через gateway, `/api/v1`)

| Метод | Путь | |
|---|---|---|
| PUT | `/users/{id}/follow`, `/artists/{id}/follow` | подписаться — Bearer, **идемпотентно**, 204; повтор сохраняет исходный `followedAt`; неизвестный пользователь/артист → 404 `USER_NOT_FOUND` / `ARTIST_NOT_FOUND`; на себя → 422 `CANNOT_FOLLOW_SELF` |
| DELETE | `/users/{id}/follow`, `/artists/{id}/follow` | отписаться — Bearer, идемпотентно, 204 |
| GET | `/users/{id}/followers`, `/artists/{id}/followers` `?limit=&cursor=` | подписчики, новые первыми: `{data: [{userId, followedAt}], pagination: {nextCursor, hasMore}}` |
| GET | `/users/{id}/following?type=user\|artist&limit=&cursor=` | подписки (по умолчанию `user`): `{data: [{userId\|artistId, followedAt}], pagination}` |
| GET | `/users/{id}/follow-counts` | `{followers, followingUsers, followingArtists}` |
| GET | `/artists/{id}/follow-counts` | `{followers}` |
| GET | `/me/following/contains?type=user\|artist&ids=` | Bearer: на кого из ≤ 100 ID подписан — `{data: [id…]}` для кнопок Follow |

Списки и счётчики публичны; запросы с Bearer — `Cache-Control: private, no-store`. Gateway направляет сюда только
суффиксы `/follow`, `/followers`, `/following`, `/follow-counts`; остальное под `/users/*` и `/artists/*` — в User Profile и Catalog.

## Хранение

- Keyset-пагинация по `(followed_at, id)`: индекс `follows_followers_idx` для подписчиков, `follows_following_idx` для подписок.
- Счётчики денормализованы в `social.counters` и меняются **тем же SQL-выражением**, что и ребро (data-modifying CTE):
  ребро и счётчики не расходятся, заголовок профиля/артиста не делает `count(*)` по большому множеству подписчиков.
  Цена — конкуренция за строку счётчика популярного артиста; для очень большого графа — шардирование счётчика или
  асинхронный пересчёт из `social.events` (`specs/services/social.md`, scale note).
- Подписка на себя запрещена и в домене, и `CHECK` в таблице.

## События (`social.events`, key = follower ID)

`social.followed`, `social.unfollowed` (`libs/contracts/events/socialv1`) — только при реальном изменении.
Transactional outbox (`libs/platform/outbox`): событие пишется в `social.outbox` в транзакции изменения — at-least-once.

## Конфигурация

`DATABASE_URL` (обязателен), `REDIS_ADDR`, `KAFKA_BROKERS`/`KAFKA_ENABLED`, `USER_PROFILE_URL`, `CATALOG_URL`,
`DIRECTORY_TIMEOUT` (800ms), `AUTH_JWKS_URL`; локально — `make run-social` (:8099).
