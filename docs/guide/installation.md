---
title: Installation Guide
icon: download
order: 2
category:
  - Getting Started
tag:
  - Installation
  - Environment Setup
---

# Installation Guide

This guide describes in detail how to install and configure each Croupier component.

## Table of Contents

[[toc]]

## System Requirements

### Minimum Requirements

| Component | Minimum Version     | Recommended |
| --------- | ------------------- | ----------- |
| Go        | 1.27.2              | 1.27.2+     |
| Node.js   | 24                  | 24+         |
| pnpm      | 11                  | 11+         |
| buf       | -                   | latest      |
| OS        | Linux/macOS/Windows | Linux/macOS |

### Hardware Requirements

| Scenario           | CPU      | Memory | Disk    |
| ------------------ | -------- | ------ | ------- |
| Development        | 2 cores  | 4 GB   | 10 GB   |
| Production (small) | 4 cores  | 8 GB   | 50 GB   |
| Production (large) | 8+ cores | 16 GB+ | 100 GB+ |

## Development Environment Installation

### 1. Install Go

#### macOS

```bash
# Using Homebrew
brew install go

# Verify the installation
go version
```

#### Linux (Ubuntu/Debian)

```bash
# Download and install
wget https://go.dev/dl/go1.27.2.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.27.2.linux-amd64.tar.gz

# Configure PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

#### Windows

Download the installer from the [Go website](https://go.dev/dl/) and run it.

### 2. Install Node.js and pnpm

```bash
# Install Node.js using fnm (recommended)
curl -fsSL https://fnm.vercel.app/install | bash
fnm install 22
fnm use 22

# Install pnpm
npm install -g pnpm

# Verify the installation
node -v
pnpm -v
```

### 3. Install buf

```bash
# Using go install
go install github.com/bufbuild/buf/cmd/buf@latest

# Or using Homebrew (macOS)
brew install bufbuild/buf/buf

# Verify the installation
buf --version
```

### 4. Clone the Project

```bash
# Clone the main repository
git clone https://github.com/cuihairu/croupier.git
cd croupier
```

## Production Environment Installation

### Docker Deployment

#### Using Docker Compose

```bash
# Build the images
docker compose build

# Start the services
docker compose up -d

# View logs
docker compose logs -f
```

#### Standalone Deployment

```bash
# Build the Server image
docker build -t croupier-server:latest -f docker/Dockerfile.server .

# Run the Server
docker run -d \
  --name croupier-server \
  -p 19090:19090 \
  -p 18780:18780 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/configs:/app/configs \
  croupier-server:latest
```

### Binary Deployment

#### Download Prebuilt Binaries

Download the binaries for your platform from the [Release page](https://github.com/cuihairu/croupier/releases).

#### Build from Source

```bash
# Clone the repository
git clone https://github.com/cuihairu/croupier.git
cd croupier

# Build the binaries (server / agent / devops-agent / capture-agent / worker / ingest / tools)
make build

# The binaries are located in the bin/ directory
ls -la bin/
```

## SDK Installation

The SDKs for each language are source code in this repository under `sdks/<lang>/` (for installation details, follow the per-language guides in
[docs/sdks/](../sdks/index.md)):

### Go SDK

```bash
go get github.com/cuihairu/croupier/sdks/go@latest
```

### C++ SDK

```bash
# Build from this repository's source (vcpkg dependency manifest in sdks/cpp/vcpkg.json)
cd sdks/cpp
cmake --preset linux-x64-release-vcpkg
cmake --build --preset linux-x64-debug
```

### Java SDK

```bash
# Build from this repository's source (the release pipeline is configured with OSSRH staging and published via the release workflow)
cd sdks/java && ./gradlew publishToMavenLocal
```

### JavaScript/TypeScript SDK

```bash
# Package name croupier-js-sdk (built from source)
cd sdks/js && pnpm install && pnpm build
```

### Python SDK

```bash
# Package name croupier-sdk (built from source)
cd sdks/python && pip install .
```

## Configuration Files

### Server Configuration

```yaml
server:
  host: 0.0.0.0
  port: 18780 # HTTP listen address
  control:
    addr: ":19090" # Control (agent access) listen address
database:
  # Supports sqlite, mysql, postgres, sqlserver
  driver: postgres
  dataSource: "postgres://user:***@localhost:5432/croupier"
  # SQLite example:
  # driver: sqlite
  # dataSource: "data/croupier.db"
  # MySQL example:
  # driver: mysql
  # dataSource: "user:pass@tcp(localhost:3306)/croupier_meta?charset=utf8mb4&parseTime=True"
log:
  level: info
  format: json
```

### Agent Configuration

```yaml
name: croupier-agent
host: 0.0.0.0
port: 19091

server:
  addr: "localhost:19090"
  insecure: false

agent:
  gameId: "my-game"
  env: "dev"
  localAddr: "localhost:19091"

outboundTLS:
  enabled: true
  caFile: "data/ca.crt"
  certFile: "data/agent.crt"
  keyFile: "data/agent.key"
```

## Verify the Installation

### Health Check

```bash
# Server health check
curl http://localhost:18780/healthz

# Expected output (ok / timestamp / uptimeSeconds / components)
{"ok":true,"timestamp":"2026-10-13T00:00:00Z","uptimeSeconds":3600,"components":{"database":"ok","registry":"ok","ops":"ok"}}
```

### Functional Testing

```bash
# Run unit tests
make test

# Run integration tests
make test-integration
```

## Troubleshooting

### Common Issues

<details>
<summary>Incompatible Go version</summary>

Make sure the Go version is 1.27.2 or higher:

```bash
go version
```

If not, upgrade Go.

</details>

<details>
<summary>buf command not found</summary>

Make sure `$GOPATH/bin` is in your `PATH`:

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

</details>

<details>
<summary>Port already in use</summary>

Change the ports in the configuration file:

```yaml
control:
  addr: ":19092"
port: 19080
```

</details>

## Next Steps

After installation, refer to:

- [Configuration Guide](../operations/config-server.md) - detailed configuration options
- [Deployment Guide (Chinese only)](/zh/operations/deploy-docker) - production deployment
- [Core Concepts (Chinese only)](/zh/guide/concepts/overview) - understanding the system design
