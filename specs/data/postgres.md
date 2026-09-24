# PostgreSQL

## Назначение
Основное транзакционное хранилище.

## Домены
- users;
- catalog;
- playlists;
- library;
- social;
- subscriptions;
- listening history;
- moderation.

## Начальная стратегия
Один PostgreSQL cluster + отдельные schemas.

## Эволюция
При разделении сервисов возможен подход database-per-service.

## Индексы
Особое внимание:
- foreign keys;
- `(user_id, created_at)`;
- `(artist_id, release_date)`;
- unique constraints;
- partial indexes для активных записей.

## Connection pooling
Рекомендуется PgBouncer или корректно настроенный pool в Go.
