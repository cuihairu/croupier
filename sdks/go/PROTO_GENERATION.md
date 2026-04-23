# Proto Generation Guide

This document explains how to generate gRPC code from protobuf files for the Croupier Go SDK.

## Overview

The Go SDK can operate in two modes:
1. **Mock mode** - Uses mock gRPC implementation (default, no dependencies required)
2. **Real gRPC mode** - Uses generated gRPC code (requires proto generation)

## Quick Start

### Option 1: Use Mock Mode (Easiest)

No setup required! The SDK automatically uses mock gRPC implementation:

```bash
go run examples/basic/main.go
```

### Option 2: Use Real gRPC

#### Step 1: Install Dependencies

**macOS:**
```bash
brew install protobuf
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

**Ubuntu/Debian:**
```bash
sudo apt-get install protobuf-compiler
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

**Other systems:**
Download protoc from [GitHub releases](https://github.com/protocolbuffers/protobuf/releases)

#### Step 2: Generate gRPC Code

```bash
# Using the convenient script
./generate_proto.sh

# Or using Makefile
make proto

# Or directly with Go
go run scripts/generate_proto.go
```

#### Step 3: Build with Real gRPC

```bash
# Using Makefile
make build-with-grpc

# Or with go build
go build -tags=croupier_real_grpc ./...

# Run examples
make example
```

## Advanced Usage

### Custom Proto Branch

To generate from a different branch:

```bash
./generate_proto.sh --branch develop
# or
CROUPIER_PROTO_BRANCH=develop ./generate_proto.sh
```

### Skip Proto Generation

If you have previously generated proto files and want to skip regeneration:

```bash
./generate_proto.sh --skip
# or
CROUPIER_SKIP_PROTO_GEN=1 ./generate_proto.sh
```

### Verbose Output

For detailed output during generation:

```bash
./generate_proto.sh --verbose
```

## Using the Makefile

The Makefile provides convenient targets:

```bash
# Build with mock gRPC (default)
make build

# Build with real gRPC (generates proto first)
make build-with-grpc

# Only generate proto files
make proto

# Check if dependencies are installed
make check-deps

# Install dependencies automatically
make install-deps

# Run tests
make test

# Run comprehensive example
make example

# Clean generated files
make clean

# Show build information
make info
```

## Environment Variables

- `CROUPIER_SKIP_PROTO_GEN`: Set to 1 to skip proto generation
- `CROUPIER_PROTO_BRANCH`: Specify proto branch (default: main)
- `CROUPIER_CI_BUILD`: Set to 1 to enable CI mode (strict error checking)

## Build Tags

The generated gRPC code is protected by the build tag `croupier_real_grpc`. This means:

- Without the tag: Mock implementation is used
- With the tag: Real gRPC implementation is used

Example:
```bash
# Mock implementation
go build ./...

# Real gRPC implementation
go build -tags=croupier_real_grpc ./...
```

## Directory Structure

```
.
├── downloaded_proto/     # Downloaded proto files (temporary)
├── proto/                # Generated Go code
│   ├── build_tags.go     # Build tag indicator
│   ├── croupier/         # Generated packages
│   └── google/           # Generated Google APIs
├── scripts/
│   └── generate_proto.go # Proto generation script
└── generate_proto.sh     # Convenience wrapper script
```

## Troubleshooting

### protoc not found

Install protoc:
```bash
# macOS
brew install protobuf

# Ubuntu
sudo apt-get install protobuf-compiler
```

### Go protoc plugins not found

Install the plugins:
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

Make sure `$GOPATH/bin` is in your `$PATH`.

### proto download failed

Check your internet connection and branch name:
```bash
./generate_proto.sh --branch main --verbose
```

### Generated code conflicts

Clean and regenerate:
```bash
make clean
make proto
```

## CI Integration

In CI environments, set `CROUPIER_CI_BUILD=1` to enable strict error checking:

```yaml
- name: Generate proto
  run: |
    export CROUPIER_CI_BUILD=1
    make proto
    make build-with-grpc
```

## Difference Between Mock and Real gRPC

| Feature | Mock Mode | Real gRPC Mode |
|---------|-----------|---------------|
| Dependencies | None | protoc, Go plugins |
| Network calls | Simulated | Real gRPC |
| Performance | Faster | Slower (real calls) |
| Testing | Good for unit tests | Integration tests |
| Production use | No | Yes |

## Best Practices

1. **Development**: Use mock mode for faster iteration
2. **Testing**: Use mock mode for unit tests
3. **Integration**: Use real gRPC for integration tests
4. **Production**: Always use real gRPC
5. **CI**: Enable strict mode with `CROUPIER_CI_BUILD=1`