# Поток воспроизведения аудио

## Главный принцип

Backend не должен проксировать каждый байт аудио через Go-сервис.

## Поток

```text
React Player
   |
   | GET /tracks/{id}/play
   v
Playback Service
   |
   v
Stream Authorization Service
   |
   | signed URL
   v
React Player
   |
   | HTTP Range
   v
CDN
   |
   v
S3 / MinIO
```

## Шаги

1. Пользователь нажимает Play.
2. Playback Service создаёт или продолжает playback session.
3. Stream Authorization проверяет доступ к треку.
4. Сервис определяет доступный media variant.
5. Генерируется временный URL.
6. Клиент использует URL для прямой загрузки через CDN.
7. Проигрыватель использует HTTP Range для перемотки.
8. Клиент периодически отправляет события воспроизведения.

## Форматы

Для MVP:

- AAC
- MP3

Для развития:

- Opus
- HLS/DASH для адаптивного битрейта

## Варианты качества

Пример:

```text
64 kbps
128 kbps
256 kbps
```

## Object key

```text
tracks/{track_id}/audio/64.aac
tracks/{track_id}/audio/128.aac
tracks/{track_id}/audio/256.aac
```

## Безопасность URL

Signed URL должен иметь небольшой TTL, например 1–10 минут.

Он не должен содержать пользовательские секреты.
