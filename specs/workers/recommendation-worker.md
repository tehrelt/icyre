# Recommendation Worker

## Назначение
Асинхронный расчёт персональных рекомендаций.

## Input
- listening history;
- likes;
- follows;
- audio features;
- popularity.

## MVP algorithm
```text
score =
genre_similarity * 0.35 +
artist_affinity * 0.30 +
popularity * 0.20 +
freshness * 0.15
```

## Output
Готовые списки пишутся в Redis.

## Future
Возможны collaborative filtering и hybrid models.
