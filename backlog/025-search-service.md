# EPIC-025 — Search Service

> Проект: **Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса**

**Status:** [x] DONE

**Priority:** P1

## Задачи

- [x] TASK-025.1 Создать module.

- [x] TASK-025.2 Реализовать full-text search.

- [x] TASK-025.3 Реализовать autocomplete.

- [x] TASK-025.4 Реализовать фильтры по entity type.

- [x] TASK-025.5 Добавить ranking.

- [x] TASK-025.6 Реализовать:
  - `GET /search`
  - `GET /search/suggest`

## Итог реализации

`services/search`, `GET /api/v1/search` и `GET /api/v1/search/suggest` через gateway (только GET).

- Full-text: multi_match cross_fields + fuzziness AUTO + edge n-gram, по всем четырём индексам одним `_msearch`.
- Autocomplete: `/search/suggest` — префиксы названий/имён, слияние типов по score.
- Фильтр по типу: `type=all|tracks|artists|albums|playlists`; `counts` всегда по всем типам.
- Ranking: text relevance, exact/prefix boosts, popularity (`log2p`), recency (gauss по дате релиза) — формула в README.
- Ответ совпадает с контрактом экрана Search (zod-схема фронтенда): экран на реальных данных проверен в браузере,
  включая «Search for “nova hale”» на опечатку.
- Популярности пока нет (Listening History/Analytics) — поле в документах есть, сигнал нулевой.

---
