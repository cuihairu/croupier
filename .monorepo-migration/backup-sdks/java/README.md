# Croupier Java SDK

Java SDK for Croupier game function registration and execution system.

## Overview

The Croupier Java SDK enables game servers to register functions with the Croupier system and handle incoming function calls through gRPC communication. This SDK is aligned with the official Croupier proto definitions.

## Features

- **Proto-aligned data structures**: All types match the official Croupier proto definitions
- **Dual build system**: Mock implementation for local development, real gRPC for CI/production
- **Multi-tenant support**: Built-in support for game_id/env isolation
- **Function registration**: Register game functions with descriptors and handlers
- **gRPC communication**: Efficient bi-directional communication with agents
- **Async support**: CompletableFuture-based async operations
- **Error handling**: Comprehensive error handling and connection management
- **Builder pattern**: Fluent API for easy configuration

## Requirements

- Java 11 or later
- Maven 3.6 or later

## Quick Start

### Installation

Add to your `pom.xml`:

```xml
<dependency>
    <groupId>com.croupier</groupId>
    <artifactId>croupier-sdk-java</artifactId>
    <version>1.0.0</version>
</dependency>
```

### Basic Usage

```java
import com.croupier.sdk.*;

public class GameServer {
    public static void main(String[] args) throws Exception {
        // Create client configuration
        ClientConfig config = new ClientConfig("my-game", "my-service");
        config.setAgentAddr("localhost:19090");
        config.setEnv("development");
        config.setInsecure(true); // For development

        // Create client
        CroupierClient client = CroupierSDK.createClient(config);

        // Register a function
        FunctionDescriptor desc = CroupierSDK.functionDescriptor("player.ban", "1.0.0")
                .category("moderation")
                .risk("high")
                .entity("player")
                .operation("update")
                .build();

        FunctionHandler handler = (context, payload) -> {
            // Handle the function call
            return "{\"status\":\"success\"}";
        };

        client.registerFunction(desc, handler);

        // Start serving
        client.serve(); // Blocks until stopped
    }
}
```

### Async Usage

```java
// Connect and serve asynchronously
client.connect()
    .thenCompose(v -> client.serveAsync())
    .thenRun(() -> System.out.println("Service started"))
    .exceptionally(throwable -> {
        System.err.println("Failed: " + throwable.getMessage());
        return null;
    });
```

## Data Types

### FunctionDescriptor

Aligned with `control.proto`:

```java
FunctionDescriptor descriptor = CroupierSDK.functionDescriptor("player.ban", "1.0.0")
        .category("moderation")     // grouping category
        .risk("high")              // "low"|"medium"|"high"
        .entity("player")          // entity type
        .operation("update")       // "create"|"read"|"update"|"delete"
        .enabled(true)             // whether enabled
        .build();
```

### LocalFunctionDescriptor

Aligned with `agent/local/v1/local.proto`:

```java
LocalFunctionDescriptor localDesc = new LocalFunctionDescriptor("player.ban", "1.0.0");
```

### FunctionHandler

Functional interface for implementing game functions:

```java
FunctionHandler handler = (context, payload) -> {
    // context: execution context (JSON string)
    // payload: function payload (JSON string)
    // return: result (JSON string)
    return "{\"status\":\"success\"}";
};
```

## Configuration

### ClientConfig

```java
ClientConfig config = new ClientConfig();
config.setAgentAddr("localhost:19090");     // Agent address
config.setGameId("my-game");                // Game identifier
config.setEnv("development");               // Environment
config.setServiceId("my-service");          // Service identifier
config.setServiceVersion("1.0.0");         // Service version
config.setLocalListen(":0");                // Local server (auto-port)
config.setTimeoutSeconds(30);              // Connection timeout
config.setInsecure(true);                   // Use insecure gRPC

// TLS settings (when not insecure)
config.setCaFile("/path/to/ca.pem");
config.setCertFile("/path/to/cert.pem");
config.setKeyFile("/path/to/key.pem");
```

## API Reference

### CroupierClient Interface

```java
public interface CroupierClient {
    // Function registration
    void registerFunction(FunctionDescriptor descriptor, FunctionHandler handler);

    // Connection management
    CompletableFuture<Void> connect();

    // Service operations
    void serve();                           // Blocks until stopped
    CompletableFuture<Void> serveAsync();  // Non-blocking

    // Lifecycle
    void stop();
    void close();

    // Status
    boolean isConnected();
    boolean isServing();
    String getLocalAddress();
}
```

### Factory Methods

```java
// Simple creation
CroupierClient client = CroupierSDK.createClient("game-id", "service-id");

// With agent address
CroupierClient client = CroupierSDK.createClient("game-id", "service-id", "localhost:19090");

// With full configuration
CroupierClient client = CroupierSDK.createClient(config);
```

## Build Modes

### Local Development (Mock gRPC)

For local development, the SDK uses mock implementations:

```bash
mvn clean compile
mvn exec:java -Dexec.mainClass="com.croupier.sdk.examples.BasicExample"
```

### CI/Production (Real gRPC)

For CI builds with real proto-generated code:

```bash
export CROUPIER_CI_BUILD=ON
mvn clean compile -Pci-build
```

The CI system automatically:
1. Downloads proto files from main repository
2. Generates Java gRPC code using protobuf-maven-plugin
3. Builds with real gRPC implementation
4. Runs tests and examples

## Architecture

```
Game Server → Java SDK → Agent → Croupier Server
```

The SDK implements a two-layer registration system:
1. **SDK → Agent**: Uses `LocalControlService` (from `local.proto`)
2. **Agent → Server**: Uses `ControlService` (from `control.proto`)

## Error Handling

The SDK provides comprehensive error handling:

```java
try {
    client.registerFunction(descriptor, handler);
    client.serve();
} catch (CroupierException e) {
    System.err.println("Croupier error: " + e.getMessage());
    e.printStackTrace();
} catch (Exception e) {
    System.err.println("Unexpected error: " + e.getMessage());
}
```

Async error handling:

```java
client.connect()
    .exceptionally(throwable -> {
        if (throwable.getCause() instanceof CroupierException) {
            System.err.println("Croupier error: " + throwable.getCause().getMessage());
        }
        return null;
    });
```

## Examples

See the `examples/` directory for complete usage examples:

- `examples/basic/`: Basic function registration and serving
- More examples coming soon...

## Development

### Building

```bash
# Local development (mock)
mvn clean compile

# CI build (real gRPC)
mvn clean compile -Pci-build

# Run tests
mvn test

# Package
mvn package
```

### Project Structure

```
src/
├── main/java/com/croupier/sdk/
│   ├── CroupierSDK.java           # Factory class
│   ├── CroupierClient.java        # Client interface
│   ├── CroupierClientImpl.java    # Client implementation
│   ├── FunctionDescriptor.java    # Proto-aligned descriptor
│   ├── LocalFunctionDescriptor.java # Local descriptor
│   ├── FunctionHandler.java       # Handler interface
│   ├── ClientConfig.java          # Configuration
│   ├── GrpcManager.java          # gRPC management
│   └── scripts/
│       └── ProtoGenerator.java    # CI proto generator
├── test/java/                     # Unit tests
└── main/resources/                # Resources

examples/
└── basic/                         # Basic example
```

## Contributing

1. Ensure all types align with proto definitions
2. Add tests for new functionality
3. Update documentation for API changes
4. Test both local and CI build modes
5. Follow Java coding conventions

## License

See LICENSE file for details.