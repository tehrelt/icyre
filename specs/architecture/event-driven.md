# Событийная архитектура

## Назначение

Kafka позволяет отделить пользовательский запрос от тяжёлой вторичной обработки.

## Пример

```text
Playback Service
   |
   | playback.finished
   v
Kafka
   |
   +--> Listening History Worker
   +--> Analytics Worker
   +--> Recommendation Worker
```

## Гарантии

Система проектируется с учётом доставки `at least once`.

Следствие: consumers должны быть идемпотентными.

## Envelope события

```json
{
  "eventId": "uuid",
  "eventType": "playback.finished",
  "eventVersion": 1,
  "occurredAt": "2026-09-24T19:00:00Z",
  "producer": "playback-service",
  "traceId": "uuid",
  "payload": {}
}
```

## Версионирование

Изменения схемы должны быть обратно совместимыми.

Предпочтительно:
- добавлять поля;
- не переименовывать существующие поля без новой версии;
- не менять семантику поля.

## Dead Letter Queue

Для необрабатываемых событий используется DLQ.

Пример:

```text
playback.events.dlq
media.jobs.dlq
```

## Retry

Consumer выполняет ограниченное количество повторов.

После превышения лимита сообщение переводится в DLQ.
