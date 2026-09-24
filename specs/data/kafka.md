# Kafka

## Topics
Пример:
```text
catalog.events
playback.events
library.events
social.events
media.events
notification.events
```

## Partition key
Часто используется:
- user_id для пользовательских событий;
- track_id для медиа/каталога.

Это сохраняет порядок событий для конкретного aggregate.

## Consumer groups
Каждый логический consumer использует собственную group.

## Delivery
At least once.

## Required design
Consumers должны поддерживать idempotency.
