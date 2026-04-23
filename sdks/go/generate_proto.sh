#!/bin/bash

# Croupier Go SDK Proto Generation Script
# This script helps generate gRPC code from protobuf files

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Croupier Go SDK Proto Generator${NC}"
echo "=================================="

# Check if we're in the right directory
if [ ! -f "scripts/generate_proto.go" ]; then
    echo -e "${RED}Error: Please run this script from the Go SDK root directory${NC}"
    exit 1
fi

# Parse arguments
SKIP_PROTO_GEN=false
BRANCH="main"
VERBOSE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --skip)
            SKIP_PROTO_GEN=true
            shift
            ;;
        --branch)
            BRANCH="$2"
            shift 2
            ;;
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --skip       Skip proto generation (uses mock implementation)"
            echo "  --branch BRANCH Use specific branch for proto files (default: main)"
            echo "  --verbose     Show detailed output"
            echo "  --help        Show this help message"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Set environment variables
if [ "$SKIP_PROTO_GEN" = true ]; then
    export CROUPIER_SKIP_PROTO_GEN=1
    echo -e "${YELLOW}Skipping proto generation (mock mode)${NC}"
else
    export CROUPIER_PROTO_BRANCH="$BRANCH"
    if [ "$VERBOSE" = true ]; then
        echo "Using branch: $BRANCH"
    fi
fi

# Run the Go script
echo "Running proto generation script..."
cd "$(dirname "$0")"
go run scripts/generate_proto.go

# Show next steps
if [ "$SKIP_PROTO_GEN" = false ]; then
    echo ""
    echo -e "${GREEN}Proto generation completed successfully!${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Build with real gRPC:"
    echo "   go build -tags=croupier_real_grpc ./..."
    echo ""
    echo "2. Or use the Makefile:"
    echo "   make build-with-grpc"
    echo ""
    echo "3. To skip proto generation next time:"
    echo "   ./generate_proto.sh --skip"
else
    echo ""
    echo -e "${YELLOW}Using mock gRPC implementation${NC}"
    echo ""
    echo "To generate real gRPC code:"
    echo "1. Install dependencies:"
    echo "   brew install protobuf  # macOS"
    echo "   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
    echo "   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
    echo ""
    echo "2. Run proto generation:"
    echo "   ./generate_proto.sh"
fi