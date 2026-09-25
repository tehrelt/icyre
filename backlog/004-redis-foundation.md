# EPIC-004 — Redis foundation

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Цель

Подготовить Redis для cache, sessions и transient state.

## Задачи

- [x] TASK-004.1 Добавить Redis в Docker Compose.

- [x] TASK-004.2 Добавить Redis healthcheck.

- [x] TASK-004.3 Создать `libs/platform/redis`.

- [x] TASK-004.4 Настроить timeout.

- [x] TASK-004.5 Настроить connection pool.

- [x] TASK-004.6 Определить naming convention ключей.

- [x] TASK-004.7 Добавить helper для TTL cache.

## Definition of Done

- Redis доступен из сервисов.
- Cache API не содержит бизнес-типов.
- Timeout и ошибки корректно обрабатываются.

## Итог реализации

- Compose: `redis:8-alpine` без persistence (кэш и transient state), `maxmemory 256mb`, `allkeys-lru`, healthcheck `redis-cli ping`.
- `libs/platform/redis`: `Open` (ping при старте; dial 2s, read/write 500ms, pool timeout 1s, настраиваемый pool), `Check` для readiness.
- Ключи: `<namespace>:<entity>:<id>` через `redis.Key(...)`, TTL обязателен (например `revoked:session:<sid>`,
  `ratelimit:login:<hash>:<bucket>`, `cache:<name>:<id>`).
- `Cache[T]` — generic TTL cache без бизнес-типов: `Get/Set/Delete/GetOrLoad`; при недоступном Redis `GetOrLoad` идёт в источник
  (cache — не source of truth), метрика `redis_cache_requests_total{cache,result}`.
- Проверено: unit-тесты + integration (`make test-integration`, реальный Redis), readiness Auth/User Profile включает `redis`.

---
