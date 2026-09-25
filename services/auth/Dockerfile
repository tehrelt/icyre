# syntax=docker/dockerfile:1
# Build context: repository root (the service depends on libs/*).
FROM golang:1.26-alpine AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOWORK=off
WORKDIR /src

# Dependency layer: only go.mod/go.sum files.
COPY libs/platform/go.mod libs/platform/go.sum libs/platform/
COPY libs/contracts/go.mod libs/contracts/go.sum libs/contracts/
COPY services/auth/go.mod services/auth/go.sum services/auth/
RUN cd services/auth && go mod download

COPY libs/ libs/
COPY services/auth/ services/auth/
ARG VERSION=dev
RUN cd services/auth && go build -trimpath -ldflags="-s -w" -o /out/auth ./cmd/auth

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/auth /usr/local/bin/auth
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/auth"]
