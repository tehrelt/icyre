# Analytics Worker

## Назначение
Подготовка аналитических данных: `playback.events` → сырые события и дневные агрегаты в ClickHouse.

## Storage
ClickHouse (`specs/data/clickhouse.md`).

## Метрики
- total plays;
- unique listeners;
- completion rate;
- skips;
- popular tracks;
- popular artists;
- daily active listeners.

Определения и расчёт — `workers/analytics/README.md`.

## Обработка
- batch consumer: вставка блоком, commit offsets после вставки;
- обогащение альбомом и артистами из Catalog (с кэшем);
- aggregation jobs: периодический пересчёт сегодняшнего и предыдущих дней, backfill командой `aggregate`.

## Consistency
Аналитика допускает eventual consistency: сырые события видны через секунды (batch linger),
агрегаты — после очередного пересчёта (по умолчанию раз в минуту). Доставка at-least-once,
дубли схлопываются по `event_id`.
