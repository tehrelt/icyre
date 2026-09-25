# syntax=docker/dockerfile:1
# Build context: repository root (the service depends on libs/*).
FROM golang:1.26-alpine AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOWORK=off
WORKDIR /src

# Dependency layer: only go.mod/go.sum files.
COPY libs/platform/go.mod libs/platform/go.sum libs/platform/
COPY libs/contracts/go.mod libs/contracts/go.sum libs/contracts/
COPY services/playback/go.mod services/playback/go.sum services/playback/
RUN cd services/playback && go mod download

COPY libs/ libs/
COPY services/playback/ services/playback/
ARG VERSION=dev
RUN cd services/playback && go build -trimpath -ldflags="-s -w" -o /out/playback ./cmd/playback

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/playback /usr/local/bin/playback
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/playback"]
