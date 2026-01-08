# Croupier C# SDK

C# SDK for Croupier with full OpenAPI 3.0 support.

## Features

- ✅ **OpenAPI 3.0 Compatible** - Full support for OpenAPI 3.0 Operation Object specification
- ✅ **gRPC Support** - Built on Grpc.Net.Client for high-performance RPC communication
- ✅ **Async/Await** - Modern async patterns throughout
- ✅ **Type-Safe** - Full C# type safety with nullable reference types
- ✅ **.NET 8.0** - Targeting the latest .NET LTS version

## Installation

```bash
dotnet add package Croupier.SDK
```

Or reference the project directly:

```xml
<ItemGroup>
  <ProjectReference Include="../path/to/Croupier.SDK/Croupier.SDK.csproj" />
</ItemGroup>
```

## Quick Start

```csharp
using Croupier.SDK.Client;
using Croupier.SDK.Models;

// Create client
var config = new ClientConfig
{
    AgentAddr = "localhost:19090",
    GameId = "my-game",
    Env = "development",
    ServiceId = "my-service"
};

var client = new CroupierClient(config);

// Register a function with OpenAPI 3.0 descriptor
client.RegisterOpenAPIFunction(
    id: "player.ban",
    summary: "Ban a player from the game",
    handler: async (context, payload) =>
    {
        // Handle the function call
        return JsonSerializer.SerializeToUtf8Bytes(new { status = "success" });
    },
    configure: descriptor =>
    {
        descriptor.Tags = new List<string> { "player", "moderation" };
        descriptor.OperationId = "banPlayer";
        // ... more OpenAPI 3.0 fields
    }
);

// Connect to the agent
await client.ConnectAsync();
```

## OpenAPI 3.0 Function Descriptor

The SDK supports the following OpenAPI 3.0 compatible fields:

### Basic Fields

| Field | Type | Description |
|-------|------|-------------|
| `Id` | `string` | Function identifier (e.g., "player.ban") |
| `Version` | `string` | Semver version (e.g., "1.2.0") |

### OpenAPI 3.0 Standard Fields

| Field | Type | Description |
|-------|------|-------------|
| `Tags` | `List<string>` | Tags for categorization |
| `Summary` | `string` | Short summary |
| `Description` | `string` | Detailed description (supports CommonMark) |
| `Deprecated` | `bool` | Whether the function is deprecated |
| `OperationId` | `string` | OpenAPI 3.0 operationId |
| `ExternalDocs` | `ExternalDocumentation` | External documentation link |
| `Parameters` | `List<ParameterDescriptor>` | Parameter definitions |
| `RequestBody` | `RequestBodyDescriptor` | Request body schema |
| `Response` | `ResponseDescriptor` | Response schema |

### UI Extension Fields (x-*)

| Field | Type | Description |
|-------|------|-------------|
| `DisplayName` | `DisplayName` | i18n display names |
| `Menu` | `MenuConfig` | Menu configuration |

## Examples

See the `examples/Basic` directory for a complete working example.

## License

Apache License 2.0
