# Catalog Service

## Ответственность
Каталог исполнителей, альбомов, треков и жанров.

## Основные сущности
```text
Artist
Album
Track
Genre
TrackArtist
AlbumArtist
```

## Track
```text
id
album_id
title
duration_ms
track_number
disc_number
explicit
isrc
status
created_at
updated_at
```

## Статусы
```text
DRAFT
PROCESSING
READY
BLOCKED
DELETED
```

## API
```http
GET /tracks/{id}
GET /albums/{id}
GET /albums/{id}/tracks
GET /artists/{id}
GET /artists/{id}/albums
GET /genres
```

## Events
```text
track.created
track.updated
track.deleted
album.updated
artist.updated
```

## Cache
Популярные объекты кешируются в Redis.
