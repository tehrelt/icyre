# Поток пользовательского запроса

## Пример: открытие страницы альбома

```text
React
  |
  | GET /api/v1/pages/albums/{id}
  v
API Gateway
  |
  v
BFF
  |
  +--> Catalog Service
  +--> Library Service
  +--> Playback Service
  |
  v
JSON Response
```

## Последовательность

1. Клиент отправляет HTTPS-запрос.
2. Gateway создаёт или прокидывает `X-Request-ID`.
3. Gateway проверяет базовые правила rate limiting.
4. BFF извлекает user context из JWT.
5. BFF вызывает необходимые доменные сервисы.
6. Сервисы читают данные из Redis или PostgreSQL.
7. BFF агрегирует ответ.
8. Клиент получает DTO, специально оптимизированный под страницу.

## Почему нужен BFF

Без BFF frontend вынужден знать внутреннюю топологию сервисов.

BFF:
- уменьшает количество round-trip;
- скрывает внутреннюю структуру;
- даёт возможность независимо менять backend;
- формирует UI-oriented DTO.

## Таймауты

Рекомендуемые базовые значения:

```text
Gateway request timeout: 10s
Internal unary gRPC: 1-3s
Redis: 50-200ms
PostgreSQL query: <500ms target
OpenSearch: <1s target
```

Точные значения должны задаваться конфигурацией.
