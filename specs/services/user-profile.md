# User Profile Service

## Ответственность
- username;
- display name;
- avatar;
- bio;
- locale;
- user settings.

## Entity
```text
id
username
display_name
avatar_key
bio
country
language
created_at
updated_at
```

## API
```http
GET /users/{id}
GET /users/me
PATCH /users/me
```

## Events
```text
profile.updated
avatar.changed
```
