# EPIC-009 — Catalog Service: HTTP API

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Priority:** P0

## Задачи

- [ ] TASK-009.1 Добавить HTTP server.

- [ ] TASK-009.2 Реализовать:
  - `POST /api/v1/artists`
  - `GET /api/v1/artists/{id}`
  - `POST /api/v1/albums`
  - `GET /api/v1/albums/{id}`
  - `POST /api/v1/tracks`
  - `GET /api/v1/tracks/{id}`
  - `GET /api/v1/albums/{id}/tracks`

- [ ] TASK-009.3 Добавить request validation.

- [ ] TASK-009.4 Добавить error mapper.

- [ ] TASK-009.5 Добавить DTO отдельно от domain entities.

- [ ] TASK-009.6 Добавить httptest.

## Definition of Done

- API работает через реальный HTTP.
- Domain errors корректно маппятся в HTTP status.
- Handler не содержит бизнес-логики.

---
