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

Formily 是蚂蚁金服开源的表单方案，其 JSON Schema 规范通过 `x-` 前缀字段（`x-component`、`x-decorator`、`x-component-props`、`x-reactions`）表达 UI 元信息。这些是 [Formily 官方规范](https://react.formilyjs.org/api/shared/schema) 定义的标准字段，不是 Croupier 自定义扩展。

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
    ├── SchemaRenderer：直接渲染 Formily Schema（无转换）
    └── UISchemaEditor：Formily ↔ FieldConfig 双向转换（编辑器内部）
```

> **注意**：`UISchemaEditor` 内部使用 FieldConfig 格式（widget/placeholder）驱动编辑 UI，
> 读写时与 Formily Schema 双向转换。转换会保留核心字段（type/title/x-component/x-component-props），
> 但复杂的 Formily 扩展（x-reactions 联动、x-decorator-props、x-data-source、嵌套 properties/items）
> 需要通过代码编辑器 Tab 直接编辑 JSON 来配置。
>
> `Functions/Invoke` 页面仍使用 `FunctionFormRenderer`（antd Form）渲染调用表单，
> 通过 JSON Schema type 推断 widget，不消费 Formily 扩展字段。

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
| `SchemaRenderer` | 渲染表单（管理页预览） | Formily Schema | React 组件 |
| `UISchemaEditor` | 编辑 UI 配置 | Formily Schema ↔ FieldConfig | Formily Schema |
| `FunctionFormRenderer` | 渲染调用表单（执行页） | JSON Schema | antd Form |
| `FunctionUIManager` | UI 管理面板 | functionId | 管理界面 |

**已删除的组件**：
- `FunctionFormRenderer` 的 legacy 预览分支（`featureFlags.formilyDesigner=false`）
- `function-ui-generator.ts`（前端默认 UI 生成，已由后端承担）
- `functionUi.ts`（旧格式转换工具，已无引用）
- `FunctionComponents/`（旧 barrel 导出，已无引用）

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
