# Масштабирование

## HTTP-сервисы

Stateless сервисы масштабируются горизонтально:

```text
1 replica -> 3 replicas -> 10 replicas
```

Балансировка выполняется Gateway/Kubernetes Service.

## Catalog

Основная нагрузка — чтение.

Меры:
- Redis cache;
- read replicas PostgreSQL;
- CDN для обложек;
- денормализованные read-модели при необходимости.

## Search

OpenSearch масштабируется отдельным кластером.

Поиск не должен зависеть от основной PostgreSQL на каждый запрос.

## Playback

Playback Service хранит краткоживущее состояние в Redis.

Это позволяет запускать несколько экземпляров.

## Media

Масштабирование аудиотрафика выполняется CDN, а не application backend.

## Kafka workers

Consumer Group позволяет параллельно обрабатывать partitions.

```text
topic partitions = 12
consumer replicas <= 12
```

## Kubernetes HPA

Примеры сигналов:
- CPU;
- memory;
- requests/sec;
- Kafka consumer lag;
- custom Prometheus metrics.

## Bottleneck analysis

Отдельно тестируются:
- PostgreSQL connections;
- Redis latency;
- Kafka lag;
- OpenSearch query latency;
- S3 throughput;
- CDN cache hit ratio.
