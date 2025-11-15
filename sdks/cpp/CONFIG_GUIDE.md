# Croupier C++ SDK - Advanced Configuration System

The Croupier C++ SDK now features a comprehensive, modular configuration system that supports multiple environments, validation, and flexible deployment scenarios.

## 🏗️ Architecture Overview

The configuration system is built with the following modular components:

### Core Modules

- **`utils/JsonUtils`** - Cross-platform JSON processing with nlohmann/json support
- **`utils/FileSystemUtils`** - File and directory operations across platforms
- **`config/ClientConfigLoader`** - Client configuration loading and validation
- **`ConfigDrivenLoader`** - Component and virtual object loading from configuration

### Benefits

✅ **Modular Design** - Each utility is in its own file for maintainability
✅ **Environment Support** - Development, staging, production configurations
✅ **Validation** - Comprehensive configuration validation with detailed errors
✅ **Override Support** - Environment variables can override file settings
✅ **Cross-Platform** - Works on Windows, Linux, and macOS
✅ **Schema Support** - Virtual object schemas with field validation

## 🚀 Quick Start

### 1. Basic Configuration Loading

```cpp
#include "croupier/sdk/config/client_config_loader.h"

// Load configuration
ClientConfigLoader loader;
ClientConfig config = loader.LoadFromFile("./configs/development.json");

// Validate configuration
auto errors = loader.ValidateConfig(config);
if (!errors.empty()) {
    for (const auto& error : errors) {
        std::cerr << "Config error: " << error << std::endl;
    }
    return 1;
}

// Create client
CroupierClient client(config);
```

### 2. Environment-Based Configuration

```cpp
// Load configuration with environment variable overrides
ClientConfig config = loader.LoadWithEnvironmentOverrides(
    "./configs/production.json",
    "CROUPIER_"  // Environment variable prefix
);

// Environment variables like CROUPIER_GAME_ID will override config file values
```

### 3. Profile-Based Loading

```cpp
// Load configuration profile (base + environment-specific)
ClientConfig config = loader.LoadProfile("./configs", "production");

// This loads base.json and production.json, merging them together
```

## 📁 Configuration File Structure

### Environment-Specific Configurations

Create separate configuration files for each environment:

```
configs/
├── development.json    # Development environment
├── staging.json       # Staging environment
├── production.json    # Production environment
└── base.json          # Common base configuration
```

### Development Configuration Example

```json
{
  "game_id": "my-game-dev",
  "env": "development",
  "service_id": "backend-dev",
  "agent_addr": "127.0.0.1:19090",
  "local_listen": "0.0.0.0:0",
  "insecure": true,
  "timeout_seconds": 30,
  "auth": {
    "headers": {
      "X-Game-Version": "1.0.0-dev",
      "X-Environment": "development"
    }
  }
}
```

### Production Configuration Example

```json
{
  "game_id": "my-game-prod",
  "env": "production",
  "service_id": "backend-prod-01",
  "agent_addr": "croupier-agent.internal:19090",
  "local_listen": "0.0.0.0:0",
  "insecure": false,
  "timeout_seconds": 60,
  "security": {
    "cert_file": "/etc/tls/client.crt",
    "key_file": "/etc/tls/client.key",
    "ca_file": "/etc/tls/ca.crt",
    "server_name": "croupier.internal"
  },
  "auth": {
    "token": "Bearer ${JWT_TOKEN}",
    "headers": {
      "X-Game-Version": "2.1.0",
      "X-Service-Region": "us-west-2"
    }
  }
}
```

## 🔧 Environment Variable Overrides

You can override any configuration value using environment variables with the `CROUPIER_` prefix:

```bash
# Override basic settings
export CROUPIER_GAME_ID="my-override-game"
export CROUPIER_ENV="staging"
export CROUPIER_AGENT_ADDR="staging-agent:19090"

# Override security settings
export CROUPIER_INSECURE="false"
export CROUPIER_CERT_FILE="/path/to/cert.pem"

# Override authentication
export CROUPIER_AUTH_TOKEN="Bearer abc123..."

# Run your application
./your-app
```

## 🛠️ Building and Running Examples

### Build the SDK with Examples

```bash
# Configure build
cmake -B build -DBUILD_EXAMPLES=ON \
    -DCMAKE_TOOLCHAIN_FILE=[vcpkg-root]/scripts/buildsystems/vcpkg.cmake

# Build all targets
cmake --build build --config Release

# Examples will be in build/bin/
```

### Run Configuration Example

```bash
# Run with default (development) configuration
./build/bin/croupier-config-example

# Run with specific environment
./build/bin/croupier-config-example production

# Use environment variable
export CROUPIER_ENV=staging
./build/bin/croupier-config-example
```

## 📊 Configuration Validation

The system provides comprehensive validation with detailed error messages:

```cpp
auto errors = loader.ValidateConfig(config);
for (const auto& error : errors) {
    std::cout << "❌ " << error << std::endl;
}
```

Common validation checks:

- ✅ Required fields (game_id, agent_addr)
- ✅ Network address formats (host:port)
- ✅ File path existence (TLS certificates)
- ✅ Environment values (development, staging, production)
- ✅ Security configuration consistency
- ✅ Authentication token formats

## 🔒 Security Configuration

For production deployments, the SDK supports comprehensive TLS configuration:

```json
{
  "insecure": false,
  "security": {
    "cert_file": "/etc/tls/croupier/client.crt",
    "key_file": "/etc/tls/croupier/client.key",
    "ca_file": "/etc/tls/croupier/ca.crt",
    "server_name": "croupier.internal"
  },
  "auth": {
    "token": "Bearer eyJhbGciOiJIUzI1NiIs...",
    "headers": {
      "X-Client-ID": "backend-service-01",
      "X-Service-Region": "us-west-2"
    }
  }
}
```

## 🐳 Docker Deployment

### Dockerfile Example

```dockerfile
FROM ubuntu:22.04

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    libgrpc++1.54 \
    libprotobuf32 \
    && rm -rf /var/lib/apt/lists/*

# Copy application and configs
COPY build/bin/your-app /usr/local/bin/
COPY configs/ /etc/croupier/configs/

# Set environment
ENV CROUPIER_ENV=production
ENV CROUPIER_GAME_ID=your-production-game

# Run application
CMD ["/usr/local/bin/your-app"]
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: game-backend
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: backend
        image: your-registry/game-backend:latest
        env:
        - name: CROUPIER_ENV
          value: "production"
        - name: CROUPIER_GAME_ID
          valueFrom:
            configMapKeyRef:
              name: game-config
              key: game-id
        - name: CROUPIER_AUTH_TOKEN
          valueFrom:
            secretKeyRef:
              name: croupier-auth
              key: jwt-token
        volumeMounts:
        - name: tls-certs
          mountPath: /etc/tls
          readOnly: true
      volumes:
      - name: tls-certs
        secret:
          secretName: croupier-tls
```

## 🧪 Testing Your Configuration

Use the provided example to test your configuration:

```bash
# Test development configuration
./build/bin/croupier-config-example development

# Test production configuration
./build/bin/croupier-config-example production

# Test with environment overrides
CROUPIER_GAME_ID="test-game" \
CROUPIER_TIMEOUT_SECONDS="60" \
./build/bin/croupier-config-example
```

## 🚨 Troubleshooting

### Common Issues

**Configuration file not found**
```
⚠️ Configuration file not found: ./configs/production.json
📄 Generating example configuration...
```
*Solution: The SDK will generate an example configuration file for you to edit.*

**TLS certificate not found**
```
❌ cert_file does not exist: /etc/tls/client.crt
```
*Solution: Ensure TLS certificates exist or set `insecure: true` for development.*

**Invalid network address**
```
❌ agent_addr format is invalid (should be host:port)
```
*Solution: Use proper host:port format like `127.0.0.1:19090` or `agent.internal:19090`*

**Environment variable override not working**
```cpp
// Make sure to use the LoadWithEnvironmentOverrides method
ClientConfig config = loader.LoadWithEnvironmentOverrides(
    config_file,
    "CROUPIER_"  // This prefix is important
);
```

### Debug Configuration Loading

Enable verbose output in your application:

```cpp
// Add debug logging
std::cout << "Loading config from: " << config_file << std::endl;
ClientConfig config = loader.LoadFromFile(config_file);

// Validate and show errors
auto errors = loader.ValidateConfig(config);
for (const auto& error : errors) {
    std::cout << "Validation error: " << error << std::endl;
}
```

## 📖 API Reference

### ClientConfigLoader Methods

- `LoadFromFile(file_path)` - Load configuration from JSON file
- `LoadFromJson(json_content)` - Load configuration from JSON string
- `LoadWithEnvironmentOverrides(file_path, prefix)` - Load with env var overrides
- `LoadProfile(config_dir, profile)` - Load configuration profile
- `ValidateConfig(config)` - Validate configuration and return errors
- `GenerateExampleConfig(environment)` - Generate example configuration
- `CreateDefaultConfig()` - Create configuration with defaults
- `MergeConfigs(base, overlay)` - Merge two configurations

### Utility Classes

- `utils::JsonUtils` - JSON processing utilities
- `utils::FileSystemUtils` - Cross-platform file operations

---

🎮 **Ready to build next-generation game backends with Croupier C++ SDK!**