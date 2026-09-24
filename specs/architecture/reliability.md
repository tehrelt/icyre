# Надёжность и отказоустойчивость

## Принципы

- timeout на каждый сетевой вызов;
- retry только для безопасных операций;
- exponential backoff;
- idempotency;
- circuit breaker при необходимости;
- graceful shutdown;
- readiness/liveness probes.

## Graceful degradation

Если Recommendation Service недоступен:

```text
Главная страница продолжает работать
+
используется fallback: popular tracks
```

Если OpenSearch недоступен:
- каталог работает;
- поиск временно возвращает 503.

Если Analytics недоступна:
- playback не должен останавливаться.

## PostgreSQL

Для production:
- backups;
- PITR;
- read replica;
- connection pool.

## Redis

Если используется только как cache, система должна уметь восстановить данные из primary storage.

Критичные данные нельзя хранить только в Redis без явного решения.

## Kafka

Рекомендуется replication factor >= 3 для production-кластера.

## S3

Object Storage считается основным хранилищем media-файлов.

Удаление объектов должно быть защищено политиками и lifecycle rules.
