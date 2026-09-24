# ADR-0002: хранить аудио в S3 и отдавать через CDN

## Статус
Accepted.

## Контекст
Передача аудио через основной backend резко увеличивает bandwidth и количество долгоживущих соединений.

## Решение
Хранить media в S3-compatible Object Storage и использовать CDN.

Backend выдаёт только авторизацию и временный URL.

## Последствия
Плюсы:
- application backend не несёт media bandwidth;
- кеширование на edge;
- легче масштабировать streaming.

Минусы:
- появляется дополнительная инфраструктура;
- сложнее invalidation и access control.
