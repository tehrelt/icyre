# Auth Service

## Ответственность
- регистрация;
- login/logout;
- refresh token;
- OAuth/OIDC;
- управление сессиями;
- revoke.

## API
```http
POST /auth/register
POST /auth/login
POST /auth/refresh
POST /auth/logout
GET /auth/sessions
DELETE /auth/sessions/{id}
```

## Данные
PostgreSQL:
- credentials;
- oauth identities;
- refresh token metadata.

Redis:
- session cache;
- revoked token identifiers.

## События
```text
user.registered
session.created
session.revoked
```

## Security
Пароли хэшируются Argon2id или bcrypt.
