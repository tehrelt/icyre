# Security

## Authentication

OAuth2 / OIDC + JWT.

## Passwords

Использовать Argon2id или bcrypt.

Пароли:
- никогда не логируются;
- не передаются между сервисами;
- не хранятся в plaintext.

## Tokens

Access token short-lived.

Refresh token:
- rotation;
- revoke;
- device/session binding при необходимости.

## Authorization

RBAC:

```text
USER
ARTIST
MODERATOR
ADMIN
```

Дополнительно используется resource ownership.

## Transport

Внешний трафик только HTTPS.

Для production internal traffic может защищаться mTLS.

## Object Storage

Bucket не должен быть публично доступен без необходимости.

Media delivery — через CDN/signed URLs.

## Secrets

Secrets не хранятся в Git.

Использовать:
- Kubernetes Secrets + external secret store;
- Vault/cloud secret manager.

## Abuse protection

- rate limit;
- login throttling;
- upload limits;
- MIME validation;
- file size limits;
- audit logs.

## Input validation

Каждый внешний ввод проверяется на границе сервиса.
