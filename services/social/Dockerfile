# syntax=docker/dockerfile:1
# Build context: repository root (the service depends on libs/*).
FROM golang:1.26-alpine AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOWORK=off
WORKDIR /src

# Dependency layer: only go.mod/go.sum files.
COPY libs/platform/go.mod libs/platform/go.sum libs/platform/
COPY libs/contracts/go.mod libs/contracts/go.sum libs/contracts/
COPY services/social/go.mod services/social/go.sum services/social/
RUN --mount=type=cache,id=icyre-gomod,target=/go/pkg/mod \
    cd services/social && go mod download

COPY libs/ libs/
COPY services/social/ services/social/
ARG VERSION=dev
RUN --mount=type=cache,id=icyre-gomod,target=/go/pkg/mod \
    --mount=type=cache,id=icyre-gobuild,target=/root/.cache/go-build \
    cd services/social && go build -trimpath -ldflags="-s -w" -o /out/social ./cmd/social

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/social /usr/local/bin/social
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/social"]
