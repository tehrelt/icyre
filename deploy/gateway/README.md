# API Gateway (nginx)

- `nginx.conf` — локальный стенд (`:8080`), CORS для `localhost` / `127.0.0.1` и опциональных `/etc/nginx/cors-origins*.conf`.
- `routes.conf` — общие upstream-маршруты, `X-Request-ID`, rate limits и error model.
- `nginx.prod.conf` + `compose.prod.yml` — production-конфигурация с TLS 1.2/1.3 (`:8443`), HSTS, редиректом HTTP (`:8080`) → HTTPS и явным списком CORS origins.

## Production TLS

Положите `tls.crt` и `tls.key` в каталог сертификатов (например, `deploy/gateway/certs`, не коммитится в Git) и запустите:

```bash
TLS_CERT_DIR=./deploy/gateway/certs \
CORS_ORIGINS_FILE=./deploy/gateway/cors-origins.example.conf \
docker compose -f docker-compose.yml -f deploy/gateway/compose.prod.yml up -d
```

Для локальной проверки TLS можно сгенерировать самоподписанный сертификат:

```bash
mkdir -p deploy/gateway/certs
openssl req -x509 -newkey rsa:2048 -nodes -days 30 \
  -keyout deploy/gateway/certs/tls.key \
  -out deploy/gateway/certs/tls.crt \
  -subj "/CN=localhost"
```
