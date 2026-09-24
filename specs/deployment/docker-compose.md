# Docker Compose

## Назначение

Локальная среда разработки и демонстрационный стенд.

## Компоненты

```text
frontend
api-gateway
backend services
postgres
redis
kafka
opensearch
clickhouse
minio
prometheus
grafana
jaeger
```

## Профили

Можно использовать compose profiles:

```text
core
search
analytics
observability
```

## Development rule

Система должна запускаться одной командой:

```bash
docker compose up -d
```

## Data persistence

Для PostgreSQL, Kafka, OpenSearch, ClickHouse и MinIO используются named volumes.

## Healthchecks

Каждый инфраструктурный сервис должен иметь healthcheck.
