# Observability

## Три сигнала

- metrics;
- logs;
- traces.

## Metrics

Prometheus.

Примеры:

```text
http_requests_total
http_request_duration_seconds
grpc_client_duration_seconds
db_query_duration_seconds
redis_hit_ratio
kafka_consumer_lag
media_transcode_duration_seconds
cdn_origin_requests_total
```

## Logs

Structured JSON logs.

Поля:

```text
timestamp
level
service
request_id
trace_id
user_id_hash
message
```

Не логировать access token и секреты.

## Tracing

OpenTelemetry + Jaeger.

Trace должен проходить:

```text
Gateway -> BFF -> Service -> DB
```

`traceId` желательно добавлять в Kafka envelope.

## Dashboards

Grafana:
- global health;
- API latency;
- DB;
- Redis;
- Kafka;
- search;
- media processing;
- business metrics.

## Alerts

Примеры:
- p95 latency > threshold;
- error rate;
- Kafka lag;
- PostgreSQL connections;
- disk/object storage errors;
- failed transcoding spike.
