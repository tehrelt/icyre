# Listening History Worker

## Назначение
Формирование пользовательской истории прослушивания.

## Consumes
```text
playback.started
playback.finished
playback.skipped
```

## Логика завершённого прослушивания
Пример правила:
- минимум 30 секунд;
- либо >= 50% трека.

Порог является бизнес-конфигурацией.

## Storage
PostgreSQL для персональной истории.
