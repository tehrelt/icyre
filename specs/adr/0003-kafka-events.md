# ADR-0003: Kafka для событий

## Статус
Accepted.

## Контекст
Playback и catalog события используются многими независимыми consumers.

## Решение
Использовать Kafka как durable event stream.

## Последствия
Плюсы:
- слабая связанность;
- replay;
- независимые consumers;
- горизонтальное масштабирование workers.

Минусы:
- eventual consistency;
- необходимость idempotency;
- сложнее локальная инфраструктура.
