# libs/contracts

Межсервисные контракты ICYRE:

- `events` — единый `Envelope` Kafka-событий, имена topics и DLQ convention;
- `events/catalogv1`, `events/authv1`, `events/profilev1`, `events/libraryv1`, `events/playbackv1`, `events/mediav1` — payload-схемы событий (версия 1);
- `media` — bucket `icyre-media`, ключи объектов (`tracks/{trackId}/audio/{64|128|256}.aac`, `original/`, `cover/`)
  и варианты качества — общий контракт media pipeline и выдачи;
- `search` — алиасы (`tracks`, `albums`, `artists`, `playlists`), документы OpenSearch и шаблоны индексов
  (анализаторы, `<field>.keyword`, `<field>.autocomplete`, `suggest`) — общий контракт Search Indexer и Search Service;
- далее здесь появится код, сгенерированный из `api/proto` (gRPC, EPIC-034).

Это **не** склад DTO: сюда попадает только то, что пересекает границу сервиса
(Kafka, gRPC). HTTP DTO и доменные сущности живут внутри своих сервисов.

## Envelope

```json
{
  "eventId": "0192…",          // UUIDv7, ключ идемпотентности для consumers
  "eventType": "track.created",
  "eventVersion": 1,
  "occurredAt": "2026-09-24T19:00:00Z",
  "producer": "catalog-service",
  "traceId": "4bf92f3577b34da6a3ce929d0e0e4736",
  "payload": { }
}
```

## Правила эволюции

- добавлять поля можно; переименовывать или менять смысл — только с новой `eventVersion`;
- consumers игнорируют неизвестные поля;
- доставка at-least-once: consumer хранит обработанные `eventId` (или делает upsert по
  естественному ключу) и безопасно переживает повтор;
- необрабатываемое сообщение после N ретраев уходит в `<topic>.dlq`.
