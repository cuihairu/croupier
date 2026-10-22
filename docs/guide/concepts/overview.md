---
title: System Overview
icon: circle-info
order: 1
category:
  - Core Concepts
tag:
  - Architecture
  - Overview
---

# System Overview

Croupier is a Server / Agent / SDK platform for game operations and control scenarios.
The current architecture has explicitly converged on a "unified session transport":

- `Agent <-> Server`: `TCP session + TLS`
- `SDK <-> Agent`: `TCP session`, with `TLS` disabled by default

In terms of the business model, Croupier by default serves multiple games and multiple environments inside a single game company:

- The standard business boundary is `gameId + env` (`game_id` at the proto/DB layer, HTTP header `X-Game-ID`)
- `gameId` expresses game ownership
- `env` expresses the logical environment, e.g. `dev`, `test`, `staging`, `prod`
- `env` does not directly equal a database, node, or cluster
- Deployment location should be expressed through separate concepts such as `target`, `agent`, and `node`

## Core Characteristics

| Feature                      | Description                                                                                                 |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------- |
| Unified session transport    | Bidirectional requests, reconnect, heartbeat, drain, backpressure                                           |
| Function-registration driven | SDK / Agent reports function/provider/process capabilities                                                  |
| Contract-driven console      | OpenAPI / JSON Schema provides capability contracts; the platform generates publishable ProComponents pages |
| JSON payload                 | User business data is JSON by default                                                                       |
| protobuf envelope            | Platform control fields and message routing uniformly use protobuf                                          |

## Key Components

### Server

- Stores the registry
- Handles RBAC / approval / audit
- Routes invoke/task/ops requests
- Holds the `Agent session`

### Agent

- Actively connects to the `Server`
- Integrates locally with `GameServer / SDK / third-party applications`
- Maintains sessions on both the local and upstream sides

### SDK

- Acts as an embedded client and actively connects to the `Agent`
- Does not listen on a local port by default
- Exposes function capabilities through a provider session

## Key Terms

### shared session runtime

Refers to the shared session runtime foundation:

- `tcp/tls`
- framing
- request/response multiplexing
- reconnect
- heartbeat
- drain
- backpressure

### subprotocol

Refers to the concrete subprotocols running on top of the shared session runtime:

- `sdk-agent subprotocol`
- `agent-server subprotocol`

It is not a "configuration template" but "application-layer protocol variants on different boundaries".

## UI Layering: Function Catalog vs Page Studio vs Runtime Console

Croupier's admin interface splits the "Functions" module into three layers, each with distinct, non-overlapping responsibilities:

| Layer                | Positioning                | Data source                            | Typical operations                                                                                                                |
| -------------------- | -------------------------- | -------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Function Catalog** | Capability supply layer    | FunctionContract / function registry   | Confirm whether a function is registered, whether the Schema is correct, whether callable instances exist, single-function invoke |
| **Resource Catalog** | Capability semantics layer | FunctionContract / CapabilitySemantics | Review resources, CRUD/task/report semantics and generation diagnostics                                                           |
| **Page Studio**      | Page assembly layer        | PageProposal / PageDraft               | Accept default pages, semantic editing, preview, publish, merge, rollback                                                         |
| **Runtime Console**  | Execution layer            | PublishedPageSpec / ConsoleMenuSpec    | End users / operators execute business operations                                                                                 |

The three form one main pipeline:

```
Function registration → Function Catalog (confirm capabilities) → Resource Catalog (confirm semantics) → Page Studio (assemble pages) → Runtime Console (execute)
```

- The Function Catalog only handles a single function's metadata, Schema, instances, and single-function invocation; it does not handle final business pages.
- Page Studio composes multiple functions into ResourcePage, OperationPage, TaskPage, or ReportPage, and manages drafts, published versions, and rollback.
- CRUD Resources are the default high-quality path; non-CRUD functions also get their own Operation/Task/Report pages and are never forced into a resource page.
- Default pages are generated by the Server as Proposals from FunctionContract / CapabilitySemantics; users can publish directly or edit locally.
- The Runtime Console only displays published PageSpecs; the dynamic menu does not depend on static i18n files.

See also [Function Management](./function-management.md), [Function Registration and the Default UI](./function-registration-ui.md), and the [Dashboard Resource/Page Model](../../architecture/dashboard-page-model.md).

## Core Dashboard Concepts

The Dashboard currently has a single target model:

```text
FunctionContract -> ResourceCapability -> CapabilitySemantics -> PageProposal -> PageSpec -> PublishedPageSpec -> ConsoleMenuSpec
```

### FunctionContract

`FunctionContract` is the normalized capability description produced after function registration. It states "what functions exist, how to invoke them, what the inputs/outputs are, and what the risks are".

It is not a page and does not determine the left-side menu.

### ResourceCapability

`ResourceCapability` is the set of resource capabilities the Dashboard uses to organize pages, e.g. `player`, `mail`, `inventory`, `analytics`.

A Resource can carry CRUD operations as well as game-operations actions such as banning, granting rewards, sending mail, and order compensation.

### CapabilitySemantics

`CapabilitySemantics` describes verifiable business usage such as collection, identity, CRUD, action, task, and report; it does not store final page locations, columns, or menus.

The SDK/OpenAPI `operation` remains just a business-action key. The SDK/OpenAPI may provide a controlled `capability`, but not menus, columns, mappings, button positions, or conditional display fields; page type, action placement, and localized titles can only be expressed by PageSpec.

### PageSpec

`PageSpec` is the strongly typed business page orchestration, covering the query area, pagination, tables, details, modals, batch operations, task status, and report charts. It contains no React component tree; display is handled by the ProComponents renderer.

A Page can compose multiple functions. Paginated queries belong to the Page's state and field mapping, not to a single function's input form.

### Page Studio

Page Studio manages PageProposal and PageSpec drafts. It can show Server-generated default page suggestions, allow direct publishing, and allow users to semantically edit, preview, validate, merge, publish, and roll back.

Page Studio is not the Runtime Console. Unpublished drafts must not appear in the Runtime Console's left-side menu.

### Runtime Console

The Runtime Console consumes only published pages:

```text
PublishedPageSpec[] -> ConsoleMenuSpec
```

Dynamic categories, page titles, and menu localization come from the strongly typed fields of PublishedPageSpec. Frontend static locale files are only used for fixed system menus.

## Current Boundaries

- Agents and SDKs connect through the session model; Server calls reuse the established link.
- The Resource API manages capability semantics and does not directly modify business object data.
- PageSpec is the sole page protocol of the Runtime Console; dynamic menu categories come from PublishedPageSpec.
- Static i18n files only cover fixed system menus and carry no dynamic page facts.
