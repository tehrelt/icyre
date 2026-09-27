# EPIC-038 — Load testing

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P2

## Задачи

- [x] TASK-038.1 Добавить k6.

- [x] TASK-038.2 Catalog baseline test.

- [x] TASK-038.3 Redis vs no Redis test.

- [x] TASK-038.4 1 replica vs 3 replicas.

- [x] TASK-038.5 Kafka async vs synchronous write.

- [x] TASK-038.6 Media through backend vs CDN/Object Storage.

- [x] TASK-038.7 Собирать:
  - RPS;
  - p50;
  - p95;
  - p99;
  - CPU;
  - memory;
  - DB load;
  - Kafka lag.

## Итог реализации

- k6 — образ `grafana/k6` в overlay `deploy/load/compose.yml` (сервис `k6`, profile `load`), сценарии в
  `tests/load` (`catalog.js`, `playback.js`, `media.js`, общий `lib.js` с профилями 100/1000/5000/10000 VU
  и SLO-порогами). Overlay снимает host-порт Catalog (масштабирование) и выводит генератор из rate limit
  gateway (`geo $rate_limit_exempt` + `deploy/load/ratelimit-exempt.conf`).
- `scripts/load-run.ts` + `make load-baseline|load-redis|load-replicas|load-analytics|load-media|load-all`:
  переключение варианта через переменные compose, прогон k6, сбор `docker stats` (CPU/память по
  сервисам), дельты `pg_stat_database`, максимального lag consumer groups Kafka → отчёт
  `tests/load/results/*.md|json`; в конце стенд возвращается к умолчаниям.
- Переключатели экспериментов (по умолчанию выключены):
  - Catalog `CACHE_ENABLED` — Redis read-through кэш репозиториев (`internal/adapters/cache`, TTL 30 с,
    инвалидация при записи трека);
  - Playback `ANALYTICS_MODE=sync` — обогащение из Catalog и запись в ClickHouse прямо в запросе вместо
    `playback.events`;
  - Stream Authorization `MEDIA_PROXY_ENABLED` — `GET /api/v1/stream/proxy/{trackId}`: те же проверки, что
    у `authorize`, но байты идут через сервис и gateway.
- Результаты измерений и выводы — EPIC-040.
