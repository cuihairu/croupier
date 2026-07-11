# Croupier Go SDK Makefile
# Uses unified proto files from project root

.PHONY: help build clean test proto check-deps install-deps example

# Default target
help:
	@echo "Croupier Go SDK Build System"
	@echo "=========================="
	@echo ""
	@echo "Available targets:"
	@echo "  build                 Build SDK (uses proto from project root)"
	@echo "  proto                 Generate gRPC code from project proto/ directory"
	@echo "  check-deps            Check if proto generation dependencies are installed"
	@echo "  test                  Run tests"
	@echo "  clean                 Clean generated files"
	@echo "  install-deps          Install proto generation dependencies"
	@echo ""
	@echo "Note: This SDK uses proto files from the project root directory: ../../proto"
	@echo "      Generated code is output to: pkg/pb/"

# Build SDK (uses proto from project root)
build: proto
	@echo "Building SDK..."
	go build ./...

# Generate proto files from project root
proto:
	@echo "Generating gRPC code from project proto/ directory..."
	@echo "Using fixed version plugins: protoc-gen-go@v1.36.11, protoc-gen-go-grpc@v1.5.1"
	go run scripts/generate_proto.go

# Check if dependencies for proto generation are installed
check-deps:
	@echo "Checking proto generation dependencies..."
	@command -v protoc >/dev/null 2>&1 || { echo "❌ protoc not found"; exit 1; }
	@echo "✅ protoc found: $$(protoc --version)"
	@command -v protoc-gen-go >/dev/null 2>&1 || { echo "❌ protoc-gen-go not found"; exit 1; }
	@command -v protoc-gen-go-grpc >/dev/null 2>&1 || { echo "❌ protoc-gen-go-grpc not found"; exit 1; }
	@echo "✅ Go protoc plugins found"
	@echo ""
	@echo "Using fixed plugin versions:"
	@echo "  - protoc-gen-go@v1.36.11"
	@echo "  - protoc-gen-go-grpc@v1.5.1"

# Install dependencies with fixed versions
install-deps:
	@echo "Installing proto generation dependencies (fixed versions)..."
	@echo "Installing protoc..."
	@if command -v brew >/dev/null 2>&1; then \
		brew install protobuf; \
	elif command -v apt-get >/dev/null 2>&1; then \
		sudo apt-get update && sudo apt-get install -y protobuf-compiler; \
	else \
		echo "Please install protoc manually from https://github.com/protocolbuffers/protobuf/releases"; \
	fi
	@echo "Installing Go protoc plugins (fixed versions)..."
	@echo "  - protoc-gen-go@v1.36.11"
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	@echo "  - protoc-gen-go-grpc@v1.5.1"
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	@echo "✅ Dependencies installed"

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
	rm -rf proto/*.go
	rm -rf proto/build_tags.go
	find pkg/pb -name "*.pb.go" -delete

# Install SDK as a library
install:
	@echo "Installing SDK..."
	go install ./...

# Show build info
info:
	@echo "Go SDK Build Info"
	@echo "================="
	@echo "Go version: $$(go version)"
	@echo "GOPATH: $$GOPATH"
	@echo "Proto source: ../../proto"
	@echo "Generated output: pkg/pb/"
	@echo "Proto files used: $$(find pkg/pb -name '*.pb.go' 2>/dev/null | wc -l | tr -d ' ')"
