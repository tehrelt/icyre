# syntax=docker/dockerfile:1
# Build context: repository root (the service depends on libs/*).
FROM golang:1.26-alpine AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOWORK=off
WORKDIR /src

# Dependency layer: only go.mod/go.sum files.
COPY libs/platform/go.mod libs/platform/go.sum libs/platform/
COPY libs/contracts/go.mod libs/contracts/go.sum libs/contracts/
COPY services/stream-auth/go.mod services/stream-auth/go.sum services/stream-auth/
RUN cd services/stream-auth && go mod download

COPY libs/ libs/
COPY services/stream-auth/ services/stream-auth/
ARG VERSION=dev
RUN cd services/stream-auth && go build -trimpath -ldflags="-s -w" -o /out/stream-auth ./cmd/stream-auth

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/stream-auth /usr/local/bin/stream-auth
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/stream-auth"]
