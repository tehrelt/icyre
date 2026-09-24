# Media Ingest Service

## Назначение
Приём новых аудиофайлов и создание задания на обработку.

## Поток
```text
Artist/Admin Portal
  -> Ingest Service
  -> S3 original
  -> Kafka track.uploaded
```

## API
```http
POST /media/uploads
POST /media/uploads/{id}/complete
GET /media/uploads/{id}
```

Предпочтительно выдавать pre-signed upload URL вместо проксирования большого файла через backend.

## Events
```text
track.uploaded
media.ingest.failed
```
