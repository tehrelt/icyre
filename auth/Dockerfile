FROM golang:1.22.0-alpine3.19 as builder

RUN apk update --no-cache
WORKDIR /app
COPY . /app
RUN go clean --modcache
RUN go build -mod=readonly -o migrator cmd/migrator/main.go &&\
  go build -mod=readonly -o app cmd/app/main.go

FROM alpine

RUN apk update --no-cache
WORKDIR /app
COPY --from=builder /app /app

ADD https://github.com/ufoscout/docker-compose-wait/releases/download/2.12.1/wait /app/wait
RUN chmod +x /app/wait

CMD  ./wait && ./migrator && ./app
