---
title: PageSpec 协议规范
icon: schema
order: 7
category:
  - 系统架构
tag:
  - JSON Schema
  - ProComponents
  - PageSpec
---

# PageSpec 协议规范

> **状态**：Current -- 本文是 PageSpec/表单展示/selector 等 wire 契约的唯一文档出处。权威实现是 `internal/dashboard/spec`（Go DTO）与 `web/src/types/dashboard.ts`（前端共享类型），两侧逐项对应；本文与实现不一致时以代码为准并修正本文。

## 两种不同的 schema

| 协议        | 所属层           | 用途                                                   | 是否来自注册 |
| ----------- | ---------------- | ------------------------------------------------------ | ------------ |
| JSON Schema | FunctionContract | 输入、输出、校验和字段候选                             | 是           |
| PageSpec    | 页面编排         | 页面类型、列表、详情、动作、任务、报表、导航和 binding | 否           |

JSON Schema 不被当作页面布局树。PageSpec 也不复用 JSON Schema 的组件扩展字段。

## FormPresentationSpec

表单展示由以下稳定结构描述（对应 `spec/form_presentation.go`）：

```ts
interface FormPresentationSpec {
  jsonSchema: JSONSchema; // 表单校验的事实源
  layout?: "vertical" | "horizontal" | "inline" | "grid";
  groups?: FormGroupSpec[]; // 字段分组（key/title/fields/collapsible）
  fields?: FormFieldSpec[]; // 逐字段展示覆盖
  submitButton?: FormButtonSpec;
  cancelButton?: FormButtonSpec;
}

interface FormFieldSpec {
  key: string; // JSON Schema 字段路径，如 "name"、"address.city"
  label?: LocalizedText; // 覆盖 schema title
  description?: LocalizedText; // 帮助文本
  placeholder?: LocalizedText;
  widget?: FormWidget; // 受控 antd 组件名枚举
  width?: number; // grid 栅格 1-12
  order?: number; // 排序，小者在前
  visible?: boolean;
  visibleWhen?: ConditionSpec; // 只读 form/page state
  disabled?: boolean;
  required?: boolean; // 覆盖 schema required
  defaultValue?: JSONValue;
  enumOptions?: EnumOption[]; // select/radio 选项
  widgetProps?: Record<string, JSONValue>;
  validationRules?: ValidationRule[];
}
```

`widget` 是受控枚举，取值为 antd/ProComponents 组件名（PascalCase）：`Input`、`TextArea`、`InputNumber`、`Password`、`Select`、`MultiSelect`、`Radio`、`Checkbox`、`Switch`、`DatePicker`、`TimePicker`、`DateRange`、`Upload`、`ImageUpload`、`FileUpload`、`RichText`、`Code`、`Cascader`、`TreeSelect`、`Color`、`Slider`、`Rate`、`JSON`、`KeyValue`、`Array`、`Object`。扩展 widget 需要修改 spec 包并同步前端类型，不允许前端私加。

可见性条件是受限表达式，不能读 row/详情数据或调用函数：

```ts
interface ConditionSpec {
  kind: "equals" | "notEquals" | "exists" | "all" | "any";
  path?: string; // 当前表单或 page state 的指针
  value?: JSONValue; // equals/notEquals 的比较值
  conditions?: ConditionSpec[]; // all/any 的子条件
}
```

Server 从 input JSON Schema 生成默认 FormPresentationSpec；管理员只能在 Page Studio 改展示信息，不能改变 FunctionContract payload。前端渲染器固定使用 `@rjsf/antd + @rjsf/validator-ajv8` 校验 payload。rjsf `uiSchema` 只能作为 adapter 的内存派生结果，不得成为 SDK/OpenAPI 注册字段或持久页面协议。

历史表单展示数据不提供转换或导入路径；只能在删除旧路径前导出、备份，并由管理员在 Page Studio 按当前 FormPresentationSpec 人工重建。旧数据不得进入新页面发布流程。

## PageSpec 视图节点

PageSpec 使用业务节点而非组件名。节点集合固定为：

```text
ListViewSpec | DetailViewSpec | ActionSpec | ConfirmActionSpec
| TaskViewSpec | ResultViewSpec | FormPresentationSpec
| DatasetSpec | ChartSpec | CompositePageSpec
```

各页面类型的编排见 [Dashboard Resource/Page 模型](./dashboard-page-model.md)。每个节点有版本化、强类型字段和服务端校验器。页面 renderer 根据节点类型选择 ProComponents；PageSpec 不得出现具体 React 组件名或任意组件 props。

### CompositePageSpec（组合页 wire 契约）

`type: composite` 页面的主体（权威实现 `internal/dashboard/spec/types.go`，前端 `web/src/types/dashboard.ts`）。创建入口 `POST /api/v1/versioning/pages/composite`，请求为 `CompositeSectionRequest[]`（字段与 spec 同名透传，见 [使用指南 §3](../dashboard/composite-editor-v3.md)）。

```json
{
  "sections": [
    {
      "key": "player.list",
      "bindingId": "player.list",
      "view": "table",
      "span": 24,
      "autoRun": true,
      "refreshOn": [],
      "table": {
        "columns": [
          { "key": "uid", "title": { "zh-CN": "用户" }, "dataType": "string" }
        ],
        "rowActions": [
          {
            "label": { "zh-CN": "发邮件" },
            "targetSection": "modal-ab12cd",
            "params": { "player_id": "uid" },
            "chain": [{ "kind": "refreshNode", "target": "player.list" }]
          }
        ]
      },
      "toolbar": {
        "actions": [
          {
            "label": { "zh-CN": "批量补偿" },
            "targetSection": "modal-ab12cd",
            "danger": true
          }
        ]
      }
    },
    {
      "key": "mail.send",
      "group": "modal-ab12cd",
      "bindingId": "mail.send",
      "view": "form",
      "display": "dialog",
      "onSuccessRefresh": ["player.list"]
    }
  ]
}
```

字段语义（详细规则见 [Dashboard 页面模型 CompositePage 节](./dashboard-page-model.md)）：

- `key`：区块唯一标识；**同函数多实例**依次 `fid`/`fid-2`…，创建端点重复 key 显式报错
- `display`: `inline`（默认）| `dialog`（弹窗，不占栅格）
- `group`：弹窗分组——`dialog` 区块按 group 聚合渲染进同一弹窗（表单+字段卡+表格混排）；动作目标（`targetSection`）指向 group
- `rowActions[].params`：行字段→表单参数映射（`"player_id": "uid"` = 行的 uid 填入弹窗表单 player_id）
- `chain`：动作链，主动作后按序执行 `runBinding|refreshNode`
- `onSuccessRefresh`：表单提交成功后自动重跑的区块 key
- `events`：通用事件绑定（`rowClick`/`rowSelected`/`success`/`error`/`click` → 动作 + 链）；动作 kind：`runBinding`/`refreshNode`/`openModal`/`closeModal`/`navigate`/`showMessage`；步骤 `params` 支持来源引用（`"区块key.字段"`、`"row.字段"`、字面量）
- `refreshOn`：page_state 联动——上游区块 key 变化自动重跑，上游输出顶层字段同名合并进本区块输入

## 数据引用和 mapping（Selector AST）

输入输出 mapping 必须是可校验的 AST（对应 `spec/selector_ast.go`）：

```ts
type ValueSource =
  | { kind: "form"; path: JsonPointer }
  | { kind: "row"; path: JsonPointer }
  | { kind: "selection"; path: JsonPointer; transform?: { type: "pick" } }
  | { kind: "detail"; path: JsonPointer }
  | { kind: "page_state"; key: string; path?: JsonPointer }
  | { kind: "literal"; value: JSONValue };

interface InputAssignment {
  target: JsonPointer;
  source: ValueSource;
}

interface OutputAssignment {
  stateKey: string;
  source: JsonPointer;
  shape: "scalar" | "object" | "collection" | "task" | "dataset";
}
```

`transform` 是白名单受控变换，当前仅 `pick`（selection 提取行 identity 数组）；新增变换必须扩展 spec 包并同步校验器。

校验器必须确认：

- target 存在于绑定函数的 input JSON Schema。
- source 只引用已定义的表单、行、选择、详情或页面状态。
- source/target 类型可赋值；不允许裸整行对象自动传给函数。
- ListView 的 collection、分页和 identity 引用可被 output JSON Schema 或 CapabilitySemantics 验证。
- Report 的 dataset、指标和维度引用可验证。

禁止保存无结构 `map[string]any`、任意 JSONPath 字符串或组件级临时 mapping 覆盖。

## PageBinding

```ts
interface PageFunctionBinding {
  id: string;
  functionId: string;
  usage:
    | "query"
    | "detail"
    | "action"
    | "task"
    | "task_status"
    | "task_events"
    | "task_result"
    | "task_cancel"
    | "task_retry"
    | "report";
  selectors?: {
    input: SelectorAST; // InputAssignment 集
    output: OutputAssignment[];
  };
  execution: {
    mode: "sync" | "task";
    requireConfirm?: boolean;
  };
}
```

task 生命周期能力拆分为独立 usage（`task_status`/`task_events`/`task_result`/`task_cancel`/`task_retry`），由 TaskSemantic 的 start/status/events/result/cancel 函数一一映射；不存在 `create`/`update`/`delete` usage——写操作统一为 `action`，由 ResourcePage 的 CreateForm/UpdateForm/DeleteAction 节点引用。确认行为由 `execution.requireConfirm` 表达，没有独立的 ConfirmationSpec。

Schema 节点和页面动作只能引用 `bindingId`。运行时由服务端根据 active PublishedPageSpec 找到 functionId、权限、风险、scope 和 dispatch target；浏览器无权选择这些信息。

## 导航与多语言

分类、标题、图标与排序是 PageSpec 顶层字段（`category{key,labels,order}`、`title`、`icon`、`order`）。`NavigationSpec` 仅承载返回导航行为：

```ts
interface NavigationSpec {
  title?: LocalizedText;
  breadcrumb?: LocalizedText[];
  showBack?: boolean;
  backPath?: string;
}
```

PageProposal 根据 resource/page key 提供默认值；PageDraft 保存最终值；PublishedPageSpec 是 Console 动态菜单的唯一来源。静态 locale 与字典都不得成为动态页面事实源。`ConditionSpec` 只读取当前表单或 page state，禁止通过可见性条件访问 row、详情、外部函数或任意 JSONPath，以保证保存、发布和运行时具有一致语义。

## ABI 与版本

`rendererSchemaVersion`（当前 `page-spec:1`）、`generatorVersion`（当前 `page-generator:1`）和各 snapshot digest 必须单独保存。发布校验必须拒绝未知版本、未知节点、未知字段和非法 mapping。页面运行时不得尝试降级、猜测或转换成其他 layout/schema。

## 安全边界

- PageSpec 不存储 route、target service、scope 覆盖、Secret 或任意 HTTP 参数。
- 未知 JSON 只能存在于 JSON Schema 的标准扩展边界；核心 DTO 使用明确类型和 `JSONValue`，禁止 `any`。
- 表单展示变化不允许静默改变已发布页面；发布快照必须包含 FormPresentationSpec。
