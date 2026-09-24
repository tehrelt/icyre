# Stream Authorization Service

## Назначение
Проверить право пользователя на воспроизведение и выдать временный media URL.

## API
```http
POST /stream/authorize
```

## Input
```json
{
  "trackId": "...",
  "quality": "256"
}
```

## Output
```json
{
  "url": "https://cdn.example/...",
  "expiresAt": "..."
}
```

## Проверки
- track READY;
- пользователь не заблокирован;
- региональные ограничения при наличии;
- подписка при наличии premium-модели.

## Security
URL должен быть short-lived и scoped.
