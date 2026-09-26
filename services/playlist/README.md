# Playlist Service

EPIC-013 (`specs/services/playlist.md`). Схема `playlist`: `playlists`, `playlist_tracks`
(`playlist_id, track_id, position, added_by, added_at`).

## API

| Метод | Путь | |
|---|---|---|
| POST | `/api/v1/playlists` | `{title}` → 201 (Bearer); title 1–100 символов, иначе 422 |
| GET | `/api/v1/me/playlists` | свои плейлисты, недавно изменённые первыми (Bearer) — сайдбар |
| GET | `/api/v1/playlists/{id}` | плейлист и треки по позиции (публично) |
| POST | `/api/v1/playlists/{id}/tracks` | `{trackId}` → 204; в конец, повтор — no-op; трек проверяется в Catalog (404 `TRACK_NOT_FOUND`) |
| DELETE | `/api/v1/playlists/{id}/tracks/{trackId}` | 204, идемпотентно |
| PATCH | `/api/v1/playlists/{id}` | `{title}` → 200 и плейлист; тот же title — no-op без события |
| DELETE | `/api/v1/playlists/{id}` | 204 вместе с треками; повтор — 404 |
| GET | `/internal/v1/playlists?after={id}&limit=` | все плейлисты по ID (keyset, до 500) → `{data, nextAfter}` — для `reindex` Search Indexer; gateway не проксирует |
| PATCH | `/api/v1/playlists/{id}/tracks/order` | `{trackIds}` — полный новый порядок → 204; список не совпал с треками плейлиста — 409 `PLAYLIST_ORDER_MISMATCH` |

Менять плейлист может только владелец (403 `FORBIDDEN`). Позиции: добавление под блокировкой строки плейлиста
(`FOR UPDATE`) — параллельные добавления получают разные позиции; удаление оставляет пропуски, порядок сохраняется.
Reorder под той же блокировкой сверяет список с текущими треками и переписывает позиции в 1..n одним `UPDATE`;
уникальность `(playlist_id, position)` проверяется на commit (`DEFERRABLE`, миграция 00002).

## События

`playlist.events` (ключ — ID плейлиста, `libs/contracts/events/playlistv1`): `playlist.created`, `playlist.updated`,
`playlist.deleted`, `playlist.track_added` (с позицией), `playlist.track_removed`, `playlist.tracks_reordered`
(полный порядок). Публикуются только реальные изменения, после commit; сбой публикации логируется (outbox — позже).

Search Indexer индексирует плейлисты по этим событиям. Дальше: плейлисты в «Recently played», UI редактирования.

Конфигурация: `DATABASE_URL`, `REDIS_ADDR`, `CATALOG_URL`, `AUTH_JWKS_URL`, `KAFKA_BROKERS` (`KAFKA_ENABLED=false` —
без событий); локально порт 8091.
