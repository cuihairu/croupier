# Extension Runtime Service Draft

更新时间：2026-03-15
状态：草案

本文件定义核心扩展运行时的后端服务骨架，作为 `Phase 2` 的实施依据。

---

## 1. 目标

扩展运行时服务负责把“目录中的扩展版本”变成“系统内可管理、可同步、可执行的安装实例”。

它必须完成：

- catalog 管理
- release 解析
- installation 生命周期管理
- runtime binding 生成
- reconcile
- health / event 记录
- agent sync
- 权限与审计接入

说明：

- 运行时是核心能力，不是业务扩展
- 它不负责具体业务逻辑，只负责编排与状态流转

---

## 2. 核心子模块

建议拆成以下子模块：

```text
internal/core/extension/
  catalog/
  release/
  installation/
  manifest/
  runtime/
  reconciler/
  binding/
  health/
  sync/
```

### 2.1 `catalog`

职责：

- 管理扩展目录项
- 查询可安装扩展
- 过滤已隐藏 / 已废弃版本

### 2.2 `release`

职责：

- 管理扩展版本
- 解析与校验 manifest
- 存储 release 元数据

### 2.3 `installation`

职责：

- 创建安装实例
- 更新配置
- 启用 / 停用 / 升级 / 卸载
- 维护安装状态机

### 2.4 `runtime`

职责：

- 将 installation 转为 runtime plan
- 协调 binding、health、sync

### 2.5 `reconciler`

职责：

- 比较 desired state 与 actual state
- 生成执行计划
- 驱动 target 进入正确状态

### 2.6 `binding`

职责：

- 解析 manifest 中的 provider/function/page/workflow 声明
- 生成 runtime binding 记录

### 2.7 `health`

职责：

- 主动 / 被动健康检查
- 聚合状态

### 2.8 `sync`

职责：

- 向 Agent 下发安装实例
- 处理 Agent 回执

---

## 3. 服务边界

### 3.1 Runtime 不做的事

- 不直接执行业务 API
- 不代替 driver 做外部调用
- 不做 Dashboard 渲染逻辑
- 不自带独立权限体系

### 3.2 Runtime 必须做的事

- 安装状态流转合法性控制
- manifest 校验
- binding 生成与回收
- audit / event 记录
- Agent 同步编排

---

## 4. 核心接口草案

### 4.1 CatalogService

```go
type CatalogService interface {
    List(ctx context.Context, query CatalogQuery) ([]CatalogItem, error)
    Get(ctx context.Context, extensionID string) (*CatalogItem, error)
    ListReleases(ctx context.Context, extensionID string) ([]ReleaseItem, error)
    UpsertCatalog(ctx context.Context, item CatalogItem) error
    UpsertRelease(ctx context.Context, item ReleaseItem) error
}
```

### 4.2 ManifestService

```go
type ManifestService interface {
    Parse(raw []byte) (*Manifest, error)
    Validate(manifest *Manifest) error
    Normalize(manifest *Manifest) (*Manifest, error)
}
```

### 4.3 InstallationService

```go
type InstallationService interface {
    Install(ctx context.Context, req InstallRequest) (*Installation, error)
    Get(ctx context.Context, installationID int64) (*Installation, error)
    List(ctx context.Context, query InstallationQuery) ([]Installation, error)
    UpdateConfig(ctx context.Context, installationID int64, req UpdateConfigRequest) error
    Enable(ctx context.Context, installationID int64, operator string) error
    Disable(ctx context.Context, installationID int64, operator string) error
    Upgrade(ctx context.Context, installationID int64, req UpgradeRequest) error
    Uninstall(ctx context.Context, installationID int64, operator string) error
}
```

### 4.4 RuntimeService

```go
type RuntimeService interface {
    Reconcile(ctx context.Context, installationID int64) (*ReconcileResult, error)
    ReconcileBatch(ctx context.Context, installationIDs []int64) ([]ReconcileResult, error)
    BuildPlan(ctx context.Context, installationID int64) (*RuntimePlan, error)
    RefreshBindings(ctx context.Context, installationID int64) error
    RefreshHealth(ctx context.Context, installationID int64) error
}
```

### 4.5 AgentSyncService

```go
type AgentSyncService interface {
    BuildAgentPayload(ctx context.Context, agentID string) (*AgentExtensionSyncPayload, error)
    MarkDispatched(ctx context.Context, installationID int64, agentID string) error
    ApplyAgentReport(ctx context.Context, report AgentExtensionReport) error
}
```

### 4.6 EventService

```go
type EventService interface {
    Append(ctx context.Context, event ExtensionEvent) error
    List(ctx context.Context, installationID int64) ([]ExtensionEvent, error)
}
```

---

## 5. 关键 DTO 草案

### 5.1 InstallRequest

```go
type InstallRequest struct {
    ExtensionID    string
    ReleaseVersion string
    ScopeType      string
    ScopeID        string
    TargetType     string
    TargetID       string
    Config         map[string]any
    SecretRefs     map[string]string
    Operator       string
}
```

### 5.2 RuntimePlan

```go
type RuntimePlan struct {
    InstallationID int64
    DesiredState   string
    Bindings       []RuntimeBindingPlan
    HealthChecks   []HealthCheckPlan
    AgentTargets   []string
}
```

### 5.3 ReconcileResult

```go
type ReconcileResult struct {
    InstallationID int64
    Status         string
    Applied        int
    Failed         int
    Message        string
}
```

### 5.4 AgentExtensionSyncPayload

```go
type AgentExtensionSyncPayload struct {
    AgentID        string
    GeneratedAt    time.Time
    Installations  []AgentInstallationPayload
}
```

### 5.5 AgentInstallationPayload

```go
type AgentInstallationPayload struct {
    InstallationID int64
    ExtensionID    string
    ReleaseVersion string
    Driver         string
    Enabled        bool
    Config         map[string]any
    SecretRefs     map[string]string
    Bindings       []AgentBindingPayload
}
```

---

## 6. 生命周期流程

### 6.1 安装流程

1. 校验 catalog / release
2. 解析 manifest
3. 校验 scope / target / config / secret refs
4. 创建 installation 记录
5. 生成初始 event
6. 触发 reconcile
7. 生成 bindings
8. 如需 Agent 执行，生成 sync payload
9. 更新状态为 `installed` 或 `enabled`

### 6.2 启用流程

1. 校验当前状态允许启用
2. 写入 `desired_state=enabled`
3. 执行 reconcile
4. 推送 Agent sync
5. 写入 event

### 6.3 升级流程

1. 校验目标 release
2. 对比 manifest 兼容性
3. 执行 config migration
4. 刷新 bindings
5. 推送 Agent sync
6. 记录回滚点

### 6.4 卸载流程

1. 校验未处于启用状态或允许强制卸载
2. 回收 bindings
3. 撤销 Agent target
4. 写入 uninstall event
5. 标记为 `uninstalled`

---

## 7. 状态机约束

### 7.1 合法状态迁移

- `pending -> installing`
- `installing -> installed`
- `installed -> enabling`
- `enabling -> enabled`
- `enabled -> disabling`
- `disabling -> disabled`
- `enabled -> upgrading`
- `upgrading -> enabled`
- `disabled -> uninstalling`
- `uninstalling -> uninstalled`
- `* -> failed`
- `enabled -> degraded`
- `degraded -> enabled`

### 7.2 非法状态迁移示例

- `enabled -> installing`
- `uninstalled -> enabled`
- `failed -> enabled` 但未走 reconcile

这些必须由 service 层拦截。

---

## 8. 与权限和审计的集成

### 8.1 权限

建议标准操作：

- `extension.catalog.read`
- `extension.installation.read`
- `extension.installation.install`
- `extension.installation.update`
- `extension.installation.enable`
- `extension.installation.disable`
- `extension.installation.upgrade`
- `extension.installation.uninstall`

### 8.2 审计

以下动作必须写入审计：

- 安装
- 启用
- 停用
- 升级
- 卸载
- 修改配置
- 手工 reconcile

---

## 9. 第一阶段最小实现范围

第一阶段只要求做到：

- catalog / release 查询
- installation 建立
- enable / disable / uninstall
- runtime binding 落库
- event 记录
- 手工 reconcile

第一阶段可以暂不做：

- 自动回滚
- 灰度升级
- 复杂 dependency graph
- 多 Agent 分批投放策略

---

## 10. 下一步

下一步需要补：

- GORM model 草案
- Service method 具体错误码
- Agent sync payload 详细 schema
- Reconciler 执行顺序图
