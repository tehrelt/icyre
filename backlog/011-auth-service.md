# EPIC-011 — Auth Service

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Задачи

- [x] TASK-011.1 Создать module.

- [x] TASK-011.2 Реализовать регистрацию.

- [x] TASK-011.3 Реализовать login.

- [x] TASK-011.4 Реализовать access token.

- [x] TASK-011.5 Реализовать refresh token.

- [x] TASK-011.6 Реализовать logout.

- [x] TASK-011.7 Реализовать sessions.

- [x] TASK-011.8 Добавить password hashing.

- [x] TASK-011.9 Добавить Redis session cache.

- [x] TASK-011.10 Публиковать `user.registered`.

## Definition of Done

- Пользователь может зарегистрироваться и войти.
- Access/refresh flow работает.
- Пароли не хранятся plaintext.
- Logout инвалидирует сессию.

## Итог реализации

`services/auth` (hexagonal, как Catalog). Схема `auth`: `accounts`, `sessions`.

- Пароли: Argon2id (m=19 MiB, t=2, p=1, PHC-строка); CHECK в БД запрещает не-Argon2 хеши. Для неизвестного email проверяется
  dummy-хеш — время ответа не выдаёт существование аккаунта.
- Access token: JWT EdDSA (Ed25519), 15 мин, `iss=icyre-auth`, `aud=icyre-api`, claims `sub`, `sid`, `roles`; публичные ключи —
  `GET /api/v1/auth/.well-known/jwks.json`. Проверка в сервисах — `libs/platform/authn` (middleware `Required/Optional`, JWKS cache).
- Refresh token: 256 бит, cookie `icyre_refresh` (HttpOnly, SameSite=Strict, Secure вне local, Path `/api/v1/auth`), в БД только SHA-256.
  Ротация на каждый refresh (CAS по старому хешу); предъявление уже ротированного токена → вся сессия отзывается (reuse detection).
- Sessions: `GET /api/v1/auth/sessions`, `DELETE /api/v1/auth/sessions/{id}`, `POST /logout` (идемпотентный).
- TASK-011.9: source of truth сессий — PostgreSQL; в Redis — denylist отозванных сессий `revoked:session:<sid>` на время жизни
  access token (проверяется на каждом запросе во всех сервисах) и throttling входа (10 неудач / 15 мин на аккаунт → 429).
  Отдельный read-cache сессий не нужен: проверка access token stateless.
- События `auth.events`: `user.registered`, `session.created`, `session.revoked` (без секретов — есть тест).
- Gateway: `/api/v1/auth/*`, для login/register/refresh отдельный rate limit 5 r/s на клиента.
- Проверено e2e через gateway: register → cookie → `/users/me`; refresh (ротация); повтор старого refresh → 401 и отзыв сессии,
  старый access token сразу 401; logout → 401; 11-я неудачная попытка входа → 429; повторная регистрация → 409.
- Локально ключ подписи эфемерный (новый `kid` на процесс); вне `APP_ENV=local` `AUTH_SIGNING_KEY` обязателен.

---
