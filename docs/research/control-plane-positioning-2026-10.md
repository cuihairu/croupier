# Croupier — Game Operations Control Plane 定位评审

## 状态

- 状态: 评审归档（用户 2026-10-04 架构评审意见，待落地为演进路线图）
- 日期: 2026-10-04
- 范围: 产品定位 / 核心模型（Scope / Target / Function / Task / Workflow）/ 传输边界 / 权限与审计 / SDK 战略 / Dashboard 方向

## 结论先行

Croupier 当前已不是简单的「GM 工具」，而是在收敛为**游戏运营控制平台（Game Operations Control Plane）**：
面向游戏研发、运营与基础设施的统一能力注册、远程调用、任务执行和运行控制平台。最有价值的抽象不是 Dashboard，也不是 GM CRUD，而是：**统一函数（能力）注册 → 调度 → 调用 → 结果**，配合 `scope = game_id + env` 与 `target` 分离。

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

## 一、核心抽象已经找到

README 中最重要的不是 "GM backend"，而是：

> 统一函数注册、调度、调用与作业模型

再配合：

- `scope = game_id + env`
- `target` 与 `scope` 分离

三者组合后 Croupier 开始具备平台味道。

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

### 不应该

把 `game_id / env / server_id / host / port / process` 全部塞成一个东西。

### 应该

```text
Scope      → "我操作的是谁？"
Target     → "这个能力最终在哪里执行？"
```

这是成熟控制面的基本思想。

## 二、真正要解决的不是「GM」

继续叫 "Universal GM Backend" 反而限制项目——当前能力已经明显超过 GM：

```text
GM:      add_item / kick_player / ban_player / change_level
运营:     send_mail / send_reward / announcement / event_control
运维:     restart / drain / reload / inspect / health / metrics
研发:     debug / invoke / test / profiling
自动化:   scheduled task / batch task / workflow
```

这些本质是同一个模型：

```text
Capability → Invoke → Execution → Result
```

核心概念应从 "GM Command" 提升为 "Function / Capability"。

## 三、Function 是最核心的资源

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

GM、运维、Debug、自动化在此模型下本质全部统一。

## 四、Function / Task / Workflow 三级模型

### 1. Function — 一次能力调用

```text
player.add_item
player.kick
server.reload
server.broadcast
```

### 2. Task — 一次执行实例

```text
Task #12345
  function: player.add_item
  target:   game-01
  status:   running
  progress: 43%
  created_at / started_at ...
```

状态机：`queued / running / success / failed / cancelled / timeout / partial_success`。

### 3. Workflow — 多个 Task / Function 编排

```text
活动开启 → disable matchmaking → reload config → broadcast announcement → grant reward → enable matchmaking
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

## 五、Agent 设计方向正确

```text
Server → outbound session → Agent → local session → Game Server
```

而非 `Server → TCP → Game Server`，更非 `Server → rpc_addr → Agent`。README 已明确清理 `rpc_addr / LocalControl / reverse direct connection` 等历史设计，是显著的架构进步。

## 六、Agent 是 Game Infrastructure Edge

- **网络**：Game VPC → Agent → Internet/管理网 → Server
- **安全**：Game Server 无需暴露公网
- **服务发现**：Server 无需知道 `10.1.2.31:9001` 之类的具体地址
- **连接管理**：heartbeat / reconnect / drain / backpressure
- **协议适配**：SDK / Game Server / third-party / local tools

类比：Kubernetes Control Plane → Agent → Node，只是 Croupier 面向游戏运营能力。

## 七、Session 设计较成熟

`TCP / TLS / framing / multiplexing / heartbeat / reconnect / drain / backpressure` 形成 Shared Session Runtime；`SDK-Agent` 与 `Agent-Server` 作为 Subprotocol。分层：

```text
Transport Runtime → Subprotocol → Business
```

不要再回到「每个 SDK 自己实现一套 TCP、每个 Agent 自己实现一套 RPC」。

## 八、不要让 Session 变成业务协议

保持严格分层：

```text
Session  = transport primitive
Function = business contract
Task     = execution instance
```

不要最终变成 `Session 里塞 Invoke / Register / Task / GM / Operation ...` 的巨型 RPC framework。

```text
┌───────────────────────────────┐
│ Business Protocol             │  Register / Invoke / Task ...
├───────────────────────────────┤
│ Session Runtime               │  framing / mux / heartbeat / reconnect / backpressure / drain
├───────────────────────────────┤
│ TCP / TLS                     │
└───────────────────────────────┘
```

## 九、protobuf Envelope + JSON payload：方向对，控制边界

- **Envelope 必须 protobuf**（version / request_id / session_id / type / payload 形态）
- **Payload 默认 JSON**，允许 `json / protobuf / msgpack / binary`，经 capability negotiation 支持——不要让 JSON 成为性能瓶颈

## 十、SDK 最大风险是「胖」

维护 6 语言 ×（transport × auth × invoke × task × error × reconnect × compatibility）成本爆炸。建议 SDK 分层：

```text
croupier protocol → transport SDK → language SDK → game integration
```

`proto/` 单源方向正确，protocol 必须是唯一真源。

## 十一、SDK 两层

- **L1 Transport SDK**：connect / register / heartbeat / invoke / stream / error / reconnect
- **L2 Game SDK**：例如 C++ 下 `croupier.register_function("player.add_item", schema, handler)`——游戏开发者无需接触 protobuf / TCP / session / mux / request_id

## 十二、权限升级：Principal + Action + Resource + Scope + Condition

现 RBAC/Audit 是必须的，但最终应向「谁、能做什么、对什么、在哪个游戏、哪个环境、满足什么条件」演进：

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

## 十三、Dangerous Operation 风险分级

游戏后台高频事故：发 100 万金币 / 删除角色 / 全服邮件 / 重启服务器 / 修改玩家属性 / 批量封号。Function 应有风险等级：

```yaml
risk:
  level: high
  confirmation: required
```

```text
LOW      查询
MEDIUM   单玩家修改
HIGH     批量修改
CRITICAL 全服操作
```

`CRITICAL → 二次确认 → 审批 → 执行`，甚至 two-person rule——非常适合生产游戏运营。

## 十四、Audit 不是「日志」，而是 Audit Event

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

回答：谁、什么时候、对哪个游戏、哪个环境、哪个目标、执行什么、参数是什么、结果是什么——这才是运营审计。

## 十五、Task 系统强化 Event Stream

现状（tasks / :id / :id/events / :id/cancel）很好，坚持：

```text
Task
 ├── state
 ├── progress
 ├── result
 ├── error
 └── events[]

queued / started / progress / log / warning / retry / completed / failed / cancelled
```

Dashboard 应做实时任务终端，而非每 3 秒轮询一次数据库。

## 十六、Agent 与 Target 严格区分

```text
Agent  = 网络与执行代理
Target = 可被操作的对象（Host / Container / Process / GameServer / Cluster / Service / Instance）

Agent SH-001
  ├── game-server-01
  ├── game-server-02
  └── redis-01
```

`server.reload` 不是 `Agent.reload`，而是 `resolve target → dispatch through agent`。

## 十七、Target Discovery

```text
Agent → register / heartbeat / report targets
Server → Agent → Targets → Capabilities
```

使系统可动态发现（如 Shanghai → Game A → gateway-01 / zone-01 / zone-02），成为真正的 Game Control Plane。

## 十八、Dashboard 不要做成传统 GM 菜单

避免：玩家管理 / 道具管理 / 邮件 / 公告 / 封禁 / 服务器。

建议：

```text
Operations: Overview / Functions / Tasks / Targets / Agents / Workflows / Audit / Access
```

**Functions 才是入口**：点 `player.add_item` 自动生成 Player ID / Item ID / Amount / Reason 表单——基于 JSON Schema + Ant Design Pro 已有基础。

## 十九、JSON Schema 是差异化主线

```yaml
id: player.add_item
input_schema:
  type: object
  properties:
    player_id: { type: string }
    item_id: { type: integer }
    amount: { type: integer, minimum: 1, maximum: 10000 }
```

Dashboard 自动渲染表单 + 校验 + 执行 → **Schema-driven Operations UI**。

## 二十、未来支持 UI Schema

```text
JSON Schema + UI Schema → Operations UI
```

例如 `widget: player-selector, source: player.search`——player_id 不再是文本框而是可搜索的玩家选择器。

## 二十一、与其他项目生态定位

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

Croupier 不应自己重新发明 RPC / Messaging / Game Server，而应把已有能力**暴露成可管理、可调用、可审计、可自动化的运营能力**。

## 二十二、优先级排序（参考）

| 优先级 | 工作                                               |   重要程度 |
| ------ | -------------------------------------------------- | ---------: |
| P0     | 固化 Function / Task / Target / Scope 四大核心模型 | ⭐⭐⭐⭐⭐ |
| P0     | 固化 Session Runtime / Subprotocol 边界            | ⭐⭐⭐⭐⭐ |
| P0     | 权限模型升级为 Resource + Action + Scope           | ⭐⭐⭐⭐⭐ |
| P0     | Audit Event 完整化                                 | ⭐⭐⭐⭐⭐ |
| P1     | Task Event Stream / Progress                       | ⭐⭐⭐⭐⭐ |
| P1     | Target / Agent Discovery                           |   ⭐⭐⭐⭐ |
| P1     | Schema-driven UI                                   |   ⭐⭐⭐⭐ |
| P2     | Workflow / Automation                              | ⭐⭐⭐⭐⭐ |

## 二十三、不建议继续疯狂增加 SDK

Go / C# / Java / C++ / Python / JS 已足够证明跨语言能力。重点从「再支持一种语言」转向「任何语言都能非常容易接入」——5 分钟让一个游戏 Server 注册 `player.get / player.add_item / player.kick` 并在 Dashboard 自动出现，比增加 Rust/Lua SDK 更重要。

## 二十四、定位表述建议

从 `Croupier is a universal GM backend system...` 调整为：

> **Croupier is a game operations control plane for registering, invoking, automating, and auditing capabilities across game servers and environments.**

> **Croupier 是面向游戏研发与运营的统一控制平面，用于注册、调用、编排和审计分布在不同游戏服务器与运行环境中的业务能力。**

Game Master / Live Operations / Server Operations / Developer Tools / Automation 均成为其下的 use cases。

## 二十五、总体评价

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

**最大的问题不是代码，而是产品概念还未完全收敛。** 核心建议：把 Scope → Target → Function → Task → Workflow 这条主线彻底钉死，让代码、Proto、API、Dashboard、SDK、文档全部围绕它们统一。
