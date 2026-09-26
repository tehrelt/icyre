# syntax=docker/dockerfile:1
# Build context: repository root (the service depends on libs/*).
FROM golang:1.26-alpine AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOWORK=off
WORKDIR /src

# Dependency layer: only go.mod/go.sum files.
COPY libs/platform/go.mod libs/platform/go.sum libs/platform/
COPY libs/contracts/go.mod libs/contracts/go.sum libs/contracts/
COPY services/history/go.mod services/history/go.sum services/history/
RUN --mount=type=cache,id=icyre-gomod,target=/go/pkg/mod \
    cd services/history && go mod download

COPY libs/ libs/
COPY services/history/ services/history/
ARG VERSION=dev
RUN --mount=type=cache,id=icyre-gomod,target=/go/pkg/mod \
    --mount=type=cache,id=icyre-gobuild,target=/root/.cache/go-build \
    cd services/history && go build -trimpath -ldflags="-s -w" -o /out/history ./cmd/history

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/history /usr/local/bin/history
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/history"]
