# Search Service

## Ответственность
Полнотекстовый поиск и autocomplete.

## Backend
OpenSearch.

## Индексы
```text
tracks
albums
artists
playlists
```

## API
```http
GET /search?q=...
GET /search/suggest?q=...
```

## Ranking
Факторы:
- text relevance;
- popularity;
- recency;
- exact prefix match.

## Обновление индекса
Search Service не читает PostgreSQL на каждый поиск.
Search Indexer обновляет OpenSearch из Kafka.
