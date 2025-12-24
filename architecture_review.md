# Croupier 架构审查与建议（历史审查 + 当前状态）

## 1. 当前架构分析

### 结构概览

本项目采用了基于 `go-zero` 的 **模块化单体 (Modular Monolith)** 模式。

> 注：本文件最初用于记录 go-zero 迁移早期的目录问题；部分结论已过期。当前可运行入口在 `services/server/server.go` + `services/server/cmd/*`。

- **入口点**: `services/server/server.go`（CLI 入口：`services/server/cmd/root.go`）
- **API 定义**: `services/server/server.api` + `services/server/modules/*.api`（模块拆分）
- **业务逻辑**: `services/server/internal/logic`
- **生成代码**: 主要集中在 `services/server/internal/{handler,logic,svc,types,config}`

### 关键问题

1.  **（历史）目录不匹配**:

    - 早期存在 `services/server/modules/server.go` 引用不存在目录的情况，导致无法编译。
    - 当前版本已修复：可执行入口位于 `services/server/server.go`，`modules/` 主要用于 `.api` 定义拆分。

2.  **Internal 分裂（仍需关注）**:

    - 若未来继续使用 goctl/模板生成，需要确保生成目标目录一致，避免重复 `internal` 树。

3.  **逻辑为空**:
    - 逻辑文件 (例如 `agent_meta_logic.go`) 目前只是空的脚手架代码。

## 2. 建议 (Go-Zero 风格)

### 方案 A: 标准单体结构 (推荐)

简化结构为标准的 go-zero 布局。保持 `modules/*.api` 作为 API 拆分文件是可行的，但建议统一入口与生成目录。

**建议结构:**

```text
services/server/
├── api/                 # 存放所有 .api 文件
│   ├── server.api       # 主入口 api 文件
│   └── modules/         # 模块化 api 定义 (agent.api, etc.)
├── etc/                 # 配置文件
├── internal/            # 所有生成的代码 (logic, handler, svc, types, config)
├── server.go            # 主入口点 (从 modules/server.go 移动过来)
└── server.api           # (可选) 可以放在 server 根目录或 api/ 目录下
```

**执行计划:**

1.  将 `services/server/modules/*.api` 移动到 `services/server/api/`。
2.  将 `services/server/modules/server.go` 移动到 `services/server/server.go`。
3.  更新 `server.api` 中的 import 路径。
4.  在 `services/server` 目录下运行 `goctl` 重新生成代码:
    ```bash
    goctl api go -api api/server.api -dir . -style gozero
    ```
5.  清理 `services/server/modules` 和 `services/server/internal` (合并它们)。

### 方案 B: 严格模块化单体 (如果需要模块隔离)

如果你希望保持模块物理分离 (例如为了以后更容易拆分为微服务)，可以将它们结构化为独立的服务，但一起运行 (这在标准 go-zero `goctl` 中较难实现)。

**当前的方法试图这样做，但由于路径问题失败了。**

### 重构建议

1.  **修复构建**:

    - 更新 `server.go` 的 import，指向 `github.com/cuihairu/croupier/services/server/internal/...`。
    - 或者将 `server.go` 移动到 `services/server/` 并重新生成。

2.  **依赖注入**:

    - 确保 `ServiceContext` (`internal/svc`) 是依赖项 (DB, Redis, RPC 客户端) 的唯一真实来源。
    - 目前 `ServiceContext` 同时存在于 `modules/internal` 和 `server/internal` 中。**请合并它们。**

3.  **领域逻辑**:
    - 不要将复杂的业务逻辑直接放在 `logic/` 中。
    - 创建一个 `services/server/internal/domain` 或 `services/server/internal/service` 包，用于存放独立于 HTTP 传输的核心业务规则。
    - `logic` 层应该只处理 HTTP 请求/响应映射，并调用领域层。

## 3. 下一步

1.  **合并**: 决定使用单一的 `internal` 目录 (推荐 `services/server/internal`)。
2.  **移动**: 将 `server.go` 移动到 `services/server/`。
3.  **重新生成**: 运行 `goctl` 修复 import 和文件位置。
