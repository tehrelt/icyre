# OpenSearch

## Назначение
Полнотекстовый поиск.

## Indices
```text
tracks-v1
artists-v1
albums-v1
playlists-v1
```

## Aliases
Для zero-downtime reindex:
```text
tracks -> tracks-v2
```

## Mapping
Нужно различать:
- analyzed text;
- keyword;
- autocomplete fields;
- numeric popularity signals.

## Source of truth
OpenSearch не считается основной БД.
Индекс может быть полностью перестроен.
