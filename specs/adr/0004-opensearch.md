# ADR-0004: OpenSearch для полнотекстового поиска

## Статус
Accepted.

## Контекст
PostgreSQL подходит для MVP-поиска, но сложный autocomplete, fuzzy search и ranking лучше выделить.

## Решение
Использовать OpenSearch как производный поисковый индекс.

## Source of truth
PostgreSQL остаётся source of truth.

## Синхронизация
Kafka -> Search Indexer -> OpenSearch.

## Последствия
Появляется eventual consistency между каталогом и поиском, но поисковая нагрузка изолируется от основной БД.
