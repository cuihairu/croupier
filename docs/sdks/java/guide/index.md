---
title: Java SDK Guide
---

# Java SDK Guide

## Integration Advice

- Manage the SDK version uniformly through Gradle or Maven
- Layer business handlers separately from connection configuration to avoid coupling them into startup classes
- When upgrading the protocol, check generated code and tests at the same time

## Concurrency Model

The Java SDK works best as a long-lived service component. Thread pools, timeouts, and retry policies should be controlled explicitly by the business side.

## Spring Boot Starter

`io.github.cuihairu.croupier:croupier-spring-boot-starter` exposes the same hand-written SDK as auto-configured beans. Connection settings live under the `croupier.*` property prefix (`game-id`, `service-id`, `agent-address`, `env`, `timeout-seconds`, `insecure`, `ca-file`, ...):

```yaml
croupier:
  game-id: my-game
  service-id: my-service
  agent-address: 127.0.0.1:19091
  # Provider handshake token (Bearer). Bound onto ClientConfig.authToken and
  # sent as the reserved authToken metadata key on ProviderConnectRequest;
  # the agent verifies it and never persists it into session metadata.
  auth-token: Bearer my-token
```

- Omitting `croupier.auth-token` leaves `ClientConfig.authToken` unset, and no `authToken` key is added to the handshake metadata.
- A user-defined `ClientConfig` bean wins: the auto-configuration backs off (`@ConditionalOnMissingBean`).
- Known boundary: only the fields declared in `CroupierProperties` are reachable from Spring; other `ClientConfig` fields (e.g. headers, heartbeat intervals) still require hand-written configuration.

## OpenAPI Import (Descriptor v2)

`OpenAPIImporter.registerFromOpenAPI(client, spec, options, resolver)` parses the OpenAPI 3 JSON locally, converts each operation into a `FunctionDescriptor`, and registers it. Handlers are looked up by function ID (i.e. the operationId, falling back to a path-derived `a.b.c` when missing):

```java
OpenAPIImporter.ImportOptions options = new OpenAPIImporter.ImportOptions()
    .resourcePrefix("game")
    .tagPrefix("svc-")
    .defaultTimeoutMs(30000)
    .continueOnError(true);

OpenAPIImporter.registerFromOpenAPIWithHandlers(
    client, specJson, options,
    Map.of("player_ban", (ctx, payload) -> "{\"ok\":true}"));
```

The conversion rules match the Go SDK: `inputSchema` comes from the requestBody's `application/json` schema, `outputSchema` from the 200 response schema; `x-resource`/`x-operation`/`x-permission`/`x-capability`/`x-execution`/`x-risk` map directly to the same-named fields; `x-approval: {required, policyKey}` maps to `approvalRequired`/`approvalPolicyKey`.

- `capability` enum: `collection_query|item_query|create|update|delete|action|task|report`
- `execution` enum: `sync|task`
- `risk` vocabulary: `safe|warning|high|danger`
- `defaultTimeoutMs` exists for Go contract alignment; the Java descriptor has no timeout field yet, so it is recorded but has no effect
- With `continueOnError` enabled, an operation missing a handler or failing registration is skipped and the import continues

## Agent Inbound Requests and Probes

The Agent periodically sends `ProviderHeartbeatRequest` probes to providers (provider keepalive). The SDK handles these out of the box: it replies with an empty `ProviderHeartbeatResponse` (pong), no business code required. Failing to answer probes makes the agent treat the session as dead and remove the registration.

When an unrecognized inbound message type arrives, the `CroupierException` thrown by the SDK carries the message type name (e.g. `Unsupported local request type: RegisterRequest (0x...)`) to make log triage easier.

## Keep Reading

- [API Reference](../api/)
