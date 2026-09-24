# Notification Service

## Ответственность
Оркестрация уведомлений.

## Каналы
- push;
- email;
- in-app.

## Events
Сервис подписывается на Kafka-события, например:
```text
artist.release_published
playlist.followed
security.new_login
```

## Providers
Внешние SMTP/push providers должны быть скрыты адаптерами.

## Reliability
Уведомление не является причиной отката основной пользовательской операции.
