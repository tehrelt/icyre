# Pagination

## Предпочтительный подход

Cursor-based pagination.

```http
GET /tracks?limit=50&cursor=...
```

## Ответ

```json
{
  "data": [],
  "pagination": {
    "nextCursor": "...",
    "hasMore": true
  }
}
```

## Почему не offset

Большие `OFFSET` становятся дорогими и дают нестабильную пагинацию при изменении набора данных.

## Cursor content

Cursor является opaque для клиента.

Внутри может кодироваться:

```text
sort_value
entity_id
```

Клиент не должен зависеть от внутреннего формата cursor.
