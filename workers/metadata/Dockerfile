# syntax=docker/dockerfile:1
# Build context: repository root (the service depends on libs/*).
FROM golang:1.26-alpine AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOWORK=off
WORKDIR /src

# Dependency layer: only go.mod/go.sum files.
COPY libs/platform/go.mod libs/platform/go.sum libs/platform/
COPY libs/contracts/go.mod libs/contracts/go.sum libs/contracts/
COPY workers/metadata/go.mod workers/metadata/go.sum workers/metadata/
RUN --mount=type=cache,id=icyre-gomod,target=/go/pkg/mod \
    cd workers/metadata && go mod download

COPY libs/ libs/
COPY workers/metadata/ workers/metadata/
ARG VERSION=dev
RUN --mount=type=cache,id=icyre-gomod,target=/go/pkg/mod \
    --mount=type=cache,id=icyre-gobuild,target=/root/.cache/go-build \
    cd workers/metadata && go build -trimpath -ldflags="-s -w" -o /out/metadata ./cmd/metadata

# ffprobe (alpine ffmpeg package) is a runtime dependency, so no distroless here.
FROM alpine:3.22
RUN apk add --no-cache ffmpeg \
    && adduser -D -H -u 65532 nonroot
COPY --from=build /out/metadata /usr/local/bin/metadata
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/metadata"]
