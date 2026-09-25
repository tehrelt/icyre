# EPIC-023 — OpenSearch foundation

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Задачи

- [x] TASK-023.1 Добавить OpenSearch в Docker Compose.

- [x] TASK-023.2 Создать index templates.

- [x] TASK-023.3 Создать indices:
  - tracks;
  - albums;
  - artists;
  - playlists.

- [x] TASK-023.4 Добавить aliases.

- [x] TASK-023.5 Настроить analyzers.

- [x] TASK-023.6 Добавить autocomplete fields.

## Итог реализации

- Compose: `opensearchproject/opensearch:3.3.0`, single-node, security plugin выключен только на локальном стенде,
  disk watermarks выключены (узел видит весь диск хоста), volume `opensearch-data`, healthcheck.
- Templates: `icyre-<alias>` для `<alias>-v*` — `libs/contracts/search/mapping.go` (общий контракт индексатора и поиска).
- Indices и aliases: `tracks`, `albums`, `artists`, `playlists` → `<alias>-v<N>`; `search-indexer migrate` создаёт v1,
  `reindex` — v<N+1> и атомарно переключает алиасы (`_aliases` add/remove одним запросом), старые удаляет.
- Analyzers: `icyre_text` (lowercase + asciifolding), normalizer `icyre_keyword`; autocomplete — edge n-gram 1–20
  (`<field>.autocomplete`, search analyzer без n-gram); `suggest` для «did you mean»; `dynamic: strict`.
- `libs/platform/opensearch` — клиент на net/http (bulk с external versioning, index/alias admin, readiness).
- Проверено на реальном кластере: asciifolding, autocomplete, нормализованный keyword, отказ строгого маппинга.

---
