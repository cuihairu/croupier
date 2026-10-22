# Resources and Operations

This page explains how the C++ SDK expresses resource page-generation semantics through the descriptor v2. The current target model is `FunctionContract / ResourceCapability / CapabilitySemantics / PageSpec`.

A Resource is neither a database table nor a runtime object. It is only a stable key under which a group of functions are organized around the same business capability domain — e.g. wallet, inventory, orders, player profile. The SDK descriptor only supplies function capability contracts; page categories, dynamic labels, page types, and placement are decided in the Dashboard, not in the SDK.

## ID Reference Pattern

Functions should not pass large object instances as arguments; they should reference business objects by stable ID:

```cpp
invoke("wallet.transfer", {
  {"from_player_id", "player123"},
  {"to_player_id", "player456"},
  {"currency_code", "gold"},
  {"amount", "100.0"}
});
```

This keeps functions stateless, which enables horizontal scaling, permission auditing, and automated page generation.

## Descriptor v2 Essentials

| Field                                       | Description                                                                                                               |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `id`                                        | Unique function id, e.g. `wallet.transfer`                                                                                |
| `version`                                   | Function version                                                                                                          |
| `summary` / `description`                   | Function summary and detailed description                                                                                 |
| `input_schema` / `output_schema`            | JSON payload input/output contracts (member names; contract keys `inputSchema` / `outputSchema`])                         |
| `resource`                                  | Business resource or capability-domain key                                                                                |
| `operation`                                 | Business operation key                                                                                                    |
| `capability`                                | Controlled resource semantics, e.g. `collection_query`, `update`, `action`                                                |
| `execution`                                 | Execution semantics: `sync` / `task` (batch fan-out etc. use `task`)                                                      |
| `approval_required` / `approval_policy_key` | Approval switch and approval policy key for high-risk operations (contract keys `approvalRequired` / `approvalPolicyKey`) |
| `risk`                                      | Governance risk level                                                                                                     |

## Recommendations

- Default page generation derives from input/output contracts and CapabilitySemantics; the SDK may provide a controlled `capability` hint, and the frontend should not guess from function names.
- Dynamic categories, resource titles, action titles, and button placement must come from PageSpec, not from SDK descriptors or frontend static locale files.
- Use ID references, not heavyweight object instances.
- Keep functions stateless; look up objects through repositories or services.
- Keep descriptor IDs and operation keys stable.

## Related Pages
