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
  remoteOptions?: RemoteOptionsSpec; // 远程选项源（x-options-source 派生）；
  // 与 enumOptions 互斥，优先 remoteOptions
  widgetProps?: Record<string, JSONValue>;
  validationRules?: ValidationRule[];
}

interface RemoteOptionsSpec {
  functionId: string; // 数据源函数 ID（同 scope 已注册的 collection_query）；
  // 调用走既有 RBAC，失败静默降级为普通输入
  labelPath?: string; // 选项标签取值路径（JSON Pointer，支持 * 通配数组段）
  valuePath?: string; // 选项取值路径；缺省复用 labelPath
  searchParam?: string; // 搜索关键词参数名；声明后下拉搜索以该参数重新调用
}
```

渲染器生效语义（`SchemaFormRenderer`，2026-09 收口）：`required` 与 `validationRules[].type="required"` 并入运行时 schema `required`（`required: false` 可撤销 schema 原必填；visibleWhen 隐藏字段仍豁免）；`min`/`max` 按字段类型映射（string→`minLength/maxLength`，number/integer→`minimum/maximum`，array→`minItems/maxItems`），`pattern` 直写 JSON Schema `pattern`；`custom` 无法用 JSON Schema 表达，渲染时显式告警并忽略（不静默丢弃）。required 的字符串字段（schema `required` 与 fields 覆盖两来源一致生效）在派生运行时 schema 时若未显式声明 `minLength` 自动补 `minLength: 1`——AJV `required` 只查键存在、空串合法，不补则「必填」拦不住清空后的提交（BUG-038 关联观察：空串出网到游戏侧才报错）；显式 `minLength`（含 `validationRules` 映射产物）不覆盖，visibleWhen 隐藏豁免字段不注入；错误文案对 `minLength limit=1` 特化为「{title} 不能为空」。`defaultValue` 参与表单初值种入：仅补未提供值的字段，显式 `initialValues` 优先，`null`/空串视为已提供不覆盖。

`widget` 是受控枚举，取值为 antd/ProComponents 组件名（PascalCase）：`Input`、`TextArea`、`InputNumber`、`Password`、`Select`、`MultiSelect`、`Radio`、`Checkbox`、`Switch`、`DatePicker`、`TimePicker`、`DateRange`、`Upload`、`ImageUpload`、`FileUpload`、`RichText`、`Code`、`Cascader`、`TreeSelect`、`Color`、`Slider`、`Rate`、`JSON`、`KeyValue`、`Array`、`Object`。扩展 widget 需要修改 spec 包并同步前端类型，不允许前端私加。

可见性条件是受限表达式，不能读 row/详情数据或调用函数：

```ts
interface ConditionSpec {
  kind: "equals" | "notEquals" | "exists" | "all" | "any";
  path?: string; // 当前表单或 page state 的指针
  key?: string; // 区块级条件专用：来源区块 key（非空时从页面状态
  // results[key] 按 path 取值，如 /values/mode、/data/total；
  // 空 = 表单内相对路径，字段级 visibleWhen 原语义）
  value?: JSONValue; // equals/notEquals 的比较值
  conditions?: ConditionSpec[]; // all/any 的子条件
}
```

Server 从 input JSON Schema 生成默认 FormPresentationSpec；管理员只能在 Page Studio 改展示信息，不能改变 FunctionContract payload。前端渲染器固定使用 `@rjsf/antd + @rjsf/validator-ajv8` 校验 payload。rjsf `uiSchema` 只能作为 adapter 的内存派生结果，不得成为 SDK/OpenAPI 注册字段或持久页面协议。

历史表单展示数据不提供转换或导入路径；只能在删除旧路径前导出、备份，并由管理员在 Page Studio 按当前 FormPresentationSpec 人工重建。旧数据不得进入新页面发布流程。

### Descriptor 呈现 hints（x-ui-\*）

SDK/游戏方可在 `input_schema` 的字段 schema 上声明 `x-ui-*` 扩展字段（widget/label/
分组/联动/远程选项源等），前端经 `derivePresentationSpec`（调试路径）与服务端
`buildFormFields`（发布路径，U4 起含 `x-options-source` → `remoteOptions`）推导为本
协议的 `FormPresentationSpec`，两端表现一致。字段清单、推导规则与治理边界见
呈现 Hints 契约。hints 不进入页面编排（PageSpec），wire
契约零改动。

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

**编辑器预览端点（B1）**：`POST /api/v1/versioning/pages/composite/preview` 请求体与创建入口完全相同，
返回 `{ pageKey, pageSpec }`，其中 `pageSpec` 为完整生成产物（`type`/`title`/`composite`/`bindings`/
`componentTemplates`，外加生成元信息 `quality` 与 `diagnostics`）。它走与创建相同的生成链——区块契约
解析、绑定分配、static 区块交错、模板快照透传、发布级 selector 校验——因此返回的 spec 与保存/发布落库
的 spec 逐字段一致（由 `TestBuildCompositePageSpec_SameAsPersistedProposal` 与
`TestV9Handler_PreviewCompositePage_SameSpecAsSaved` 验证）。该端点只读：不写提案/草稿/审计行，因此
不需要 `pages:publish` 权限；契约可见性仍受请求链上的 scope 与权限中间件约束。任一区块缺契约时按统一
`{ error, message, details }` 形状返回 `4xx`，而不是返回残缺 spec。

输入校验失败一律不返回残缺 spec：`pageKey` 缺失、区块 `key` 缺失或冲突、static 区块缺
`form.jsonSchema`、区块引用未登记契约的函数、画布无功能性区块可生成，均返回
`400 + { error: "validation_failed", message, details }`，`details` 的每个键都是请求体字段路径——
`pageKey`、`sections[i].key`、`sections[i].functionId`、`sections[i].form.jsonSchema`，无功能性区块
时落裸 `sections`；生成器级诊断按同一形态映射（取其 `field`，缺省回退 `code`）。契约查询遇到数据库
故障不属画布问题，仍按 `500` 兜底。预览栏与保存/发布弹窗经共享的 `ErrorDetailList` 把这些条目内联
逐条呈现，便于定位到具体区块。

**草稿预览执行端点（B1，2026-10）**：`POST /api/v1/versioning/pages/composite/preview/execute` 让编辑器预览栏
在草稿上真实执行区块，而不只是模拟数据。请求体是预览端点的请求体加 `bindingId` 与 `context`：

```json
{
  "pageKey": "composite--preview",
  "bindingId": "player.list",
  "sections": [
    { "key": "player.list", "functionId": "player.list", "view": "table" }
  ],
  "componentTemplates": [{ "key": "tpl--card", "digest": "d1" }],
  "context": {
    "form": {},
    "row": {},
    "selection": [],
    "detail": {},
    "pageState": {}
  }
}
```

服务端按同一生成链从 `sections` 重建草稿 `PageSpec`（即预览栏渲染的那一份），重跑发布级 selector 校验，
再走与已发布页面相同的执行门禁——权限校验、审批闸门、绑定的 `execution.functionId` 快照（浏览器不提交
`functionId`/`route`/`scope`；`bindingId` 缺失或多余均按统一形状返回 `4xx`）。响应为 `{ result }`，与已发布
区块执行端点的 `PageExecutionResult` 同形，渲染器对两侧按同一套结果语义处理。该端点仍然只读——不落
提案/草稿/审批/审计——但会下发真实函数调用，因此操作类函数与审批在草稿模式下确有真实副作用。

**常量表单区块（static，2026-09）**：`CompositeSection` 增加 `static: true` 形态——
不绑定 bindingId、不执行任何函数；`form.jsonSchema` 由编辑器设计期定义并经
`CompositeSectionRequest.form` 透传（服务端校验 schema 存在性），值仅写入页面状态供
refreshOn/动作链消费。发布校验：static 区块禁止携带 bindingId、view 必须为 form、
必须包含 form.jsonSchema。区块顺序：static 区块**按请求输入位置交错落库**（2026-09
修订，此前服务端把 static 统一追加到 sections 末尾，导致"常量筛选放在页首"的页面
发布后顺序漂移）；accept-and-publish 后发布快照与请求 sections 顺序一致。

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

- `key`：区块唯一标识；**同函数多实例**依次 `fid`/`fid-2`…，编辑器可声明固定 key（`sectionKey` 固化，round-trip 不漂移），创建端点重复 key 显式报错
- `display`: `inline`（默认）| `dialog`（弹窗，不占栅格）| `tab`（页签，渲染端聚合进 Tabs）| `card`（卡片分组，渲染端聚合进 Card）
- `group`：弹窗/页签/卡片分组——`dialog` 区块按 group 聚合渲染进同一弹窗（表单+字段卡+表格混排）；动作目标（`targetSection`）指向 group；`tab` 区块按 group 聚合渲染进同一 Tabs；`card` 区块按 group 聚合渲染进同一 Card
- `tab`：页签标签（LocalizedText；`display=tab` 时有效）——同 group 内按标签聚合到 Tabs 对应页，页内区块整行堆叠；缺省时渲染端兜底「页签 N」
- `cardTitle`：卡片标题（LocalizedText；`display=card` 时有效）——同 group 区块渲染进同一 Card（整行）时的组标题；缺省回退组名。回读时还原为卡片容器的 `props.title`
- `rowActions[].params`：行字段→表单参数映射（`"player_id": "uid"` = 行的 uid 填入弹窗表单 player_id）
- `chain`：动作链，主动作后按序执行 `runBinding|refreshNode`
- `onSuccessRefresh`：表单提交成功后自动重跑的区块 key——仅业务执行成功才重跑：`approval`/`task` 结果不重跑目标（D4，见下方执行结果反馈条目）
- `events`：通用事件绑定（`rowClick`/`rowSelected`/`success`/`error`/`click` → 动作 + 链）；动作 kind：`runBinding`/`refreshNode`/`openModal`/`closeModal`/`navigate`/`showMessage`；步骤 `params` 支持来源引用（`"区块key.字段"`、`"row.字段"`、字面量）
- `refreshOn`：page_state 联动——上游区块 key 变化自动重跑，上游输出顶层字段同名合并进本区块输入
- `visibleWhen`：**区块级条件显示**（U10）——`ConditionSpec` 叶子必填 `key`（来源区块），从页面状态 `results[key]` 按 `path`（`/values/字段`、`/data/字段`、`/selectedRow/字段`）取值求值；false 的 inline/tab 区块不渲染但执行照常（autoRun/refreshOn 不受影响）；支持嵌套 `all`/`any`（深度 ≤4）；发布校验 key ∈ 页面区块、path 为 JSON Pointer、equals/notEquals 带 value；dialog 区块不参与（弹窗由动作显式触发）
- `cascadePolicy`：**refreshOn 级联失败策略**（U9，`"pause" | "clear" | "keep"`，空 = 缺省 `pause`）——任一上游依赖最新结果为失败时本区块的行为。三策略均不重跑：`pause` 数据保持 + message.warning 提示（联动已暂停）；`keep` 静默保留上次结果；`clear` 清空本区块数据。渲染端实现（B2）：区块执行异常时向 `results[key]` 写入 `{error}` 失败标记，级联信号按「区块 key : 结果修订号 : 失败标记（o/e）」计算——修订号每次产出新结果 +1（成功与失败都算新结果），因此上游同态新值（连续两次成功但数据不同）同样触发下游重跑；失败→成功翻转同样触发（上游恢复后暂停的级联自动续跑）。同一区块仅最新在飞响应落账（被取代的旧响应拒收），`refreshOn` 图回溯到自身的区块拒绝联动（无重复执行与无限级联），`pause` 提示仅在上游失败首次出现时给出、同态连续失败不重复打扰。发布校验枚举合法（`composite_section_cascade_policy_invalid`）。static 区块不执行绑定（自身不会失败），该字段无实际作用但允许透传

- `table.pagination`：**表格区块服务端分页**（D3）——与资源页 `PaginationSpec` 同构
  （`{enabled, defaultSize?, pageSizes?}`）。`enabled` 时渲染器向本区块请求 form 追加
  `page`（1-based）/ `pageSize` / `offset=(page-1)*pageSize`——offset 由渲染器派生并与
  页码一并下发（不把页码当偏移量传，也不把偏移量当页码传；平台输入 schema 默认放行
  附加键）。响应按 `items` + 可选 `total` 消费：带 `total` 原样使用；缺 `total` 时按
  「本页满则可能有下一页」估算上界、不满页即收口——缺 total 不会被误判为「只剩当前
  页」。翻页/改每页条数只变分页参数重跑本区块；上游筛选/查询条件变化（本区块的非分页
  入参）回首页；操作成功后的刷新（`onSuccessRefresh`/事件 `refreshNode`）保持当前页码
  与筛选。未设置或 `enabled: false` 保持既有客户端分页（请求不带分页参数，存量页面行为
  不变）。该字段由组合页编辑器的表格组件设置（服务端分页开关 + 每页条数 + 可选条数），
  经组合区块请求的区块级 `pagination` 字段透传，生成器写入 `table.pagination`；生成器
  从不为组合区块推导分页（无资源语义上下文），故为逐表格显式 opt-in。绑定函数须实际
  消费 `page`/`pageSize`——平台放行附加输入键，函数忽略即每页都返回全量（每页内容相同、
  页数按上界虚高）。边界：游标分页与自定义 `totalKey` 未实现（wire `PaginationSpec` 无
  游标字段）；弹窗（dialog）内渲染的表格不分页且无表头排序；服务端筛选下推未做（组合页
  筛选走查询表单 refreshOn 级联——已实现的排序下推见下方列排序条目）

- `table.selection`：**组合区块表格行选择**（D2）——`{multiple?, identityKey?}`。未设置或
  `multiple: false` 保持既有单选 radio（选中行即 `selectedRow`，`rowSelected` 事件以该单行
  触发）。`multiple: true` 时本区块表格切换为 checkbox 多选，并在表格上方渲染选择栏：已选
  总数计数；当部分已选对象不在当前页时显示「本页可见」计数（算子无法只靠当前页核对目标）；
  「查看目标清单」入口，弹窗逐条列出全部已选对象（身份列置前，最多再带两列展示列）；
  「取消选择」；以及筛选变化后的旧选择核对提示（翻页不算筛选变化；筛选变化且选择非空即提示
  ——跨页保留的键无法区分「仍在其它页」与「已被新筛选排除」，需算子核对后清除或重新勾选）。
  `rowKey`、目标清单、计数与运行时 `selectedRow`/`selectedRows` 状态取同一行身份口径：
  `selection.identityKey` → `table.identityKey` → 行 `id`/`key` → 行对象本身。选择写入区块
  运行时状态（`selectedRow`=首行、`selectedRows`=全行）——与 `rowSelected` 事件、
  <code v-pre>{{...selectedRow.x}}</code> 表达式及服务端 inputAssignment 同源；`rowSelected` 仍按首次勾选的
  首行触发。该字段由组合页编辑器表格组件设置（行选择开关：启用多选 + 从输出 schema 选行
  身份列），经组合区块请求的区块级 `selection` 字段透传，生成器写入 `table.selection`；
  生成器从不为组合区块推导行选择（无资源语义上下文），故为逐表格显式 opt-in。边界：弹窗
  （dialog）内渲染的表格无选择栏；目标清单弹窗最多展示三列（身份列置前）；「本页可见」计数
  按本区块 `items` 计算（服务端分页即当前页；客户端分页即全量结果集，此时与已选数相等、
  该提示不出现）；除表头全选外无逐行选择/禁用控制

- `table.columns[].sortable`：**表格区块列排序**（D3）——生成器按输出 schema 数据类型机械
  标注：`dataType` 为 `number` 或 `datetime` 的列可排序（数据类型派生，非资源语义推断；与
  分页/行选择不同，这不是编辑器显式 opt-in——未启用服务端分页时排序不构成对绑定函数接受
  排序参数的隐式假设）。可排序列渲染 antd 表头排序 UI；`dataType` 为
  `string`/`boolean`/`object`/`array` 的列不可排序。`table.pagination.enabled` 为 false（或
  未设置）时排序在本地：渲染器对返回行做 compare 排序（number 按数值、其余按字符串码点、
  缺失值排末尾），不下发排序请求参数。服务端分页开启时 sorter 仅表头 UI（`sorter: true`），
  由受控 order 状态驱动请求：点击表头以 `sortBy`（列 key）与 `sortOrder`（`asc`/`desc`）
  附加进请求表单并回首页重跑；再点同列表头切换相反序（两态，无取消态）。排序不属于区块
  筛选指纹：状态存渲染器按区块维护的表格状态（不写入 `sectionInputs`），故排序重跑不触发
  D2 旧选择核对提示；上游筛选变化（回首页）保持当前排序。边界：仅两态序（无三态取消、
  无多列排序）；排序不跨页面刷新保留；请求参数为 `sortBy`/`sortOrder`（无 `orderBy`/
  `direction` 别名）；绑定函数须真正消费这两个键（平台放行附加输入键，函数忽略即返回
  未排序页、UI 却显示排序箭头）；弹窗内渲染的表格逐列原样渲染（无表头排序 UI）

- `danger` + `chain`（rowActions / toolbar.actions）：**危险操作确认契约**（D2）——`danger`
  动作在用户显式确认前绝不执行。带 `targetSection` 时确认框列出目标弹窗的预填参数与动作链
  步骤，确认后才打开弹窗并执行动作链；不带 `targetSection`（纯动作链按钮）时确认框列出动作
  链步骤。确认内容按目标表单的字段标题渲染参数、枚举值映射为选项文案、参数名人性化展示
  （不显示原始字段 key），所见即实际将提交的内容。非危险动作带 `targetSection` 时直接开弹窗
  并执行动作链（详情弹窗用动作链预载自身 fields/table 数据）；非危险纯链动作直接执行动作链。
  边界：确认框不做字段语义强调（游戏/环境/区服/玩家身份、原因、封禁到期时间、补偿每人数量
  不单独突出——依赖 D5 的显式字段语义）；`openModal` 链步骤无 danger 标志位（wire 无该
  字段），链内打开弹窗不触发确认；确认发生在弹窗打开之前，弹窗内二次编辑不再次确认
- **执行结果反馈**（D4，无新增 wire 字段）：渲染器按 `PageExecutionResult.kind` 分类
  每次执行结果，不再把任何非 error 返回都当作成功。`sync`（或无 kind）为业务执行成功——
  触发表单 `success` 事件绑定、重跑 `onSuccessRefresh` 目标并关闭弹窗。`approval` 保持
  弹窗开启，显示「已提交审批」提示、审批单号（`approvalId`）与审批中心入口，且不发
  success 事件、不触发刷新（审批通过前业务侧尚未变更）。`task` 保持弹窗开启，显示
  「任务已提交」提示与任务号（`taskId`）。抛出的异常保持弹窗与已填参数开启，并在弹窗内
  渲染失败反馈：响应体稳定 `error` 码、服务端 `message`，以及 `details` 的字段级条目
  （按表单字段标题映射，不显示原始字段 key）；结果体自带顶层 `error` 字段（未抛异常）
  同样按失败处理，不发 success 事件。`details` 中字段存在于区块表单 schema 的条目经共享
  表单渲染器的 `externalErrors` prop 注入对应控件下方（RJSF `extraErrors`），反馈条只汇总
  表单外字段的错误。边界：inline（非弹窗）表单区块提交期间整卡进 loading 骨架，失败后
  输入值不保留（弹窗表单会保留）；无任务中心路由，待处理任务只给任务号不给导航目标；
  审批入口固定跳 `/approvals`
- **执行幂等**（D4，`Idempotency-Key` 请求头；无新增 wire 字段）：每次页面 binding 执行
  ——已发布与草稿预览一视同仁——都带客户端生成的幂等键，服务端保证同一 key 在占位窗口内
  至多真实执行一次。占位时机在全部执行门禁通过之后（执行器已绑定、契约新鲜、治理、
  payload 校验）、审批/下发分支之前，门禁失败不消耗 key。结果语义：已完成的 key 原样
  重放存储结果（响应体带 `replayed: true`，不重复下发、不重复写审计与执行留痕——一次
  动作一条记录）；占位窗口内仍进行中的 key 判 `409 idempotency_key_in_use`（结果未知，
  请稍后用同一请求重试）；同 key 但请求内容不同（页面版本、绑定、函数或 payload）判
  `409 idempotency_key_reused`；超过 15 分钟仍为 `running` 的占位视为遗留，可由同内容
  请求回收重跑。不带头即不启用幂等——既有行为（每次提交都执行）完全不变。客户端按
  「一次执行意图」（页面 + 绑定 + 上下文）保留同一个 key 直至该意图闭环：`sync` 结果释放
  （之后同内容提交是新意图），`approval`/`task` 结果保留（重复提交重放同一审批单/任务号
  而非新建），保留窗口 10 分钟（每标签页内存缓存），刻意短于服务端占位窗口；重放的
  同步成功提示为「返回服务端记录的结果」而非新的成功。记录惰性清扫（保留 24 小时，每
  进程每 10 分钟至多一次 DELETE）。边界：下发失败后占位保持 `running` 满 15 分钟窗口
  （副作用确实不可判定——窗口内重试得到 `idempotency_key_in_use` 而非静默重跑）；客户端
  key 仅存内存（刷新页面重新生成新键，缓存淘汰后的重试退化为新意图）；遗留占位在窗口
  到期后的回收是真实重跑而非重放；异步（`task` 模式）执行在提交时占位（任务系统自身的
  幂等另计）；`replayed` 标记走响应包外层，不落在 `PageExecutionResult` 内

**页面级可选字段 `componentTemplates`**（U11 模板使用快照，位于 PageSpec 根而非
section 内）：`componentTemplates?: Array<{ key: string; digest: string }>`——
创建端点 `POST /api/v1/versioning/pages/composite` 请求体可携带，服务端规范化
（trim / 同 key 去重保留首个 / 空白 key 丢弃）后随 `PageSpec.componentTemplates`
落库，proposal→draft→published 全程 JSON 透传。语义：记录页面创建/编辑时所用
组件模板的内容指纹（digest 算法与模板 API 透出见
组件模板 API），编辑器打开页面时与模板库当前
digest 比对提示「所用模板有新版本」。**不参与发布校验、不参与渲染**——实例化是
复制语义，页面内容始终是保存时的副本（提醒不自动同步）。

**创建端点响应的发布分级字段（T10）**：响应在 `proposalKey`/`pageKey`/
`pageType`/`quality` 之外新增 `published: boolean` 与可选 `publishError?: string`——

- `pages.publishReview=required`（缺省策略）：`published: false`，无 `publishError`
  字段，行为与历史一致（保存只建提案，ProposalInbox 人工接受并发布）；
- `pages.publishReview=auto`：保存成功后直接发布，`published: true`；发布被质量
  门槛（error 级诊断/发布校验）拒绝或失败时 `published: false` 且 `publishError`
  带回原因——**保存本身已成功、提案保留**，前端按提示降级人工链重试；
- 发布走真实链路（快照/版本历史照常），发布权限为 `pages:publish`；策略只
  声明免审核，不豁免发布权限。详见
  [Dashboard Resource/Page 模型](./dashboard-page-model.md) 的「发布分级」小节。

## 数据引用和 mapping（Selector AST）

输入输出 mapping 必须是可校验的 AST（对应 `spec/selector_ast.go`）：

```ts
type TransformSpec =
  | { type: "pick" }
  | { type: "rename"; params: Record<string, string> }
  | { type: "default"; params: { value: JSONValue } };

type ValueSource =
  | { kind: "form"; path: JsonPointer; transform?: TransformSpec }
  | { kind: "row"; path: JsonPointer; transform?: TransformSpec }
  | { kind: "selection"; path: JsonPointer; transform?: TransformSpec }
  | { kind: "detail"; path: JsonPointer; transform?: TransformSpec }
  | {
      kind: "page_state";
      key: string;
      path?: JsonPointer;
      transform?: TransformSpec;
    }
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

`transform` 是白名单受控变换，新增变换必须扩展 spec 包并同步校验器。当前三项：

- **`pick`**（仅 selection）：从每个选中行提取 path 指定字段，输出 identity 数组。
- **`rename`**（row / selection / page_state，且 path 必须为空——变换作用于整对象）：
  `params` 是「源字段名 → 目标字段名」映射表（值必须是字符串）。输出对象**只包含
  映射表命中的字段**（受控白名单，与 pick 同理）；selection 源逐元素应用。上游字段
  名与函数参数名对不齐时用它改名，映射表未覆盖的字段被丢弃。非对象（且非数组的
  selection）源在执行期报 422。
- **`default`**（任意 kind）：`params: { value: <JSON 字面量> }`。源值缺失（上游区块
  未产出该字段）或为 `null` 时兜底为 `value`；有值时不覆盖。用于参数映射的「缺省值」。

组合页编辑器的参数映射会编译产出这些变换：跨区块字段改名 → `rename` 映射；
「缺省值」输入 → `default`（编辑器输入是字符串，纯数字/布尔/null 保持 JSON 类型）。

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

### binding execute 错误码（执行阻断契约）

`POST /api/v1/console/pages/:pageKey/bindings/:bindingId/execute` 在执行前做两道结构化阻断，均返回 `409 + { error, message, details }`（错误码为前端分支稳定码）：

| error              | 触发条件                                                                                        | details                               |
| ------------------ | ----------------------------------------------------------------------------------------------- | ------------------------------------- |
| `executor_unbound` | binding 指向的契约 `executionState=unbound`（上传物料尚未绑定运行时执行器，检查先于 freshness） | `bindingId`、`functionId`             |
| `binding_stale`    | 发布快照契约与当前契约 digest 漂移（schema/version 变化）                                       | `bindingId`、`functionId`、`statuses` |

前端按 `error` 稳定码分支：`executor_unbound` 渲染「未绑定执行器」空态并引导去 OpenAPI Sources 完成绑定（同名函数注册后 T6 自动翻转 bound，无需改页重发）；`binding_stale` 走 diff/重新发布链路。发布页（Console 运行时）禁止任何 mock 数据兜底——执行失败就是显式失败。

### Selector 一键同步（sync-selectors wire 契约）

schema 漂移后对**草稿**做精准修复，只改受影响的 assignment：

```ts
// POST /api/v1/pages/:pageKey/sync-selectors   （权限 pages:edit）
interface PageSyncSelectorsRequest {
  draftRevision: number; // 必填；乐观锁（409 冲突与 SaveDraft 同语义）
  dryRun: boolean; // true 只出报告不落库
  bindingIds?: string[]; // 省略 = 全部 binding
}

interface PageSyncSelectorsResponse {
  pageKey: string;
  dryRun: boolean;
  applied: boolean;
  draftRevision: number; // dryRun=原值；apply=+1
  syncedBindings: BindingSelectorSyncReport[];
  remainingDiagnostics?: Diagnostic[]; // 同步后发布级校验（不因错误拒绝保存）
}

interface BindingSelectorSyncReport {
  bindingId: string;
  functionId: string;
  changed: boolean;
  executionModeFixed?: boolean; // task/sync 模式按最新契约修正
  input?: SelectorSyncInputEntry[];
  output?: SelectorSyncOutputEntry[];
  manual?: Diagnostic[]; // 不可自动修复项（函数缺失/governance 漂移等）
}

interface SelectorSyncInputEntry {
  target: JsonPointer;
  action:
    | "kept" // 未受影响，原样保留（含 Transform/literal 定制）
    | "renamed" // 消失 target 重映射（newTarget）
    | "removed" // 摘除（无唯一候选且非必需）
    | "added" // required 差集补齐：identity 字段语义命中接 row 源，否则 form 同名映射
    | "type_changed" // 类型漂移，保留待人工核对
    | "manual_required"; // 无法安全自动处理
  newTarget?: JsonPointer;
  sourceKind?: ValueSource["kind"]; // added 时：row（identity 语义命中）| form（同名回落）
  confidence?: "high" | "low"; // high=prev schema 精确命中或 identity 语义命中；low=启发式
  reason: string;
}

interface SelectorSyncOutputEntry {
  stateKey: string;
  source: JsonPointer;
  action: SelectorSyncAction;
  newSource?: JsonPointer;
  newShape?: OutputAssignment["shape"];
  required: boolean; // 必需输出（items/detail/dataset/taskStatus/…矩阵）
  confidence?: "high" | "low";
  reason: string;
}
```

语义要点：dry-run 与 apply 共用同一 planner（`spec/selector_sync.go`），预览与落库不会漂移；apply 不自动 publish，`remainingDiagnostics` 有 error 时发布会继续被阻断，须先处理 `manual_required` 项；必需输出在任何路径上都不被摘成缺失（推导失败保留原 assignment + `manual_required`）。prev schema 的来源与精确性判定（`previousInputSchema`/`previousOutputSchema`、digest 双算法匹配降级策略）见 [Dashboard Resource/Page 模型](./dashboard-page-model.md)。

### 批量 selector 同步（bulk-sync-selectors wire 契约，M4）

契约变更队列的草稿侧批量收口——逐页跑与单页 sync-selectors 同源的 planner/apply：

```ts
// POST /api/v1/pages/bulk-sync-selectors   （权限 pages:edit；body 可为空）
interface PageBulkSyncSelectorsRequest {
  pageKeys?: string[]; // 省略/空白 = 审批收件箱 ContractChanges 全量（去重）
}

interface PageBulkSyncSelectorsResult {
  total: number;
  synced?: string[]; // 成功页（selector 拉齐 + DraftRevision+1）
  skipped?: {
    pageKey: string;
    reason: string; // 恒 "manual_required"
    manual?: Diagnostic[]; // 不可自动修复漂移的诊断透传（governance/version 等）
  }[];
  failed?: { pageKey: string; error: string }[]; // 单页失败不中断其余页面
}
```

语义要点：**严格只写 draft**（`published_page_specs` 不动），上线仍需 `bulk-republish`；revision 由服务端读取当前草稿版本（并发冲突在事务内 409，按页计入 `failed` 继续）；先 dry-run 判定——存在 Manual 诊断（governance/version 等不可由 selector 同步修复的漂移）的页面整体 `skipped` 并透传诊断，不做半吊子同步；不自动 publish。

## 草稿列表过滤与资源聚合（wire 契约）

页面工作台的列表过滤、关键词搜索与分页全部由服务端执行（OPEN-ISSUES #13/#30）：前端不得拉全量自行推导，列表响应永远是过滤后的一个分页窗口。

```ts
// GET /api/v1/pages?resourceKey=&status=&keyword=&page=&pageSize=
//   （权限 pages:read；query 条件可组合）
// - resourceKey：按「页面涉及资源」过滤（resourceKey 列 ∪ binding 函数契约资源，
//   服务端读取时计算；旧单列页面同样命中其 binding 关联资源）
// - status：draft / published / archived（SQL 条件下推）
// - keyword：对 pageKey、标题各语言、涉及资源做不区分大小写的包含匹配
//   （scope 内候选集内存过滤，scope 本身已限定数据面）
// - page/pageSize：服务端分页窗口；缺省 page=1、pageSize=20，pageSize 上限 200，
//   非法值（<1 或超上限）钳制，不报错
// 响应：{
//   items: PageSpecDraftSummary[];  // 当前窗口；条目含 resources 投影（见下）
//   total: number;   // 过滤后的总条数（分页依据）
//   page: number;    // 实际生效页码（钳制后）
//   pageSize: number; // 实际生效页大小（钳制后）
// }

// GET /api/v1/pages/resources   （权限 pages:read）
// scope 内页面涉及资源的服务端聚合——过滤下拉的选项来源，
// 前端禁止从当前列表页自行推导（列表只是过滤后子集）。
interface PageResourceOption {
  resourceKey: string;
  pageCount: number; // 该资源出现在多少个页面（按 #30 关联口径，多资源页参与计数）
}
// 响应：{ items: PageResourceOption[] }，按 resourceKey 升序
```

`PageSpecDraftSummary.resources`（#30）：条目级「页面→涉及资源」投影，服务端读取时由 `resourceKey` 列 ∪ 顶层 bindings 对应函数契约的 `resourceKey` 聚合去重（升序）。多资源页在此展开展示；旧 payload 缺该字段时前端回退 `resourceKey` 单列。该关联**只读时计算，不落库**——无新列/新表，函数契约重算后下一次列表即反映最新关联。

边界（诚实声明）：`resources` 关联依赖 binding 函数契约在库内的 `resourceKey`；契约缺失或未重建索引时，该 binding 不贡献资源（此时退化为 `resourceKey` 列口径）。`/pages/resources` 聚合与列表侧关联同口径（#30 起多资源页参与 `pageCount`）。resource catalog（函数/契约侧资源清单）是另一份聚合，两者不复用缓存。菜单管理页的「已挂载页面」视图固定拉 `pageSize=200` 一页取 scope 全量挂载映射（挂载态是低频小数据面），非分页绕过。

## 导航与多语言

分类、标题、图标与排序是 PageSpec 顶层字段（`category{key,order}`、`title`、`icon`、`order`）。`category` 只保留 `key` 与 `order`，是页面工作台侧的分组元数据；运行控制台导航由菜单系统驱动（`menu_items` 树 + `page_specs.menu_id` 挂载映射，见 [运行控制台动态菜单](./console-dynamic-menu.md)），不消费 `category`，分类多语言名称由菜单系统（`menu_items.labels`）提供。`NavigationSpec` 仅承载返回导航行为：

```ts
interface NavigationSpec {
  title?: LocalizedText;
  breadcrumb?: LocalizedText[];
  showBack?: boolean;
  backPath?: string;
}
```

PageProposal 根据 resource/page key 提供默认值；PageDraft 保存最终值；PublishedPageSpec 是 Console 动态菜单的唯一来源。静态 locale 与字典都不得成为动态页面事实源。`ConditionSpec` 只读取当前表单或 page state（区块级经 `key` 读页面状态 `results[key]`——表单当前值/函数输出/表格选中，同属 page state 投影），禁止通过可见性条件访问 row、详情、外部函数或任意 JSONPath，以保证保存、发布和运行时具有一致语义。

## 组件模板 API（相关 wire 契约）

组件模板（V4 复用层）的 REST 契约不承载 PageSpec 本体，但模板 `tree`
字段持有组合页 PageNode 子树（经组合页编辑器编译为 PageSpec 后发布）。
端点与 DTO 定义见 组件模板 API。

## ABI 与版本

`rendererSchemaVersion`（当前 `page-spec:1`）、`generatorVersion`（当前 `page-generator:1`）和各 snapshot digest 必须单独保存。发布校验必须拒绝未知版本、未知节点、未知字段和非法 mapping。页面运行时不得尝试降级、猜测或转换成其他 layout/schema。

## 安全边界

- PageSpec 不存储 route、target service、scope 覆盖、Secret 或任意 HTTP 参数。
- 未知 JSON 只能存在于 JSON Schema 的标准扩展边界；核心 DTO 使用明确类型和 `JSONValue`，禁止 `any`。
- 表单展示变化不允许静默改变已发布页面；发布快照必须包含 FormPresentationSpec。
