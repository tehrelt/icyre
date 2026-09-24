# Web BFF

## Назначение
Агрегировать ответы нескольких внутренних сервисов для Web/PWA клиента.

## Ответственность
- page-oriented API;
- агрегация;
- преобразование DTO;
- fan-out внутренних запросов;
- user context propagation.

## Пример
```http
GET /api/v1/pages/home
GET /api/v1/pages/albums/{id}
GET /api/v1/pages/artists/{id}
```

## Ограничения
BFF не владеет доменными данными и не должен становиться вторым монолитом.
