# ICYRE

Масштабируемый музыкальный стриминг-сервис: monorepo с Go-backend и React/Bun-frontend.

| Источник правды | Где |
|---|---|
| Архитектура и спецификации | [`specs/`](specs/README.md) |
| План и статусы | [`backlog/`](backlog/README.md), текущий фокус — [`backlog/CURRENT.md`](backlog/CURRENT.md) |
| Design System | https://claude.ai/artifact/RA7b9Qphfgy2MmAVzsxcDT |
| Продуктовые экраны | https://claude.ai/artifact/5GypRL4RmbSQUxEGMDkwfM |

## Структура

```text
apps/web/            React + TypeScript + Vite (Bun workspace)
services/catalog/    Catalog Service — эталонная vertical slice (Go module)
services/bff/        Web BFF — page-oriented API для веб-клиента (агрегация сервисов)
services/auth/       Auth Service — аккаунты, сессии, JWT (EdDSA) + JWKS, refresh cookie
services/user-profile/ User Profile Service — публичный профиль, создаётся из user.registered
services/stream-auth/ Stream Authorization — проверка трека и short-lived signed URL на аудио
services/library/    Library Service — сохранённые треки и альбомы слушателя
services/playlist/   Playlist Service — плейлисты слушателя: CRUD, треки, reorder → playlist.events
services/playback/   Playback Service — телеметрия плеера → playback.events (сессии и очередь — позже)
services/history/    Listening History — playback.events → история прослушиваний, /me/history
services/recommendation/ Recommendation Service — готовые рекомендации из Redis, fallback popular
services/search/     Search Service — полнотекстовый поиск и autocomplete (OpenSearch)
workers/search-indexer/ Search Indexer — catalog.events → OpenSearch, reindex с переключением алиасов
workers/analytics/   Analytics Worker — playback.events → ClickHouse, дневные агрегаты, отчёт (топы, completion rate)
workers/recommendation/ Recommendation Worker — history, likes, popularity, audio features → scoring → Redis
libs/platform/       инфраструктура: config, logger, httpserver, health, shutdown, postgres, redis, kafka,
                     objectstore (S3/MinIO), opensearch, clickhouse, telemetry, authn
libs/contracts/      межсервисные контракты: Kafka envelope, event payloads, media object keys,
                     поисковые документы и маппинги, схема аналитики ClickHouse
api/proto/           protobuf (gRPC, позже)
deploy/              API gateway (nginx), Prometheus, Grafana, Kafka topics, MinIO bucket bootstrap
scripts/             seed-скрипты (Bun): каталог из canvas, аудио-варианты
go.work              Go workspace
package.json         Bun workspace
```

Модули и сервисы появляются по мере backlog — пустые каталоги заранее не создаются.

## Требования

- Go 1.26 (`go.work` указывает `toolchain go1.26.8`; с `GOTOOLCHAIN=auto` он скачается сам)
- Bun ≥ 1.3
- Docker + Docker Compose
- ffmpeg — только для `make seed-media`

Аудио — AAC (как в `specs/data/object-storage.md`): Chrome, Edge, Safari и Firefox его играют; open-source сборки
Chromium (в том числе из Playwright) — нет, там плеер покажет «This track could not be played».

## Быстрый старт

```bash
bun run stack:up                  # образы батчами по 4 (make up), затем compose up: Postgres, Redis, Kafka, MinIO, сервисы, Gateway…
make seed                         # контент из product canvas → Catalog
make seed-media                   # аудио-варианты 64/128/256 kbps → MinIO (нужен ffmpeg, ~3 мин)
docker compose run --rm search-indexer reindex   # поисковый индекс из каталога (дальше — по событиям)
bun install && bun run dev        # http://localhost:5173 (mock API по умолчанию)
bun run dev:real                  # тот же UI на реальных данных через gateway (любая ОС)
```

`docker compose up -d --build` собирает все Go-образы одновременно (~20 мин); `bun run stack:build` собирает их
по 4 за раз (`COMPOSE_BUILD_BATCH=n` или `--batch n`, можно назвать сервисы: `bun run stack:build playlist bff`).
Go-модули и build cache лежат в общих BuildKit cache mounts (`icyre-gomod`, `icyre-gobuild`): зависимости компилируются
один раз на весь стенд, пересборка после правки сервиса — секунды. Сбросить: `docker builder prune --filter type=exec.cachemount`.

| Что | URL |
|---|---|
| Web | http://localhost:5173 |
| API Gateway (публичный `/api/v1`) | http://localhost:8080/api/v1 |
| Catalog API (напрямую, включая запись) | http://localhost:8081/api/v1 · `/health/ready` · `/metrics` |
| Web BFF | http://localhost:8082/api/v1/pages/home |
| Auth | http://localhost:8083/api/v1/auth/.well-known/jwks.json |
| User Profile | http://localhost:8084/health/ready |
| Stream Authorization | http://localhost:8085/health/ready |
| Search | http://localhost:8080/api/v1/search?q=nova · напрямую :8086 |
| Library | http://localhost:8088/health/ready (API — через gateway, `/api/v1/me/library/*`) |
| OpenSearch | http://localhost:9200 |
| Recommendations | http://localhost:8080/api/v1/recommendations/home · напрямую :8097 |
| ClickHouse | http://localhost:8123/play (icyre / icyre) |
| MinIO console | http://localhost:9001 (icyre / icyre-secret) |
| Jaeger | http://localhost:16686 |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3000 (admin / admin) → ICYRE → «ICYRE — services» |

Экранов входа в canvas пока нет (EPIC-051), поэтому аккаунт создаётся через API — cookie сессии ставится на тот же
origin, что и у веб-клиента:

```js
// в DevTools на http://localhost:5173 (VITE_API_MOCKS=false), затем перезагрузить страницу
await fetch('/api/v1/auth/register', { method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email: 'rin@example.com', password: 'correct horse battery' }) });
```

### Windows

Скрипты стенда (`deploy/**/*.sh`) выполняются в Linux-контейнерах, поэтому `.gitattributes` держит их в LF
при любом `core.autocrlf`. Если репозиторий был склонирован до появления `.gitattributes` и `kafka-init`
падает с `exit 2`, перевыпишите файлы: `git rm -r --cached -q deploy && git reset -q --hard`.

## Команды

```bash
make help              # все цели
make sync fmt vet test # Go
make test-integration  # нужен `make up-core`
make web-build web-test web-lint
make check             # всё вместе
```
