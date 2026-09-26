# syntax=docker/dockerfile:1
# Build context: repository root (the service depends on libs/*).
FROM golang:1.26-alpine AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOWORK=off
WORKDIR /src

# Dependency layer: only go.mod/go.sum files.
COPY libs/platform/go.mod libs/platform/go.sum libs/platform/
COPY libs/contracts/go.mod libs/contracts/go.sum libs/contracts/
COPY services/user-profile/go.mod services/user-profile/go.sum services/user-profile/
RUN --mount=type=cache,id=icyre-gomod,target=/go/pkg/mod \
    cd services/user-profile && go mod download

COPY libs/ libs/
COPY services/user-profile/ services/user-profile/
ARG VERSION=dev
RUN --mount=type=cache,id=icyre-gomod,target=/go/pkg/mod \
    --mount=type=cache,id=icyre-gobuild,target=/root/.cache/go-build \
    cd services/user-profile && go build -trimpath -ldflags="-s -w" -o /out/user-profile ./cmd/user-profile

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/user-profile /usr/local/bin/user-profile
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/user-profile"]
