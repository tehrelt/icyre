# Search Service

Полнотекстовый поиск и autocomplete (EPIC-025, `specs/services/search.md`). Читает только OpenSearch —
PostgreSQL на поиске не участвует. Индексы ведёт `workers/search-indexer`.

## API (через gateway, только GET)

`GET /api/v1/search?q=&type=all|tracks|artists|albums|playlists&limit=` — контракт экрана Search
(`apps/web/src/pages/search/api/search.ts`):

```json
{ "query": "nova", "type": "all",
  "counts": { "tracks": 17, "artists": 1, "albums": 3, "playlists": 0 },
  "topResult": { "item": { "kind": "artist", "id": "…", "name": "Nova Hale", "coverUrl": null, "art": 3 }, "verified": false },
  "tracks": [ … ], "artists": [ … ], "albums": [ … ], "playlists": [],
  "didYouMean": null }
```

- `type=all`: до 4 треков и до 6 артистов/альбомов/плейлистов; иначе — до `limit` (≤ 50) одного типа.
  `counts` всегда полные — для счётчиков вкладок.
- `topResult` (только `all`): артист, чьё имя начинается с запроса; иначе такой альбом; иначе лучший по score.
- `didYouMean` — только когда ничего не найдено (term suggester по полю `suggest`).

`GET /api/v1/search/suggest?q=&limit=` (≤ 20) → `{ "query", "suggestions": [{ "kind", "id", "text", "subtitle" }] }`.

Ошибки: 422 `VALIDATION_FAILED` (пустой или > 200 символов `q`, неизвестный `type`, плохой `limit`),
503 `SERVICE_UNAVAILABLE` — OpenSearch недоступен. `Cache-Control: public, max-age=30`.

## Ranking

```text
score = relevance × (log10(2 + popularity) + 0.25 × recency)
```

- relevance: слова (cross_fields, AND) + опечатки (fuzziness AUTO, первая буква точная) + search-as-you-type
  (edge n-gram); бусты: точное название ×10, префикс названия ×4, фраза ×2; название весит ×3 против имён артистов/альбома;
- popularity — поле документа (0, пока нет Listening History / Analytics);
- recency — gauss по дате релиза (для артистов — `updatedAt`): 1 → 0.5 за год после 30 дней.

Проверено integration-тестами на OpenSearch 3.3 (`OPENSEARCH_URL=… go test -tags integration ./...`):
точное > префикс > слово, свежий релиз выше при равном тексте, опечатки, диакритика, «did you mean».

## Конфигурация

`OPENSEARCH_URL`, `OPENSEARCH_USERNAME`/`PASSWORD`, `OPENSEARCH_TIMEOUT` (1s), общие `HTTP_*`, `LOG_*`, `OTEL_*`.
