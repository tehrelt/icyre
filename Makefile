# ICYRE monorepo tasks. Go modules are listed in go.work; the frontend is a
# separate Bun workspace (package.json). Run `make help` for the list.

GO_MODULES := $(shell go work edit -json | sed -n 's/.*"DiskPath": "\(.*\)".*/\1/p')
PG_TEST_DSN ?= postgres://icyre:icyre@localhost:5432/icyre?sslmode=disable
KAFKA_TEST_BROKERS ?= localhost:9094
OPENSEARCH_TEST_URL ?= http://localhost:9200

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# --- Go ---------------------------------------------------------------------

.PHONY: sync
sync: ## go work sync
	go work sync

.PHONY: tidy
tidy: ## go mod tidy in every module (standalone, as Docker builds them)
	@for m in $(GO_MODULES); do echo "==> tidy $$m"; (cd $$m && GOWORK=off go mod tidy) || exit 1; done

.PHONY: mod-check
mod-check: ## Fail if any go.mod/go.sum is not tidy on its own (Docker builds use GOWORK=off)
	@for m in $(GO_MODULES); do (cd $$m && GOWORK=off go mod tidy -diff > /dev/null) || { echo "go.mod/go.sum of $$m is not tidy: run make tidy"; exit 1; }; done

.PHONY: fmt
fmt: ## gofmt every module
	@for m in $(GO_MODULES); do echo "==> fmt $$m"; (cd $$m && go fmt ./...) || exit 1; done

.PHONY: fmt-check
fmt-check: ## Fail on Go files that gofmt would change (make fmt fixes them)
	@out=$$(gofmt -l $(GO_MODULES)); if [ -n "$$out" ]; then echo "not gofmt-ed (run make fmt):"; echo "$$out"; exit 1; fi

.PHONY: vet
vet: ## go vet every module
	@for m in $(GO_MODULES); do echo "==> vet $$m"; (cd $$m && go vet ./...) || exit 1; done

.PHONY: test
test: ## Unit tests in every module
	@for m in $(GO_MODULES); do echo "==> test $$m"; (cd $$m && go test ./...) || exit 1; done

.PHONY: test-race
test-race: ## Unit tests with the race detector
	@for m in $(GO_MODULES); do echo "==> test -race $$m"; (cd $$m && go test -race ./...) || exit 1; done

.PHONY: test-integration
test-integration: ## Integration tests (needs `make up-core`)
	cd services/catalog && CATALOG_TEST_DATABASE_DSN="$(PG_TEST_DSN)" go test -tags integration -count=1 ./...
	cd services/auth && AUTH_TEST_DATABASE_DSN="$(PG_TEST_DSN)" go test -tags integration -count=1 ./...
	cd services/user-profile && PROFILE_TEST_DATABASE_DSN="$(PG_TEST_DSN)" go test -tags integration -count=1 ./...
	cd services/library && LIBRARY_TEST_DATABASE_DSN="$(PG_TEST_DSN)" go test -tags integration -count=1 ./...
	cd services/history && HISTORY_TEST_DATABASE_DSN="$(PG_TEST_DSN)" go test -tags integration -count=1 ./...
	cd services/playlist && PLAYLIST_TEST_DATABASE_DSN="$(PG_TEST_DSN)" go test -tags integration -count=1 ./...
	cd services/media-ingest && MEDIA_TEST_DATABASE_DSN="$(PG_TEST_DSN)" go test -tags integration -count=1 ./...
	cd workers/search-indexer && OPENSEARCH_URL=$(OPENSEARCH_TEST_URL) go test -tags integration -count=1 ./...
	cd services/search && OPENSEARCH_URL=$(OPENSEARCH_TEST_URL) go test -tags integration -count=1 ./...
	cd libs/platform && OUTBOX_TEST_DATABASE_DSN="$(PG_TEST_DSN)" go test -tags integration -count=1 ./outbox/...
	cd libs/platform && KAFKA_BROKERS="$(KAFKA_TEST_BROKERS)" REDIS_ADDR=localhost:6379 \
		S3_ENDPOINT=localhost:9000 S3_ACCESS_KEY=icyre S3_SECRET_KEY=icyre-secret \
		go test -tags integration -count=1 ./kafka/... ./redis/... ./objectstore/...

.PHONY: lint
lint: ## golangci-lint every module
	@for m in $(GO_MODULES); do echo "==> lint $$m"; (cd $$m && golangci-lint run ./...) || exit 1; done

.PHONY: build
build: ## Build service binaries into ./bin
	@mkdir -p bin
	cd services/catalog && go build -o ../../bin/catalog ./cmd/catalog
	cd services/bff && go build -o ../../bin/bff ./cmd/bff
	cd services/auth && go build -o ../../bin/auth ./cmd/auth
	cd services/user-profile && go build -o ../../bin/user-profile ./cmd/user-profile
	cd services/stream-auth && go build -o ../../bin/stream-auth ./cmd/stream-auth
	cd services/search && go build -o ../../bin/search ./cmd/search
	cd services/library && go build -o ../../bin/library ./cmd/library
	cd services/playback && go build -o ../../bin/playback ./cmd/playback
	cd services/history && go build -o ../../bin/history ./cmd/history
	cd services/playlist && go build -o ../../bin/playlist ./cmd/playlist
	cd services/media-ingest && go build -o ../../bin/media-ingest ./cmd/media-ingest
	cd workers/search-indexer && go build -o ../../bin/search-indexer ./cmd/search-indexer
	cd workers/transcoder && go build -o ../../bin/transcoder ./cmd/transcoder

.PHONY: migrate
migrate: ## Apply all service migrations to the local database
	cd services/catalog && DATABASE_URL="$(PG_TEST_DSN)" KAFKA_ENABLED=false go run ./cmd/catalog migrate
	cd services/auth && DATABASE_URL="$(PG_TEST_DSN)" KAFKA_ENABLED=false go run ./cmd/auth migrate
	cd services/user-profile && DATABASE_URL="$(PG_TEST_DSN)" KAFKA_ENABLED=false go run ./cmd/user-profile migrate
	cd services/library && DATABASE_URL="$(PG_TEST_DSN)" KAFKA_ENABLED=false go run ./cmd/library migrate
	cd services/history && DATABASE_URL="$(PG_TEST_DSN)" KAFKA_ENABLED=false go run ./cmd/history migrate
	cd services/playlist && DATABASE_URL="$(PG_TEST_DSN)" go run ./cmd/playlist migrate
	cd services/media-ingest && DATABASE_URL="$(PG_TEST_DSN)" KAFKA_ENABLED=false S3_ACCESS_KEY=icyre S3_SECRET_KEY=icyre-secret go run ./cmd/media-ingest migrate

.PHONY: seed
seed: ## Fill Catalog with the product-canvas content (needs a running Catalog)
	bun scripts/seed-catalog.ts http://localhost:8081

.PHONY: seed-media
seed-media: ## Generate audio variants for Catalog tracks into MinIO (needs ffmpeg)
	bun scripts/seed-media.ts --catalog http://localhost:8081

.PHONY: run-bff
run-bff: ## Run the Web BFF locally against Catalog on :8081
	cd services/bff && CATALOG_URL=http://localhost:8081 HTTP_ADDR=:8082 LOG_FORMAT=text go run ./cmd/bff

.PHONY: run-catalog
run-catalog: ## Run Catalog locally against `make up-core`
	cd services/catalog && DATABASE_URL="$(PG_TEST_DSN)" KAFKA_BROKERS="$(KAFKA_TEST_BROKERS)" LOG_FORMAT=text go run ./cmd/catalog

.PHONY: run-auth
run-auth: ## Run Auth locally on :8083 (ephemeral signing key)
	cd services/auth && DATABASE_URL="$(PG_TEST_DSN)" KAFKA_BROKERS="$(KAFKA_TEST_BROKERS)" HTTP_ADDR=:8083 LOG_FORMAT=text go run ./cmd/auth

.PHONY: run-user-profile
run-user-profile: ## Run User Profile locally on :8084 against Auth on :8083
	cd services/user-profile && DATABASE_URL="$(PG_TEST_DSN)" KAFKA_BROKERS="$(KAFKA_TEST_BROKERS)" HTTP_ADDR=:8084 LOG_FORMAT=text go run ./cmd/user-profile

.PHONY: run-stream-auth
run-stream-auth: ## Run Stream Authorization locally on :8085 (Catalog :8081, Auth :8083)
	cd services/stream-auth && S3_ACCESS_KEY=icyre S3_SECRET_KEY=icyre-secret HTTP_ADDR=:8085 LOG_FORMAT=text go run ./cmd/stream-auth

.PHONY: run-library
run-library: ## Run Library locally on :8088 (Catalog :8081, Auth :8083)
	cd services/library && DATABASE_URL="$(PG_TEST_DSN)" KAFKA_BROKERS="$(KAFKA_TEST_BROKERS)" HTTP_ADDR=:8088 LOG_FORMAT=text go run ./cmd/library

.PHONY: run-media-ingest
run-media-ingest: ## Run Media Ingest locally on :8092 (Catalog :8081, Auth :8083, MinIO :9000)
	cd services/media-ingest && DATABASE_URL="$(PG_TEST_DSN)" KAFKA_BROKERS="$(KAFKA_TEST_BROKERS)" S3_ACCESS_KEY=icyre S3_SECRET_KEY=icyre-secret HTTP_ADDR=:8092 LOG_FORMAT=text go run ./cmd/media-ingest

.PHONY: run-transcoder
run-transcoder: ## Run the Transcoder locally (consumes media.events; needs ffmpeg, MinIO :9000)
	cd workers/transcoder && KAFKA_BROKERS="$(KAFKA_TEST_BROKERS)" S3_ACCESS_KEY=icyre S3_SECRET_KEY=icyre-secret HTTP_ADDR=:8093 LOG_FORMAT=text go run ./cmd/transcoder

.PHONY: run-search-indexer reindex run-search
run-search-indexer: ## Run the Search Indexer locally (consumes catalog.events)
	cd workers/search-indexer && HTTP_ADDR=:8087 LOG_FORMAT=text go run ./cmd/search-indexer
reindex: ## Rebuild the search indices from Catalog (new versioned indices, alias swap)
	cd workers/search-indexer && LOG_FORMAT=text go run ./cmd/search-indexer reindex
run-search: ## Run the Search Service locally on :8086
	cd services/search && HTTP_ADDR=:8086 LOG_FORMAT=text go run ./cmd/search

# --- Frontend (Bun) ---------------------------------------------------------

.PHONY: web-install web-dev web-build web-test web-lint scripts-typecheck
web-install: ## bun install
	bun install
web-dev: ## Vite dev server for apps/web
	bun run --filter @icyre/web dev
web-build: ## Typecheck + build apps/web
	bun run build
web-test: ## Vitest
	bun run test
web-lint: ## ESLint
	bun run lint
scripts-typecheck: ## Typecheck scripts/*.ts
	bun run typecheck:scripts

# --- Docker -----------------------------------------------------------------

.PHONY: up up-core down logs images
images: ## Build the compose images 4 at a time (COMPOSE_BUILD_BATCH=n to change)
	bun scripts/compose-build.ts
up: images ## docker compose up -d (everything), images built in batches
	docker compose up -d
up-core: ## Infrastructure only: PostgreSQL, Redis, Kafka, MinIO, OpenSearch (for go run / integration tests)
	docker compose up -d postgres redis kafka kafka-init minio minio-init opensearch
down: ## Stop the stand
	docker compose down
logs: ## Follow logs
	docker compose logs -f

# --- Everything -------------------------------------------------------------

.PHONY: check
check: sync mod-check fmt-check vet test web-lint web-build web-test scripts-typecheck ## Full local verification
