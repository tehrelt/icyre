# Current focus

## Backend — первая Catalog vertical slice

- [x] EPIC-001 — Bootstrap монорепозитория
- [x] EPIC-002 — Shared platform library
- [x] EPIC-003 — PostgreSQL foundation
- [x] EPIC-005 — Kafka foundation
- [-] EPIC-006 — Observability foundation (осталось: gRPC propagation → вместе с EPIC-034)
- [x] EPIC-007 — Catalog Service: domain model
- [x] EPIC-008 — Catalog Service: PostgreSQL adapter
- [x] EPIC-009 — Catalog Service: HTTP API
- [x] EPIC-010 — Catalog events

Catalog Service — эталон для остальных сервисов (`services/catalog/README.md`).

## Frontend

- [x] EPIC-041 — Frontend Foundation
- [x] EPIC-042 — Design Tokens
- [-] EPIC-043 — Core UI Components
- [x] EPIC-044 — Application Shell
- [-] EPIC-045 — Global Player
- [x] EPIC-046 — Home
- [x] EPIC-047 — Catalog Screens (Album)
- [x] EPIC-048 — Search
- [-] EPIC-052 — Responsive / Mobile
- [-] EPIC-053 — Frontend testing

## Next

Все экраны canvas (Home, Album, Search + состояния) реализованы. Дальше — реальные данные вместо mock:

1. EPIC-033 — Web BFF: `/api/v1/pages/home`, `/api/v1/pages/albums/{id}` поверх Catalog Service.
2. EPIC-004 Redis → EPIC-011 Auth Service (нужен для персональных данных BFF).
3. EPIC-023…025 — OpenSearch, Search Indexer, Search Service (контракт `/api/v1/search` уже задан фронтендом).
4. Transactional outbox для catalog events (at-least-once end to end).
5. Новые frontend-экраны (Library, Playlist, Auth, Artist) — после макетов в canvas.
