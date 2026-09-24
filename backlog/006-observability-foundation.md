# EPIC-006 — Observability foundation

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [-] IN PROGRESS

**Priority:** P1

## Цель

С самого начала сделать сервисы наблюдаемыми.

## Задачи

- [x] TASK-006.1 Подключить OpenTelemetry SDK.

- [x] TASK-006.2 Добавить trace propagation для HTTP.

- [ ] TASK-006.3 Подготовить propagation для gRPC.

- [x] TASK-006.4 Прокидывать `traceId` в Kafka envelope.

- [x] TASK-006.5 Добавить Prometheus metrics endpoint.

- [x] TASK-006.6 Добавить базовые HTTP metrics.

- [x] TASK-006.7 Добавить DB metrics.

- [x] TASK-006.8 Добавить Kafka consumer metrics.

- [x] TASK-006.9 Добавить Jaeger в Docker Compose.

- [x] TASK-006.10 Добавить Prometheus.

- [x] TASK-006.11 Добавить Grafana.

## Definition of Done

- HTTP request виден в trace.
- Метрики сервиса доступны Prometheus.
- Grafana подключена к Prometheus.
- В labels отсутствуют high-cardinality ID.

## Итог реализации

Готово: OTel SDK (OTLP/HTTP → Jaeger), W3C propagation для HTTP и Kafka headers, `traceId` в envelope, `/metrics`,
HTTP/DB/Kafka producer+consumer метрики (только low-cardinality labels), Jaeger/Prometheus/Grafana в compose,
provisioned datasource и dashboard «ICYRE — services». Проверено: спаны `GET /api/v1/... → db select` в Jaeger,
target `catalog-service` = up в Prometheus.

Осталось: TASK-006.3 — gRPC interceptors (появятся вместе с EPIC-034; глобальный propagator уже настроен).

---
