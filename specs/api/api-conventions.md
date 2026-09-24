# API conventions

## Base path
```text
/api/v1
```

## Format
```text
application/json
UTF-8
```

## Timestamps
UTC, ISO 8601.

## IDs
UUIDv7 preferred.

## Naming
JSON поля — `camelCase`.

## Idempotency
Для чувствительных POST-запросов:

```http
Idempotency-Key: <uuid>
```

Примеры:
- payment;
- upload completion;
- playlist batch operations.

## Request ID
```http
X-Request-ID
```

Если клиент не прислал ID, gateway создаёт его.

## Versioning
Breaking changes требуют новой major API version.
