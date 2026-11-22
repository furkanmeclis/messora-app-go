# Messora developer tasks. Everything that needs a pinned tool runs it in
# Docker, so only Go (any version with GOTOOLCHAIN=auto), Node and Docker are
# required locally.

GO_PKGS         := ./cmd/... ./internal/... ./migrations/...
GOLANGCI_IMAGE  := golangci/golangci-lint:v2.15.0
SQLC_IMAGE      := sqlc/sqlc:1.31.1
GITLEAKS_IMAGE  := zricethezav/gitleaks:v8.30.1
GOVULNCHECK     := golang.org/x/vuln/cmd/govulncheck@v1.10.0
VERSION         ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*## "}{printf "  \033[36m%-18s\033[0m %s\n",$$1,$$2}'

## ── Backend ──────────────────────────────────────────────────────────────────
.PHONY: build
build: ## Build the messora binary into ./bin
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X github.com/furkanmeclis/messora-app-go/internal/app.Version=$(VERSION)" -o bin/messora ./cmd/messora

.PHONY: test
test: ## Run unit tests
	go test -race -count=1 $(GO_PKGS)

.PHONY: test-integration
test-integration: ## Run unit + integration tests against real PostgreSQL/Redis containers (needs Docker)
	go test -race -count=1 -tags=integration -timeout 15m $(GO_PKGS)

.PHONY: lint
lint: ## Run golangci-lint (pinned, in Docker)
	docker run --rm -v "$(CURDIR)":/src -v "$$(go env GOMODCACHE)":/go/pkg/mod -w /src -e GOTOOLCHAIN=auto -e GOFLAGS=-buildvcs=false $(GOLANGCI_IMAGE) golangci-lint run ./...

.PHONY: vuln
vuln: ## Scan Go dependencies for known vulnerabilities
	go run $(GOVULNCHECK) $(GO_PKGS)

.PHONY: sqlc
sqlc: ## Regenerate type-safe query code from queries/ and migrations/
	docker run --rm -u "$$(id -u):$$(id -g)" -v "$(CURDIR)":/src -w /src $(SQLC_IMAGE) generate

.PHONY: sqlc-check
sqlc-check: ## Fail if generated sqlc code is out of date
	docker run --rm -u "$$(id -u):$$(id -g)" -v "$(CURDIR)":/src -w /src $(SQLC_IMAGE) diff

.PHONY: fmt
fmt: ## Format Go and frontend code
	gofmt -w cmd internal migrations
	cd frontend && npm run format

## ── Frontend ─────────────────────────────────────────────────────────────────
.PHONY: web-install web-lint web-build
web-install: ## Install frontend dependencies from the lockfile
	cd frontend && npm ci --no-audit --no-fund
web-lint: ## Lint and type-check the frontend
	cd frontend && npm run lint && npm run typecheck
web-build: ## Production build of the frontend
	cd frontend && npm run build

## ── Repository hygiene ───────────────────────────────────────────────────────
.PHONY: secrets-scan
secrets-scan: ## Scan the working tree for committed secrets
	docker run --rm -v "$(CURDIR)":/repo -w /repo $(GITLEAKS_IMAGE) dir . --no-banner --redact -c /repo/.gitleaks.toml

.PHONY: check
check: lint test web-lint vuln secrets-scan ## Everything CI runs except integration tests

## ── Stack ────────────────────────────────────────────────────────────────────
.PHONY: up down logs ps
up: ## Build and start the full stack
	@test -f .env || cp .env.example .env
	docker compose up -d --build
down: ## Stop the stack (data volumes are kept)
	docker compose down
logs: ## Follow logs of all services
	docker compose logs -f --tail=100
ps: ## Show service status
	docker compose ps -a
