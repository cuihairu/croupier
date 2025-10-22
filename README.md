# Croupier - 游戏GM后台系统

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.21+-green.svg)
![Status](https://img.shields.io/badge/status-in%20development-yellow.svg)

Croupier 是一个专为游戏运营设计的通用 GM 后台系统，支持多语言游戏服务器接入，提供统一的管理界面与强大的扩展能力。

本 README 描述的是推荐的 vNext 架构：gRPC + mTLS、Descriptor 驱动 UI、Agent 外连拓扑。与现有实现兼容演进（现有 `croupier-proxy` 在本文中称为 Agent）。

## 🎯 核心特性

- 🔐 gRPC + mTLS：双向身份与加密传输（HTTP/2/443），内置重试/流控
- 🧩 IDL 生成：以 Proto 定义服务与消息，生成多语言 SDK（Go/Java/C++/Python）
- 🧱 Descriptor 驱动 UI：函数入参/出参、校验、敏感字段、超时等描述，自动生成表单与结果展示
- 📡 实时流式：支持长任务进度/日志流、订阅/推送
- 🛰️ Agent 外连：内网仅出站至 DMZ/Core，无需内网入站；多服务多路复用一条长连
- 🔑 细粒度权限：功能级/资源级/环境级 RBAC/ABAC，支持高危操作双人审批与审计
- 🧪 易扩展：Function 版本化与兼容协商、幂等键、灰度/回滚

## 🏗️ 系统架构

### 整体架构图（vNext）

```
                 ┌────────────── Web 管理界面 ──────────────┐
                 │                                           │
           ┌──────────────────────────────────────────────────────┐
           │                    Croupier Core                      │
           │  Auth/OIDC  RBAC/ABAC  UI Generator  Function Router  │
           └──────────────────────────────────────────────────────┘
                               ▲  gRPC/mTLS :443 (HTTP/2)
                               │
                     ┌─────────┴─────────┐
                     │                   │
            ┌─────────────────┐  ┌─────────────────┐
            │  Croupier Agent │  │  Croupier Agent │   ← 内网仅“主动外连”Core
            └─────────────────┘  └─────────────────┘
                    ▲                     ▲
                    │ 本地 gRPC           │ 本地 gRPC
            ┌───────────────┐     ┌───────────────┐
            │ Game Server A │     │ Game Server B │  ← SDK 连接本机/就近 Agent
            └───────────────┘     └───────────────┘
```

### 调用与数据流
- Query（查询）同步返回；Command（命令）异步返回 `job_id`
- 长任务通过流式接口返回进度/日志，可取消/重试，保证幂等（`idempotency-key`）
- 所有函数字段由 Descriptor（JSON Schema/Proto 选其一）定义，UI/校验/鉴权共享同一描述
  
开发便捷性说明：骨架阶段为便于本地联调，Agent 在 `Register` 时会上报 `rpc_addr`，Core 通过该地址直连 Agent 完成调用（DEV ONLY）。生产将改为“Agent 外连双向流”模式，不需 Core 入内网。

## 🚀 快速开始

> 说明：如当前仓库仍提供 `croupier-proxy`，在落地 Agent 前，先以 `croupier-proxy` 作为 Agent 使用；命名将逐步迁移为 `croupier-agent`。

### 模式 1：同网部署（直连，简化）

适用于 Core 与 Game 在同一内网且允许直连的场景（仍建议使用 mTLS）。

```bash
# 1) 启动 Core（默认监听 443 或自定义）
./croupier-server --config configs/croupier.yaml

# 2) 游戏服务器 SDK 直接连接 Core（gRPC/mTLS）
./game-server
```

### 模式 2：Agent 外连（推荐）

Core 位于 DMZ/公网，Agent 在游戏内网，仅出站到 Core。游戏服只连本机/就近 Agent。

```bash
# 1) DMZ 启动 Core
./croupier-server --config configs/croupier.yaml

# 2) 内网启动 Agent（若二进制名仍为 proxy，请先用 proxy）
./croupier-agent --config configs/agent.yaml
# 或
./croupier-proxy  --config configs/agent.yaml

# 3) 游戏服务器连接本机 Agent（gRPC）
./game-server
```

### SDK 集成示例

以 Go 为例（通过 Proto 生成的 SDK）。

```proto
// proto/gm/function.proto
service FunctionService {
  rpc Invoke(InvokeRequest) returns (InvokeResponse);          // 短任务/查询
  rpc StartJob(InvokeRequest) returns (StartJobResponse);      // 长任务/命令
  rpc StreamJob(JobStreamRequest) returns (stream JobEvent);   // 进度/日志
}
```

```json
// descriptors/player.ban.json - 函数描述符（驱动 UI/校验/鉴权）
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
      "player_id": { "type": "string" },
      "reason": { "type": "string" }
    },
    "required": ["player_id"]
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
cli := sdk.NewClient(sdk.ClientConfig{Addr: "127.0.0.1:19090", LocalListen: "127.0.0.1:0"})
_ = cli.RegisterFunction(sdk.Function{ID: "player.ban", Version: "1.2.0"}, handler)
_ = cli.Connect(context.Background())
```

访问 `http://localhost:8080` 可使用由 Descriptor 自动生成的管理界面。

## 📋 项目结构（建议）

```
croupier/
├── cmd/
│   ├── server/               # Core 进程
│   ├── agent/                # Agent 进程（原 proxy）
│   └── cli/                  # 命令行工具
├── proto/                    # gRPC Proto（IDL 源）
├── descriptors/              # 函数描述符（JSON Schema/元数据）
├── internal/
│   ├── server/               # Core 业务
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

### 传输与身份
- mTLS：Client/Server 双向校验；证书颁发与轮换可接入 SPIFFE/SPIRE、ACME 或企业 CA
- 通信仅走 443/HTTP/2；Agent/SDK 统一出站，便于穿透防火墙/代理

### 用户与权限
- 用户侧：OIDC 登录（SAML/LDAP 可兼容），支持 MFA
- 权限：功能级/资源级/环境级 RBAC/ABAC（如 `player:ban@prod`），可配置双人审批
- 脱敏：支持字段级脱敏（如手机号、IP），按权限查看明文/脱敏值

### 审计与防护
- 全量审计：功能 ID、调用人、参数摘要（敏感字段散列）、目标资源、结果、耗时、traceId
- 日志防篡改：链式哈希或外部归档；保留周期与合规策略可配置
- 限流与背压：连接数/并发/速率限制，超时与熔断策略

## ⚙️ 调用模型

- Query：同步调用，超时短；适用于查询/校验
- Command：异步调用，返回 `job_id`；支持取消/重试/进度/日志
- 幂等：以 `idempotency-key` 去重；服务端记录窗口以防重放
- 版本协商：函数 `id@semver`；Core/Agent/SDK 通过特性协商降级

## 🗺️ 演进与兼容

- 现有 `croupier-proxy` 可作为 Agent 使用；后续重命名为 `croupier-agent`
- 保持向后兼容：先引入 TLS 与 Descriptor，再平滑迁移到 gRPC 接口

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
  - 目标：Core/Agent/Go SDK 直连，具备注册/调用/健康检查能力。
  - 任务：
    - 定义基础 Proto：`FunctionService.Invoke`、`ControlService.Register/Heartbeat`、标准错误码
    - mTLS：自签或 SPIFFE/SPIRE 接入；Keepalive/连接复用/超时配置
    - Agent：出站长连到 Core，承载多游戏服复用；本地 gRPC 监听供 SDK 使用
    - Go SDK：连接管理、拦截器（超时/重试/trace）与简单示例
  - DoD：示例游戏服通过 Agent 注册 1 个函数，并被 Core 端成功 Invoke；TLS 轮换演练通过；e2e 冒烟用例通过

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

里程碑验收清单（节选）
- e2e：`examples/go-server` 可注册/调用/长任务/取消/审计全链路跑通
- 安全：mTLS 双向认证；OIDC/MFA 登录；审批 + 审计链可验证
- 可靠性：连接保活/重连、限流背压、幂等去重；灰度与版本协商
- 观测：Tracing 贯通 Core/Agent/SDK；指标完整并可告警

## 🤝 贡献

```bash
# 克隆
git clone https://github.com/your-org/croupier.git
cd croupier

# Go 依赖（需网络）
go mod download

# 生成开发用 TLS 证书（本地自签，生成到 configs/dev/）
./scripts/dev-certs.sh

# 生成 Proto 代码（需安装 buf 与 protoc 插件，或在 CI 里跑；本地有手写 stub 可直接编译）
buf lint && buf generate

# 构建 Core 与 Agent
make build

# 本地运行（在两个终端中）：
# 1) Core（示例参数，需自备 TLS 证书）
./bin/croupier-server --addr :8443 --http_addr :8080 --rbac_config configs/rbac.json \
  --cert configs/dev/server.crt --key configs/dev/server.key --ca configs/dev/ca.crt
# 2) Agent（本地明文监听，mTLS 连接 Core）
./bin/croupier-agent --local_addr :19090 --core_addr 127.0.0.1:8443 --cert configs/dev/agent.crt --key configs/dev/agent.key --ca configs/dev/ca.crt
# 3) 示例游戏服连接 Agent
go run ./examples/go-server

# 子模块（前端、SDK）
# 初始化/更新子模块
git submodule update --init --recursive

# 前端开发（在子模块仓库中运行；建议 antd-pro/umi 默认 8000 端口）
cd web
npm install
npm run dev  # 或 npm run start

# 生产构建
npm run build  # 产物到 web/dist，Core 会优先静态服务 web/dist

# Go SDK（子模块：sdks/go）
# 当前仓库仍保留内置样例 SDK（pkg/sdk）用于演示闭环。后续将迁移至子模块。
# 使用子模块 SDK 时，建议直接引用模块路径 github.com/cuihairu/croupier-sdk-go，
# 或在本仓库 go.mod 中通过 replace 指向 ./sdks/go 做本地联调。

# C++ SDK（子模块：sdks/cpp）
# 当前仅添加为子模块占位，优先完成 Go 版本后再逐步实现 C++ 版本。

# Java SDK（子模块：sdks/java）
# 同上，作为占位先引入，优先保证 Go 版本稳定，随后实现 Java 版本。

CI 提示
- CI 已配置检出子模块（submodules: recursive）。如需在本地一键初始化，请运行：`make submodules`。

# 调用验证（浏览器访问）
# 开发：访问 http://localhost:8000（前端 dev server）
# 生产：构建后访问 http://localhost:8080（Core 静态服务 web/dist）；/api/* 为后端接口
# 前端请求需带 `X-User: user:dev`（开发模式 RBAC 放行），也可在前端配置 proxy/header
```

提交流程：Fork → 分支 → 提交 → 推送 → PR。

## 📖 文档

- docs/api.md
- docs/sdk-development.md
- docs/deployment.md
- docs/security.md

## 📄 许可证

本项目采用 MIT 许可证 - 详见 LICENSE。

---

Croupier - 让游戏运营变得简单而强大 🎮
