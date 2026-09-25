# EPIC-012 — User Profile Service

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [-] IN PROGRESS

**Priority:** P1

## Задачи

- [x] TASK-012.1 Создать module.

- [x] TASK-012.2 Реализовать Profile entity.

- [x] TASK-012.3 Реализовать PostgreSQL repository.

- [x] TASK-012.4 Реализовать:
  - `GET /users/{id}`
  - `GET /users/me`
  - `PATCH /users/me`

- [ ] TASK-012.5 Добавить avatar metadata.

- [x] TASK-012.6 Публиковать `profile.updated`.

## Итог реализации

`services/user-profile`. Схема `profile.profiles` (username unique).

- Профиль создаётся из `user.registered` (consumer group `user-profile`, retries + DLQ, идемпотентно через
  `ON CONFLICT (user_id) DO NOTHING`); username выводится из email, при коллизии — суффикс (`rin`, `rin2`, …).
- `GET /api/v1/users/{id}` — публичные поля; `GET/PATCH /api/v1/users/me` — владелец (JWT через JWKS Auth + Redis denylist).
  Ошибки: 422 `VALIDATION_FAILED`, 409 `USERNAME_TAKEN`, 404 `PROFILE_NOT_FOUND`.
- `profile.updated` в `profile.events` при создании и при реальном изменении.
- Frontend: `entities/user` читает `/users/me`; access token в памяти, bootstrap через `POST /auth/refresh`, single-flight
  refresh + один retry на 401 (`shared/api/session.ts`).
- **Осталось TASK-012.5**: колонка `avatar_key` есть, `avatarUrl` пока `null` — загрузка и выдача аватаров требуют
  EPIC-018 (MinIO). Экран профиля в canvas отсутствует — UI редактирования не делается.

---
