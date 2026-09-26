# Auth Service

Аккаунты, пароли, сессии и выдача токенов (EPIC-011). Остальные сервисы проверяют токены сами через
`libs/platform/authn` и JWKS — в Auth на каждый запрос не ходят.

## API (`/api/v1/auth`)

| Метод | Путь | |
|---|---|---|
| POST | `/register` | `{email, password}` → 201 + токен, cookie `icyre_refresh`; 409 `EMAIL_TAKEN`, 422 |
| POST | `/login` | → 200 + токен; 401 `INVALID_CREDENTIALS`; 429 `TOO_MANY_ATTEMPTS` + `Retry-After` |
| POST | `/refresh` | cookie → новый access token и новый refresh cookie; 401 `SESSION_EXPIRED` |
| POST | `/logout` | 204, идемпотентно |
| GET | `/sessions` | активные сессии пользователя (Bearer) |
| DELETE | `/sessions/{id}` | отзыв своей сессии (Bearer) |
| GET | `/.well-known/jwks.json` | публичные ключи Ed25519 |

Ответ с токеном: `{accessToken, tokenType: "Bearer", expiresIn, expiresAt, user: {id, email, roles}}`, `Cache-Control: no-store`.

## Модель безопасности

- **Пароли** — Argon2id (m=19 MiB, t=2, p=1), PHC-строка; 10–128 символов. Неизвестный email проверяется против dummy-хеша.
- **Access token** — JWT EdDSA, 15 мин, `iss=icyre-auth`, `aud=icyre-api`, `sub`, `sid`, `roles`, заголовок `kid`.
- **Refresh token** — 256 случайных бит в HttpOnly cookie (SameSite=Strict, Path `/api/v1/auth`), в БД — только SHA-256.
  Каждый refresh ротирует токен (CAS по старому хешу); старый хеш хранится в `previous_hash`: если его предъявят снова,
  сессия отзывается целиком (reuse detection) — и украденный, и легитимный токен перестают работать.
- **Отзыв** (logout, reuse, `DELETE /sessions/{id}`) пишется в PostgreSQL и в Redis `revoked:session:<sid>` на время жизни
  access token — сервисы отклоняют уже выданные access token сразу, а не через 15 минут.
- **Throttling** — `AUTH_LOGIN_MAX_FAILURES` неудач за `AUTH_LOGIN_WINDOW` на аккаунт (Redis, ключ по хешу email); плюс
  per-IP лимит на gateway.

## События (`auth.events`, key = user ID)

`user.registered`, `session.created`, `session.revoked` — `libs/contracts/events/authv1`. Секретов в событиях нет.
Transactional outbox: аккаунт, сессия и их события коммитятся вместе (`auth.outbox`, миграция 00002), relay
отправляет их в Kafka — at-least-once; без записи события регистрация не проходит, и User Profile узнаёт о
каждом аккаунте.

## Конфигурация

| ENV | По умолчанию | |
|---|---|---|
| `DATABASE_URL` | — | обязателен |
| `REDIS_ADDR` | `localhost:6379` | |
| `KAFKA_BROKERS` / `KAFKA_ENABLED` | `localhost:9094` / `true` | |
| `AUTH_SIGNING_KEY` | — | Ed25519 seed (base64, 32 байта). Обязателен вне `APP_ENV=local`; локально генерируется эфемерный ключ |
| `AUTH_SIGNING_KEY_ID` | `icyre-dev-1` | `kid` |
| `AUTH_ACCESS_TTL` / `AUTH_SESSION_TTL` | `15m` / `720h` | |
| `AUTH_SECURE_COOKIES` | `true` вне local | |
| `AUTH_LOGIN_MAX_FAILURES` / `AUTH_LOGIN_WINDOW` | `10` / `15m` | |

## Запуск

```bash
make up-core && make migrate
make run-auth                  # :8083, эфемерный ключ
cd services/auth && AUTH_TEST_DATABASE_DSN=postgres://icyre:icyre@localhost:5432/icyre?sslmode=disable \
  go test -tags integration ./...
```
