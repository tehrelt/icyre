# EPIC-035 — Security hardening

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Задачи

- [x] TASK-035.1 Проверить secret handling.

- [x] TASK-035.2 Запретить secret logging.

- [x] TASK-035.3 Добавить rate limits login.

- [x] TASK-035.4 Добавить upload size limits.

- [x] TASK-035.5 Добавить authorization ownership checks.

- [x] TASK-035.6 Добавить security headers.

- [x] TASK-035.7 Добавить audit events.

- [x] TASK-035.8 Проверить SQL injection surface.

- [x] TASK-035.9 Проверить SSRF вокруг signed URLs.

## Итог реализации

- **Secret handling & redaction** (`libs/platform/config/env.go`, `libs/platform/logger/logger.go`): тип `config.Secret` (`Env.Secret`, `Env.RequiredSecret`) маскирует значение как `[REDACTED]` при форматировании и текстовом маршалинге; `logger.New` через `ReplaceAttr` (`redactAttr`) редактирует чувствительные ключи (`password`, `secret`, `token`, `authorization`, `cookie`, `dsn`, `presigned_url`, `signed_url` и суффиксы `_password`, `_secret`, `_token`, `_key`) и значения (`Bearer ...`, `X-Amz-Signature=`/`X-Amz-Credential=`, DSN с `user:pass@`).
- **Login throttling & gateway rate limits** (`services/auth/internal/adapters/redis/throttle.go`, `services/auth/internal/application/service.go`, `deploy/gateway/nginx.conf`, `deploy/gateway/nginx.prod.conf`, `deploy/gateway/routes.conf`): Redis-лимитер `LoginThrottle` (`ratelimit:login:<sha256(email)[:16]>:<window>`, без PII в ключах, fail-open при сбое Redis) + отдельная зона `auth` (`5r/s`, `burst=10 nodelay`) на `/api/v1/auth/(login|register|refresh)` в nginx.
- **Upload size limits** (`deploy/gateway/nginx.conf`, `deploy/gateway/nginx.prod.conf`, `services/media-ingest/internal/application/service.go`, `services/user-profile/internal/application/service.go`): `client_max_body_size 1m` и `server_tokens off` на gateway; проверка `MaxSize` при создании upload-сессии и повторная сверка фактического размера (`Stat`), `Content-Type`, `SHA-256` и magic bytes (`ReadHead`) при завершении загрузки треков и аватаров.
- **Ownership checks** (`services/auth/internal/application/service.go`, `services/media-ingest/internal/application/service.go`, `services/user-profile/internal/application/service.go`): проверка владельца в `auth.RevokeSession` (`sess.UserID != userID` → `ErrSessionNotFound`), `media-ingest.Get`/`Complete` (`u.UploaderID != actor.ID && !actor.Admin` → `ErrNotFound`) и `user-profile.CompleteAvatarUpload` (`u.UserID != userID` → `ErrAvatarUploadNotFound`).
- **Security headers** (`libs/platform/httpserver/middleware.go`, `deploy/gateway/nginx.conf`, `deploy/gateway/nginx.prod.conf`): `httpserver.SecurityHeaders()` в `httpserver.Standard` и nginx выставляют `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer` (в `nginx.prod.conf` также `Strict-Transport-Security`).
- **Audit events** (`services/auth/internal/application/service.go`, `services/media-ingest/internal/application/service.go`, `services/user-profile/internal/application/service.go`): структурированные audit-записи (`"audit": ...`, `"event": ...`) для `auth` (`register`, `login_throttled`, `login_failed`, `login_succeeded`, `refresh_reuse_revoked`, `session_revoked`), `media-ingest` (`upload_created`, `upload_completed`, `upload_rejected`) и `user-profile` (`avatar_upload_created`, `avatar_updated`, `avatar_rejected`, `avatar_deleted`).
- **SQL identifier validation** (`libs/platform/clickhouse/clickhouse.go`): `Insert` валидирует имя таблицы по `^[a-z_][a-z0-9_]*(\.[a-z_][a-z0-9_]*)?$`, значения передаются только через типизированные параметры `{name:Type}`.
- **SSRF / key validation & redirect blocking** (`libs/platform/objectstore/objectstore.go`, `libs/platform/httpclient/httpclient.go`): `objectstore.New` валидирует `Endpoint`/`PublicEndpoint`, `ValidateKey` отклоняет пустые ключи, ведущий `/`, `..`/`.`, `\`, `?`, `#` и управляющие символы во всех операциях с объектами и presigned URL; `httpclient.New` запрещает следование редиректам (`http.ErrUseLastResponse`).

---
