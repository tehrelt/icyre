# Recommendation Service

## Ответственность
Выдача подготовленных персональных рекомендаций.

## API
```http
GET /recommendations/home
GET /recommendations/tracks
GET /recommendations/artists
```

## Источник
Recommendation Worker строит рекомендации асинхронно.

## Cache
Готовые подборки хранятся в Redis:

```text
recommendations:user:{id}
```

## Fallback
Если персональные данные отсутствуют:
- trending;
- popular by genre;
- editorial playlists.
