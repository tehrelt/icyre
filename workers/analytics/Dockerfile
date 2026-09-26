# syntax=docker/dockerfile:1
# Build context: repository root (the service depends on libs/*).
FROM golang:1.26-alpine AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOWORK=off
WORKDIR /src

# Dependency layer: only go.mod/go.sum files.
COPY libs/platform/go.mod libs/platform/go.sum libs/platform/
COPY libs/contracts/go.mod libs/contracts/go.sum libs/contracts/
COPY workers/analytics/go.mod workers/analytics/go.sum workers/analytics/
RUN --mount=type=cache,id=icyre-gomod,target=/go/pkg/mod \
    cd workers/analytics && go mod download

COPY libs/ libs/
COPY workers/analytics/ workers/analytics/
ARG VERSION=dev
RUN --mount=type=cache,id=icyre-gomod,target=/go/pkg/mod \
    --mount=type=cache,id=icyre-gobuild,target=/root/.cache/go-build \
    cd workers/analytics && go build -trimpath -ldflags="-s -w" -o /out/analytics ./cmd/analytics

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/analytics /usr/local/bin/analytics
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/analytics"]
