# syntax=docker/dockerfile:1
# Build context: repository root (the service depends on libs/*).
FROM golang:1.26-alpine AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOWORK=off
WORKDIR /src

# Dependency layer: only go.mod/go.sum files.
COPY libs/platform/go.mod libs/platform/go.sum libs/platform/
COPY libs/contracts/go.mod libs/contracts/go.sum libs/contracts/
COPY workers/search-indexer/go.mod workers/search-indexer/go.sum workers/search-indexer/
RUN cd workers/search-indexer && go mod download

COPY libs/ libs/
COPY workers/search-indexer/ workers/search-indexer/
ARG VERSION=dev
RUN cd workers/search-indexer && go build -trimpath -ldflags="-s -w" -o /out/search-indexer ./cmd/search-indexer

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/search-indexer /usr/local/bin/search-indexer
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/search-indexer"]
