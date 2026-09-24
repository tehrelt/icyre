# Документация масштабируемого музыкального стриминг-сервиса

Эта папка содержит техническую документацию и спецификации системы для проекта:

**«Исследование архитектурных подходов и реализация масштабируемого музыкального стриминг-сервиса»**

## Основной стек

- Frontend: React + TypeScript
- Backend: Go
- Public API: REST
- Internal API: gRPC
- Database: PostgreSQL
- Cache: Redis
- Event streaming: Kafka
- Search: OpenSearch
- Analytics: ClickHouse
- Object Storage: S3 / MinIO
- Media processing: FFmpeg
- Gateway: Traefik / Nginx
- Observability: Prometheus + Grafana + Loki + Jaeger
- Containers: Docker
- Orchestration: Kubernetes

## Содержание

### Архитектура
- [Обзор архитектуры](architecture/overview.md)
- [Поток пользовательского запроса](architecture/request-flow.md)
- [Поток воспроизведения аудио](architecture/media-flow.md)
- [Событийная архитектура](architecture/event-driven.md)
- [Масштабирование](architecture/scalability.md)
- [Надёжность и отказоустойчивость](architecture/reliability.md)

### Сервисы
- [API Gateway](services/api-gateway.md)
- [Web BFF](services/bff.md)
- [Auth Service](services/auth.md)
- [User Profile Service](services/user-profile.md)
- [Catalog Service](services/catalog.md)
- [Search Service](services/search.md)
- [Playlist Service](services/playlist.md)
- [Library Service](services/library.md)
- [Social Service](services/social.md)
- [Recommendation Service](services/recommendation.md)
- [Playback Service](services/playback.md)
- [Stream Authorization Service](services/stream-auth.md)
- [Media Ingest Service](services/media-ingest.md)
- [Media Origin Service](services/media-origin.md)
- [Admin Service](services/admin.md)
- [Notification Service](services/notification.md)

### Workers
- [Transcoder Worker](workers/transcoder.md)
- [Metadata Worker](workers/metadata.md)
- [Audio Analysis Worker](workers/audio-analysis.md)
- [Search Indexer](workers/search-indexer.md)
- [Listening History Worker](workers/listening-history.md)
- [Analytics Worker](workers/analytics.md)
- [Recommendation Worker](workers/recommendation-worker.md)

### Данные
- [PostgreSQL](data/postgres.md)
- [Redis](data/redis.md)
- [Kafka](data/kafka.md)
- [OpenSearch](data/opensearch.md)
- [ClickHouse](data/clickhouse.md)
- [S3 / MinIO](data/object-storage.md)

### API
- [API conventions](api/api-conventions.md)
- [Error model](api/error-model.md)
- [Pagination](api/pagination.md)

### Остальное
- [Security](security/security.md)
- [Observability](observability/observability.md)
- [Docker Compose](deployment/docker-compose.md)
- [Kubernetes](deployment/kubernetes.md)
- [Load testing](testing/load-testing.md)
- [ADR: Modular monolith → services](adr/0001-modular-monolith.md)
- [ADR: S3 + CDN для аудио](adr/0002-object-storage-cdn.md)
- [ADR: Kafka для событий](adr/0003-kafka-events.md)
- [ADR: OpenSearch для поиска](adr/0004-opensearch.md)
