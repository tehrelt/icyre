# Admin Service

## Назначение
Административные и модерационные операции.

## Возможности
- блокировка треков;
- блокировка пользователей;
- управление артистами;
- управление каталогом;
- просмотр ingest jobs;
- moderation audit log.

## API
```http
GET /admin/tracks
PATCH /admin/tracks/{id}
GET /admin/users
PATCH /admin/users/{id}
GET /admin/jobs
```

## Security
Доступ только ролям MODERATOR/ADMIN.
Все изменения пишутся в audit log.
