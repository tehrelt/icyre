# Media Origin Service

## Назначение
Origin для CDN и точка доступа к media variant metadata.

## Ответственность
- resolve object key;
- проверки готовности;
- origin authentication;
- корректные headers;
- HTTP Range support при необходимости.

## Главное правило
Media Origin не должен выполнять тяжёлую бизнес-логику на каждый сегмент/байт.

## Cache headers
Настраиваются так, чтобы immutable media variants хорошо кешировались CDN.
