# Load testing

## Инструмент

k6.

## Цели исследования

### Эксперимент 1: без Redis / с Redis

Измерять:
- RPS;
- p50;
- p95;
- p99;
- DB CPU;
- DB QPS.

### Эксперимент 2: аудио через backend / CDN

Сравнить:
- bandwidth backend;
- CPU;
- latency;
- concurrent streams.

### Эксперимент 3: sync analytics / Kafka

Сравнить latency пользовательского запроса.

### Эксперимент 4: 1 / 3 / N replicas

Проверить горизонтальное масштабирование.

## Нагрузочные профили

```text
100 users
1 000 users
5 000 users
10 000 users
```

## SLO candidates

Пример целей:

```text
Catalog p95 < 300 ms
Search p95 < 500 ms
Playback auth p95 < 300 ms
5xx < 1%
```

Это исследовательские целевые значения, которые необходимо подтвердить экспериментально.
