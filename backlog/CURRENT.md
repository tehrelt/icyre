# Current focus

## Backend — первая Catalog vertical slice

- [x] EPIC-001 — Bootstrap монорепозитория
- [x] EPIC-002 — Shared platform library
- [x] EPIC-003 — PostgreSQL foundation
- [x] EPIC-004 — Redis foundation
- [x] EPIC-005 — Kafka foundation
- [-] EPIC-006 — Observability foundation (осталось: gRPC propagation → вместе с EPIC-034)
- [x] EPIC-007 — Catalog Service: domain model
- [x] EPIC-008 — Catalog Service: PostgreSQL adapter
- [x] EPIC-009 — Catalog Service: HTTP API
- [x] EPIC-010 — Catalog events

## Backend — Identity

- [x] EPIC-011 — Auth Service (Argon2id, EdDSA JWT + JWKS, rotating refresh cookie, reuse detection, sessions)
- [-] EPIC-012 — User Profile Service (осталось: аватары — хранилище есть, нужен upload flow)

## Backend — Media delivery

- [x] EPIC-018 — MinIO / S3 foundation
- [x] EPIC-017 — Stream Authorization (signed URL, аудио идёт браузер → MinIO напрямую)

Catalog Service — эталон для остальных сервисов (`services/catalog/README.md`).

## Edge и агрегация

- [-] EPIC-032 — API Gateway (nginx: routing catalog/bff/auth/users, request ID, rate limits, access logs; осталось CORS, TLS)
- [-] EPIC-033 — Web BFF (home + album pages; осталось artist page — нет макета)

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

Home и Album работают на реальных данных: Postgres → Catalog → BFF → Gateway → UI (`make seed`);
сессия восстанавливается из refresh cookie, профиль — из User Profile; плеер играет реальный звук
(`make seed-media`) по signed URL. Экранов входа/регистрации в canvas нет
(EPIC-051 BLOCKED) — войти можно через API (`POST /api/v1/auth/register|login`).
Недостающие источники данных, по порядку ценности для экранов canvas:

1. EPIC-023…025 — OpenSearch, Search Indexer, Search Service (контракт `/api/v1/search` уже задан фронтендом).
2. EPIC-014 Library (sidebar: счётчики, плейлисты), EPIC-026 Listening History (Recently played).
3. Transactional outbox для catalog events (at-least-once end to end).
4. EPIC-019/020 Media Ingest + Transcoder — заменят `seed-media` настоящим pipeline (presigned upload уже есть).
