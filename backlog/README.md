# Backlog проекта

Каждый epic вынесен в отдельный Markdown-файл.

## Статусы

```text
[ ] TODO
[-] IN PROGRESS
[x] DONE
[!] BLOCKED
```

## Приоритеты

```text
P0 — блокирует дальнейшую разработку
P1 — необходимо для MVP
P2 — важно, но можно реализовать после основной вертикали
P3 — развитие / исследовательская часть
```

## Рекомендуемый порядок старта

1. EPIC-001 — Bootstrap монорепозитория
2. EPIC-002 — Shared platform library
3. EPIC-003 — PostgreSQL foundation
4. EPIC-005 — Kafka foundation
5. EPIC-006 — Observability foundation
6. EPIC-007 — Catalog Service: domain model
7. EPIC-008 — Catalog Service: PostgreSQL adapter
8. EPIC-009 — Catalog Service: HTTP API
9. EPIC-010 — Catalog events

После этого Catalog Service используется как эталонная реализация для остальных Go-сервисов.

## Все эпики

- [EPIC-001 — Bootstrap монорепозитория](001-bootstrap-монорепозитория.md)
- [EPIC-002 — Shared platform library](002-shared-platform-library.md)
- [EPIC-003 — PostgreSQL foundation](003-postgresql-foundation.md)
- [EPIC-004 — Redis foundation](004-redis-foundation.md)
- [EPIC-005 — Kafka foundation](005-kafka-foundation.md)
- [EPIC-006 — Observability foundation](006-observability-foundation.md)
- [EPIC-007 — Catalog Service: domain model](007-catalog-service-domain-model.md)
- [EPIC-008 — Catalog Service: PostgreSQL adapter](008-catalog-service-postgresql-adapter.md)
- [EPIC-009 — Catalog Service: HTTP API](009-catalog-service-http-api.md)
- [EPIC-010 — Catalog events](010-catalog-events.md)
- [EPIC-011 — Auth Service](011-auth-service.md)
- [EPIC-012 — User Profile Service](012-user-profile-service.md)
- [EPIC-013 — Playlist Service](013-playlist-service.md)
- [EPIC-014 — Library Service](014-library-service.md)
- [EPIC-015 — Social Service](015-social-service.md)
- [EPIC-016 — Playback Service](016-playback-service.md)
- [EPIC-017 — Stream Authorization Service](017-stream-authorization-service.md)
- [EPIC-018 — MinIO / S3 foundation](018-minio-s3-foundation.md)
- [EPIC-019 — Media Ingest Service](019-media-ingest-service.md)
- [EPIC-020 — Transcoder Worker](020-transcoder-worker.md)
- [EPIC-021 — Metadata Worker](021-metadata-worker.md)
- [EPIC-022 — Audio Analysis Worker](022-audio-analysis-worker.md)
- [EPIC-023 — OpenSearch foundation](023-opensearch-foundation.md)
- [EPIC-024 — Search Indexer](024-search-indexer.md)
- [EPIC-025 — Search Service](025-search-service.md)
- [EPIC-026 — Listening History Worker](026-listening-history-worker.md)
- [EPIC-027 — ClickHouse foundation](027-clickhouse-foundation.md)
- [EPIC-028 — Analytics Worker](028-analytics-worker.md)
- [EPIC-029 — Recommendation Service](029-recommendation-service.md)
- [EPIC-030 — Notification Service](030-notification-service.md)
- [EPIC-031 — Admin Service](031-admin-service.md)
- [EPIC-032 — API Gateway](032-api-gateway.md)
- [EPIC-033 — Web BFF](033-web-bff.md)
- [EPIC-034 — gRPC contracts](034-grpc-contracts.md)
- [EPIC-035 — Security hardening](035-security-hardening.md)
- [EPIC-036 — CI](036-ci.md)
- [EPIC-037 — Integration testing](037-integration-testing.md)
- [EPIC-038 — Load testing](038-load-testing.md)
- [EPIC-039 — Kubernetes](039-kubernetes.md)
- [EPIC-040 — Исследовательская часть](040-исследовательская-часть.md)
