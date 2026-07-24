# Core / Extension Mapping

更新时间：2026-03-15
状态：草案

本文件用于把当前代码目录映射到目标架构，作为重构拆分依据。

一句话边界：
核心只保留稳定底座；业务能力默认以扩展安装形态交付。

---

## 1. 目标目录结构

建议目标形态：

```text
internal/
  core/
    auth/
    permission/
    audit/
    workspace/
    game/
    node/
    function/
    invocation/
    config/
    secret/
    ops/
    extension/
      catalog/
      installation/
      manifest/
      runtime/
      reconciler/
  drivers/
    openapi/
    webhook/
    grpc/
    workflow/
    internalui/
  agent/
    runtime/
    extension/
    bridge/
  extensions/
    official/
      analytics/
      externalplatform/
      alerting/
      approval/
      backup/
```

说明：

- `core` 只保留平台底座。
- `drivers` 是少量稳定驱动，不承载业务。
- `agent` 放 Agent 执行时逻辑。
- `extensions` 放官方扩展实现。

---

## 2. 当前目录映射

### 2.1 直接归入核心

这些目录应保留为核心能力，后续只做收敛和归类，不做扩展化：

| 当前目录 | 目标归属 | 说明 |
|---|---|---|
| `internal/api/auth` | `internal/core/auth` | 核心认证能力 |
| `internal/api/permission` | `internal/core/permission` | 权限体系必须统一 |
| `internal/api/role` | `internal/core/permission` | 角色仍属于权限域 |
| `internal/api/audit` | `internal/core/audit` | 扩展必须复用审计 |
| `internal/api/config` | `internal/core/config` | 配置中心属于底座 |
| `internal/api/agent` | `internal/core/node` | 节点与 Agent 管理 |
| `internal/api/node` | `internal/core/node` | 节点管理 |
| `internal/api/function` | `internal/core/function` | 函数注册与编排核心 |
| `internal/api/functioncall` | `internal/core/function` | 调用记录与控制属于函数域 |
| `internal/api/workspace` | 删除；迁移至 `internal/api/page` | 历史 WorkspaceConfig，不能作为 Dashboard 核心模型 |
| `internal/api/game` | `internal/core/game` | 游戏主体模型 |
| `internal/api/meta` | `internal/core/config` | 元信息接口可收敛到核心 |
| `internal/api/ops` | `internal/core/ops` | 仅保留基础 ops 能力 |
| `internal/app` | `internal/agent` | Agent 运行逻辑 |
| `internal/agent transport` | `internal/core` + `internal/agent` | 按 server/agent 分层重组 |
| `internal/service/permission` | `internal/core/permission` | 中间件和服务保留统一 |
| `internal/middleware` | `internal/core` | 收敛为核心通用中间件 |
| `internal/router` | `internal/core` | 核心 HTTP/路由装配 |
| `internal/handler` | `internal/core` | 核心 handler 装配层 |
| `internal/svc` | `internal/core` | 服务上下文，后续要瘦身 |
| `internal/security` | `internal/core` | 身份、安全、令牌属于底座 |
| `internal/tasks` | `internal/core` | 任务基础设施 |
| `internal/config` | `internal/core/config` | 配置结构定义 |
| `internal/model` | `internal/core` | 后续按领域拆，不直接外放给扩展 |
| `internal/repo` | `internal/core` | 核心数据访问层 |

### 2.2 先保留在核心，但需要拆分

这些目录不是完整业务模块，但内部混有未来会扩展化的内容，需要二次拆：

| 当前目录 | 暂定归属 | 后续动作 |
|---|---|---|
| `internal/platform` | `drivers` + `extensions` + `core` | 把 driver、绑定、历史平台逻辑拆开 |
| `internal/logic` | `core` + `extensions` | 按领域重分，不能继续做混合逻辑仓 |
| `internal/api/provider` | 过渡层 | 需要判断是 runtime API 还是业务 API |
| `internal/api/openapi` | `drivers/openapi` 或扩展辅助层 | 不应继续作为独立业务域存在 |
| `internal/api/monitoring` | `core/ops` + `official.monitoring` | 保留基础监控，其余迁扩展 |
| `internal/analytics` | `core telemetry` + `official.analytics` | 基础采集与分析产品拆开 |
| `internal/telemetry` | `core telemetry` | 仅保留底层指标和 tracing 能力 |

### 2.3 优先迁移为官方扩展

这些模块不应长期留在核心：

| 当前目录 | 目标扩展 | 原因 |
|---|---|---|
| `internal/api/analytics` | `official.analytics` | 重业务、重 UI、重依赖 |
| `internal/api/alert` | `official.alerting` | 告警规则和通知不属核心 |
| `internal/api/approval` | `official.approval` | 业务流程域，应扩展化 |
| `internal/api/backup` | `official.backup` | 高级备份能力可选安装 |
| `internal/api/feedback` | `official.feedback` | 非底座能力 |
| `internal/api/ticket` | `official.feedback` 或 `official.support` | 工单系统是业务域 |
| `internal/api/platform` | `official.external-platform` | 第三方平台接入应成为第一批样板扩展 |
| `internal/api/provider` | `official.external-platform` 过渡层 | 与平台接入高度相关 |
| `internal/api/monitoring` | `official.monitoring` | 除基础节点监控外都应外移 |
| `internal/platform/quicksdk` | `official.external-platform` | 明确属于外部平台适配 |
| `internal/platform/openapi` | `drivers/openapi` | 作为驱动复用，不再当业务模块 |

### 2.4 暂不纳入扩展主线

这些目录先不作为本次重构重点：

| 当前目录 | 处理建议 |
|---|---|
| `internal/cache` | 暂保留基础设施 |
| `internal/connpool` | 暂保留基础设施 |
| `internal/common` | 后续按核心共用能力收敛 |
| `internal/helper` | 后续清理，不作为扩展依据 |
| `internal/pkg2` | 后续清理命名和边界 |
| `internal/ports` | 视抽象质量决定保留与否 |
| `internal/runtime` | 需要与新 extension runtime 避免命名混淆 |
| `internal/validation` | 保留通用验证层 |

---

## 3. API 分域目标

### 3.1 核心 API

最终建议保留的核心 API 分组：

- auth
- permission
- audit
- workspace
- game
- node
- function
- functioncall
- config
- ops
- extension

### 3.2 扩展 API

这些 API 最终应由扩展独立提供：

- analytics
- alert
- approval
- backup
- platform
- feedback
- ticket
- monitoring advanced

---

## 4. 优先迁移顺序

按收益和风险排序：

1. `external-platform`
2. `analytics`
3. `alerting`
4. `approval`
5. `backup`
6. `feedback / ticket`

---

## 5. 当前结论

本轮重构不追求一步把目录完全改完，而是先完成：

1. 新扩展运行时骨架
2. `official.external-platform` 首次迁移
3. `official.analytics` 拆出核心

只要这两块跑通，后续其余模块按相同路径推进即可。
