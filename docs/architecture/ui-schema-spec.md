# UI Schema 协议规范

## 概述

Croupier 全系统使用 **Formily Schema** 作为唯一的 UI Schema 格式。后端生成、前端编辑、渲染器消费，全链路同一格式，无转换层。

## 格式规范

### 顶层结构

```json
{
  "type": "object",
  "properties": {
    "fieldName": { ... }
  },
  "required": ["fieldName"]
}
```

符合 [Formily JSON Schema 规范](https://react.formilyjs.org/api/shared/schema)。

### 字段定义

每个字段是一个 Formily Schema 节点：

```json
{
  "type": "string",
  "title": "显示名称",
  "description": "字段描述",

  "x-component": "Input",
  "x-decorator": "FormItem",
  "x-component-props": {
    "placeholder": "请输入",
    "maxLength": 64
  }
}
```

### 类型 → 组件映射

| JSON Schema type | 格式/枚举 | x-component | x-component-props |
|-----------------|----------|-------------|-------------------|
| `string` | — | `Input` | `placeholder`, `maxLength` |
| `string` | `format: "date"` | `DatePicker` | `format: "YYYY-MM-DD"` |
| `string` | `format: "date-time"` | `DatePicker` | `showTime: true` |
| `string` | `format: "time"` | `TimePicker` | `format: "HH:mm:ss"` |
| `string` | `format: "textarea"` | `Input.TextArea` | `rows: 3` |
| `string` | `enum: [...]` | `Select` | `options` |
| `integer` / `number` | — | `NumberPicker` | `min`, `max`, `step`, `precision` |
| `boolean` | — | `Switch` | — |
| `array` | `items: { enum }` | `Select` | `mode: "multiple"` |
| `array` | `items: { object }` | `ArrayTable` | — |
| `object` | — | `Card` + 递归 properties | — |

### 约束映射

| JSON Schema 约束 | Formily 位置 |
|-----------------|-------------|
| `minimum` | `x-component-props.min` |
| `maximum` | `x-component-props.max` |
| `minLength` | `x-component-props.minLength` |
| `maxLength` | `x-component-props.maxLength` |
| `pattern` | `x-component-props.pattern` |
| `enum` | 顶层 `enum` 字段 |
| `required` | 顶层 `required` 数组 |
| `default` | 顶层 `default` 字段 |

### 条件显示

使用 Formily 的 `x-reactions` 实现：

```json
{
  "x-reactions": {
    "dependencies": ["mode"],
    "fulfill": {
      "state": {
        "visible": "{{$deps[0] !== 'readonly'}}"
      }
    }
  }
}
```

### 布局

使用 Formily 的 `FormGrid` 组件：

```json
{
  "type": "void",
  "x-component": "FormGrid",
  "x-component-props": {
    "minColumns": 2,
    "maxColumns": 3
  },
  "properties": {
    "field1": { ... },
    "field2": { ... }
  }
}
```

### 分组

使用 Formily 的 `Card` 或自定义分组组件：

```json
{
  "type": "void",
  "x-component": "Card",
  "x-component-props": {
    "title": "基本信息"
  },
  "properties": {
    "playerId": { ... }
  }
}
```

---

## 历史格式（已废弃）

以下格式曾存在于系统中，现已统一为 Formily Schema：

| 格式 | 位置 | 状态 |
|------|------|------|
| JSON Schema 格式 `{type, properties}` | 后端 `deriveUISchemaFromJSONSchema` | 已废弃，改为输出 Formily Schema |
| fields 格式 `{fields, ui:layout}` | 前端 `buildUISchemaFromJSONSchema` | 已废弃，改为输出 Formily Schema |
| fields 格式 `{fields, ui:groups}` | `UISchemaEditor` | 已废弃，改为编辑 Formily Schema |
| 分离格式 JSON Schema + UISchema | `FunctionFormRenderer` | 已废弃，改用 `SchemaRenderer` |
