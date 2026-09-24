# Redis

## Использование
- cache;
- sessions;
- rate limit;
- playback state;
- recommendation cache;
- distributed short-lived locks.

## Key naming
```text
cache:track:{id}
session:{id}
playback:{userId}
recommendations:{userId}
ratelimit:{subject}:{window}
```

## TTL
Cache всегда имеет TTL, если нет обоснования для постоянного хранения.

## Правило
Redis не является source of truth для каталога и пользовательской библиотеки.
