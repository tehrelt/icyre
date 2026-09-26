# syntax=docker/dockerfile:1
# Build context: repository root (the service depends on libs/*).
FROM golang:1.26-alpine AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOWORK=off
WORKDIR /src

# Dependency layer: only go.mod/go.sum files.
COPY libs/platform/go.mod libs/platform/go.sum libs/platform/
COPY services/bff/go.mod services/bff/go.sum services/bff/
RUN --mount=type=cache,id=icyre-gomod,target=/go/pkg/mod \
    cd services/bff && go mod download

COPY libs/ libs/
COPY services/bff/ services/bff/
ARG VERSION=dev
RUN --mount=type=cache,id=icyre-gomod,target=/go/pkg/mod \
    --mount=type=cache,id=icyre-gobuild,target=/root/.cache/go-build \
    cd services/bff && go build -trimpath -ldflags="-s -w" -o /out/bff ./cmd/bff

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/bff /usr/local/bin/bff
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/bff"]
