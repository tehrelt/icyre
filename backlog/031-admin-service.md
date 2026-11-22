# EPIC-031 — Admin Service

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P2

## Задачи

- [x] TASK-031.1 Реализовать RBAC.

- [x] TASK-031.2 Реализовать moderation endpoints.

- [x] TASK-031.3 Добавить блокировку трека.

- [x] TASK-031.4 Добавить блокировку пользователя.

- [x] TASK-031.5 Добавить audit log.

- [x] TASK-031.6 Добавить просмотр ingest/transcode jobs.

## Итог реализации

- **Admin Service (`services/admin`)**:
  - Схема PostgreSQL `admin` (`00001_admin_core.sql`): `moderation_operations` (идемпотентность по `(actor_id, idempotency_key)` и `request_hash`), `audit_logs` (неизменяемость обеспечена триггерами `audit_logs_immutable` на `BEFORE UPDATE OR DELETE` и `audit_logs_no_truncate` на `BEFORE TRUNCATE` с функцией `admin.prevent_audit_mutation()`), `transcode_jobs` (проекция событий из Kafka-топика `media.events`).
  - Фоновый reconciler для повторного доведения `PENDING`-операций модерации (`admin.moderation_operations`).
  - REST API под `/api/v1/admin/*`: `GET/PATCH /tracks`, `GET/PATCH /users`, `GET /jobs`, `GET /audit-logs`, `GET /operations/{id}`.
- **RBAC (`libs/platform/authn`, `services/admin`)**:
  - В `libs/platform/authn` добавлены `VerifyStrict` / `RequiredStrict` (fail-closed `503` при недоступности хранилища отзывов), `RequireAnyRole` и `RequireInternalToken`.
  - Роль `MODERATOR` (и `ADMIN`) имеет доступ к модерации треков, просмотру `jobs` и собственных операций (`GET /operations/{id}`); роль `ADMIN` — к управлению пользователями и `audit-logs`. Запрещены самоблокировка и блокировка администраторов.
- **Owner moderation endpoints (защищены `X-Internal-Token`)**:
  - `services/catalog`: `GET/PATCH /internal/v1/moderation/tracks*`, таблица идемпотентности `catalog.moderation_receipts` (`00004_moderation_receipts.sql`); публичный/общий `PATCH /tracks/{id}` отклоняет переходы в/из `BLOCKED`.
  - `services/auth`: миграция `00003_account_blocking.sql` (`accounts.blocked`, `blocked_at`, `auth.moderation_receipts`, `auth.pending_revocations`, причина отзыва `account_blocked`), проверка `blocked` в `Login`/`Refresh`, массовый отзыв сессий с записью в `auth.pending_revocations` (досылаются в Redis фоновым flusher `FlushPendingRevocations`) и Redis-отзывом (`revoked:session:<sid>`), `GET/PATCH /internal/v1/moderation/users*`.
  - `services/media-ingest`: `GET /internal/v1/moderation/uploads`.
- **Инфраструктура и маршрутизация**:
  - `deploy/gateway/routes.conf`: маршрут `/api/v1/admin/` проксируется на `admin:8080` (внутренние `/internal/v1/*` наружу не экспонируются).
  - `docker-compose.yml`: сервисы `admin-migrate` и `admin` (порт `127.0.0.1:8100:8080`).
  - Обновлены `deploy/prometheus/prometheus.yml`, `Makefile` и `go.work`.

---
