# Croupier - 游戏GM后台系统

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.24+-green.svg)
![Status](https://img.shields.io/badge/status-in%20development-yellow.svg)

Croupier 是一个专为游戏运营设计的通用 GM 后台系统，支持多语言游戏服务器接入，提供统一的管理界面与强大的扩展能力。

## 🧠 设计理念

Croupier 围绕**"让游戏运营既安全又高效"**的核心目标设计，将传统单体 GM 后台拆分为三个独立但协同的层次：

### **1. 权限控制层（独立的安全基座）**
- **独立的权限模型**：RBAC/ABAC 权限系统完全独立于游戏逻辑
- **统一的安全策略**：所有游戏、所有环境共享同一套权限框架
- **多层安全机制**：身份认证、授权控制、操作审批、审计追踪

### **2. 游戏控制层（函数注册驱动）**
- **函数注册机制**：游戏服务器向 Agent 注册可调用函数
- **标准化接口**：所有游戏操作抽象为统一的函数调用模型
- **业务逻辑内聚**：游戏相关的控制逻辑完全在游戏服务器内部
- **作用域隔离**：`game_id`/`env` 确保不同游戏/环境的逻辑隔离

### **3. 基于 X-Render 的动态展示层 ✨**
- **X-Render 驱动 UI**：基于阿里巴巴 X-Render 框架，JSON Schema 自动生成动态表单
- **零代码运营界面**：运营人员无需开发即可获得专业管理界面
- **智能表单系统**：自动参数校验、敏感字段脱敏、风险标识、进度追踪
- **可观测性集成**：指标、日志、链路追踪统一展示

**技术选型**：Form-Render + Ant Design 5.x + TypeScript + JSON Schema 驱动

**核心理念**：*让专业的人做专业的事* - 平台专注安全和基础设施，游戏专注业务逻辑实现，运营专注策略和执行。

## 🎯 核心特性

- 🔐 **安全传输**：gRPC + mTLS 双向身份与加密传输（HTTP/2/443），内置重试/流控
- 🧩 **IDL 生成**：以 Proto 定义服务与消息，生成多语言 SDK（Go/Java/C++/Python）
- ✨ **X-Render 驱动 UI**：基于阿里巴巴 X-Render 框架，JSON Schema 自动生成动态表单界面
- 📡 **实时流式**：支持长任务进度/日志流、订阅/推送
- 🛰️ **Agent 外连**：内网仅出站至 DMZ/Server，无需内网入站；多服务多路复用一条长连
- 🔑 **细粒度权限**：功能级/资源级/环境级 RBAC/ABAC，支持高危操作双人审批与审计
- 🧪 **易扩展**：Function 版本化与兼容协商、幂等键、灰度/回滚
- 🎨 **现代前端**：基于 Ant Design 5.x + TypeScript + X-Render 的企业级管理界面

## 🏗️ 系统架构

### 整体架构图（采集与控制面解耦）

```mermaid
graph TB
  subgraph "客户端"
    Client[游戏客户端<br/>iOS/Android/Web]
  end

  subgraph "管理控制层（内网）"
    UI[Web 管理界面<br/>Ant Design + TypeScript]
    Server[Croupier Server<br/>控制面/权限/查询]
  end

  subgraph "DMZ/公网"
    Edge[Edge（可选）<br/>控制面转发]
    Ingest[Analytics Ingestion<br/>HTTP/OTLP · CDN/WAF/限流]
    OtelColPub[OTel Collector<br/>公共/DMZ接入 可选]
  end

  subgraph "分布式代理层（游戏内网）"
    A1[Croupier Agent 1]
    A2[Croupier Agent 2]
  end

  subgraph "游戏服务层（游戏内网）"
    GS1[Game Server A + SDK<br/>+SimpleAnalytics]
    GS2[Game Server B + SDK<br/>+OTel Integration]
    GS3[Game Server C + SDK]
    GS4[Game Server D + SDK]
  end

  subgraph "数据处理层（内网）"
    Redis[(Redis Streams<br/>analytics:events<br/>analytics:payments)]
    Worker[Analytics Worker Group<br/>实时数据处理]
  end

  subgraph "存储观测层（内网）"
    ClickHouse[(ClickHouse<br/>分析数据存储)]
    Jaeger[Jaeger<br/>分布式追踪]
    Prometheus[Prometheus<br/>指标收集]
    Grafana[Grafana<br/>可视化面板]
  end

  %% 控制面
  UI -->|HTTP REST| Server
  Server -->|gRPC mTLS| A1
  Server -->|gRPC mTLS| A2
  Server -->|可选| Edge
  Edge -->|gRPC mTLS| A1
  Edge -->|gRPC mTLS| A2

  %% 客户端数据上报（公网）
  Client -->|HTTPS| Ingest

  %% 服务器端数据（游戏内网）
  GS1 -->|SDK 事件| Redis
  GS2 -->|OTLP/HTTP| OtelColPub

  %% 数据管道
  Ingest -->|写入| Redis
  OtelColPub -- "导出事件(可选)" --> Redis
  Redis -->|stream consume| Worker
  Worker -->|batch insert| ClickHouse

  %% 观测性
  OtelColPub -->|traces| Jaeger
  OtelColPub -->|metrics| Prometheus
  Prometheus --> Grafana
  Jaeger --> Grafana
  ClickHouse --> Grafana

  classDef ui fill:#e8f5ff,stroke:#1890ff
  classDef server fill:#f6ffed,stroke:#52c41a
  classDef agent fill:#f6ffed,stroke:#52c41a
  classDef game fill:#fff7e6,stroke:#fa8c16
  classDef data fill:#f0f9e6,stroke:#52c41a
  classDef storage fill:#f9f0ff,stroke:#722ed1
  classDef dmz fill:#fffbe6,stroke:#faad14

  class UI ui
  class Server server
  class A1,A2 agent
  class GS1,GS2,GS3,GS4 game
  class Redis,Worker data
  class ClickHouse,Jaeger,Prometheus,Grafana storage
  class Edge,Ingest,OtelColPub dmz
```

### 调用与数据流
- 控制面：Query（查询）同步返回；Command（命令）异步返回 `job_id`。长任务通过流式接口返回进度/日志，可取消/重试，保证幂等（`idempotency-key`）。
- 采集面：客户端事件走 `Analytics Ingestion`（公网/DMZ，前置 CDN/WAF/签名/限流）；服务端遥测走 `OTel Collector`（traces/metrics）。
- Server 常驻内网，不对公网提供数据上报入口；采集与控制面解耦，统一写入 MQ（Redis/Kafka）后由 `Analytics Worker` 入库 ClickHouse。
- 所有函数字段由 Descriptor（JSON Schema）定义，UI/校验/鉴权共享同一描述；HTTP 层通过 `X-Game-ID`/`X-Env` 透传至南向调用。

```mermaid
sequenceDiagram
  participant UI as Web UI
  participant Server as Server
  participant Edge as Edge Optional
  participant Agent as Agent
  participant GS as Game Server
  UI->>Server: POST /api/invoke {function_id, payload, X-Game-ID}
  alt Server 直连
    Server->>Agent: FunctionService.Invoke
  else Server 经 Edge 转发
    Server->>Edge: Forward Invoke
    Edge->>Agent: Tunnel Invoke (bidi)
  end
  Agent->>GS: local gRPC Invoke
  GS-->>Agent: response
  Agent-->>Server: response (via Edge/直连)
  Server-->>UI: result
```

## 🚀 快速开始

### 模式 1：游戏分析快速集成（5分钟）

适用于快速接入游戏分析和监控功能。

```bash
# 1) 启动 Croupier Server（已集成 Analytics API）
./croupier server --config configs/server.example.yaml

# 2) 配置 Analytics 环境变量
export ANALYTICS_MQ_TYPE=redis
export REDIS_URL=redis://localhost:6379/0

# 3) 启动 Analytics Worker
./analytics-worker

# 4) 游戏服务器中集成 SimpleAnalytics（Go示例）
```

```go
import "github.com/cuihairu/croupier/examples/otel-integration/internal/telemetry"

// 初始化（一次性）
telemetry.Init(telemetry.SimpleConfig{
    GameID:    "my-game",
    ServerURL: "http://localhost:8080",
})

// 发送事件（随时调用）
telemetry.Login("user123", "ios", "cn-north")
telemetry.StartLevel("user123", "session456", "level-1", "tutorial")
telemetry.Buy("user123", "order789", "coin_pack", 0.99, "USD", true)
```

**🎮 完整演示：**
```bash
cd examples/otel-integration
make demo-simple  # 一键体验完整流程
```

### 模式 2：OpenTelemetry 标准集成

适用于需要完整可观测性功能（traces + metrics + logs）的场景。

```bash
# 1) 启动完整 OTel 环境
cd examples/otel-integration
make start

# 2) 验证服务
make health-check

# 3) 运行演示
make demo

# 访问监控界面
# Grafana: http://localhost:3000 (admin/admin)
# Jaeger: http://localhost:16686
# Prometheus: http://localhost:9090
```

### 模式 3：传统部署（直连，简化）

适用于 Server 与 Game 在同一内网且允许直连的场景（仍建议使用 mTLS）。

```bash
# 1) 启动 Server
./croupier server \
  --addr :8443 --http_addr :8080 \
  --rbac_config configs/rbac.json --games_config configs/games.json --users_config configs/users.json \
  --cert configs/dev/server.crt --key configs/dev/server.key --ca configs/dev/ca.crt

# 2) 游戏服务器 SDK 直接连接 Server（gRPC/mTLS）
./game-server
```

提示：
- 已支持使用 `--config` 指定 YAML 配置文件，或通过环境变量覆盖（前缀 `CROUPIER_SERVER_` 和 `CROUPIER_AGENT_`，例如 `CROUPIER_SERVER_ADDR=":8443"`、`CROUPIER_AGENT_SERVER_ADDR="127.0.0.1:8443"`）。
- 日志：新增 `--log.level`（debug|info|warn|error）、`--log.format`（console|json）与 `--log.file`（启用滚动日志：`--log.max_size`/`--log.max_backups`/`--log.max_age`/`--log.compress`）。
- 首次启动种子数据（开发环境）：当数据库为空时，Server 会自动插入一个“示例游戏（Example Game）”，包含 `dev/staging/production` 三个环境，图标使用 Iconify 公共图标（mdi:gamepad-variant）。如需关闭，设置环境变量 `SEED_EXAMPLE_GAME=false`。
- 数据库：支持 Postgres/MySQL/SQLite/SQL Server 多后端，参见“数据库配置”一节。
可参考 `configs/server.example.yaml` 与 `configs/agent.example.yaml`。

配置分层与叠加（profiles/include）
- 支持 `--config-include` 顺序叠加多个配置文件（后者覆盖前者）。
- 支持在 YAML `server:`/`agent:` 下声明 `profiles:` 子段，并用 `--profile` 选择叠加（优先级低于环境变量与命令行）。
  例如：
  ```yaml
  server:
    addr: :8443
    log: { level: debug }
    profiles:
      prod:
        log: { level: info, format: json, file: logs/server.log }
  ```
  启动：`./croupier server --config base.yaml --config-include overlay.yaml --profile prod`

指标导出
- JSON：Server `/metrics`、Agent `/metrics`、Edge `/metrics` 都包含 `logs` 统计。
- Prometheus 文本：Server `/metrics.prom`，Agent `/metrics.prom`，Edge `/metrics.prom`。
  - Server：`croupier_invocations_total`、`croupier_invocations_error_total`、`croupier_jobs_started_total`、`croupier_jobs_error_total`、`croupier_rbac_denied_total`、`croupier_audit_errors_total`、`croupier_logs_total{level=...}`
  - Agent：`croupier_agent_instances`、`croupier_tunnel_reconnects`、`croupier_logs_total{level=...}`
 - Edge：`croupier_logs_total{level=...}`

## 数据库配置

支持的驱动：`postgres` | `mysql` | `sqlite` | `mssql` | `sqlserver` | `auto`

配置方式 1：YAML（推荐）

```yaml
server:
  db:
    driver: auto   # postgres | mysql | sqlite | mssql | sqlserver | auto
    dsn: ""       # Postgres:   postgres://user:pass@host:5432/croupier?sslmode=disable
                   # MySQL(URL): mysql://user:pass@host:3306/croupier?charset=utf8mb4
                   # MySQL(DSN): user:pass@tcp(host:3306)/croupier?parseTime=true&charset=utf8mb4
                   # SQL Server: sqlserver://user:pass@host:1433?database=croupier
                   # SQLite:     file:data/croupier.db
```

配置方式 2：环境变量

```bash
export DB_DRIVER=sqlserver
export DATABASE_URL="sqlserver://user:pass@localhost:1433?database=croupier"
```

注意事项（SQL Server）：
- 建议在连接串指定 `database`，默认端口 1433；启用 TCP 连接
- GORM 驱动：`gorm.io/driver/sqlserver`；本项目已引入
- JSON 字段在某些版本上为 `NVARCHAR` 存储（由 GORM 扩展类型管理），若需复杂 JSON 检索建议在 PG/MySQL 使用 JSONB/JSON
- 权限/登录建议使用 SQL 认证（默认混合模式）

注意事项（MySQL）：
- DSN 需包含 `parseTime=true`；字符集推荐 `utf8mb4`
- 建议使用 InnoDB 存储引擎

### 模式 2：Agent 外连（推荐）

Server 位于 DMZ/公网，Agent 在游戏内网，仅出站到 Server。游戏服只连本机/就近 Agent。

```bash
# 1) DMZ 启动 Server（统一 CLI）
./croupier server \
  --addr :8443 --http_addr :8080 \
  --rbac_config configs/rbac.json --games_config configs/games.json --users_config configs/users.json \
  --cert configs/dev/server.crt --key configs/dev/server.key --ca configs/dev/ca.crt

# 2) 内网启动 Agent
./croupier agent \
  --local_addr :19090 --server_addr 127.0.0.1:8443 --game_id default --env dev \
  --cert configs/dev/agent.crt --key configs/dev/agent.key --ca configs/dev/ca.crt

# 3) 游戏服务器连接本机 Agent（gRPC）
./game-server
```

```mermaid
graph LR
  subgraph DMZ[DMZ/公网]
    Server[Croupier Server]
  end
  subgraph NETA[游戏内网]
    A[Croupier Agent]
    GS[Game Servers + SDK]
  end
  A -->|gRPC mTLS 443 outbound| Server
  GS -->|local gRPC multi-instance| A
  classDef server fill:#e8f5ff,stroke:#1890ff;
  classDef agent fill:#f6ffed,stroke:#52c41a;
  class Server server
  class A agent
```
### 模式 3：Edge 转发（Server 在内网）

适用于 Server 无法部署在 DMZ/公网、又需要管理多条游戏内网的场景。

思路：在 DMZ/公网部署轻量 Edge，所有 Agent 主动外连 Edge；Server 从企业内网“仅出站”连到 Edge（mTLS/443），由 Edge 转发请求与路由。

```mermaid
graph LR
  subgraph INTRANET[企业内网]
    Server[Croupier Server]
  end
  subgraph DMZ[DMZ/公网]
    Edge[Croupier Edge]
  end
  subgraph GNETA[游戏内网A]
    A1[Croupier Agent]
    GS1[Game Servers + SDK]
  end
  Server -->|gRPC mTLS 443 outbound| Edge
  A1 -->|gRPC mTLS 443 outbound| Edge
  GS1 -->|local gRPC multi-instance| A1
  classDef server fill:#e8f5ff,stroke:#1890ff;
  classDef agent fill:#f6ffed,stroke:#52c41a;
  classDef edge fill:#fffbe6,stroke:#faad14;
  class Server server
  class A1 agent
  class Edge edge
```

运行流程（PoC 设计）：
- Edge：监听 9443，接受 Agent 外连并注册（ControlService）；同时暴露 FunctionService，对 Server 作为调用入口并转发到 Agent。
- Server：使用 `--edge_addr` 将 FunctionService 调用转发到 Edge；HTTP/UI 不变。
- Agent：将 `--server_addr` 指向 Edge 地址，实现"仅外连"注册。


### 命令行快速验证（示例）

```bash
# 登录获取 token（默认示例用户）
curl -sS http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | jq -r .token | tee /tmp/token

# 查看当前用户
curl -sS http://localhost:8080/api/v1/users/current \
  -H "Authorization: Bearer $(cat /tmp/token)" | jq

# 查看可用的函数描述符
curl -sS http://localhost:8080/api/descriptors \
  -H "Authorization: Bearer $(cat /tmp/token)" | jq '.[] | {id,version}'

#（可选）查看某函数的实例列表（用于 targeted 路由）
curl -sS "http://localhost:8080/api/function_instances?function_id=player.ban&game_id=default" \
  -H "Authorization: Bearer $(cat /tmp/token)" | jq

# 执行函数（lb 路由）
curl -sS http://localhost:8080/api/invoke \
  -H "Authorization: Bearer $(cat /tmp/token)" \
  -H 'Content-Type: application/json' \
  -H 'X-Game-ID: default' \
  -d '{"function_id":"player.ban","payload":{"player_id":"1001","reason":"test"},"route":"lb"}' | jq

# 执行函数（broadcast，对所有实例执行并聚合结果）
curl -sS http://localhost:8080/api/invoke \
  -H "Authorization: Bearer $(cat /tmp/token)" \
  -H 'Content-Type: application/json' \
  -H 'X-Game-ID: default' \
  -d '{"function_id":"player.ban","payload":{"player_id":"1002"},"route":"broadcast"}' | jq

# 执行函数（targeted，定向到目标实例，需要先查询 service_id）
TARGET=$(curl -sS "http://localhost:8080/api/function_instances?function_id=player.ban&game_id=default" \
  -H "Authorization: Bearer $(cat /tmp/token)" | jq -r '.instances[0].service_id')
curl -sS http://localhost:8080/api/invoke \
  -H "Authorization: Bearer $(cat /tmp/token)" \
  -H 'Content-Type: application/json' \
  -H 'X-Game-ID: default' \
  -d '{"function_id":"player.ban","payload":{"player_id":"1003"},"route":"targeted","target_service_id":"'"$TARGET"'"}' | jq

# 执行函数（hash，基于字段稳定路由到实例；需提供 hash_key）
curl -sS http://localhost:8080/api/invoke \\
  -H "Authorization: Bearer $(cat /tmp/token)" \\
  -H 'Content-Type: application/json' \\
  -H 'X-Game-ID: default' \\
  -d '{"function_id":"player.ban","payload":{"player_id":"1004"},"route":"hash","hash_key":"1004"}' | jq
```

更多接口（示例）
```bash
# 查看注册表（Agent 概览与函数覆盖）
curl -sS http://localhost:8080/api/registry \
  -H "Authorization: Bearer $(cat /tmp/token)" | jq

# 查询审计（可带 game_id/env/actor/kind）
curl -sS "http://localhost:8080/api/audit?game_id=default&limit=50" \
  -H "Authorization: Bearer $(cat /tmp/token)" | jq '.events[-5:]'

# 健康与指标（Server/Edge/Agent）
curl -sS http://localhost:8080/healthz && echo
curl -sS http://localhost:8080/metrics | jq
curl -sS http://localhost:9080/metrics | jq   # Edge
curl -sS http://localhost:19091/metrics | jq  # Agent

# 查询作业结果（Server 直连模式与 Edge 转发模式均可用）
curl -sS "http://localhost:8080/api/job_result?id=<job_id>" \
  -H "Authorization: Bearer $(cat /tmp/token)" | jq
```

## 🧭 多游戏管理（Game/Env 作用域）

为支持一个 Server 管理多款游戏/多环境，引入作用域并贯穿全链路。

- 作用域字段
  - `game_id`：必填，游戏标识（示例：`game_kr`、`game_en`、`game_x`）
  - `env`：可选，环境（`prod`/`stage`/`test`）
  - `cluster`/`region`：可选标签，便于进一步路由与展示

- 协议与注册（建议）
  - Control.RegisterRequest：新增 `game_id`、`env`（Agent 注册时上报自身作用域）
  - Function.InvokeRequest：`Metadata["game_id"]`/`Metadata["env"]` 作为路由依据

- 路由与索引
  - Registry 改为“按 (game_id, function_id)”索引 Agent；pickAgent 时需传入 `game_id`
  - Job 路由：记录 job_id → (game_id, agent_addr) 映射

- HTTP & UI
  - HTTP 请求头：`X-Game-ID`、`X-Env`；后端透传到 `InvokeRequest.Metadata`
  - 前端提供 Game/Env 切换器（全局状态），所有 API 自动附带头信息

- SDK
  - 增加全局默认 `game_id` 与每次调用覆盖的能力；或在注册阶段绑定作用域

- RBAC 与审计
  - RBAC：支持作用域的细粒度控制（示例：`game:<game_id>:function:<id>` 或 ABAC 属性匹配）
  - 审计：记录 `game_id`/`env`，查询时按作用域过滤

## 📋 项目结构（建议）

```
croupier/
├── cmd/
│   ├── server/               # Server 进程
│   ├── agent/                # Agent 进程（原 proxy）
│   └── cli/                  # 命令行工具
├── proto/                    # gRPC Proto（IDL 源）
├── descriptors/              # 函数描述符（JSON Schema/元数据）
├── internal/
│   ├── server/               # Server 业务
│   ├── agent/                # Agent 业务
│   ├── auth/                 # OIDC/mTLS/会话管理
│   ├── function/             # 路由、幂等、重试、版本协商
│   ├── jobs/                 # Job 状态机与队列
│   └── web/                  # Web 后端
├── pkg/
│   ├── protocol/             # 公共协议常量/拦截器（暂留）
│   └── types/                # 公共类型（暂留）
├── sdks/
│   └── go/                   # Go SDK 子模块（croupier-sdk-go）
│   └── cpp/                  # C++ SDK 子模块（croupier-sdk-cpp）（后续实现）
│   └── java/                 # Java SDK 子模块（croupier-sdk-java）（后续实现）
├── web/                      # 前端子模块（croupier-web）
├── configs/                  # 配置
├── scripts/                  # 部署脚本
├── docs/                     # 文档
└── examples/                 # 示例
```

## 🔐 安全与权限

### 认证与授权（MVP）
- 登录：POST `/api/v1/auth/login`（Body: `{username,password}`）返回 `{token,user}`；前端保存 token 并通过 `Authorization: Bearer <token>` 访问 /api/*
- 会话：GET `/api/v1/users/current` 返回 `{username,roles}`；未登录 401
- 权限：RBAC 支持函数级 + 作用域（例如 `game:<game_id>:function:<id>`、`game:<game_id>:*`、`*`）；支持基于 `role:<role>` 的规则

### 传输与身份
- mTLS：Server/Edge/Agent 默认要求提供 `--cert/--key/--ca`，Agent 外连必须启用 mTLS；证书颁发与轮换可接入 SPIFFE/SPIRE、ACME 或企业 CA
- 出站：通信仅走 443/HTTP/2；Agent/SDK 统一出站（便于穿透防火墙/代理）

### 审计与防护
- 全量审计：功能 ID、调用人、参数摘要（敏感字段散列）、目标资源、结果、耗时、traceId
- 日志防篡改：链式哈希或外部归档；保留周期与合规策略可配置
- 限流与背压：连接数/并发/速率限制，超时与熔断策略

## 部署与配置（建议）

- TLS/mTLS（默认开启）
  - Server/Edge/Agent 均要求 `--cert/--key/--ca`（Agent 外连必须 mTLS）
  - 开发可使用 `./scripts/dev-certs.sh` 生成自签证书
  - 证书颁发建议 SPIFFE/SPIRE 或企业 CA，并定期轮换
- 认证与前端
  - 登录 `POST /api/v1/auth/login` → 保存 token；前端请求自动附带 `Authorization: Bearer <token>`
  - 开发时 CORS 放开；生产建议反向代理或同域部署前端
- 多游戏作用域
  - 后台添加 game_id/env（`/api/games`）后，Agent 才能注册成功（白名单 Gate）
  - 所有调用带 `X-Game-ID`/`X-Env`，后端透传到元数据用于路由与审计
- 可观测与运行
  - Server/Edge/Agent 暴露 `/healthz` 与 `/metrics`（JSON）
  - Edge 指标包含隧道连接数/待处理/作业映射与累积事件计数
- 容器化
  - 提供 `Dockerfile.*` 与 `docker-compose.yml`，一键构建与运行（需先生成 dev 证书）

## ⚙️ 调用模型

- Query：同步调用，超时短；适用于查询/校验
- Command：异步调用，返回 `job_id`；支持取消/重试/进度/日志
- 幂等：以 `idempotency-key` 去重；服务端记录窗口以防重放
- 版本协商：函数 `id@semver`；Server/Agent/SDK 通过特性协商降级

## 🗺️ 演进与兼容

- 保持向后兼容：先引入 TLS 与 Descriptor，再平滑迁移到 gRPC 接口
- 版本协商：函数 `id@semver`；Server/Agent/SDK 通过特性协商降级

## 🗓️ 开发计划（详细）

说明：以下为以“可运行骨架优先”的拆解，默认以周为单位推进，可并行的任务已标注。

- Phase 0：基础设施与脚手架（1 周）
  - 目标：统一 IDL/生成链路与目录结构，打通本地开发。
  - 任务：
    - 引入 Buf/Protobuf 工具链（`proto/` + `buf.yaml` + `buf.gen.yaml`）
    - 规划目录：`cmd/server`、`cmd/agent`、`pkg/sdk`、`internal/{server,agent,function,jobs}`、`descriptors/`
    - Make 目标与 CI（lint、build、unit、buf lint/breaking）
  - DoD：`make dev` 一键起本地开发；`buf lint`、`go test ./...` 通过

- Phase 1：gRPC + mTLS 南向最小骨架（2 周）
  - 目标：Server/Agent/Go SDK 直连，具备注册/调用/健康检查能力。
  - 任务：
    - 定义基础 Proto：`FunctionService.Invoke`、`ControlService.Register/Heartbeat`、标准错误码
    - mTLS：自签或 SPIFFE/SPIRE 接入；Keepalive/连接复用/超时配置
    - Agent：出站长连到 Server，承载多游戏服复用；本地 gRPC 监听供 SDK 使用
    - Go SDK：连接管理、拦截器（超时/重试/trace）与简单示例
  - DoD：示例游戏服通过 Agent 注册 1 个函数，并被 Server 端成功 Invoke；TLS 轮换演练通过；e2e 冒烟用例通过

- Phase 2：Descriptor 驱动 UI（2 周，可与 Phase 1 后半重叠）
  - 目标：由描述符自动生成参数表单与校验，实现从 UI 到后端的真实闭环。
  - 任务：
    - 定义 Descriptor Schema（JSON Schema + 元数据：风险、敏感字段、超时、幂等键等）
    - 后端提供 Descriptor 列表/详情 API；参数校验与错误返回标准化
    - 前端：动态表单渲染、字段级脱敏占位、结果展示
  - DoD：`player.ban` 通过 UI 表单执行成功，前后端共享同一 Schema 校验

- Phase 3：Job 模型与流式通道（2 周）
  - 目标：支持长任务异步执行、进度/日志流、取消与幂等。
  - 任务：
    - gRPC：`StartJob`、`StreamJob`、`CancelJob`；事件模型（进度、日志、完成、失败）
    - Job Store：内存实现 + 可插拔（后续 Redis/SQL）；并发/队列与背压控制
    - 幂等键与窗口；超时与重试策略；UI 进度条/日志流
  - DoD：10k+ 事件稳定流式播放；取消/重试可用；参数相同 + 幂等键重复提交不产生重复副作用

- Phase 4：认证与权限（2 周）
  - 目标：落地 OIDC 登录、细粒度授权、审批与审计。
  - 任务：
    - OIDC 登录 + 会话；角色与权限模型（功能/资源/环境 维度）
    - 高危操作双人审批；执行理由与变更单号记录
    - 审计：不可篡改（链式哈希/外部归档）；字段级脱敏
  - DoD：`player.ban@prod` 需审批方可执行；审计链完整且可校验

- Phase 5：多语言 SDK 生成与示例（2 周）
  - 目标：以 IDL 生成 Go/Java/Python/C++ 客户端，提供最小示例与文档。
  - 任务：
    - Buf 多语言生成；统一拦截器（鉴权/重试/trace）与示例工程（`examples/*`）
    - 文档：集成指南、错误码、超时/重试/幂等最佳实践
  - DoD：多语言 e2e 冒烟用例通过（注册 + 调用 + Job 流）

- Phase 6：可观测性与 SRE（1 周）
  - 目标：上线所需的观测与基线性能。
  - 任务：
    - 指标：QPS、P99、失败率、活动连接、队列长度；Tracing（OpenTelemetry）
    - Dashboards/Alerts；压测报告与基线（目标 P99/吞吐）
  - DoD：仪表盘与告警生效；压测指标达标

- Phase 7：兼容与迁移（1 周）
  - 目标：从现有 Proxy/TCP 迁移到 Agent/gRPC，保障平滑过渡。
  - 任务：
    - `croupier-proxy` 重命名与配置兼容；必要时提供桥接层
    - 迁移指引文档与回滚策略
  - DoD：试点业务零停机迁移，出现问题可一键回滚

- Phase 8：多游戏作用域（1 周）
  - 目标：引入 Game/Env 作用域，打通注册、路由、调用、审计
  - 任务：
    - 协议：Control.RegisterRequest 增 `game_id`/`env`；Invoke 元数据透传
    - Registry：按 (game_id,function_id) 索引；HTTP 透传 `X-Game-ID`/`X-Env`
    - UI：全局 Game/Env 切换器；RBAC 权限与审计增加作用域
  - DoD：不同 `game_id` 的函数路由隔离；审计可按 `game_id` 查询

- Phase 9：Edge PoC（1 周）
- 目标：在 Server 不出网场景，通过 Edge 转发实现 Server↔Agent 联通
  - 任务：
    - `cmd/edge` 进程：接收 Agent 外连；Server 出站连 Edge；双向流隧道
    - 转发：Function/Control 请求/响应的多路复用与路由
    - TLS 与鉴权：沿用 mTLS 身份，Edge 仅转发合法实体
  - DoD：Server 内网仅出站，Agent 外连 Edge，功能调用正常

里程碑验收清单（节选）
- e2e：`examples/go-server` 可注册/调用/长任务/取消/审计全链路跑通
- 安全：mTLS 双向认证；OIDC/MFA 登录；审批 + 审计链可验证
- 可靠性：连接保活/重连、限流背压、幂等去重；灰度与版本协商
- 观测：Tracing 贯通 Server/Agent/SDK；指标完整并可告警

## 🤝 贡献

### 开发环境设置

```bash
# 1. 克隆项目
git clone https://github.com/cuihairu/croupier.git
cd croupier

# 2. 初始化子模块（前端、SDK）
git submodule update --init --recursive

# 3. 安装 Go 依赖
go mod download

# 4. 生成开发用 TLS 证书（本地自签，生成到 configs/dev/）
./scripts/dev-certs.sh
```

### 生成协议文件与构建

⚠️ **重要**：`gen/` 目录包含自动生成的文件，**不应该**提交到版本控制中。请使用以下命令生成：

```bash
# 方法 1: 一键开发构建（推荐）
make dev                    # 执行: clean + proto + build

# 方法 2: 分步骤执行
make proto                  # 生成 protobuf Go 代码
make pack                   # 生成 gen/croupier/ 目录（函数描述符、UI schema 等）
make build                  # 构建所有二进制文件

# 方法 3: 清理重建
make clean                  # 清理 bin/ 和 gen/ 目录
make dev                    # 重新生成和构建
```

**生成的文件结构：**
```
gen/croupier/
├── descriptors/          # 函数描述符 JSON 文件
├── ui/                   # UI 模式和配置文件
├── fds.pb               # Protocol Buffer 文件描述符集
├── manifest.json        # 包清单文件
└── pack.tgz            # 打包的构件
```

**注意事项：**
- `gen/` 目录已在 `.gitignore` 中，不会被提交
- 首次开发或切换分支后，必须运行 `make dev` 重新生成
- 如果遇到 "gen/croupier: no such file or directory" 错误，运行 `make pack` 即可解决

### 本地运行

构建完成后，可以在多个终端中运行：

```bash
# 终端 1: 启动 Server
./bin/croupier-server --config configs/server.example.yaml

# 终端 2: 启动 Agent（连接本地 Server）
./bin/croupier-agent --config configs/agent.example.yaml

# 终端 3: 运行示例游戏服务器（连接本地 Agent）
go run ./examples/go-server

# 终端 4: 运行示例调用客户端
go run ./examples/go-invoker
```

### 前端开发

```bash
cd web
npm install

# 安装 X-Render 依赖
npm install form-render @ant-design/icons

# 启动开发服务器（默认端口 8000）
npm run dev  # 或 npm run start

# 生产构建
npm run build  # 产物到 web/dist，Server 会优先静态服务 web/dist
```

### IP 属地（GeoIP，登录/操作日志的属地显示）

后台会在审计事件中尽量填充 `meta.ip_region`，并在“登录日志 / 操作日志 / 审批列表”等页面显示“属地”。该功能默认可用，行为如下：

- 本地与内网地址无需查询：
  - `127.0.0.1`/`::1` → “本地”
  - 私网/链路本地地址（10/172.16–31/192.168/169.254、fc00::/7、fe80::/10）→ “局域网”
- 公网地址解析（可选，二选一或同时启用）：
  1) 离线库 IP2Location（推荐，可离线）：
     - 需自行下载 LITE 数据库 BIN 文件（免费）：
       - IPv4：`IP2LOCATION-LITE-DB3.BIN`
       - IPv6：`IP2LOCATION-LITE-DB3.IPV6.BIN`
       - 下载地址：https://lite.ip2location.com/database/db3-ip-country-region-city
     - 放置到项目 `configs/` 目录（文件名保持一致）即可自动启用；或通过环境变量指定绝对路径：
       - `IP2LOCATION_BIN_PATH=/abs/path/IP2LOCATION-LITE-DB3.BIN`
       - `IP2LOCATION_BIN_PATH_V6=/abs/path/IP2LOCATION-LITE-DB3.IPV6.BIN`
     - 构建无需任何 build tag，系统会在运行时自动探测；若文件不存在则回退。
  2) 在线 HTTP 解析（可选）：
     - 设置 `GEOIP_HTTP_URL`（如 `https://your-geo.example.com/lookup?ip={{ip}}`），以及超时 `GEOIP_TIMEOUT_MS`（默认 1500）。
     - 返回 JSON 中常见字段名将被自动识别（如 country/region/city）。

校验方式：
- 登录一次后台 → “后台用户 → 登录日志”中“属地”列应出现“本地/局域网/国家/省/市”。
- 也可调用 `/api/audit?kinds=login`，查看 `events[].meta.ip_region`。

注意：
- `configs/*.BIN` 文件不会提交到版本库（已加入 `.gitignore`），请按需自行下载放置。
- 开发脚本 `scripts/dev-run-server.sh` 会在检测到 `configs/` 下存在 BIN 时自动导出相应环境变量并设置合理的 GEOIP 超时时间。

### 开发工作流程

1. **首次设置**：`make dev` - 完整的开发环境构建
2. **日常开发**：
   - 修改 `.proto` 文件后：`make proto && make pack`
   - 修改 Go 代码后：`make build`
   - 修改前端代码：在 `web/` 目录中 `npm run dev`
3. **清理重建**：`make clean && make dev`

### 验证安装

```bash
# 检查生成的文件
ls -la gen/croupier/                    # 应该包含 descriptors/, ui/, fds.pb 等

# 检查二进制文件
ls -la bin/                             # 应该包含 croupier-server, croupier-agent 等

# 访问管理界面
# 开发环境：http://localhost:8000（前端 dev server）
# 生产环境：http://localhost:8080（Server 静态服务）
# 默认登录：用户名 admin，密码 admin
```

### SDK 开发

- **Go SDK**：子模块在 `sdks/go`，建议直接引用 `github.com/cuihairu/croupier-sdk-go`
- **C++ SDK**：子模块 `sdks/cpp`（规划中）
- **Java SDK**：子模块 `sdks/java`（规划中）

### CI 说明

- CI 已配置检出子模块（`submodules: recursive`）
- 本地一键初始化：`make submodules`

### 提交流程

1. **Fork** 本仓库到您的 GitHub 账户
2. **创建分支**：`git checkout -b feature/your-feature-name`
3. **开发**：使用上述开发工作流程
4. **提交**：`git commit -m "your message"`
5. **推送**：`git push origin feature/your-feature-name`
6. **创建 PR**：在 GitHub 上创建 Pull Request

**注意**：请确保不要提交 `gen/` 和 `bin/` 目录中的文件。

## 🎨 Web 管理界面

Croupier 提供了基于 X-Render 的现代化 Web 管理界面：

- **仓库**：https://github.com/cuihairu/croupier-web
- **子模块路径**：`web`（已在本仓库引入）
- **技术栈**：Umi Max + Ant Design Pro + X-Render + TypeScript

### 🚀 X-Render 集成特性：
- **动态表单生成**：基于 JSON Schema 自动生成函数调用表单
- **智能 UI 渲染**：X-Render 驱动的零配置界面生成
- **函数调用界面**：支持 lb/broadcast/targeted/hash 路由的智能表单
- **实体管理器**：基于 X-Render 的可视化实体 CRUD 操作
- **Schema 预览**：实时预览实体和函数的 UI 效果

### 🎯 传统功能：
- **游戏/环境切换器**：全局状态管理
- **实时作业进度追踪**：WebSocket + SSE 长连接
- **审批工作流界面**：双人审批流程
- **审计日志查看器**：支持过滤和搜索
- **注册表管理**：Agent 和函数实例概览
- **函数包管理**：上传/导入/导出
- **多语言支持**：中文、英文、日文等

### 开发模式：
```bash
cd web
npm install

# 安装 X-Render 依赖
npm install form-render @ant-design/icons

npm run dev    # 开发服务器（默认端口 8000）
```

### 前端 X-Render 启动与使用指南

#### 🚀 快速启动步骤

1. **环境准备**
   ```bash
   cd web
   npm install                          # 安装基础依赖
   npm install form-render @ant-design/icons  # 安装 X-Render 依赖
   ```

2. **启动开发服务器**
   ```bash
   npm run dev                          # 启动前端开发服务器（端口 8000）
   ```

3. **访问管理界面**
   - 开发环境：http://localhost:8000
   - 生产环境：http://localhost:8080 (需要先构建)

#### 🎯 X-Render 核心功能使用

**函数表单渲染 (GmFunctions 页面)**
- 支持 Form-Render 和 Legacy 两种渲染模式切换
- 基于 JSON Schema 自动生成表单字段
- 支持复杂字段类型：对象、数组、枚举、日期等
- 自动参数校验和错误提示

**实体管理 (Entities 页面)**
- 基于 ProTable 和 form-render 的 CRUD 操作
- 实时预览实体 UI 效果
- JSON Schema 验证和错误展示
- 支持拖拽排序和批量操作

**开发者最佳实践**
- 使用 `FormRender` 组件进行 schema 驱动的表单开发
- 通过 `uiSchema` 自定义字段组件和布局
- 利用 `show_if` 和 `required_if` 实现动态表单逻辑
- 集成 Ant Design 组件保持 UI 一致性

### 生产构建：
```bash
npm run build  # 产物到 web/dist，Server 会静态服务此目录
```

### X-Render 使用示例：
```jsx
// 基于函数描述符自动生成表单
import FormRender from 'form-render/lib/antd';
import { useFunctionInvoke } from '@/hooks/useFunctionInvoke';

const FunctionForm = ({ functionId }) => {
  const { invoke, loading } = useFunctionInvoke();
  const [schema, setSchema] = useState(null);
  const [formData, setFormData] = useState({});

  // 从后端获取函数 JSON Schema
  useEffect(() => {
    fetchFunctionSchema(functionId).then(setSchema);
  }, [functionId]);

  const handleSubmit = (data) => {
    return invoke(functionId, data);
  };

  return (
    <FormRender
      schema={schema}
      formData={formData}
      onChange={setFormData}
      onFinish={handleSubmit}
      submitProps={{ loading }}
    />
  );
};
```

访问 `http://localhost:8080` 使用完整的管理界面，或 `http://localhost:8000`（开发模式）。

## 📦 SDKs

### Go SDK（生产就绪）
- **仓库**：https://github.com/cuihairu/croupier-sdk-go
- **子模块路径**：`sdks/go`（已在本仓库引入）
- **文档**：`sdks/go/README.md`
- **功能特性**：
  - 注册本地函数到 Agent
  - 承载 FunctionService gRPC 服务
  - JSON 编解码与 Schema 校验
  - 超时/重试拦截器
  - 幂等键支持

### 集成示例

以 Go SDK 为例：

```proto
// proto/croupier/function/v1/function.proto
service FunctionService {
  rpc Invoke(InvokeRequest) returns (InvokeResponse);          // 短任务/查询
  rpc StartJob(InvokeRequest) returns (StartJobResponse);      // 长任务/命令
  rpc StreamJob(JobStreamRequest) returns (stream JobEvent);   // 进度/日志
}
```

```json
// descriptors/player.ban.json - 函数描述符（驱动 X-Render UI 生成）
{
  "id": "player.ban",
  "version": "1.2.0",
  "category": "player",
  "risk": "high",
  "auth": { "permission": "player.ban", "two_person_rule": true },
  "params": {
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "type": "object",
    "properties": {
      "player_id": {
        "type": "string",
        "title": "玩家ID",
        "description": "要封禁的玩家唯一标识"
      },
      "reason": {
        "type": "string",
        "title": "封禁原因",
        "enum": ["cheating", "harassment", "spam", "other"]
      },
      "duration": {
        "type": "integer",
        "title": "封禁时长(小时)",
        "minimum": 1,
        "maximum": 8760
      }
    },
    "required": ["player_id", "reason"]
  },
  "semantics": {
    "mode": "command",
    "idempotency_key": true,
    "timeout": "30s",
    "returns": "job"
  }
}
```

```go
// examples/go-server/main.go（最小示例，已在仓库提供）
// 1) 连接本机 Agent 2) 注册函数 3) 启动本地服务并向 Agent 报到
cli := sdk.NewClient(sdk.ClientConfig{
    Addr: "127.0.0.1:19090",
    LocalListen: "127.0.0.1:0"
})
_ = cli.RegisterFunction(sdk.Function{
    ID: "player.ban",
    Version: "1.2.0"
}, handler)
_ = cli.Connect(context.Background())
```

访问 `http://localhost:8080` 可使用由 X-Render 自动生成的动态管理界面。上述 JSON Schema 将自动渲染为包含下拉选择、数值输入、必填验证等功能的表单。

### C++ SDK（规划中）
- **仓库**：https://github.com/cuihairu/croupier-sdk-cpp
- **子模块路径**：`sdks/cpp`
- **状态**：占位符，优先完成 Go 版本后逐步实现

### Java SDK（规划中）
- **仓库**：https://github.com/cuihairu/croupier-sdk-java
- **子模块路径**：`sdks/java`
- **状态**：占位符，优先保证 Go 版本稳定后实现

**使用建议**：生产接入建议以 IDL 生成多语言 SDK；当前阶段 Go SDK 为最优先完善路径，其他语言逐步跟进。

## 📖 文档

- docs/api.md
- docs/sdk-development.md
- docs/deployment.md
- docs/security.md

## 📄 许可证

本项目采用 MIT 许可证 - 详见 LICENSE。

---

## 🚀 X-Render 技术优势

### 为什么选择 X-Render？

**技术匹配度 100%**:
- ✅ **JSON Schema 原生支持**: 直接使用项目现有的函数描述符
- ✅ **Ant Design 深度集成**: 无缝融入现有 UI 体系
- ✅ **动态表单生成**: 零代码实现复杂 GM 操作界面
- ✅ **企业级稳定性**: 阿里巴巴维护，7.8k+ stars 社区

**开发效率提升**:
- 🚀 **减少 80% UI 开发工作**: 自动生成替代手写组件
- 🚀 **维护成本降低**: 统一的 Schema 驱动架构
- 🚀 **快速迭代**: 修改 JSON Schema 即可更新界面

**功能完整性**:
- 📋 **丰富表单控件**: DatePicker、Select、Upload、Tree 等
- 📋 **智能验证**: 基于 JSON Schema 的实时校验
- 📋 **可扩展性**: 支持自定义组件和主题

### X-Render 在 Croupier 中的应用

1. **函数调用表单**: 基于函数描述符自动生成调用界面
2. **实体管理界面**: 动态生成 CRUD 操作表单
3. **配置管理器**: 游戏配置的可视化编辑
4. **数据预览**: 实时渲染数据结构预览

---

Croupier - 让游戏运营变得简单而强大 🎮
Examples & Adapters
-------------------

- HTTP adapter PoC: `tools/adapters/http`
- Prom adapter PoC: `tools/adapters/prom`
- These are non-critical examples for development and demos; not required in production.

Architecture Overview
---------------------

Core layering (C-architecture, Ports/Adapters + Wire DI):

- `internal/app/*` (process assemblers): HTTP/gRPC servers, routes, middleware, Wire injectors
- `internal/service/*` (use-cases): business logic, depends only on `internal/ports`
- `internal/ports/*` (interfaces): repository/service contracts (no infra deps)
- `internal/repo/gorm/*` (adapters): GORM implementations of ports; owns DB models
- `internal/platform/*` (integrations): objstore, tlsutil, etc.
- `internal/security/*` (security): rbac (Casbin), token (JWT)

Database Drivers
----------------

Supported via GORM drivers: Postgres, MySQL, SQLite, SQL Server.

- `DB_DRIVER`: `postgres|mysql|sqlite|mssql|sqlserver|auto` (default `auto`)
- `DATABASE_URL`: DSN/URL (e.g. `postgres://...`, `mysql://...`, `file:data/croupier.db`)

Auto mode attempts Postgres → MySQL → SQL Server, else falls back to SQLite at `data/croupier.db`.

Wire DI & Providers
-------------------

Server assembly uses Google Wire (with a checked-in `wire_gen.go`):

- Manual: `InitServerApp(descriptorDir, invoker, audit, rbac, registry, jwt, locator, stats)`
- Auto: `InitServerAppAuto(descriptorDir, invoker, registry, locator, stats)` — builds audit/RBAC/JWT/DB/Repos/Services from env

Providers (env-driven where applicable):

- DB: `ProvideGormDBFromEnv()` → `DB_DRIVER`, `DATABASE_URL`
- Games: `ProvideGamesDefaults()` → reads `configs/games.json`
- RBAC: `ProvideRBACPolicyAuto()` → `RBAC_MODEL` + `RBAC_POLICY` or `RBAC_CONFIG`
- JWT: `ProvideJWTManagerFromEnv()` → `JWT_SECRET` (default `dev-secret`)
- Cert store: `ProvideCertStore(db)`
- Object store: `ProvideObjectStoreFromEnv()` → `STORAGE_*` (file/S3/OSS/COS)
- ClickHouse: `ProvideClickHouseFromEnv()` → `CLICKHOUSE_DSN` (optional)

Local development:

- Install wire: `go install github.com/google/wire/cmd/wire@latest`
- Generate: `make wire` (runs in `internal/app/server/http`)
- CI already validates generation (see `.github/workflows/ci.yml`).
