# Audio Analysis Worker

Измеряет аудиопризнаки загруженного мастера (EPIC-022, `specs/workers/audio-analysis.md`): темп, громкость
по EBU R128, долю тишины. Работает параллельно с Transcoder и Metadata Worker: все читают `track.uploaded`
своими consumer group.

## Поток

```text
media.events: track.uploaded
  -> stat tracks/{trackId}/original/{uploadId}.{ext}
  -> presigned GET (TTL = ANALYSIS_TIMEOUT) -> ffmpeg декодирует мастер потоком, на диск ничего не пишется
       ├─ ebur128 (peak=true)            -> integrated loudness, LRA, true peak (сводка в stderr)
       └─ mono, 11025 Hz, f32le в stdout -> internal/dsp: onset-огибающая -> BPM; RMS -> доля тишины
  -> audio_features.track_features + audio_features.outbox (одна транзакция)
  -> relay -> media.events: audio.features_extracted
```

Consumer group `audio-analysis-worker`, `/health/*` и `/metrics` на `HTTP_ADDR`. Локально:
`make run-audio-analysis` (нужен ffmpeg в PATH, миграции применяются на старте); в compose — сервис
`audio-analysis-worker`.

## Признаки

| Поле | Как считается |
|---|---|
| `bpm` | spectral flux лог-спектров (Hann 1024, hop 128 → ~86 значений/с), вычитание локального среднего, нормированная автокорреляция на лагах 60–200 BPM с лог-нормальным априорным весом вокруг 120 BPM; параболическая интерполяция пика. Более быстрая октава побеждает, только если почти так же периодична. `NULL` — темпа нет (тишина, шум, < 6 с аудио) |
| `bpm_confidence` | значение пика автокорреляции, [0, 1]; ниже 0.1 темп не сообщается |
| `integrated_lufs` | EBU R128 integrated loudness (ffmpeg `ebur128`) |
| `loudness_range_lu` | EBU R128 loudness range (LRA) |
| `true_peak_dbtp` | максимальный true peak по каналам |
| `silence_ratio` | доля блоков по 128 сэмплов (~12 мс) с RMS ниже −60 dBFS |
| `analyzed_ms` | длительность декодированного аудио |

Уровни тишины (`-inf`, ниже абсолютного гейта) — −70. Как у любой оценки темпа, возможны октавные ошибки:
быстрый бит с сильным тактовым рисунком «бочка–малый» может прочитаться вдвое медленнее (160 → 80);
в тестах это зафиксировано отдельно (`internal/dsp/drums_test.go`).

`analyzer_version` (сейчас `1`) меняется вместе с алгоритмами: признаки разных версий несравнимы,
пересчёт — новой версией.

## Данные

`audio_features.track_features` — строка на upload, повторная загрузка трека — новая строка; текущий мастер —
последний `uploaded_at` (индекс `track_id, uploaded_at DESC`).

## События

| Событие | Когда |
|---|---|
| `audio.features_extracted` | строка записана; payload: `uploadId`, `trackId`, `bpm` (null — нет темпа), `bpmConfidence`, `integratedLufs`, `loudnessRangeLu`, `truePeakDbtp`, `silenceRatio`, `analyzedMs`, `analyzerVersion`, `sourceSha256`, `uploadedAt`, `analyzedAt` |

Топик `media.events`, ключ — track ID, тот же порядок, что у остальных событий трека.

## Идемпотентность и ошибки

- upload уже записан — `duplicate`, без работы; гонка двух доставок — `INSERT … ON CONFLICT DO NOTHING`,
  событие пишет только вставившая строку;
- мастера нет (`missing`) или ffmpeg его не декодирует (`undecodable`) — лог и commit: ошибку публикует
  Transcoder (`media.transcode.failed`);
- хранилище, ffmpeg не запустился, Postgres, истёк `ANALYSIS_TIMEOUT` — retries платформенного consumer
  (`CONSUMER_MAX_RETRIES`, `CONSUMER_RETRY_BACKOFF`), затем `media.events.dlq`;
- нераспознанный envelope/payload, неизвестная версия, неполный job — сразу в DLQ.

ffmpeg ограничен протоколами file/http(s); подпись presigned URL вырезается из его ошибок.

Метрика: `media_audio_analysis_duration_seconds{result=analyzed|duplicate|missing|undecodable|error}`.

## Конфигурация

`DATABASE_URL`, `MIGRATE_ON_START` (false), `KAFKA_BROKERS`, `S3_ENDPOINT`, `S3_SECURE`, `S3_ACCESS_KEY`,
`S3_SECRET_KEY`, `S3_REGION`, `S3_TIMEOUT`, `S3_MEDIA_BUCKET` (`icyre-media`, для readiness), `FFMPEG_PATH`
(`ffmpeg`), `ANALYSIS_TIMEOUT` (5m), `CONSUMER_MAX_RETRIES` (5), `CONSUMER_RETRY_BACKOFF` (2s).
