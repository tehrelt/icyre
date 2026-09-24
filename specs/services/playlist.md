# Playlist Service

## Ответственность
Создание и редактирование пользовательских плейлистов.

## Entities
```text
Playlist
PlaylistTrack
```

## API
```http
POST /playlists
GET /playlists/{id}
PATCH /playlists/{id}
DELETE /playlists/{id}
POST /playlists/{id}/tracks
DELETE /playlists/{id}/tracks/{trackId}
PATCH /playlists/{id}/tracks/order
```

## PlaylistTrack
```text
playlist_id
track_id
position
added_by
added_at
```

## Events
```text
playlist.created
playlist.updated
playlist.track_added
playlist.track_removed
```
