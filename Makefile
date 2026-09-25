# ICYRE monorepo tasks. Go modules are listed in go.work; the frontend is a
# separate Bun workspace (package.json). Run `make help` for the list.

GO_MODULES := $(shell go work edit -json | sed -n 's/.*"DiskPath": "\(.*\)".*/\1/p')
PG_TEST_DSN ?= postgres://icyre:icyre@localhost:5432/icyre?sslmode=disable
KAFKA_TEST_BROKERS ?= localhost:9094

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# --- Go ---------------------------------------------------------------------

.PHONY: sync
sync: ## go work sync
	go work sync

.PHONY: tidy
tidy: ## go mod tidy in every module
	@for m in $(GO_MODULES); do echo "==> tidy $$m"; (cd $$m && go mod tidy) || exit 1; done

.PHONY: fmt
fmt: ## gofmt every module
	@for m in $(GO_MODULES); do echo "==> fmt $$m"; (cd $$m && go fmt ./...) || exit 1; done

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
	cd libs/platform && KAFKA_BROKERS="$(KAFKA_TEST_BROKERS)" REDIS_ADDR=localhost:6379 go test -tags integration -count=1 ./kafka/... ./redis/...

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

.PHONY: migrate
migrate: ## Apply all service migrations to the local database
	cd services/catalog && DATABASE_URL="$(PG_TEST_DSN)" KAFKA_ENABLED=false go run ./cmd/catalog migrate
	cd services/auth && DATABASE_URL="$(PG_TEST_DSN)" KAFKA_ENABLED=false go run ./cmd/auth migrate
	cd services/user-profile && DATABASE_URL="$(PG_TEST_DSN)" KAFKA_ENABLED=false go run ./cmd/user-profile migrate

.PHONY: seed
seed: ## Fill Catalog with the product-canvas content (needs a running Catalog)
	bun scripts/seed-catalog.ts http://localhost:8081

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

# --- Frontend (Bun) ---------------------------------------------------------

.PHONY: web-install web-dev web-build web-test web-lint
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

# --- Docker -----------------------------------------------------------------

.PHONY: up up-core down logs
up: ## docker compose up -d (everything)
	docker compose up -d --build
up-core: ## Only PostgreSQL + Redis + Kafka (for local go run / integration tests)
	docker compose up -d postgres redis kafka kafka-init
down: ## Stop the stand
	docker compose down
logs: ## Follow logs
	docker compose logs -f

# --- Everything -------------------------------------------------------------

.PHONY: check
check: sync fmt vet test web-lint web-build web-test ## Full local verification
