---
title: UI Schema 与 PageSpec 规范
icon: schema
order: 7
category:
  - 系统架构
tag:
  - JSON Schema
  - ProComponents
  - PageSpec
---

# UI Schema 与 PageSpec 规范

> **状态**：Target -- 页面协议采用强类型 PageSpec；表单展示采用 JSON Schema + FormPresentationSpec。

## 两种不同的 schema

| 协议 | 所属层 | 用途 | 是否来自注册 |
| --- | --- | --- | --- |
| JSON Schema | FunctionContract | 输入、输出、校验和字段候选 | 是 |
| PageSpec | 页面编排 | 页面类型、列表、详情、动作、任务、报表、导航和 binding | 否 |

JSON Schema 不被当作页面布局树。PageSpec 也不复用 JSON Schema 的组件扩展字段。

## FormPresentationSpec

表单展示由以下稳定结构描述：

```ts
interface FormPresentationSpec {
  schema: JSONSchema;
  fields: FormFieldSpec[];
  layout: FormLayoutSpec;
}

interface FormFieldSpec {
  path: JsonPointer;
  label?: LocalizedText;
  help?: LocalizedText;
  widget?: 'text' | 'textarea' | 'number' | 'switch' | 'select' | 'date' | 'date_time' | 'json';
  group?: string;
  order: number;
  visibleWhen?: ConditionSpec;
  readOnly?: boolean;
}

type ConditionSpec =
  | { kind: 'all'; conditions: ConditionSpec[] }
  | { kind: 'any'; conditions: ConditionSpec[] }
  | { kind: 'not'; condition: ConditionSpec }
  | { kind: 'exists'; source: ConditionSource }
  | { kind: 'equals'; source: ConditionSource; value: JSONValue };

type ConditionSource =
  | { kind: 'form'; path: JsonPointer }
  | { kind: 'page_state'; key: string; path?: JsonPointer };
```

Server 从 input JSON Schema 生成默认 FormPresentationSpec；管理员只能在 Page Studio 改展示信息，不能改变 FunctionContract payload。前端渲染器固定使用 `@rjsf/antd + @rjsf/validator-ajv8` 校验 payload。rjsf `uiSchema` 只能作为 adapter 的内存派生结果，不得成为 SDK/OpenAPI 注册字段或持久页面协议。

历史表单展示数据不提供转换或导入路径；只能在删除旧路径前导出、备份，并由管理员在 Page Studio 按当前 FormPresentationSpec 人工重建。旧数据不得进入新页面发布流程。

## PageSpec 节点

PageSpec 使用业务节点而非组件名。最小可用的节点集合固定为：

```ts
type PageNode =
  | QueryViewSpec
  | ListViewSpec
  | DetailViewSpec
  | FormActionSpec
  | ConfirmActionSpec
  | TaskViewSpec
  | ReportViewSpec
  | ResultViewSpec;
```

每个节点有版本化、强类型字段和服务端校验器。页面 renderer 根据节点类型选择 ProComponents；PageSpec 不得出现具体 React 组件名或任意组件 props。

### 数据引用和 mapping

输入输出 mapping 必须是可校验的 AST：

```ts
type ValueSource =
  | { kind: 'form'; path: JsonPointer }
  | { kind: 'row'; path: JsonPointer }
  | { kind: 'selection'; path: JsonPointer }
  | { kind: 'detail'; path: JsonPointer }
  | { kind: 'page_state'; key: string; path?: JsonPointer }
  | { kind: 'literal'; value: JSONValue };

interface InputAssignment {
  target: JsonPointer;
  source: ValueSource;
}

interface OutputAssignment {
  stateKey: string;
  source: JsonPointer;
  shape: 'scalar' | 'object' | 'collection' | 'task' | 'dataset';
}
```

校验器必须确认：

- target 存在于绑定函数的 input JSON Schema。
- source 只引用已定义的表单、行、选择、详情或页面状态。
- source/target 类型可赋值；不允许裸整行对象自动传给函数。
- ListView 的 collection、分页和 identity 引用可被 output JSON Schema 或 CapabilitySemantics 验证。
- ReportView 的数据集、指标和维度引用可验证。

禁止保存无结构 `map[string]any`、任意 JSONPath 字符串或组件级临时 mapping 覆盖。

## PageBinding

```ts
interface PageBinding {
  id: string;
  functionId: string;
  usage: 'query' | 'detail' | 'create' | 'update' | 'delete' | 'action' | 'task' | 'report';
  input: InputAssignment[];
  output: OutputAssignment[];
  confirmation: ConfirmationSpec;
}
```

Schema 节点和页面动作只能引用 `bindingId`。运行时由服务端根据 active PublishedPageSpec 找到 functionId、权限、风险、scope 和 dispatch target；浏览器无权选择这些信息。

## 导航与多语言

导航是 PageSpec 的强类型部分：

```ts
interface NavigationSpec {
  category: {
    key: string;
    labels: LocalizedText;
    order: number;
  };
  title: LocalizedText;
  description?: LocalizedText;
  icon?: string;
  order: number;
}
```

PageProposal 根据 resource/page key 提供默认值；PageDraft 保存最终值；PublishedPageSpec 是 Console 动态菜单的唯一来源。静态 locale 与字典都不得成为动态页面事实源。`ConditionSpec` 只读取当前表单或 page state，禁止通过可见性条件访问 row、详情、外部函数或任意 JSONPath，以保证保存、发布和运行时具有一致语义。

## ABI 与版本

`pageSpecVersion`、`formPresentationVersion`、`generatorVersion` 和 renderer version 必须单独保存。发布校验必须拒绝未知版本、未知节点、未知字段和非法 mapping。页面运行时不得尝试降级、猜测或转换成其他 layout/schema。

## 安全边界

- PageSpec 不存储 route、target service、scope 覆盖、Secret 或任意 HTTP 参数。
- 未知 JSON 只能存在于 JSON Schema 的标准扩展边界；核心 DTO 使用明确类型和 `JSONValue`，禁止 `any`。
- 表单展示变化不允许静默改变已发布页面；发布快照必须包含 FormPresentationSpec。
