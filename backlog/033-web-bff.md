# EPIC-033 — Web BFF

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [-] IN PROGRESS

**Priority:** P2

## Задачи

- [x] TASK-033.1 Создать BFF module.

- [x] TASK-033.2 Реализовать home page aggregation.

- [x] TASK-033.3 Реализовать album page aggregation.

- [ ] TASK-033.4 Реализовать artist page aggregation.

- [x] TASK-033.5 Добавить параллельные internal calls.

- [x] TASK-033.6 Добавить timeout budget.

- [x] TASK-033.7 Добавить graceful degradation.

## Итог реализации

- `services/bff` (Go module, эталонная структура: ports → application → adapters, composition root в `cmd/bff`).
- `GET /api/v1/pages/home`, `GET /api/v1/pages/albums/{id}` — контракты совпадают с zod-схемами фронтенда.
- Параллельные вызовы (`errgroup`), batch-резолв артистов, page budget + upstream timeout,
  деградация необязательных секций с полем `unavailable`, in-memory кеш жанров.
- Catalog дополнен `GET /api/v1/albums` (новые релизы) и `GET /api/v1/artists?ids=` (batch).
- Проверено в compose: реальные данные через gateway, 503 при недоступном Catalog, деградация Home, трейс BFF → Catalog → DB.
- `GET /api/v1/pages/playlists/{id}`: плейлист (Playlist Service) → треки батчем из Catalog (`GET /tracks?ids=`) и
  владелец (User Profile) параллельно → альбомы треков, артисты, liked; удалённые треки пропускаются. «Recently
  played» на Home показывает альбомы и плейлисты в порядке истории. Плеер веба берёт треки плейлиста из этой страницы.

Осталось: TASK-033.4 — artist page (нет макета в canvas). Внутренний транспорт — REST до EPIC-034 (gRPC).

---
