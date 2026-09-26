# Recommendation Service

Отдаёт готовые рекомендации из Redis (`specs/services/recommendation.md`); строит их
`workers/recommendation`.

```text
GET /api/v1/recommendations/home
GET /api/v1/recommendations/tracks?limit=1..50
GET /api/v1/recommendations/artists?limit=1..50
```

Токен необязателен (`authn.Optional`): гость и пользователь без персонального набора получают
`popular`; нет и его — `source: none` и пустые списки. Redis недоступен — `503 RECOMMENDATIONS_UNAVAILABLE`.

| ENV | По умолчанию |
|---|---|
| `REDIS_ADDR` | `localhost:6379` |
| `AUTH_JWKS_URL` | `http://localhost:8083/api/v1/auth/.well-known/jwks.json` |

Локально: `make run-recommendation` (:8097); в compose — `recommendation` (:8097), через gateway `/api/v1/recommendations/`.
