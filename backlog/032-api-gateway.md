# EPIC-032 — API Gateway

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [-] IN PROGRESS

**Priority:** P1

## Задачи

- [x] TASK-032.1 Добавить Traefik/Nginx.

- [x] TASK-032.2 Настроить routing.

- [ ] TASK-032.3 Настроить CORS.

- [ ] TASK-032.4 Настроить TLS для production.

- [x] TASK-032.5 Настроить request ID.

- [x] TASK-032.6 Настроить rate limiting.

- [x] TASK-032.7 Настроить access logs.

## Итог реализации

- nginx (`deploy/gateway/nginx.conf`, сервис `gateway` в compose, порт 8080).
- Routing: `/api/v1/pages/*` → Web BFF; `/api/v1/(artists|albums|tracks|genres)` → Catalog, **только GET** —
  запись каталога не публикуется клиентам (403 `FORBIDDEN`).
- `X-Request-ID`: принимается валидный клиентский или генерируется, пробрасывается upstream'ам, возвращается один раз.
- Rate limit 50 r/s на клиента (burst 100) → 429 `RATE_LIMITED`; JSON access logs; ошибки в error model.
- Пути ресурсов остаются `/api/v1/<resource>` (как в backlog Catalog), а не `/api/v1/catalog/*` из spec — согласовать при появлении второго сервиса с пересекающимися именами.

Осталось: TASK-032.3 CORS (сейчас клиент same-origin через Vite proxy), TASK-032.4 TLS для production.

---
