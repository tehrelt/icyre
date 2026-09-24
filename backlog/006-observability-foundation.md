# EPIC-006 — Observability foundation

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Priority:** P1

## Цель

С самого начала сделать сервисы наблюдаемыми.

## Задачи

- [ ] TASK-006.1 Подключить OpenTelemetry SDK.

- [ ] TASK-006.2 Добавить trace propagation для HTTP.

- [ ] TASK-006.3 Подготовить propagation для gRPC.

- [ ] TASK-006.4 Прокидывать `traceId` в Kafka envelope.

- [ ] TASK-006.5 Добавить Prometheus metrics endpoint.

- [ ] TASK-006.6 Добавить базовые HTTP metrics.

- [ ] TASK-006.7 Добавить DB metrics.

- [ ] TASK-006.8 Добавить Kafka consumer metrics.

- [ ] TASK-006.9 Добавить Jaeger в Docker Compose.

- [ ] TASK-006.10 Добавить Prometheus.

- [ ] TASK-006.11 Добавить Grafana.

## Definition of Done

- HTTP request виден в trace.
- Метрики сервиса доступны Prometheus.
- Grafana подключена к Prometheus.
- В labels отсутствуют high-cardinality ID.

---
