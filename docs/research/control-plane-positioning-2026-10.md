# Croupier — Game Operations Control Plane Positioning Review

## Status

- Status: review archive (the user's 2026-10-04 architecture review comments, pending landing as an evolution roadmap)
- Date: 2026-10-04
- Scope: product positioning / the core model (Scope / Target / Function / Task / Workflow) / the transport boundary / permissions and audit / SDK strategy / the dashboard direction

## Conclusions First

Croupier is no longer a simple "GM tool"; it is converging into a **Game Operations Control Plane**:
a unified capability registration, remote invocation, task execution, and runtime-control platform for game R&D, operations, and infrastructure. The most valuable abstraction is neither the dashboard nor GM CRUD but: **unified function (capability) registration → scheduling → invocation → result**, paired with the `scope = game_id + env` and `target` separation.

```text
                 ┌──────────────────────┐
                 │       Dashboard      │
                 │  Operator / Developer│
                 └──────────┬───────────┘
                            │ HTTP / SSE
                            ▼
                 ┌──────────────────────┐
                 │        Server        │
                 │                      │
                 │ Registry             │
                 │ Dispatch             │
                 │ RBAC                 │
                 │ Audit                │
                 │ Task                 │
                 │ Scope                │
                 └──────────┬───────────┘
                            │
                    TCP Session + TLS
                            │
            ┌───────────────┴───────────────┐
            ▼                               ▼
     ┌─────────────┐                 ┌─────────────┐
     │    Agent    │                 │    Agent    │
     │ Game VPC    │                 │ Game VPC    │
     └──────┬──────┘                 └──────┬──────┘
            │ TCP Session                   │
      ┌─────┼─────┐                   ┌─────┼─────┐
      ▼     ▼     ▼                   ▼     ▼     ▼
     GS    GS    Tool                 GS    GS    Tool
```

## 1. The Core Abstraction Has Been Found

The most important thing in the README is not "GM backend" but:

> Unified function registration, scheduling, invocation, and a job model

Combined with:

- `scope = game_id + env`
- `target` separated from `scope`

The combination of the three is when Croupier starts smelling like a platform.

```text
scope
  ├── game
  └── environment

target
  ├── cluster
  ├── node
  ├── process
  └── agent
```

### What Not To Do

Cram `game_id / env / server_id / host / port / process` into one thing.

### What To Do

```text
Scope      → "Whose behalf am I operating on?"
Target     → "Where does this capability ultimately execute?"
```

This is the basic idea of a mature control plane.

## 2. The Real Problem to Solve Is Not "GM"

Continuing to call it a "Universal GM Backend" actually constrains the project — the current capabilities clearly exceed GM:

```text
GM:        add_item / kick_player / ban_player / change_level
Ops:       send_mail / send_reward / announcement / event_control
Infra:     restart / drain / reload / inspect / health / metrics
Dev:       debug / invoke / test / profiling
Automation: scheduled task / batch task / workflow
```

These are essentially one model:

```text
Capability → Invoke → Execution → Result
```

The core concept should be elevated from "GM command" to "function / capability".

## 3. The Function Is the Most Core Resource

```yaml
function:
  id: player.add_item

  scope:
    game: foo
    env: prod

  target:
    type: game_server

  input:
    schema: ...

  output:
    schema: ...

  execution:
    mode: sync

  permissions:
    - player.write
```

```text
Dashboard → invoke → Server → resolve → Target → dispatch → Agent → invoke → Game Server
```

Under this model, GM, infra, debug, and automation are all essentially unified.

## 4. The Function / Task / Workflow Three-Level Model

### 1. Function — One Capability Invocation

```text
player.add_item
player.kick
server.reload
server.broadcast
```

### 2. Task — One Execution Instance

```text
Task #12345
  function: player.add_item
  target:   game-01
  status:   running
  progress: 43%
  created_at / started_at ...
```

State machine: `queued / running / success / failed / cancelled / timeout / partial_success`.

### 3. Workflow — Orchestrating Multiple Tasks / Functions

```text
event starts → disable matchmaking → reload config → broadcast announcement → grant reward → enable matchmaking
```

```text
Workflow
   ├── Function A
   ├── Function B
   │      ├── retry
   │      └── timeout
   ├── Function C
   └── Function D
```

## 5. The Agent Design Direction Is Correct

```text
Server → outbound session → Agent → local session → Game Server
```

Rather than `Server → TCP → Game Server`, and even less `Server → rpc_addr → Agent`. The README has explicitly cleaned up the historical designs of `rpc_addr / LocalControl / reverse direct connection` — a notable architectural advance.

## 6. The Agent Is the Game Infrastructure Edge

- **Network**: the game VPC → Agent → the internet/management network → Server
- **Security**: game servers need no public exposure
- **Service discovery**: the server never needs to know concrete addresses like `10.1.2.31:9001`
- **Connection management**: heartbeat / reconnect / drain / backpressure
- **Protocol adaptation**: SDK / game server / third-party / local tools

Analogy: the Kubernetes control plane → Agent → Node, except Croupier faces game-operations capabilities.

## 7. The Session Design Is Fairly Mature

`TCP / TLS / framing / multiplexing / heartbeat / reconnect / drain / backpressure` form a Shared Session Runtime; `SDK-Agent` and `Agent-Server` act as subprotocols. The layering:

```text
Transport Runtime → Subprotocol → Business
```

Never go back to "every SDK implements its own TCP and every agent implements its own RPC".

## 8. Do Not Let the Session Become a Business Protocol

Keep the layering strict:

```text
Session  = transport primitive
Function = business contract
Task     = execution instance
```

Do not end up as a giant RPC framework with `invoke / register / task / GM / operation ...` stuffed into the session.

```text
┌───────────────────────────────┐
│ Business Protocol             │  Register / Invoke / Task ...
├───────────────────────────────┤
│ Session Runtime               │  framing / mux / heartbeat / reconnect / backpressure / drain
├───────────────────────────────┤
│ TCP / TLS                     │
└───────────────────────────────┘
```

## 9. The protobuf Envelope + JSON Payload: The Right Direction; Control the Boundary

- **The envelope must be protobuf** (version / request_id / session_id / type / payload shape)
- **The payload defaults to JSON**, allowing `json / protobuf / msgpack / binary` via capability negotiation — do not let JSON become the performance bottleneck

## 10. The SDK's Biggest Risk Is Being "Fat"

Maintaining 6 languages × (transport × auth × invoke × task × error × reconnect × compatibility) is an exploding cost. Recommended SDK layering:

```text
croupier protocol → transport SDK → language SDK → game integration
```

The single-source `proto/` direction is correct; the protocol must be the only source of truth.

## 11. The SDK's Two Layers

- **L1 Transport SDK**: connect / register / heartbeat / invoke / stream / error / reconnect
- **L2 Game SDK**: e.g. in C++, `croupier.register_function("player.add_item", schema, handler)` — game developers never touch protobuf / TCP / session / mux / request_id

## 12. Permissions Upgrade: Principal + Action + Resource + Scope + Condition

The current RBAC/audit is necessary, but ultimately it should evolve toward "who, can do what, to what, in which game, in which environment, satisfying which condition":

```text
Alice
  allow player.add_item
  game: xxx
  env: production
  condition: amount <= 1000

Bob
  allow server.reload
  env: dev
  production: DENY
```

## 13. Risk-Grading Dangerous Operations

High-frequency game-backoffice incidents: granting 1 million gold / deleting a character / a server-wide mail / restarting the server / modifying player attributes / batch bans. Functions should carry a risk level:

```yaml
risk:
  level: high
  confirmation: required
```

```text
LOW      queries
MEDIUM   single-player modifications
HIGH     batch modifications
CRITICAL server-wide operations
```

`CRITICAL → second confirmation → approval → execution`, even a two-person rule — a great fit for production game operations.

## 14. Audit Is Not "Logs" but Audit Events

```json
{
  "actor": "admin",
  "action": "player.add_item",
  "game": "foo",
  "env": "prod",
  "target": "zone-03",
  "request_id": "...",
  "task_id": "...",
  "parameters": "...",
  "result": "...",
  "timestamp": "..."
}
```

Answering: who, when, on which game, in which environment, on which target, executed what, with which parameters, resulting in what — that is operations auditing.

## 15. The Task System Should Strengthen the Event Stream

The current state (tasks / :id / :id/events / :id/cancel) is good; hold the line:

```text
Task
 ├── state
 ├── progress
 ├── result
 ├── error
 └── events[]

queued / started / progress / log / warning / retry / completed / failed / cancelled
```

The dashboard should be a live task terminal, not a database poll every 3 seconds.

## 16. Strictly Distinguish Agent from Target

```text
Agent  = the network and execution proxy
Target = the operable object (Host / Container / Process / GameServer / Cluster / Service / Instance)

Agent SH-001
  ├── game-server-01
  ├── game-server-02
  └── redis-01
```

`server.reload` is not `Agent.reload`; it is `resolve target → dispatch through agent`.

## 17. Target Discovery

```text
Agent → register / heartbeat / report targets
Server → Agent → Targets → Capabilities
```

This makes the system dynamically discoverable (e.g. Shanghai → Game A → gateway-01 / zone-01 / zone-02), becoming a true game control plane.

## 18. Do Not Build the Dashboard as a Traditional GM Menu

Avoid: player management / item management / mail / announcements / bans / servers.

Suggestion:

```text
Operations: Overview / Functions / Tasks / Targets / Agents / Workflows / Audit / Access
```

**Functions are the entry point**: clicking `player.add_item` auto-generates a Player ID / Item ID / Amount / Reason form — the JSON Schema + Ant Design Pro foundation already exists.

## 19. JSON Schema Is the Differentiating Main Line

```yaml
id: player.add_item
input_schema:
  type: object
  properties:
    player_id: { type: string }
    item_id: { type: integer }
    amount: { type: integer, minimum: 1, maximum: 10000 }
```

The dashboard auto-renders the form + validation + execution → a **schema-driven operations UI**.

## 20. Support a UI Schema in the Future

```text
JSON Schema + UI Schema → Operations UI
```

For example `widget: player-selector, source: player.search` — player_id is no longer a text box but a searchable player selector.

## 21. Positioning Within the Sibling-Project Ecosystem

```text
                     Croupier
                Game Operations Control Plane
                       │
        ┌──────────────┼──────────────┐
        │              │              │
      Chirp           Shield         Cockpit
   Communication      RPC/Codec     Control/UI
        │              │              │
        └──────────────┼──────────────┘
                       │
                  Game Servers
```

Croupier should not reinvent RPC / messaging / the game server itself; it should **expose existing capabilities as manageable, invocable, auditable, automatable operations capabilities**.

## 22. Priority Ordering (Reference)

| Priority | Work                                                             | Importance |
| -------- | ---------------------------------------------------------------- | ---------: |
| P0       | Nail down the four core models: Function / Task / Target / Scope | ⭐⭐⭐⭐⭐ |
| P0       | Nail down the Session Runtime / Subprotocol boundary             | ⭐⭐⭐⭐⭐ |
| P0       | Upgrade the permission model to Resource + Action + Scope        | ⭐⭐⭐⭐⭐ |
| P0       | Complete the audit event                                         | ⭐⭐⭐⭐⭐ |
| P1       | Task event stream / progress                                     | ⭐⭐⭐⭐⭐ |
| P1       | Target / Agent discovery                                         |   ⭐⭐⭐⭐ |
| P1       | Schema-driven UI                                                 |   ⭐⭐⭐⭐ |
| P2       | Workflow / automation                                            | ⭐⭐⭐⭐⭐ |

## 23. Do Not Keep Frantically Adding SDKs

Go / C# / Java / C++ / Python / JS already prove the cross-language capability amply. Shift the focus from "support one more language" to "any language can integrate very easily" — getting a game server to register `player.get / player.add_item / player.kick` in 5 minutes and see them appear automatically on the dashboard matters more than adding a Rust/Lua SDK.

## 24. Positioning Statement Suggestion

Adjust from `Croupier is a universal GM backend system...` to:

> **Croupier is a game operations control plane for registering, invoking, automating, and auditing capabilities across game servers and environments.**

> **Croupier 是面向游戏研发与运营的统一控制平面，用于注册、调用、编排和审计分布在不同游戏服务器与运行环境中的业务能力。**

Game Master / Live Operations / Server Operations / Developer Tools / Automation all become use cases beneath it.

## 25. Overall Assessment

```text
架构思想      9/10
传输层设计    8.5/10
Scope 模型    9/10
Agent 模型    9/10
SDK 战略      8/10
Task 模型     8/10
权限          7/10
Audit         7/10
Workflow      5/10
Target 模型   7/10
产品定位      7.5/10
```

**The biggest problem is not the code but that the product concepts have not fully converged.** The core recommendation: nail down the main line Scope → Target → Function → Task → Workflow completely, and unify the code, proto, API, dashboard, SDK, and docs all around them.
