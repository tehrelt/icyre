# Recommendation Worker

## Назначение
Асинхронный расчёт персональных рекомендаций (`workers/recommendation`).

## Input
| Сигнал | Источник | Хранение |
|---|---|---|
| listening history (plays, completions, skips) | ClickHouse `playback_events`, окно 90 дней | читается при сборке |
| likes (треки, альбомы) | `library.events` | `recommendation.liked_tracks`, `liked_albums` |
| follows | Social Service (EPIC-015) — ещё нет; в модели предусмотрены (`FollowedArtists`) | — |
| audio features | `media.events` → `audio.features_extracted` | `recommendation.track_features` (последний master) |
| popularity | ClickHouse `daily_track_stats`, окно 30 дней | читается при сборке |
| каталог, жанры, дата релиза | Catalog REST (альбомы → треки `READY`) | читается при сборке |

## MVP algorithm
```text
score =
style_similarity * 0.35 +   genre cosine; с audio features: 0.7·genre + 0.3·sound
artist_affinity  * 0.30 +
popularity       * 0.20 +   log(1+plays) / log(1+max plays)
freshness        * 0.15     0.5 ^ (возраст релиза / 90 дней)
```

Вклад сигнала в вкус трека: completion +1, like +3, like альбома +2 (артистам и жанрам альбома),
follow +3 (артисту), skip −1, незавершённое прослушивание +0.25. Из вкуса строятся affinity
по артистам и жанрам и центр «звучания» (tempo, loudness, loudness range, silence, нормированные в [0, 1];
сравниваются только features одной версии анализатора).

Фильтры: лайкнутые треки и треки лайкнутых альбомов исключаются, чаще пропускаемые, чем дослушанные, — тоже;
уже дослушанные — со штрафом ×0.5. Артист получает score своего лучшего трека. Причины (`reasons`) —
два наибольших взвешенных компонента (`artist`, `style`, `popular`, `fresh`).

## Output
Redis, контракт `libs/contracts/recommendation`:

```text
recommendations:user:{id}   персональный набор (50 треков, 20 артистов), TTL 24 ч
recommendations:popular     fallback: 0.7·popularity + 0.3·freshness; артисты по plays
```

Сборка — каждые 10 минут и командой `build`. Пользователь без сигналов набора не получает,
его обслуживает `popular`.

## Масштабирование
MVP скорит весь каталог для каждого пользователя: O(users × tracks). Дальше — генерация кандидатов
(треки любимых артистов и жанров, ближайшие по звучанию, популярное), инкрементальная пересборка
только изменившихся пользователей, копия каталога из `catalog.events` вместо обхода REST.

## Future
Возможны collaborative filtering и hybrid models.
