# UI 生成架构决策

## 状态

已采纳（2026-07-18）

## 背景

Croupier 的函数注册体系中，proto 层曾经定义了一套 UI 元数据扩展：

- `FunctionOptions` 中的 `display_name`、`summary`、`tags`、`menu`、`permissions`
- `EntityOptions` 中的 `display_name`、`summary`、`tags`、`menu`
- `UIFieldOptions` 中的 `widget`、`label`、`placeholder`、`enum_map`、`show_if`、`required_if`

这些扩展通过 `protoc-gen-croupier` 插件提取，意图是让函数定义者在 proto 源码中声明 UI 如何展示。

## 决策

**废弃 proto 层的全部 UI 元数据扩展，UI 生成完全由 Server 端负责。**

Proto 层只定义函数的 **API 契约**（行为语义），不定义 **展示细节**（UI 元素）。

## 原因

### 1. 静态定义 vs 动态需求

Proto 是编译时固定的。UI 需要根据运行时上下文动态调整：

- 用户角色不同（管理员 vs 运营 vs 客服），看到的界面不同
- 函数是否在线、有多少实例，影响展示方式
- 游戏环境（dev/staging/prod）可能需要不同的 UI 配置

### 2. 耦合代价

如果 UI 定义在 proto 中：

- UI 变更需要重新生成 proto 代码、重新发布 SDK
- 不同语言的 SDK 都需要重新编译
- 运营人员无法自行调整界面，必须找开发改 proto

### 3. 已有完整的替代方案

Server 端已有完整的 UI 生成和覆盖机制：

| 层级 | 机制 | 说明 |
|------|------|------|
| 自动生成 | `ui_resolver.go` | 从 `input_schema`（JSON Schema）自动推导 widget、label、验证规则 |
| 兜底生成 | `fallback_openapi.go` | 根据 function ID 的 entity/action 推导默认字段集 |
| 文件覆盖 | `configs/ui/functions/{id}.yaml` | 支持 YAML/JSON，运维可直接编辑 |
| 运行时编辑 | Dashboard UI 编辑器 | 通过 `PUT /api/v1/functions/:id/ui` 持久化到数据库 |
| 版本管理 | `config_versions` 表 | 支持历史查询和回滚 |

优先级链：`自定义(metadata.ui)` → `文件覆盖(configs/ui/)` → `OpenAPI x-ui` → `兜底生成`

### 4. 分离关注点

| 关注点 | 负责层 | 示例 |
|--------|--------|------|
| 函数身份 | Proto | `function_id`、`version` |
| 行为语义 | Proto | `mode`(query/command)、`risk`、`idempotent`、`timeout` |
| 路由策略 | Proto | `route`(lb/broadcast/targeted) |
| 输入/输出契约 | Proto | `input_schema`、`output_schema`（JSON Schema） |
| UI 展示 | Server + Dashboard | widget、label、菜单、权限、条件显示 |

## Proto 层保留的字段

以下字段属于 **API 契约**，保留在 `FunctionOptions` 中：

- `function_id` — 函数唯一标识
- `version` — 版本号
- `category` — 业务分类（影响路由和分组）
- `risk` — 风险等级（影响审批策略和审计）
- `route` — 路由策略
- `timeout` — 超时设置
- `two_person_rule` — 两人审批
- `placement` — 部署位置
- `mode` — 调用模式
- `idempotency_key` — 幂等支持
- `labels` — 元数据标签

## Proto 层废弃的字段

以下字段已标记 `deprecated = true`，不再被插件或 Server 消费：

### FunctionOptions（字段 12-16）

- `display_name` → Server 从 function ID 和 category 推导
- `summary` → Server 从 description 推导
- `tags` → Server 从 category 和 function ID 推导
- `menu` → Server 的 `descriptors_logic` 生成
- `permissions` → Server 的 `FunctionPolicy` 和 RBAC 系统管理

### EntityOptions（字段 17-20）

- `display_name`、`summary`、`tags`、`menu` → 同上

### UIFieldOptions（全部字段）

- `widget` → Server 从 JSON Schema type/format 自动推导
- `label` → Server 从字段名推导，或通过 configs/ui/ 覆盖
- `placeholder` → Server 从 description 推导
- `sensitive` → Server 端审计脱敏配置
- `enum_map` → Server 从 JSON Schema enum 推导
- `show_if` / `required_if` → Dashboard UI 编辑器配置

## 对现有代码的影响

### protoc-gen-croupier 插件

已清理的代码：

- `parseFunctionOptions()` 不再提取 display_name、summary、tags、menu、permissions
- 移除 `i18nToMap()`、`menuToMap()`、`permissionToMap()`
- 移除 `collectUIFieldHints()`、`UIFieldHints` 类型

保留的代码：

- `parseFunctionOptions()` 仍提取 function_id、category、risk 等行为字段
- `parseEntityOptions()` 仍提取 entity_id、primary_key 等实体定义字段
- `buildJSONSchema()` 仍从 proto 消息生成 JSON Schema

### Server 端

无变更。`ui_resolver.go`、`fallback_openapi.go`、`descriptors_logic.go` 的逻辑不受影响。

### 前端

无变更。`function-ui-generator.ts`、`UISchemaEditor`、`FunctionUIManager` 的逻辑不受影响。

## 迁移指南

如果你的 proto 文件中使用了已废弃的 UI option：

```protobuf
// 旧写法（已废弃）
service PlayerService {
  rpc Ban(BanRequest) returns (BanResponse) {
    option (croupier.options.v1.function) = {
      function_id: "player.ban"
      category: "player"
      risk: "high"
      display_name { zh: "封禁玩家" en: "Ban Player" }  // ← 删除
      menu { nodes: ["Player"] path: "/functions/invoke" }  // ← 删除
    };
  }
}

// 新写法
service PlayerService {
  rpc Ban(BanRequest) returns (BanResponse) {
    option (croupier.options.v1.function) = {
      function_id: "player.ban"
      category: "player"
      risk: "high"
      // UI 由 Server 端自动生成，或通过 Dashboard 编辑器自定义
    };
  }
}
```

如果需要自定义 UI，使用以下方式之一：

1. **文件配置**：创建 `configs/ui/functions/player.ban.yaml`
2. **Dashboard 编辑器**：在函数详情页的"函数表单" Tab 中编辑
3. **API**：`PUT /api/v1/functions/player.ban/ui`
