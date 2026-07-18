# UI 生成架构决策

## 状态

已采纳（2026-07-18），更新（2026-07-18）

## 决策概要

1. **Proto 层废弃 UI 扩展**：UI 生成完全由 Server 端负责
2. **Formily Schema 为唯一 UI Schema 格式**：全系统只用一种格式，无转换层

---

## 一、Proto 层职责分离

### 背景

Proto 层曾经定义了 UI 元数据扩展（`FunctionOptions.menu`/`permissions`、`UIFieldOptions` 全部字段）。这些已废弃。

### 保留的 Proto 字段（API 契约 + 文档）

| 字段 | 性质 |
|------|------|
| `function_id`、`version`、`category`、`risk`、`route`、`timeout` | API 契约 |
| `two_person_rule`、`placement`、`mode`、`idempotency_key`、`labels` | API 契约 |
| `display_name`、`summary`、`tags` | API 文档（流入 OpenAPI summary） |

### 废弃的 Proto 字段

- `FunctionOptions.menu`/`permissions` → Server RBAC 管理
- `EntityOptions.menu` → Server descriptors_logic 生成
- `UIFieldOptions` 全部字段 → Server 从 JSON Schema 推导

---

## 二、统一 UI Schema：Formily Schema

### 为什么选 Formily Schema

| 能力 | Formily Schema | fields 格式 | JSON Schema |
|------|---------------|-------------|-------------|
| 指定组件 | `x-component` | `widget` | ❌ |
| 组件属性 | `x-component-props` | 混在字段里 | `minimum` 等 |
| 条件显示 | `x-reactions` | `show_if` | ❌ |
| 嵌套/数组 | `properties`/`ArrayTable` | ❌ | `properties`/`items` |
| 布局 | `FormGrid`/`Space` | `ui:layout` | ❌ |
| 渲染器 | `SchemaRenderer` 直接渲染 | 需要转换 | 需要转换 |

Formily Schema 是唯一能**直接被渲染器消费**的格式，不需要任何转换层。

### 统一后的数据流

```
JSON Schema (input_schema，来自 SDK 注册)
    │
    ▼ 后端 deriveFormilySchema() — 一次生成
Formily Schema
    │
    ├── 存储：functions.metadata.ui (Formily Schema)
    ├── API：GET/PUT /api/v1/functions/:id/ui (Formily Schema)
    ├── 编辑器：UISchemaEditor 直接编辑 Formily Schema
    ├── 渲染器：SchemaRenderer 直接渲染 Formily Schema（无转换）
    ├── 函数调用页：复用 SchemaRenderer 渲染调用表单
    └── 版本管理：config_versions 表存储 Formily Schema 快照
```

**没有转换层。** 后端生成、前端编辑、渲染器消费，全链路同一格式。

### Formily Schema 规范

#### 顶层结构

```json
{
  "type": "object",
  "properties": {
    "fieldName": { /* 字段定义 */ }
  },
  "required": ["fieldName"]
}
```

#### 字段定义

```json
{
  "type": "string",
  "title": "玩家ID",
  "description": "Player identifier",
  "default": null,

  "x-component": "Input",
  "x-decorator": "FormItem",
  "x-component-props": {
    "placeholder": "请输入玩家ID",
    "maxLength": 64
  },

  "x-reactions": {
    "fulfill": {
      "state": {
        "visible": "{{$values.mode !== 'readonly'}}"
      }
    }
  }
}
```

#### 类型 → 组件映射表

| JSON Schema type | 格式/枚举 | x-component | 说明 |
|-----------------|----------|-------------|------|
| `string` | — | `Input` | 普通文本 |
| `string` | `format: date` | `DatePicker` | 日期 |
| `string` | `format: date-time` | `DatePicker` + `showTime` | 日期时间 |
| `string` | `format: textarea` | `Input.TextArea` | 多行文本 |
| `string` | `enum` | `Select` | 下拉选择 |
| `integer` / `number` | — | `NumberPicker` | 数字输入 |
| `boolean` | — | `Switch` | 开关 |
| `array` | `items.enum` | `Select` + `mode: multiple` | 多选 |
| `array` | `items.object` | `ArrayTable` | 数组表格 |
| `object` | — | `Card` + 递归 | 嵌套对象 |

#### 约束映射

| JSON Schema 约束 | Formily 位置 |
|-----------------|-------------|
| `minimum` | `x-component-props.min` |
| `maximum` | `x-component-props.max` |
| `minLength` | `x-component-props.minLength` |
| `maxLength` | `x-component-props.maxLength` |
| `pattern` | `x-component-props.pattern` |
| `enum` | `enum`（顶层） |
| `required` | `required`（顶层数组） |

#### 布局

```json
{
  "type": "void",
  "x-component": "FormGrid",
  "x-component-props": {
    "minColumns": 2,
    "maxColumns": 3
  },
  "properties": {
    "playerId": { ... },
    "amount": { ... }
  }
}
```

---

## 三、优先级链

UI 配置的加载优先级（不变）：

```
1. metadata.ui (custom)         → 用户通过编辑器保存的 Formily Schema
2. configs/ui/functions/{id}.yaml → 文件级覆盖（也是 Formily Schema）
3. open_api_spec["x-ui"]         → SDK 注册时携带的 x-ui 扩展
4. deriveFormilySchema()         → 从 input_schema 自动生成 Formily Schema
5. BuildFallbackFormilySchema()  → 从 function ID 推断的兜底 Formily Schema
```

所有层级输出的都是 **Formily Schema**，渲染器直接消费。

---

## 四、组件职责

| 组件 | 职责 | 输入 | 输出 |
|------|------|------|------|
| 后端 `ui_resolver.go` | 加载/生成 UI 配置 | 函数记录 | Formily Schema |
| 后端 `fallback.go` | 兜底生成 | function ID | Formily Schema |
| API `/functions/:id/ui` | 读写 UI 配置 | HTTP | Formily Schema |
| `SchemaRenderer` | 渲染表单 | Formily Schema | React 组件 |
| `UISchemaEditor` | 编辑 UI 配置 | Formily Schema | Formily Schema |
| `FunctionUIManager` | UI 管理面板 | functionId | 管理界面 |

**废弃的组件**：
- `FunctionFormRenderer` → 被 `SchemaRenderer` 替代
- `function-ui-generator.ts` → 前端不再需要独立的 UI 生成逻辑
- `functionUi.ts` 的 `toEditorUISchema`/`toRenderableUISchema` → 不再需要格式转换

---

## 五、API 契约

### GET /api/v1/functions/:id/ui

```json
{
  "schema": { /* Formily Schema */ },
  "layout": { "type": "grid", "cols": 2 },
  "components": {},
  "custom": true,
  "hasDefault": true,
  "uiSource": "custom_metadata",
  "uiSourceDetail": "metadata.ui (custom override)"
}
```

### PUT /api/v1/functions/:id/ui

```json
{
  "schema": { /* Formily Schema */ },
  "layout": { "type": "grid", "cols": 2 },
  "components": {}
}
```

清除自定义 UI：
```json
{
  "schema": { "__clear_custom_ui": true }
}
```

---

## 六、迁移指南

### Proto 文件

```protobuf
// 保留
option (croupier.options.v1.function) = {
  function_id: "player.ban"
  category: "player"
  risk: "high"
  display_name { zh: "封禁玩家" en: "Ban Player" }
  summary { zh: "封禁指定玩家" en: "Ban a player" }
  tags: ["player", "moderation"]
  // menu 和 permissions 已废弃，由 Server 端管理
};
```

### 自定义 UI

不再需要手动编写 JSON Schema 或 fields 格式。使用以下方式之一：

1. **Dashboard 编辑器**：函数详情页 → 函数表单 Tab → 编辑
2. **API**：`PUT /api/v1/functions/:id/ui`（直接传 Formily Schema）
3. **文件配置**：`configs/ui/functions/{id}.yaml`（Formily Schema 格式）
