# API Gateway

## Ответственность
- TLS termination
- routing
- CORS
- rate limiting
- request ID
- access logs
- базовая JWT-проверка
- балансировка

## Не должен делать
- выполнять бизнес-логику;
- напрямую обращаться к PostgreSQL;
- публиковать доменные события от имени сервисов.

## Маршруты
```text
/api/v1/auth/*        -> Auth Service
/api/v1/catalog/*     -> BFF/Catalog
/api/v1/playlists/*   -> Playlist Service
/api/v1/search/*      -> Search Service
/api/v1/playback/*    -> Playback Service
```

## Масштабирование
Gateway разворачивается минимум в двух репликах в production.
