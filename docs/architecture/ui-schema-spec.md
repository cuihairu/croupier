# UI Schema 协议规范

## 概述

Croupier 全系统使用 **Formily Schema** 作为唯一的 UI Schema 格式。后端生成、前端编辑、渲染器消费，全链路同一格式，无转换层。

任何非 Formily UI Schema 都是错误输入。保存、加载或渲染阶段必须直接报错，不能转换、猜测或静默降级。

## 关于 `x-` 前缀

Schema 中的 `x-component`、`x-decorator`、`x-component-props`、`x-reactions` 等字段**不是 Croupier 自定义扩展**，而是 [Formily JSON Schema 官方规范](https://react.formilyjs.org/api/shared/schema) 定义的标准字段。

Formily 采用 JSON Schema 的扩展机制（`x-` 前缀约定）来表达 UI 元信息，这是 JSON Schema 规范允许的标准做法（RFC 中 `x-` 前缀保留给实现自行扩展）。

| 字段 | 来源 | 说明 |
|------|------|------|
| `x-component` | Formily 官方 | 指定渲染组件（`Input`、`Select`、`DatePicker` 等） |
| `x-decorator` | Formily 官方 | 指定装饰器组件（通常为 `FormItem`） |
| `x-component-props` | Formily 官方 | 传递给组件的属性（`placeholder`、`min`、`max` 等） |
| `x-reactions` | Formily 官方 | 字段联动逻辑（条件显示、值联动等） |
| `x-data-source` | Formily 官方 | 异步数据源配置 |

Croupier 的 `SchemaRenderer` 组件通过 Formily 的 `createSchemaField` API 注册可用组件，渲染时 Formily 引擎根据 `x-component` 自动选择对应组件，无需任何转换。

---

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

顶层约束：

- `type` 必须是 `object`。
- `properties` 必须是对象。
- `required` 如果存在，必须是字符串数组。
- 字段树中必须存在 `x-component` 或 `x-decorator`。
- 不允许使用 `fields`、`ui:layout`、`ui:groups`、`ui:order`、`widget`、`ui:widget` 作为函数 UI 协议字段。

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
