BINDIR := bin
VERSION := $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -s -w

.PHONY: proto build server agent edge cli clean dev tidy test lint help all tools schema-validator pack-builder
.PHONY: build-sdks build-sdks-cpp build-sdks-go build-sdks-java build-sdks-js build-sdks-python
.PHONY: build-web build-dashboard build-website dev-dashboard dev-website
.PHONY: version version-sync

# Build all components (server + sdks + web)
all: build build-sdks build-web

# ========== Legacy Submodule Support (Removed) ==========
# Note: Submodules have been migrated to monorepo structure
# SDKs are now in sdks/ directory as source code
submodules:
	@echo "⚠️  Submodules have been migrated to monorepo structure"
	@echo "✅ SDKs are now directly available in sdks/ directory"

# Ensure local protoc plugin exists before running buf
proto: croupier-plugin
	@echo "[proto] generating code via buf..."
	buf generate

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

server:
	@echo "[build] server (pg+sqlite)"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -tags "pg sqlite" -ldflags "$(LDFLAGS)" -o $(BINDIR)/croupier-server ./services/server

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

agent:
	@echo "[build] agent"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/croupier-agent ./services/agent-stdlib

edge:
	@echo "[build] edge"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/croupier-edge ./services/edge-stdlib

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
	@echo "[build] analytics-ingest"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/analytics-ingest ./services/analytics-ingest

.PHONY: analytics-spec
analytics-spec:
	@echo "[analytics] exporting analytics spec JSON to web/dashboard/public/analytics-spec.json"
	@mkdir -p web/dashboard/public
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts/export-analytics-spec.ps1

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

build-dashboard:
	@echo "[web] building dashboard..."
	@cd dashboard && npm ci && npm run build

build-docs:
	@echo "[docs] building documentation..."
	@cd docs && npm ci && npm run build

# ========== Development Targets ==========
dev-dashboard:
	@echo "[web] starting dashboard development server..."
	@cd dashboard && npm ci && npm run dev

dev-docs:
	@echo "[docs] starting Docusaurus documentation dev server..."
	@cd docs && npm ci && npm run dev

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
	@rm -rf docs/.vuepress/dist docs/build docs/.docusaurus docs/node_modules

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
	@echo "  proto            - Generate protobuf code"
	@echo ""
	@echo "Server Targets:"
	@echo "  server           - Build croupier-server"
	@echo "  agent            - Build croupier-agent"
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
	@echo "  build-docs       - Build Docusaurus documentation"
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

# ========== Wire (DI) Generation ==========
.PHONY: wire
wire:
	@echo "[wire] generating dependency injection code..."
	@# Ensure modules and types are available for analysis; clear GOFLAGS that might interfere
	@GOFLAGS= GOWORK=off go mod download
	@GOFLAGS= GOWORK=off wire ./internal/app/server/http
