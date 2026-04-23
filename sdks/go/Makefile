# Croupier Go SDK Makefile
# Provides convenient targets for building and managing the SDK

.PHONY: help build build-with-grpc clean test proto check-deps example example-basic example-comprehensive

# Default target
help:
	@echo "Croupier Go SDK Build System"
	@echo "=========================="
	@echo ""
	@echo "Available targets:"
	@echo "  build                 Build SDK with mock gRPC (default)"
	@echo "  build-with-grpc       Build SDK with real gRPC (generates proto first)"
	@echo "  proto                 Generate gRPC code from protobuf files"
	@echo "  check-deps            Check if proto generation dependencies are installed"
	@echo "  test                  Run tests"
	@echo "  clean                 Clean generated files"
	@echo "  example               Run comprehensive example"
	@echo "  example-basic         Run basic example"
	@echo "  example-comprehensive Run comprehensive example"
	@echo "  install-deps          Install proto generation dependencies"

# Build with mock gRPC (default, no proto generation required)
build:
	@echo "Building SDK with mock gRPC..."
	go build ./...

# Build with real gRPC (requires proto generation)
build-with-grpc: proto
	@echo "Building SDK with real gRPC..."
	go build -tags=croupier_real_grpc ./...

# Generate proto files
proto:
	@echo "Generating gRPC code from protobuf files..."
	./generate_proto.sh

# Skip proto generation
skip-proto:
	@echo "Building with mock gRPC (proto generation skipped)..."
	CROUPIER_SKIP_PROTO_GEN=1 go run scripts/generate_proto.go

# Check if dependencies for proto generation are installed
check-deps:
	@echo "Checking proto generation dependencies..."
	@command -v protoc >/dev/null 2>&1 || { echo "❌ protoc not found"; exit 1; }
	@echo "✅ protoc found: $$(protoc --version)"
	@command -v protoc-gen-go >/dev/null 2>&1 || { echo "❌ protoc-gen-go not found"; exit 1; }
	@command -v protoc-gen-go-grpc >/dev/null 2>&1 || { echo "❌ protoc-gen-go-grpc not found"; exit 1; }
	@echo "✅ Go protoc plugins found"

# Install dependencies
install-deps:
	@echo "Installing proto generation dependencies..."
	@echo "Installing protoc..."
	@if command -v brew >/dev/null 2>&1; then \
		brew install protobuf; \
	elif command -v apt-get >/dev/null 2>&1; then \
		sudo apt-get update && sudo apt-get install -y protobuf-compiler; \
	else \
		echo "Please install protoc manually from https://github.com/protocolbuffers/protobuf/releases"; \
	fi
	@echo "Installing Go protoc plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -race ./...

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	rm -rf downloaded_proto
	rm -rf proto/*
	rm -f proto/build_tags.go

# Run comprehensive example
example: build-with-grpc
	@echo "Running comprehensive example..."
	cd examples/comprehensive && go run . -tags=croupier_real_grpc

# Run basic example
example-basic: build-with-grpc
	@echo "Running basic example..."
	cd examples/basic && go run . -tags=croupier_real_grpc

# Development target - watch for changes and rebuild
dev:
	@echo "Watching for changes (Ctrl+C to stop)..."
	@while true; do \
		inotifywait -e modify,create,delete -r . 2>/dev/null || true; \
		echo "Changes detected, rebuilding..."; \
		make build; \
	done

# Install SDK as a library
install:
	@echo "Installing SDK..."
	go install ./...

# Create a release build
release: test build-with-grpc
	@echo "Creating release build..."
	mkdir -p dist
	tar -czf dist/croupier-go-sdk-$(shell date +%Y%m%d).tar.gz \
		--exclude=dist \
		--exclude=downloaded_proto \
		--exclude=.git \
		--exclude=examples \
		.

# Show build info
info:
	@echo "Go SDK Build Info"
	@echo "================="
	@echo "Go version: $(go version)"
	@echo "GOPATH: $$GOPATH"
	@echo "Go mod cache: $(go env GOMODCACHE)"
	@echo "Proto files: $(shell find proto -name '*.pb.go' 2>/dev/null | wc -l | tr -d ' ')"
	@if [ -f proto/build_tags.go ]; then echo "Real gRPC: ✅"; else echo "Real gRPC: ❌ (using mock)"; fi