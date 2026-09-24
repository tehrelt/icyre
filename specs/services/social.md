# Social Service

## Ответственность
Социальные связи внутри системы.

## Возможности
- follow artist;
- follow user;
- follower/following counters;
- activity feed foundation.

## API
```http
PUT /users/{id}/follow
DELETE /users/{id}/follow
GET /users/{id}/followers
GET /users/{id}/following
```

## Events
```text
social.followed
social.unfollowed
```

## Scale note
Для MVP PostgreSQL достаточно.
Для очень большого social graph позже возможен отдельный storage/design.
