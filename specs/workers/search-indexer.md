# Search Indexer

## Назначение
Синхронизация доменных сущностей с OpenSearch.

## Consumes
```text
track.created
track.updated
track.deleted
album.updated
artist.updated
playlist.updated
```

## Idempotency
Document ID в OpenSearch совпадает с entity ID.

Повторное событие безопасно применяет `upsert`.
