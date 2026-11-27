# Messora developer tasks. Everything that needs a pinned tool runs it in
# Docker, so only Go (any version with GOTOOLCHAIN=auto), Node and Docker are
# required locally.

GO_PKGS         := ./cmd/... ./internal/... ./migrations/...
GOLANGCI_IMAGE  := golangci/golangci-lint:v2.15.0
SQLC_IMAGE      := sqlc/sqlc:1.31.1
GITLEAKS_IMAGE  := zricethezav/gitleaks:v8.30.1
GOVULNCHECK     := golang.org/x/vuln/cmd/govulncheck@v1.10.0
GO_LICENSES     := github.com/google/go-licenses/v2@v2.0.1
HADOLINT_IMAGE  := hadolint/hadolint:v2.15.1
ACTIONLINT_IMAGE:= rhysd/actionlint:1.7.12
TRIVY_IMAGE     := aquasec/trivy:0.76.2
SQUAWK          := squawk-cli@2.73.0
NPM_LICENSES    := license-checker-rseidelsohn@5.0.1
# Minimum total statement coverage (unit + integration). Raised every phase.
COVERAGE_MIN    ?= 60
NPM_ALLOWED_LICENSES := MIT;Apache-2.0;ISC;BSD-2-Clause;BSD-3-Clause;0BSD;CC-BY-4.0;BlueOak-1.0.0;OFL-1.1
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
	go test -race -count=1 -tags=integration -timeout 30m $(GO_PKGS)

.PHONY: cover
cover: ## Integration tests with race detector + coverage gate (COVERAGE_MIN)
	go test -race -count=1 -tags=integration -timeout 30m -coverpkg=./internal/... -coverprofile=coverage.out $(GO_PKGS)
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/{sub("%","",$$3); print $$3}'); \
	echo "total coverage: $$total% (minimum $(COVERAGE_MIN)%)"; \
	awk -v t="$$total" -v m="$(COVERAGE_MIN)" 'BEGIN{exit (t+0 < m+0)}' || { echo "coverage below minimum"; exit 1; }

.PHONY: tidy-check
tidy-check: ## Fail if go.mod/go.sum are not tidy
	go mod tidy -diff

.PHONY: licenses
licenses: ## Fail on dependencies with forbidden or restricted licenses
	go run $(GO_LICENSES) check ./cmd/... ./internal/... --disallowed_types=forbidden,restricted --ignore github.com/furkanmeclis/messora-app-go
	cd frontend && npx -y $(NPM_LICENSES) --production --excludePrivatePackages \
	  --excludePackagesStartingWith "@img/sharp-libvips" --onlyAllow "$(NPM_ALLOWED_LICENSES)" >/dev/null

.PHONY: migrations-lint
migrations-lint: ## Lint SQL migrations for unsafe operations (squawk)
	npx -y $(SQUAWK) migrations/*.sql

.PHONY: docker-lint
docker-lint: ## Lint Dockerfiles (hadolint)
	@for f in Dockerfile frontend/Dockerfile; do echo "hadolint $$f"; docker run --rm -i $(HADOLINT_IMAGE) hadolint --failure-threshold warning - < $$f || exit 1; done

.PHONY: actions-lint
actions-lint: ## Lint GitHub Actions workflows (actionlint)
	docker run --rm -v "$(CURDIR)":/repo -w /repo $(ACTIONLINT_IMAGE) -color

.PHONY: image-scan
image-scan: ## Scan built images for HIGH/CRITICAL vulnerabilities with a fix available
	@for img in messora/messora:$${MESSORA_VERSION:-dev} messora/messora-web:$${MESSORA_VERSION:-dev}; do \
	  echo "trivy $$img"; docker run --rm -v /var/run/docker.sock:/var/run/docker.sock -v "$(CURDIR)/.trivyignore.yaml:/.trivyignore.yaml:ro" $(TRIVY_IMAGE) image --quiet --scanners vuln --ignorefile /.trivyignore.yaml \
	    --severity HIGH,CRITICAL --ignore-unfixed --exit-code 1 $$img || exit 1; done

.PHONY: config-scan
config-scan: ## Scan Dockerfiles and Compose for misconfigurations
	docker run --rm -v "$(CURDIR)":/src $(TRIVY_IMAGE) config --quiet --severity HIGH,CRITICAL --exit-code 1 \
	  --skip-dirs /src/reference --skip-dirs /src/frontend/node_modules /src

.PHONY: lint
lint: ## Run golangci-lint (pinned, in Docker)
	@# Runs as the host user so the shared module cache never gets root-owned files.
	@# Create the cache first: Docker would create a missing bind-mount source as root.
	@mkdir -p "$$(go env GOMODCACHE)"
	docker run --rm -u "$$(id -u):$$(id -g)" -v "$(CURDIR)":/src -v "$$(go env GOMODCACHE)":/go/pkg/mod -w /src \
	  -e HOME=/tmp -e GOPATH=/tmp/gopath -e GOMODCACHE=/go/pkg/mod -e GOCACHE=/tmp/gocache -e GOLANGCI_LINT_CACHE=/tmp/lintcache -e GOTOOLCHAIN=auto -e GOFLAGS=-buildvcs=false \
	  $(GOLANGCI_IMAGE) golangci-lint run ./...

.PHONY: vuln
vuln: ## Scan Go dependencies for known vulnerabilities
	go run $(GOVULNCHECK) $(GO_PKGS)

.PHONY: sqlc
sqlc: ## Regenerate type-safe query code from queries/ and migrations/
	docker run --rm -u "$$(id -u):$$(id -g)" -v "$(CURDIR)":/src -w /src $(SQLC_IMAGE) generate

.PHONY: sqlc-check
sqlc-check: ## Fail if generated sqlc code is out of date
	docker run --rm -u "$$(id -u):$$(id -g)" -v "$(CURDIR)":/src -w /src $(SQLC_IMAGE) diff

.PHONY: api
api: ## Regenerate internal/api from api/openapi.yaml
	go tool oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml

.PHONY: api-check
api-check: ## Fail if generated API code is out of date with the spec
	@sed 's#^output: .*#output: /tmp/messora-api.gen.go#' api/oapi-codegen.yaml > /tmp/messora-oapi.yaml
	@go tool oapi-codegen -config /tmp/messora-oapi.yaml api/openapi.yaml
	@cmp -s /tmp/messora-api.gen.go internal/api/api.gen.go || { echo "internal/api is stale: run make api"; exit 1; }

.PHONY: api-docs
api-docs: ## Regenerate docs/API.md (public endpoint inventory) and bin/openapi.public.json
	go run ./cmd/apidocs -md docs/API.md -json bin/openapi.public.json -version $(VERSION)

.PHONY: api-docs-check
api-docs-check: ## Fail if a public operation is undocumented or docs/API.md is stale
	go run ./cmd/apidocs -md docs/API.md -check

.PHONY: mail-preview
mail-preview: ## Render every e-mail template (TR/EN) into tmp/mail-preview with an index.html gallery
	go run ./cmd/mailpreview -out tmp/mail-preview

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
check: tidy-check api-check api-docs-check lint test web-lint vuln licenses migrations-lint docker-lint actions-lint secrets-scan sqlc-check ## Fast local gate (no containers started)

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
