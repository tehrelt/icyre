# User Profile Service

Публичная идентичность слушателя: username, display name, bio, страна, язык (EPIC-012). User ID приходит из Auth;
email, пароли и сессии здесь не хранятся.

## API (`/api/v1/users`)

| Метод | Путь | |
|---|---|---|
| GET | `/{id}` | публичный профиль `{id, username, displayName, avatarUrl, bio, country}` |
| GET | `/me` | свой профиль + `language`, `createdAt`, `updatedAt` (Bearer) |
| PATCH | `/me` | частичное обновление `username`, `displayName`, `bio`, `country`, `language`; 422, 409 `USERNAME_TAKEN` |

Сразу после регистрации `GET /me` может кратко отвечать 404 `PROFILE_NOT_FOUND` — профиль создаётся асинхронно из события.
`avatarUrl` пока всегда `null`: загрузка аватаров ждёт EPIC-018 (MinIO).

## События

- Потребляет `auth.events` → `user.registered` (group `user-profile`, 5 retries, затем `auth.events.dlq`).
  Идемпотентно: повторная доставка находит профиль (`ON CONFLICT (user_id) DO NOTHING`) и ничего не публикует.
  Username выводится из email; при коллизии — `rin2`, `rin3`, …
- Публикует `profile.events` → `profile.updated` (`libs/contracts/events/profilev1`) при создании и реальном изменении.

## Конфигурация

`DATABASE_URL` (обязателен), `REDIS_ADDR`, `KAFKA_BROKERS`, `KAFKA_ENABLED`,
`AUTH_JWKS_URL` (по умолчанию `http://localhost:8083/api/v1/auth/.well-known/jwks.json`), общие `HTTP_ADDR`, `LOG_*`, `OTEL_*`.

## Запуск

```bash
make up-core && make migrate
make run-auth              # :8083
make run-user-profile      # :8084
```
