# Audio Analysis Worker

## Назначение
Извлечение признаков для рекомендаций и аналитики.

## Input event
```text
track.uploaded
```

## Признаки
```text
bpm, bpm_confidence
integrated_lufs, loudness_range_lu, true_peak_dbtp   (EBU R128)
silence_ratio
analyzed_ms
```

Кандидаты на следующие версии анализатора: spectral features (centroid, rolloff), energy/danceability.

## Ограничение
Для дипломной реализации достаточно BPM/loudness и нескольких простых характеристик. Темп — автокорреляция
onset-огибающей (spectral flux) с априорным весом вокруг 120 BPM; октавные ошибки возможны.

## Tools
FFmpeg: один проход декодирования по presigned GET URL — фильтр `ebur128` и моно PCM 11025 Hz для анализа
темпа и тишины в Go (`internal/dsp`).

## Result
Признаки пишутся в БД (`audio_features.track_features`, строка на upload, с `analyzer_version`) и публикуются
событием `audio.features_extracted` через transactional outbox (media.events, key = trackId).

## Event
```text
audio.features_extracted
```

## Retry
Upload ID — ключ идемпотентности: повторная доставка не пишет ни строку, ни событие. Мастер, которого нет
или который ffmpeg не декодирует, пропускается (ошибку сообщает Transcoder через `media.transcode.failed`).
Реализация — `workers/audio-analysis/README.md`.
