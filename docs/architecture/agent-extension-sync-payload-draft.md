# Agent Extension Sync Payload Draft

更新时间：2026-03-15
状态：草案

本文件定义 Server 向 Agent 下发扩展安装实例时的 payload 结构草案。

---

## 1. 设计目标

Agent sync payload 需要解决：

- Agent 应该安装和维护哪些扩展实例
- 每个实例使用什么 driver
- 当前启用状态是什么
- 绑定了哪些 runtime 行为
- 应该如何上报回执和健康状态

要求：

- Server 是主数据源
- Agent 只保存与自身相关的实例
- payload 可幂等重放

---

## 2. 同步模型

建议采用：

- Server 构建全量快照 payload
- Agent 进行幂等 reconcile
- Agent 返回结果报告

第一阶段优先全量同步，不先做复杂增量 patch。

---

## 3. 下发 Payload

### 3.1 顶层结构

```json
{
  "agent_id": "agent-01",
  "generated_at": 1710000000,
  "version": "2026-03-15T08:00:00Z",
  "installations": []
}
```

字段说明：

| 字段 | 说明 |
|---|---|
| `agent_id` | 目标 Agent |
| `generated_at` | 生成时间 |
| `version` | 本次快照版本 |
| `installations` | 该 Agent 应持有的扩展实例 |

### 3.2 Installation Payload

```json
{
  "installation_id": 1001,
  "installation_key": "official.external-platform:game:g1:env:prod:agent:agent-01",
  "extension_id": "official.external-platform",
  "release_version": "1.0.0",
  "driver": "openapi-driver",
  "enabled": true,
  "scope": {
    "type": "env",
    "id": "prod"
  },
  "target": {
    "type": "agent",
    "id": "agent-01"
  },
  "config": {},
  "secret_refs": {},
  "bindings": []
}
```

字段说明：

| 字段 | 说明 |
|---|---|
| `installation_id` | 服务端安装实例 ID |
| `installation_key` | 幂等键 |
| `extension_id` | 扩展 ID |
| `release_version` | 扩展版本 |
| `driver` | 运行 driver |
| `enabled` | 期望启用状态 |
| `scope` | 业务范围 |
| `target` | 运行目标 |
| `config` | 配置 |
| `secret_refs` | 密钥引用 |
| `bindings` | 运行时绑定 |

### 3.3 Binding Payload

```json
{
  "binding_type": "function",
  "binding_key": "platform.management.invoke",
  "target_ref": "provider:platform.call",
  "spec": {
    "timeout": "15s",
    "retry": 1
  }
}
```

允许的 `binding_type`：

- `function`
- `provider`
- `workflow`
- `job`

---

## 4. Agent 本地处理规则

Agent 在收到 payload 后应执行：

1. 校验 `agent_id`
2. 比较 `version`
3. 遍历 installations
4. 逐个按 `installation_key` reconcile
5. 初始化或更新 driver runtime
6. 处理 bindings
7. 卸载 payload 中已不存在的旧实例
8. 上报结果

要求：

- 重放同一 payload 不应产生重复副作用
- enabled=false 的实例必须停用

---

## 5. Agent 回执 Payload

### 5.1 顶层结构

```json
{
  "agent_id": "agent-01",
  "version": "2026-03-15T08:00:00Z",
  "reported_at": 1710000030,
  "results": []
}
```

### 5.2 Result Item

```json
{
  "installation_id": 1001,
  "installation_key": "official.external-platform:game:g1:env:prod:agent:agent-01",
  "status": "enabled",
  "health_status": "healthy",
  "message": "reconciled",
  "discovered_capabilities": [
    "platform.management:list",
    "platform.management:invoke"
  ],
  "registered_functions": [
    "quicksdk.day_report",
    "quicksdk.user_live"
  ],
  "error": ""
}
```

---

## 6. 对应 Go DTO 草案

### 6.1 Sync Payload

```go
type AgentExtensionSyncPayload struct {
    AgentID       string                     `json:"agent_id"`
    GeneratedAt   int64                      `json:"generated_at"`
    Version       string                     `json:"version"`
    Installations []AgentInstallationPayload `json:"installations"`
}

type AgentInstallationPayload struct {
    InstallationID  uint                      `json:"installation_id"`
    InstallationKey string                    `json:"installation_key"`
    ExtensionID     string                    `json:"extension_id"`
    ReleaseVersion  string                    `json:"release_version"`
    Driver          string                    `json:"driver"`
    Enabled         bool                      `json:"enabled"`
    Scope           AgentScopePayload         `json:"scope"`
    Target          AgentTargetPayload        `json:"target"`
    Config          map[string]any            `json:"config"`
    SecretRefs      map[string]string         `json:"secret_refs"`
    Bindings        []AgentBindingPayload     `json:"bindings"`
}

type AgentScopePayload struct {
    Type string `json:"type"`
    ID   string `json:"id"`
}

type AgentTargetPayload struct {
    Type string `json:"type"`
    ID   string `json:"id"`
}

type AgentBindingPayload struct {
    BindingType string         `json:"binding_type"`
    BindingKey  string         `json:"binding_key"`
    TargetRef   string         `json:"target_ref"`
    Spec        map[string]any `json:"spec"`
}
```

### 6.2 Report Payload

```go
type AgentExtensionReport struct {
    AgentID     string                       `json:"agent_id"`
    Version     string                       `json:"version"`
    ReportedAt  int64                        `json:"reported_at"`
    Results     []AgentExtensionReportItem   `json:"results"`
}

type AgentExtensionReportItem struct {
    InstallationID       uint     `json:"installation_id"`
    InstallationKey      string   `json:"installation_key"`
    Status               string   `json:"status"`
    HealthStatus         string   `json:"health_status"`
    Message              string   `json:"message"`
    DiscoveredCapabilities []string `json:"discovered_capabilities"`
    RegisteredFunctions  []string `json:"registered_functions"`
    Error                string   `json:"error"`
}
```

---

## 7. 第一阶段约束

第一阶段：

- 只做全量快照同步
- 不做 delta patch
- 不做批次灰度编排
- 不做复杂冲突合并

只要保证：

- 幂等
- 可重放
- 可追踪错误

就足够支撑第一批扩展落地。

---

## 8. 下一步

下一步需要补：

- Agent sync API / RPC 入口设计
- Agent 本地 reconcile 顺序图
- payload 签名与认证策略
