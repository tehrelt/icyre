# EPIC-012 — User Profile Service

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Задачи

- [x] TASK-012.1 Создать module.

- [x] TASK-012.2 Реализовать Profile entity.

- [x] TASK-012.3 Реализовать PostgreSQL repository.

- [x] TASK-012.4 Реализовать:
  - `GET /users/{id}`
  - `GET /users/me`
  - `PATCH /users/me`

- [x] TASK-012.5 Добавить avatar metadata.

- [x] TASK-012.6 Публиковать `profile.updated`.

## Итог реализации

`services/user-profile`. Схема `profile.profiles` (username unique) и `profile.avatar_uploads` (миграция `00003`).

- Профиль создаётся из `user.registered` (consumer group `user-profile`, retries + DLQ, идемпотентно через
  `ON CONFLICT (user_id) DO NOTHING`); username выводится из email, при коллизии — суффикс (`rin`, `rin2`, …).
- `GET /api/v1/users/{id}` — публичные поля; `GET/PATCH /api/v1/users/me` — владелец (JWT через JWKS Auth + Redis denylist).
  Ошибки: 422 `VALIDATION_FAILED`, 409 `USERNAME_TAKEN`, 404 `PROFILE_NOT_FOUND`.
- Аватары (TASK-012.5): `POST /api/v1/users/me/avatar/uploads` (presigned PUT в MinIO с `Content-Type` и `x-amz-checksum-sha256`),
  `POST /api/v1/users/me/avatar/uploads/{id}/complete` (проверка размера, MIME, SHA-256 и magic bytes JPEG/PNG/WebP,
  обновление `avatar_key` под row lock, удаление предыдущего объекта best-effort), `DELETE /api/v1/users/me/avatar`.
  `avatarUrl` подписывается через `PresignDownload` при чтении/обновлении профиля (или `null`, если ключ пуст / S3 не настроен).
- `profile.updated` и `avatar.changed` в `profile.events` через transactional outbox.
- Frontend: `entities/user` читает `/users/me`; access token в памяти, bootstrap через `POST /auth/refresh`, single-flight
  refresh + один retry на 401 (`shared/api/session.ts`). Экран профиля в canvas отсутствует — UI редактирования не делается.

---
