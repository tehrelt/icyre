# syntax=docker/dockerfile:1
# Build context: repository root (the service depends on libs/*).
FROM golang:1.26-alpine AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOWORK=off
WORKDIR /src

# Dependency layer: only go.mod/go.sum files.
COPY libs/platform/go.mod libs/platform/go.sum libs/platform/
COPY libs/contracts/go.mod libs/contracts/go.sum libs/contracts/
COPY workers/recommendation/go.mod workers/recommendation/go.sum workers/recommendation/
RUN --mount=type=cache,id=icyre-gomod,target=/go/pkg/mod \
    cd workers/recommendation && go mod download

COPY libs/ libs/
COPY workers/recommendation/ workers/recommendation/
ARG VERSION=dev
RUN --mount=type=cache,id=icyre-gomod,target=/go/pkg/mod \
    --mount=type=cache,id=icyre-gobuild,target=/root/.cache/go-build \
    cd workers/recommendation && go build -trimpath -ldflags="-s -w" -o /out/recommendation-worker ./cmd/recommendation-worker

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/recommendation-worker /usr/local/bin/recommendation-worker
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/recommendation-worker"]
