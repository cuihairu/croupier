BINDIR := bin
VERSION := $(shell cat VERSION 2>/dev/null || echo "0.1.0")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_DIRTY := $(shell git diff --quiet || echo "-dirty")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
FULL_VERSION := $(VERSION)$(GIT_DIRTY)
LDFLAGS := -X main.version=$(FULL_VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT) -s -w

.PHONY: proto sync-proto api build server agent edge cli clean dev tidy test lint help all tools schema-validator pack-builder
.PHONY: test test-coverage test-coverage-html test-race test-integration test-all
.PHONY: build-sdks build-sdks-cpp build-sdks-go build-sdks-java build-sdks-js build-sdks-python
.PHONY: build-web build-dashboard build-website dev-dashboard dev-website
.PHONY: version version-sync
.PHONY: clone-sdks clone-dashboard clone-all

# Build all components (server + sdks + web)
all: build build-sdks build-web

# ========== Clone External Dependencies ==========
# Clone SDK repositories (used instead of git submodules)
SDK_REPOS := croupier-sdk-cpp croupier-sdk-go croupier-sdk-java croupier-sdk-js croupier-sdk-python croupier-sdk-csharp
SDK_BASE_URL := git@github.com:cuihairu

clone-sdks:
	@echo "[clone] cloning SDK repositories..."
	@for repo in $(SDK_REPOS); do \
		if [ ! -d "sdks/$$repo" ]; then \
			echo "[clone] $$repo"; \
			git clone --depth 1 $(SDK_BASE_URL)/$$repo.git sdks/$$repo; \
		else \
			echo "[skip] $$repo already exists"; \
		fi \
	done

# Clone dashboard repository
clone-dashboard:
	@echo "[clone] cloning dashboard..."
	@if [ ! -d "dashboard" ]; then \
		echo "[clone] croupier-dashboard"; \
		git clone --depth 1 $(SDK_BASE_URL)/croupier-dashboard.git dashboard; \
	else \
		echo "[skip] dashboard already exists"; \
	fi

# Clone all external dependencies
clone-all: clone-sdks clone-dashboard
	@echo "[done] all dependencies cloned"

# Sync proto files from croupier-proto
sync-proto:
	@echo "[sync] updating proto/ from croupier-proto..."
	@cd proto && git fetch origin && \
		if git rev-parse --abbrev-ref HEAD > /dev/null 2>&1; then \
			git pull origin $$(git rev-parse --abbrev-ref HEAD); \
		else \
			git checkout main && git pull origin main; \
		fi
	@echo "[sync] proto files updated"

# Ensure local protoc plugin exists before running buf
proto: croupier-plugin
	@echo "[proto] generating code via buf..."
	buf generate proto --template buf.gen.yaml --clean

# Generate API code from .api files
api:
	@echo "[api] generating API code via goctl..."
	@PATH=$$PATH:~/go/bin which goctl > /dev/null || (echo "Error: goctl not found. Please install goctl: go install github.com/zeromicro/go-zero/tools/goctl@latest" && exit 1)
	@cd services/server && PATH=$$PATH:~/go/bin goctl api go -api server.api -dir . -style go_zero
	@cd services/agent && PATH=$$PATH:~/go/bin goctl api go -api agent.api -dir . -style go_zero
	@cd services/edge && PATH=$$PATH:~/go/bin goctl api go -api edge.api -dir . -style go_zero
	@echo "[api] code generation complete"

# Build local protoc plugin for pack generation
.PHONY: croupier-plugin
croupier-plugin:
	@echo "[build] protoc-gen-croupier"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -o $(BINDIR)/protoc-gen-croupier ./tools/protoc-gen-croupier

# Generate croupier pack artifacts (requires protoc on PATH)
.PHONY: pack
pack: croupier-plugin
	@echo "[pack] generating croupier artifacts with protoc-gen-croupier..."
	PATH="$(PWD)/$(BINDIR):$$PATH" \
	protoc \
		-I proto \
		--croupier_out=emit_pack=true:gen/croupier \
		$(shell find proto -name "*.proto" | tr '\n' ' ')

.PHONY: pack-local
pack-local:
	@"$(PWD)/scripts/generate-pack.sh"

.PHONY: packs-build
packs-build:
	@echo "[packs] building example packs..."
	@mkdir -p packs/dist
	@tar -czf packs/dist/prom.pack.tgz -C packs/prom .
	@tar -czf packs/dist/http.pack.tgz -C packs/http .
	@tar -czf packs/dist/player.pack.tgz -C packs/player .
	@tar -czf packs/dist/alertmanager.pack.tgz -C packs/alertmanager .
	@tar -czf packs/dist/grafana.pack.tgz -C packs/grafana .
	@echo "done: packs/dist/*.pack.tgz"

server: api
	@echo "[build] server (pg+sqlite)"
	@mkdir -p $(BINDIR)
	cd services/server && GOFLAGS=-mod=mod go build -tags "pg sqlite" -ldflags "-X github.com/cuihairu/croupier/services/server/cmd.Version=$(FULL_VERSION) -X github.com/cuihairu/croupier/services/server/cmd.GitCommit=$(GIT_COMMIT) -X github.com/cuihairu/croupier/services/server/cmd.BuildTime=$(BUILD_TIME) -s -w" -o ../../$(BINDIR)/croupier-server .

.PHONY: server-sqlite
server-sqlite:
	@echo "[build] server (+sqlite)"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -tags "pg sqlite" -ldflags "$(LDFLAGS)" -o $(BINDIR)/croupier-server ./services/server

.PHONY: server-ip2loc
server-ip2loc:
	@echo "[deprecated] server-ip2loc: ip2location is runtime-enabled now; building regular server"
	$(MAKE) server

.PHONY: server-sqlite-ip2loc
server-sqlite-ip2loc:
	@echo "[deprecated] server-sqlite-ip2loc: ip2location is runtime-enabled; building regular sqlite server"
	$(MAKE) server-sqlite

agent: api
	@echo "[build] agent"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/croupier-agent ./services/agent

edge: api
	@echo "[build] edge"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/croupier-edge ./services/edge

build: server agent edge worker ingest tools

.PHONY: build-ip2loc
build-ip2loc:
	@echo "[deprecated] build-ip2loc: ip2location is runtime-enabled; using default build"
	$(MAKE) build

tools: schema-validator pack-builder

schema-validator:
	@echo "[build] schema-validator"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/schema-validator ./cmd/schema-validator

pack-builder:
	@echo "[build] pack-builder"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/pack-builder ./cmd/pack-builder

.PHONY: worker
worker:
	@echo "[build] analytics-worker"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/analytics-worker ./cmd/analytics-worker

.PHONY: ingest
ingest:
	@echo "[build] ingest"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/ingest ./services/ingest

.PHONY: analytics-spec
analytics-spec:
	@echo "[analytics] exporting analytics spec JSON to dashboard/public/analytics-spec.json"
	@GOFLAGS=-mod=mod go run ./cmd/analytics-export --configs configs/analytics --out dashboard/public/analytics-spec.json

# ========== SDK Build Targets ==========
build-sdks: build-sdks-cpp build-sdks-go

build-sdks-cpp:
	@echo "[sdks] building C++ SDK..."
	@cd sdks/cpp && cmake -B build -DCMAKE_BUILD_TYPE=Release -DENABLE_GRPC=ON
	@cd sdks/cpp && cmake --build build --parallel

build-sdks-go:
	@echo "[sdks] building Go SDK..."
	@cd sdks/go && go mod tidy && go build ./...

build-sdks-java:
	@echo "[sdks] building Java SDK..."
	@cd sdks/java && ./gradlew build -x test

build-sdks-js:
	@echo "[sdks] building JavaScript SDK..."
	@cd sdks/js && npm ci && npm run build

build-sdks-python:
	@echo "[sdks] building Python SDK..."
	@cd sdks/python && pip install -e . && python -m pytest

# ========== Web & Docs Build Targets ==========
build-web: build-dashboard build-docs

build-dashboard: submodules
	@echo "[web] building dashboard..."
	@cd dashboard && npm ci && npm run build

build-docs:
	@echo "[docs] building documentation..."
	@cd docs && pnpm install --frozen-lockfile && pnpm run build

# ========== Development Targets ==========
dev-dashboard: submodules
	@echo "[web] starting dashboard development server..."
	@cd dashboard && npm ci && npm run dev

dev-docs:
	@echo "[docs] starting VuePress documentation dev server..."
	@cd docs && pnpm install --frozen-lockfile && pnpm run dev

# ========== Clean Targets ==========
clean: clean-sdks clean-web
	rm -rf $(BINDIR)
	rm -rf gen/

clean-sdks:
	@echo "[clean] cleaning SDK build artifacts..."
	@rm -rf sdks/cpp/build sdks/java/build sdks/js/dist sdks/js/node_modules
	@cd sdks/go && go clean -cache -modcache -testcache || true
	@cd sdks/python && rm -rf build/ dist/ *.egg-info/ __pycache__/ || true

clean-web:
	@echo "[clean] cleaning web and docs build artifacts..."
	@rm -rf dashboard/dist dashboard/node_modules
	@rm -rf docs/.vuepress/dist docs/.vuepress/.cache docs/.vuepress/.temp docs/node_modules

# ========== Version Management ==========
.PHONY: version version-sync
version:
	@echo "Current SDK Version: $$(cat VERSION 2>/dev/null || echo 'VERSION file not found')"
	@echo ""
	@echo "SDK Versions:"
	@echo "  JS:     $$(grep '"version"' sdks/js/package.json | head -1 | sed 's/.*: "\(.*\)".*/\1/')"
	@echo "  Python: $$(grep 'version=' sdks/python/setup.py | sed 's/.*version="\(.*\)".*/\1/')"
	@echo "  Java:   $$(grep '^version' sdks/java/build.gradle | sed "s/.*'\(.*\)'.*/\1/")"
	@echo "  C++:    $$(grep -A1 '^project' sdks/cpp/CMakeLists.txt | grep 'VERSION' | awk '{print $$2}')"
	@echo "  Go:     $$(grep 'const Version' sdks/go/version.go 2>/dev/null | sed 's/.*"\(.*\)".*/\1/' || echo 'N/A')"

version-sync:
	@echo "[version] Synchronizing all SDK versions..."
	@./scripts/sync-sdk-versions.sh
	@echo "[version] Updating JS lock file..."
	@cd sdks/js && pnpm install --lockfile-only
	@echo "✅ Version sync complete. Don't forget to commit changes!"

# ========== Help Target ==========
help:
	@echo "Croupier Build System (Monorepo)"
	@echo ""
	@echo "Core Targets:"
	@echo "  all              - Build server, SDKs, and web components"
	@echo "  build            - Build server components (server, agent, edge)"
	@echo "  sync-proto       - Sync proto files from croupier-proto"
	@echo "  proto            - Generate protobuf code"
	@echo ""
	@echo "Server Targets:"
	@echo "  server           - Build croupier-server"
	@echo "  agent            - Build croupier-agent (http+grpc core)"
	@echo "  edge             - Build croupier-edge"
	@echo ""
	@echo "SDK Targets:"
	@echo "  build-sdks       - Build all SDKs (C++, Go)"
	@echo "  build-sdks-cpp   - Build C++ SDK"
	@echo "  build-sdks-go    - Build Go SDK"
	@echo "  build-sdks-java  - Build Java SDK"
	@echo "  build-sdks-js    - Build JavaScript SDK"
	@echo "  build-sdks-python- Build Python SDK"
	@echo ""
	@echo "Web & Docs Targets:"
	@echo "  build-web        - Build web and docs components"
	@echo "  build-dashboard  - Build management dashboard"
	@echo "  build-docs       - Build VuePress documentation"
	@echo "  dev-dashboard    - Start dashboard dev server"
	@echo "  dev-docs         - Start docs dev server"
	@echo ""
	@echo "Utility Targets:"
	@echo "  clean            - Clean all build artifacts"
	@echo "  clean-sdks       - Clean SDK build artifacts"
	@echo "  clean-web        - Clean web build artifacts"
	@echo ""
	@echo "Version Management:"
	@echo "  version          - Show current SDK versions"
	@echo "  version-sync     - Sync all SDK versions from VERSION file"

.PHONY: proto-docs
proto-docs:
	@echo "[proto] generating docs..."
	buf generate --template buf.gen.docs.yaml

# ========== Test Targets ==========
# Run all tests
test:
	@echo "[test] running unit tests..."
	go test -v -short ./...

# Run tests with race detection
test-race:
	@echo "[test] running tests with race detection..."
	go test -v -race -short ./...

# Run tests with coverage report
test-coverage:
	@echo "[test] running tests with coverage..."
	go test -coverprofile=coverage.out -covermode=atomic ./...
	@echo ""
	@echo "Coverage by package:"
	@go tool cover -func=coverage.out | grep -E "^github.com/cuihairu/croupier" | \
		awk '{print $$2 " " $$NF}' | sort -t' ' -k2 -n

# Generate HTML coverage report
test-coverage-html: test-coverage
	@echo "[test] generating HTML coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run integration tests (requires external dependencies)
test-integration:
	@echo "[test] running integration tests..."
	go test -v -tags=integration ./...

# Run all tests including integration
test-all: test test-integration
	@echo "[test] all tests completed"

# Check test coverage against threshold (80%)
test-coverage-check:
	@echo "[test] checking coverage threshold (80%)..."
	@go test -coverprofile=coverage.out -covermode=atomic ./... > /dev/null 2>&1
	@total=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	if [ $$(echo "$$total < 80.0" | bc) -eq 1 ]; then \
		echo "❌ Coverage $$total% is below 80% threshold"; \
		exit 1; \
	else \
		echo "✅ Coverage $$total% meets 80% threshold"; \
	fi

# Run tests for specific package
test-package:
	@echo "[test] running tests for package $(PACKAGE)..."
	go test -v -coverprofile=$(PACKAGE)_coverage.out -covermode=atomic ./$(PACKAGE)...

# Run tests and generate coverage report for CI/CD
test-ci:
	@echo "[test] running CI tests..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic -json ./... > test-report.json 2>&1
	@go tool cover -func=coverage.out | tail -1
