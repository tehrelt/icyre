# EPIC-009 — Catalog Service: HTTP API

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P0

## Задачи

- [x] TASK-009.1 Добавить HTTP server.

- [x] TASK-009.2 Реализовать:
  - `POST /api/v1/artists`
  - `GET /api/v1/artists/{id}`
  - `POST /api/v1/albums`
  - `GET /api/v1/albums/{id}`
  - `POST /api/v1/tracks`
  - `GET /api/v1/tracks/{id}`
  - `GET /api/v1/albums/{id}/tracks`

- [x] TASK-009.3 Добавить request validation.

- [x] TASK-009.4 Добавить error mapper.

- [x] TASK-009.5 Добавить DTO отдельно от domain entities.

- [x] TASK-009.6 Добавить httptest.

## Definition of Done

- API работает через реальный HTTP.
- Domain errors корректно маппятся в HTTP status.
- Handler не содержит бизнес-логики.

## Итог реализации

Реализованы маршруты backlog + из `specs/services/catalog.md`: `GET /api/v1/artists/{id}/albums` (cursor pagination),
`GET /api/v1/genres`, `PATCH /api/v1/tracks/{id}` (title/explicit/status → `track.updated`).
Error model по `specs/api/error-model.md` (404/409/422/400/415/500), httptest-тесты, smoke через реальный HTTP в compose.
Пути `/api/v1/<resource>` заданы в backlog; gateway (EPIC-032) будет маршрутизировать `/api/v1/catalog/*` согласно spec.

---
