# Search Indexer

Держит OpenSearch в соответствии с Catalog (EPIC-024, `specs/workers/search-indexer.md`, ADR-0004).
OpenSearch — производный индекс: любой документ восстанавливается из Catalog.

## Команды

| | |
|---|---|
| `search-indexer` | consumer `catalog.events` (group `search-indexer`) + `/health/*`, `/metrics` |
| `search-indexer migrate` | шаблоны индексов, `<alias>-v1` для отсутствующих алиасов (идемпотентно; выполняется и при старте) |
| `search-indexer reindex` | перестройка всех индексов из Catalog в `<alias>-v<N+1>` и атомарная смена алиасов |

В compose: `docker compose run --rm search-indexer reindex`; локально: `make reindex`.

## Индексы

Алиасы `tracks`, `albums`, `artists`, `playlists` → физические `<alias>-v<N>`. Шаблоны и маппинги —
`libs/contracts/search` (общие с Search Service):

- `icyre_text` — standard tokenizer + lowercase + asciifolding («Café» = «cafe»);
- `<field>.autocomplete` — edge n-gram 1–20 при индексации («pri» → «Prism Hours»);
- `<field>.keyword` — нормализованный keyword для точного и префиксного совпадения;
- `suggest` — copy_to всех названий, источник «did you mean»;
- `dynamic: strict` — поле вне контракта отклоняется.

## События

| Событие | Действие |
|---|---|
| `artist.created` | upsert в `artists` |
| `album.created` | upsert в `albums` (имена артистов — из Catalog) |
| `track.created`, `track.updated` | READY/BLOCKED → upsert в `tracks` (альбом, имена — из Catalog; BLOCKED → `available=false`); иначе — удаление |

Идемпотентность: document ID = entity ID, версия документа = `occurredAt` события (external versioning,
`external_gte`) — повторная доставка и переупорядочивание не откатывают документ к старому состоянию.
Неизвестная Catalog ссылка (404) и нераспознанный payload — сразу в `catalog.events.dlq`; сбой Catalog или
OpenSearch — retries (`CONSUMER_MAX_RETRIES`, `CONSUMER_RETRY_BACKOFF`), затем DLQ. `reindex` восстанавливает всё.

## Ограничения

- Catalog пока публикует только `*.created` для альбомов и артистов: переименования не распространяются на
  денормализованные поля треков до `reindex`. Событий плейлистов нет (EPIC-013) — индекс `playlists` пуст.
- `reindex` находит артистов через альбомы и треки: артист без релизов попадёт в индекс только по событию.
- События, пришедшие во время `reindex`, попадают в старые индексы — для точного результата
  запускать при остановленном consumer или повторить.
- После ручной очистки таблиц Catalog (при живой истории в Kafka) нужен `reindex`: новая consumer group
  перечитывает `catalog.events` с начала.

## Конфигурация

`OPENSEARCH_URL`, `OPENSEARCH_USERNAME`/`PASSWORD`, `SEARCH_INDEX_SHARDS` (1), `SEARCH_INDEX_REPLICAS` (0),
`KAFKA_BROKERS`, `CATALOG_URL`, `CATALOG_TIMEOUT`, `CONSUMER_MAX_RETRIES` (8), `CONSUMER_RETRY_BACKOFF` (1s).
