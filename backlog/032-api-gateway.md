# EPIC-032 — API Gateway

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Задачи

- [x] TASK-032.1 Добавить Traefik/Nginx.

- [x] TASK-032.2 Настроить routing.

- [x] TASK-032.3 Настроить CORS.

- [x] TASK-032.4 Настроить TLS для production.

- [x] TASK-032.5 Настроить request ID.

- [x] TASK-032.6 Настроить rate limiting.

- [x] TASK-032.7 Настроить access logs.

## Итог реализации

- nginx (`deploy/gateway/nginx.conf`, общие маршруты в `deploy/gateway/routes.conf`, сервис `gateway` в compose, порт 8080).
- Routing: `/api/v1/pages/*` → Web BFF; `/api/v1/(artists|albums|tracks|genres)` → Catalog, **только GET** —
  запись каталога не публикуется клиентам (403 `FORBIDDEN`).
- CORS: `map $http_origin $cors_origin` с `Access-Control-Allow-Credentials: true`, `Vary: Origin`, preflight `OPTIONS` → `204`, локальные dev-origins в `nginx.conf` и явный allowlist через `/etc/nginx/cors-origins*.conf`.
- TLS для production: `deploy/gateway/nginx.prod.conf` + `deploy/gateway/compose.prod.yml` (TLS 1.2/1.3 на `:8443`, HSTS, HTTP `:8080` → HTTPS `301` redirect, `/health` для probes).
- `X-Request-ID`: принимается валидный клиентский или генерируется, пробрасывается upstream'ам, возвращается один раз.
- Rate limit 50 r/s на клиента (burst 100) → 429 `RATE_LIMITED`; JSON access logs; ошибки в error model.
- Пути ресурсов остаются `/api/v1/<resource>` (как в backlog Catalog), а не `/api/v1/catalog/*` из spec — согласовать при появлении второго сервиса с пересекающимися именами.

---
