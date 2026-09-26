# Recommendation Service

## Ответственность
Выдача подготовленных персональных рекомендаций (`services/recommendation`). Сервис ничего не считает:
читает наборы, которые строит Recommendation Worker.

## API
```http
GET /api/v1/recommendations/home              20 треков + 10 артистов
GET /api/v1/recommendations/tracks?limit=20   1..50
GET /api/v1/recommendations/artists?limit=20  1..50
```

Токен необязателен: гость получает `popular`. Ответ:

```json
{
  "source": "personal",
  "algorithm": "mvp-content-v1",
  "generatedAt": "2026-09-27T12:00:00Z",
  "tracks": [{ "id": "…", "score": 0.55, "reasons": ["artist", "fresh"] }],
  "artists": [{ "id": "…", "score": 0.55, "reasons": ["artist"] }]
}
```

`source`: `personal` — набор пользователя; `popular` — fallback; `none` — наборы ещё не построены.
Ответы `Cache-Control: private, no-store`. Детали треков и артистов подтягивает клиент или BFF по ID.

## Источник
Recommendation Worker строит рекомендации асинхронно (`specs/workers/recommendation-worker.md`).

## Cache
Готовые подборки хранятся в Redis:

```text
recommendations:user:{id}
recommendations:popular
```

## Fallback
Если персонального набора нет (новый пользователь, гость, истёк TTL) — `popular`
(популярность за 30 дней + свежесть релиза). Жанровые и редакционные подборки — после появления
их источника (редакционные плейлисты, EPIC-031).
