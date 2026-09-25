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

## Backend — Library

- [x] EPIC-014 — Library Service (сохранённые треки и альбомы, `library.events`; лайки в UI)

## Backend — Playback

- [-] EPIC-016 — Playback Service (срез 1: телеметрия плеера → `playback.events`)

## Backend — Listening History

- [x] EPIC-026 — Listening History (playback.events → история, `/me/history`)

## Backend — Search

- [x] EPIC-023 — OpenSearch foundation
- [x] EPIC-024 — Search Indexer (catalog.events → OpenSearch, reindex с переключением алиасов)
- [x] EPIC-025 — Search Service (`/search`, `/search/suggest`)

## Edge и агрегация

- [-] EPIC-032 — API Gateway (nginx: routing catalog/bff/auth/users, request ID, rate limits, access logs; осталось CORS, TLS)
- [-] EPIC-033 — Web BFF (home + album pages, user context → liked и Recently played; осталось artist page — нет макета)

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
(`make seed-media`) по signed URL; поиск идёт через OpenSearch (`search-indexer reindex`). Экранов входа/регистрации в canvas нет
(EPIC-051 BLOCKED) — войти можно через API (`POST /api/v1/auth/register|login`).
Недостающие источники данных, по порядку ценности для экранов canvas:

1. EPIC-013 Playlist Service (плейлисты сайдбара `/me/playlists`, плейлисты в «Recently played»).
2. Transactional outbox для catalog events (at-least-once end to end).
3. EPIC-019/020 Media Ingest + Transcoder — заменят `seed-media` настоящим pipeline (presigned upload уже есть).
4. Web BFF `GET /pages/search` (жанры, подборки до запроса) — нужен источник жанров/подборок; пока только в моках.
