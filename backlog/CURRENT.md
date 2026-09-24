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
- [-] EPIC-052 — Responsive / Mobile
- [-] EPIC-053 — Frontend testing

## Next

1. EPIC-047 — Album screen (canvas `Album.dc.html`) + TASK-043.7 (SearchInput, Tabs).
2. EPIC-048 — Search (5 состояний canvas).
3. Backend: EPIC-004 Redis foundation → EPIC-011 Auth Service; EPIC-033 Web BFF (`/api/v1/pages/home`) чтобы заменить mock Home.
4. Transactional outbox для catalog events (at-least-once end to end).
