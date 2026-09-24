# Library Service

## Ответственность
Сохранённые треки, альбомы и пользовательская музыкальная библиотека.

## API
```http
PUT /me/library/tracks/{id}
DELETE /me/library/tracks/{id}
GET /me/library/tracks

PUT /me/library/albums/{id}
DELETE /me/library/albums/{id}
```

## Data model
```text
user_id
entity_type
entity_id
created_at
```

## Events
```text
library.track_saved
library.track_removed
```
