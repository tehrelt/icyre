# Playlist Service

Первый срез EPIC-013 (`specs/services/playlist.md`). Схема `playlist`: `playlists`, `playlist_tracks`
(`playlist_id, track_id, position, added_by, added_at`).

## API

| Метод | Путь | |
|---|---|---|
| POST | `/api/v1/playlists` | `{title}` → 201 (Bearer); title 1–100 символов, иначе 422 |
| GET | `/api/v1/me/playlists` | свои плейлисты, недавно изменённые первыми (Bearer) — сайдбар |
| GET | `/api/v1/playlists/{id}` | плейлист и треки по позиции (публично) |
| POST | `/api/v1/playlists/{id}/tracks` | `{trackId}` → 204; в конец, повтор — no-op; трек проверяется в Catalog (404 `TRACK_NOT_FOUND`) |
| DELETE | `/api/v1/playlists/{id}/tracks/{trackId}` | 204, идемпотентно |

Менять плейлист может только владелец (403 `FORBIDDEN`). Позиции: добавление под блокировкой строки плейлиста
(`FOR UPDATE`) — параллельные добавления получают разные позиции; удаление оставляет пропуски, порядок сохраняется.

Дальше: PATCH/DELETE плейлиста, reorder (`PATCH /tracks/order`), события `playlist.*` (индексация в поиске).

Конфигурация: `DATABASE_URL`, `REDIS_ADDR`, `CATALOG_URL`, `AUTH_JWKS_URL`; локально порт 8091.
