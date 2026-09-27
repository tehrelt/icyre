# Load testing (EPIC-038)

k6-сценарии и эксперименты для исследовательской части (EPIC-040, `specs/testing/load-testing.md`).
k6 запускается в контейнере (`grafana/k6`, сервис `k6` из `deploy/load/compose.yml`) внутри сети стенда
и бьёт в API Gateway — локально k6 ставить не нужно.

## Подготовка

```bash
make up && make seed          # стенд и каталог
make seed-media               # аудио-варианты — только для load-media
```

## Эксперименты

| Команда | Сценарий | Варианты | Переключатель |
|---|---|---|---|
| `make load-baseline` | `catalog.js` | Catalog без кэша, 1 реплика | — |
| `make load-redis` | `catalog.js` | без Redis / с Redis | `CATALOG_CACHE_ENABLED` |
| `make load-replicas` | `catalog.js` | Catalog ×1 / ×N (`--replicas 3`) | `--scale catalog=N` |
| `make load-analytics` | `playback.js` | Kafka / синхронная запись в ClickHouse | `PLAYBACK_ANALYTICS_MODE` |
| `make load-media` | `media.js` | signed URL → MinIO / аудио через gateway + Stream Auth | `STREAM_MEDIA_PROXY` |
| `make load-all` | все подряд | | |

Нагрузка — `LOAD_ARGS`: `--profile smoke|100|1000|5000|10000` (VU и длительность из
`lib.js`; по умолчанию `100` — 100 VU, 1 мин), `--vus N`, `--duration 2m`. Например:
`make load-redis LOAD_ARGS="--profile 1000"`.

`scripts/load-run.ts` на каждый вариант:

1. переключает стенд (`docker compose -f docker-compose.yml -f deploy/load/compose.yml up -d --no-deps …`)
   с переменными варианта и ждёт gateway (+12 с на перерезолв upstream в nginx);
2. запускает k6 (`--summary-export`) и параллельно собирает:
   - CPU и память контейнеров эксперимента — `docker stats` раз в 3 с, суммарно по репликам;
   - нагрузку на PostgreSQL — дельта `pg_stat_database` (транзакции/с, прочитанные кортежи/с, доля
     попаданий в shared buffers);
   - lag consumer groups Kafka (максимум за прогон, опрос раз в 10 с) — для `analytics`;
3. пишет `tests/load/results/<эксперимент>-<время>.{md,json}` (результаты в git не попадают).

В конце стенд возвращается к значениям по умолчанию (кэш выключен, Kafka, без прокси, одна реплика).

## Overlay

`deploy/load/compose.yml`:

- снимает host-порт Catalog, чтобы его можно было масштабировать; gateway балансирует реплики через
  Docker DNS;
- монтирует в gateway `deploy/load/ratelimit-exempt.conf` — все клиенты вне rate limit, иначе
  генератор с одного IP упирается в 50 r/s и измеряется лимитер;
- сервис `k6` (profile `load`): скрипты из `tests/load`, результаты в `tests/load/results`,
  `MEDIA_ORIGIN=http://minio:9000` — presigned URL подписаны на `localhost:9000`, k6 ходит в MinIO внутри
  сети с исходным `Host` (он входит в подпись).

Сценарий вручную:

```bash
docker compose -f docker-compose.yml -f deploy/load/compose.yml run --rm --no-deps -e PROFILE=smoke k6 run catalog.js
```

## Сценарии

- `catalog.js` — чтение каталога через gateway: альбом 35 %, треки альбома 30 %, трек 20 %, артист 10 %,
  жанры 5 %; ID — из первых `ALBUMS` (50) альбомов. Порог: p95 < 300 мс.
- `playback.js` — одно прослушивание за итерацию: `started`, затем `finished` (70 %) или `skipped`;
  `USERS` (20) слушателей регистрируются в setup. Порог: p95 < 300 мс.
- `media.js` — `MODE=direct`: `POST /stream/authorize` → GET signed URL из MinIO; `MODE=proxy`:
  `GET /stream/proxy/{id}`; файл качается целиком, `audio_download_ms` — время до полного файла,
  `QUALITY` (128).

Пороги (SLO-кандидаты) не прерывают прогон: нарушение отмечается в выводе k6.

## Ограничения

- Генератор нагрузки и стенд делят одну машину: абсолютные значения зависят от железа, сравнивать
  имеет смысл варианты одного прогона.
- Окно метрик PostgreSQL включает setup k6 (выборка ID, регистрация слушателей).
- В режиме `sync` события не попадают в Kafka — lag показывает только фоновых потребителей.
