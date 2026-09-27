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
PUT    /users/{id}/follow            # Bearer, идемпотентно, 204
DELETE /users/{id}/follow            # Bearer, идемпотентно, 204
PUT    /artists/{id}/follow
DELETE /artists/{id}/follow
GET    /users/{id}/followers         # публично, keyset-пагинация
GET    /artists/{id}/followers
GET    /users/{id}/following?type=user|artist
GET    /users/{id}/follow-counts     # {followers, followingUsers, followingArtists}
GET    /artists/{id}/follow-counts   # {followers}
GET    /me/following/contains?type=artist&ids=…   # Bearer, для кнопок Follow
```

## Events
```text
social.followed    {followerId, targetType, targetId, followedAt}
social.unfollowed  {followerId, targetType, targetId, unfollowedAt}
```
Topic `social.events`, key = follower ID; transactional outbox.

## Scale note
Для MVP PostgreSQL достаточно.
Для очень большого social graph позже возможен отдельный storage/design.
