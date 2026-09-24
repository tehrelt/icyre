# Error model

## Формат

```json
{
  "error": {
    "code": "TRACK_NOT_FOUND",
    "message": "Track not found",
    "requestId": "uuid",
    "details": {}
  }
}
```

## Типовые статусы

| HTTP | Значение |
|---|---|
| 400 | malformed request |
| 401 | unauthenticated |
| 403 | forbidden |
| 404 | not found |
| 409 | conflict |
| 422 | validation/business rule |
| 429 | rate limited |
| 500 | internal |
| 503 | temporary unavailable |

## Правила

`code` стабилен и предназначен для программной обработки.

`message` предназначен для человека и может меняться.

Внутренние stack trace клиенту не возвращаются.
