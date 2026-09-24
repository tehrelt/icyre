# Kubernetes deployment

## Основные сущности

Для stateless сервисов:

```text
Deployment
Service
ConfigMap
Secret
HPA
PodDisruptionBudget
```

Для входящего трафика:

```text
Ingress / Traefik
```

## Replicas

Пример:

```text
BFF: 3
Catalog: 3
Playback: 3
Search API: 2
Workers: 1..N
```

Точные значения определяются нагрузочными тестами.

## Probes

Каждый сервис:

```text
/health/live
/health/ready
```

## HPA

Возможные сигналы:
- CPU;
- memory;
- RPS;
- Kafka lag.

## Rolling updates

Deployment должен поддерживать zero/low-downtime rollout.

## Graceful shutdown

При SIGTERM:
1. перестать принимать новые запросы;
2. завершить активные;
3. закрыть consumer;
4. закрыть DB connections;
5. выйти.
