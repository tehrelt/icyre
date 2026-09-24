# ClickHouse

## Назначение
Большие объёмы append-heavy аналитических событий.

## Примеры таблиц
```text
playback_events
daily_track_stats
daily_artist_stats
```

## Почему не PostgreSQL
Агрегации по миллионам событий лучше выполнять в column-oriented analytics DB.

## Retention
Raw events могут иметь ограниченный срок хранения.
Агрегаты могут храниться дольше.
