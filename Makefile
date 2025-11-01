BINDIR := bin
VERSION := $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -s -w

.PHONY: proto build server agent edge cli clean dev tidy test lint help all

# Build all components
all: build

submodules:
	git submodule update --init --recursive

proto:
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
		$(shell rg -n --files proto | tr '\n' ' ')

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
	@echo "[build] server"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -tags pg -ldflags "$(LDFLAGS)" -o $(BINDIR)/croupier-server ./cmd/server

.PHONY: server-sqlite
server-sqlite:
	@echo "[build] server (+sqlite)"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -tags "pg sqlite" -ldflags "$(LDFLAGS)" -o $(BINDIR)/croupier-server ./cmd/server

agent:
	@echo "[build] agent"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/croupier-agent ./cmd/agent

edge:
	@echo "[build] edge"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/croupier-edge ./cmd/edge

build: server agent edge

cli:
	@echo "[build] unified CLI"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -tags pg -ldflags "$(LDFLAGS)" -o $(BINDIR)/croupier ./cmd/croupier

.PHONY: cli-sqlite
cli-sqlite:
	@echo "[build] unified CLI (+sqlite)"
	@mkdir -p $(BINDIR)
	GOFLAGS=-mod=mod go build -tags "pg sqlite" -ldflags "$(LDFLAGS)" -o $(BINDIR)/croupier ./cmd/croupier

# Cross-compile for multiple platforms
build-linux-amd64:
	@echo "[cross-build] linux/amd64"
	@mkdir -p $(BINDIR)/linux-amd64
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/linux-amd64/croupier-server ./cmd/server
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/linux-amd64/croupier-agent ./cmd/agent
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/linux-amd64/croupier-edge ./cmd/edge

build-windows-amd64:
	@echo "[cross-build] windows/amd64"
	@mkdir -p $(BINDIR)/windows-amd64
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/windows-amd64/croupier-server.exe ./cmd/server
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/windows-amd64/croupier-agent.exe ./cmd/agent
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/windows-amd64/croupier-edge.exe ./cmd/edge

# Clean build artifacts
clean:
	@echo "[clean] removing build artifacts..."
	rm -rf $(BINDIR)
	rm -rf gen/

# Development setup
tidy:
	go mod tidy

test:
	@echo "[test] running unit tests..."
	go test -v -race ./...

lint:
	@echo "[lint] running golangci-lint..."
	golangci-lint run

dev: clean proto build
	@echo "✅ Development build complete!"
	@echo "📦 Binaries available in $(BINDIR)/"
	@echo "🔧 Run binaries in separate shells with your TLS config"
	@echo "💡 Tip: run 'make submodules' to initialize web and SDK submodules"

# Show help
help:
	@echo "Available targets:"
	@echo "  build              Build all components (server, agent, edge)"
	@echo "  server             Build server component only"
	@echo "  agent              Build agent component only"
	@echo "  edge               Build edge component only"
	@echo "  build-linux-amd64  Cross-compile for Linux AMD64"
	@echo "  build-windows-amd64 Cross-compile for Windows AMD64"
	@echo "  proto              Generate protobuf code"
	@echo "  test               Run unit tests"
	@echo "  lint               Run linter"
	@echo "  clean              Clean build artifacts"
	@echo "  tidy               Tidy Go modules"
	@echo "  dev                Full development build (clean + proto + build)"
	@echo "  submodules         Initialize git submodules"
	@echo "  help               Show this help"
