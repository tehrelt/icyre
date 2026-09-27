# scripts

Вспомогательные скрипты разработки (Bun, TypeScript). Основные команды — в корневом `Makefile`.

| Скрипт | Команда | |
|---|---|---|
| `seed-catalog.ts` | `make seed` | контент из product canvas → Catalog (напрямую, мимо gateway) |
| `load-run.ts` | `make load-*` | эксперименты EPIC-038: переключает стенд, гоняет k6 (`tests/load`), собирает CPU/память, PostgreSQL, Kafka lag → отчёт в `tests/load/results` |
| `seed-media.ts` | `make seed-media` | аудио-варианты 64/128/256 kbps для треков каталога → MinIO (нужен ffmpeg); для seed-треков без мастера; загруженные через Media Ingest транскодирует `workers/transcoder` |

Проверка типов: `make scripts-typecheck` (входит в `make check`).
