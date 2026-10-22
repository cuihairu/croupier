# Croupier JS/TS SDK Integration Guide

## Installation

```bash
npm install croupier-js-sdk
```

## Minimal Example

```typescript
import {
  createClient,
  FunctionDescriptor,
  ClientConfig,
} from "croupier-js-sdk";

async function main() {
  const config: ClientConfig = {
    agentAddr: "127.0.0.1:19091",
    serviceId: "my-service",
  };

  const client = createClient(config);
  const descriptor: FunctionDescriptor = {
    id: "game.action",
    version: "1.0.0",
    resource: "game",
    capability: "action",
    risk: "safe",
  };

  client.registerFunction(descriptor, async () =>
    JSON.stringify({ status: "success" }),
  );
  await client.connect();
  await client.serve();
}
```

## Registration Metadata

`FunctionDescriptor`'s `tags`/`summary`/`description`/`operationId`/`deprecated` are presentation-layer fields: they are reported protobuf-encoded at registration and feed control-plane function grouping and page generation. When multiple language SDKs share the same function ID space, the agent applies merge protection for empty-value registrations across providers (an empty value does not overwrite another provider's existing tags/summary), but each SDK should still fill in complete metadata at registration — do not rely on the merge fallback.

## Instance Metadata (Provider Self-Description)

`ClientConfig.instanceMetadata` carries **user-defined multi-KV instance metadata** (e.g. `serverId`, `pod`). It is reported via `ProviderConnectRequest.metadata` and is used by the console's "SDK Version Distribution" page to show instance information and support metadata-based search:

```typescript
const client = createClient({
  agentAddr: "127.0.0.1:19091",
  serviceId: "my-service",
  instanceMetadata: { serverId: "s1", pod: "game-7c4d" },
});
```

- Reserved keys `sdkLanguage` / `sdkVersion` / `sdkName` / `protocol_version` / `gameId` / `env` are generated from platform fixed fields; when user metadata collides with a reserved key, the agent drops that key and reports a warning in the registration response `warnings`.
- Metadata is for observability only (display/search/diagnostics); it does not participate in routing or load balancing.
- For wire semantics and storage design decisions, see the "Instance Metadata" section of `docs/architecture/sdk-wire-protocol.md`.

## Connection Lifecycle

The transport internally maintains a single-reader blocking read loop, so on an idle connection probes (pings/pongs) and invocation responses never race on concurrent reads. When the connection drops or errors:

- All pending calls fail immediately with `connection closed`;
- `isConnected()` becomes `false`, and after failed heartbeats the client reconnects automatically per the reconnect configuration.

The business side does not need to poll connection state; treat a `call()` rejection as an ordinary retryable failure.
