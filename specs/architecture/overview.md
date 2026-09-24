# Обзор архитектуры

## Цель

Архитектура должна поддерживать рост количества пользователей, каталога и событий прослушивания без необходимости вертикально масштабировать один центральный backend.

## Высокоуровневая схема

```text
Clients
  |
  v
CDN / API Gateway
  |
  v
BFF / Domain Services
  |
  +--> PostgreSQL
  +--> Redis
  +--> OpenSearch
  +--> Kafka
  +--> S3 / MinIO
  |
  v
Async Workers
```

## Архитектурные принципы

1. Аудиоданные не проходят через обычный application backend.
2. Тяжёлые операции выполняются асинхронно.
3. Сервисы по возможности stateless.
4. Состояние хранится в специализированных хранилищах.
5. Чтение и запись масштабируются отдельно.
6. Доменные события используются для слабой связанности.
7. Каждый критический компонент наблюдаем через метрики, логи и трассировку.

## Домены

- Identity
- User Profile
- Catalog
- Playlist
- Library
- Social
- Search
- Recommendation
- Playback
- Media
- Analytics
- Notifications
- Administration

## Синхронные взаимодействия

Для внешнего клиента используется REST.

Для внутренних вызовов допускается gRPC.

Пример:

```text
Web -> API Gateway -> BFF -> Catalog Service
```

## Асинхронные взаимодействия

Kafka используется для событий:

```text
track.uploaded
track.transcoded
track.updated
playback.started
playback.finished
track.liked
playlist.updated
user.followed
```

## Основные архитектурные качества

- horizontal scalability;
- fault isolation;
- cacheability;
- observability;
- asynchronous processing;
- eventual consistency там, где строгая консистентность не требуется.
