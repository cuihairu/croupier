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
  key: string; // 字段标识：裸顶层属性名（"name"）或 JSON Pointer（"/address/city"），见「字段标识」
  label?: LocalizedText; // 覆盖 schema title
  description?: LocalizedText; // 帮助文本
  placeholder?: LocalizedText;
  role?: "identity" | "status" | "reason" | "expiry" | "amount" | "quantity"; // x-field-role（D5）
  unit?: LocalizedText; // x-field-unit 单位后缀（D5）
  perTarget?: boolean; // x-field-per-target 数量/金额按每个选中目标逐项计（D2 跨目标总量）
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

渲染器生效语义（`SchemaFormRenderer`，2026-09 收口）：`required` 与 `validationRules[].type="required"` 并入运行时 schema `required`（`required: false` 可撤销 schema 原必填；visibleWhen 隐藏字段仍豁免）；`min`/`max` 按字段类型映射（string→`minLength/maxLength`，number/integer→`minimum/maximum`，array→`minItems/maxItems`），`pattern` 直写 JSON Schema `pattern`；`custom` 无法用 JSON Schema 表达，渲染时显式告警并忽略（不静默丢弃）。required 的字符串字段（schema `required` 与 fields 覆盖两来源一致生效）在派生运行时 schema 时若未显式声明 `minLength` 自动补 `minLength: 1`——AJV `required` 只查键存在、空串合法，不补则「必填」拦不住清空后的提交（BUG-038 关联观察：空串出网到游戏侧才报错）；显式 `minLength`（含 `validationRules` 映射产物）不覆盖，visibleWhen 隐藏豁免字段不注入；错误文案对 `minLength limit=1` 特化为「{title} 不能为空」。`defaultValue` 参与表单初值种入：仅补未提供值的字段，显式 `initialValues` 优先，`null`/空串视为已提供不覆盖。提交取值走唯一投影规则（`projectSubmitValues`，2026-11 收口）：所有提交路径——表单原生提交、函数调试页、资源创建/编辑、行操作表单与命令式句柄调用方——都用同一 `projectSubmitValues(values, spec)` 取 payload（渲染器句柄暴露为 `getSubmitValues()`）；隐藏键按当前值计算（`visible: false` 或 `visibleWhen` 不满足）并从 payload 剔除，值本身保留在表单态，可见性切回时恢复用户已填内容。裸 `getValues()` 仍返回含隐藏字段的全部值，仅用于状态镜像/回显——禁止用它拼提交 payload。

### 字段标识（SUI-019）

`FormFieldSpec.key`（以及 `groups.fields` 的每一项）是字段标识，接受两种形态：

- **旧顶层裸 key**——schema 根上的属性名（`"name"`）。Go 生成器（`buildFormFields`）与历史前端 `derivePresentationSpec` 只产出这一形态，行为不变。带点的名字如 `address.city` **不是**路径，会按字面属性名查找，通常定位不到。
- **JSON Pointer（RFC 6901）**——对象子字段写作 `/address/city`，数组项字段写作 `/tags/-/label`（`-` 指向 `items`）。`~1` 转义 `/`、`~0` 转义 `~`，因此键名含点或斜杠时也能无歧义定位。

指针标识的渲染语义（`SchemaFormRenderer`，2026-11 收口）：呈现提示（label、description、enumNames、disabled、placeholder、widget、required、validationRules、defaultValue、可见性）作用在指针解析出的 schema 节点上——`label`/`description`/`enumNames` 写入解析到的 `properties` 节点，`required` 并入该节点的宿主 schema，`uiSchema` 分支沿指针路径逐层创建（数组段用 rjsf 的 `-` 约定）。隐藏的指针字段在其嵌套分支注入 `ui:widget: "hidden"` 并从宿主 schema 摘除 required；提交投影按路径剔除（`deleteAtPath`），初值种入按路径写入嵌套 `defaultValue`（`setAtPath`）。边界：指针标识不参与 `ui:order`（rjsf 只接受根属性名）；数组项路径的值删除/写入是空操作，因为数组按整体提交与投影；前端 `derivePresentationSpec` 现在为携带 hints 的嵌套字段产出指针标识（无 hints 时嵌套结构仍整体渲染，行为不变）；Go 侧嵌套派生对齐见 SUI-025。解析不到的指针以 `console.warn` 报出原因（`presentation field path unknown: <key>：<reason>`）而非静默降级；升级为保存/发布期阻断校验见 SUI-021。

`widget` 是受控枚举，取值为 antd/ProComponents 组件名（PascalCase）：`Input`、`TextArea`、`InputNumber`、`Password`、`Select`、`MultiSelect`、`Radio`、`Checkbox`、`Switch`、`DatePicker`、`TimePicker`、`DateRange`、`Upload`、`ImageUpload`、`FileUpload`、`RichText`、`Code`、`Cascader`、`TreeSelect`、`Color`、`Slider`、`Rate`、`JSON`、`KeyValue`、`Array`、`Object`。扩展 widget 需要修改 spec 包并同步前端类型，不允许前端私加。

### Widget 能力表（SUI-020）

渲染器 `SchemaFormRenderer.widgetToRjsf` 把每个 `FormFieldSpec.widget` 取值映射到 RJSF widget/field。共享表 `web/src/utils/widgetCapabilities.ts`（`WIDGET_CAPABILITIES` + `widgetCapability(widget)`）为全部 26 个枚举值记录事实来源：可接受的 JSON Schema 类型、提交值形态，以及类型不匹配或未实现时的回退行为。它只是类型化数据，不改任何渲染。诚实边界（由 `web/src/utils/__tests__/widgetCapabilities.test.ts` 覆盖）：

- **有实现**（`implemented: true`）：`Input`/`TextArea`/`InputNumber`/`Password`/`Select`/`MultiSelect`/`Radio`/`Checkbox`/`Switch`/`DatePicker`/`TimePicker`/`Color`/`Slider` 走映射到的 RJSF 控件；`Switch` 映射到 RJSF `checkbox` 仍提交布尔值；`Code` 与 `JSON` 映射到 `textarea` 只提交普通字符串（不解析、不高亮）；`Cascader`/`TreeSelect`/`Rate` 是注册的自定义控件；`Upload`/`ImageUpload`/`FileUpload` 共用自定义 `upload` 控件；`KeyValue` 经自定义 `keyValue` field 接入；`Array`/`Object` 是 renderer 管理的复合字段，忽略 `ui:widget`。
- **仅声明**（`implemented: false`）：`DateRange` 与 `RichText` 没有任何实现，也没有注册自定义控件；`widgetToRjsf` 对两者返回 `undefined`，界面回退到类型默认控件（按 schema 类型走普通输入或 textarea），声明意图被静默忽略。
- Page Studio 的组件选择 12 个选项是已声明能力的子集，现在每个选项带原生 `title` 提示（可接受类型 + 输出形态 + 回退行为）；选项值不变。

扩展 widget 仍需修改 spec 包并同步前端类型；同一变更必须更新能力表，且表内断言覆盖全部枚举，新枚举值没有文档化能力条目就无法通过测试。

### 表单呈现配置校验（SUI-021）

保存与发布都会校验 PageSpec 携带的每一个 `FormPresentationSpec`：`resource.createForm`、`resource.updateForm`、`resource.actions[].form`、`operation.form`、`task.form`、`report.queryForm` 以及每个 `composite.sections[].form`。规则：

- **字段标识**：非空、不重复，且能在该表单的 `jsonSchema` 中解析（裸 key 与 JSON Pointer 均遵循 SUI-019 契约）；诊断定位到 `…form.fields[i].key`。
- **分组**：分组 key 非空且不重复；分组字段必须在 `form.fields` 中声明且能在 schema 中解析；同一字段不能被两个分组认领（`…groups[i].fields[j]`，冲突码 `form_group_field_conflict`）。
- **控件**：取值必须是 26 个受控枚举之一，且接受类型（SUI-020 能力表）与解析出的字段 schema 类型匹配；仅声明未实现的 `DateRange`/`RichText` 不因缺少实现而被拒。schema 类型缺省/无法识别时跳过兼容性检查。
- **宽度**：12 栏网格下必须为 `0`（未设置）或 `1–12`。

校验只有一份规则源：`ValidatePublishablePageShape` 追加 `spec.ValidateFormPresentations`（覆盖发布、accept-and-publish、回滚预检与提案校验），`ValidatePageSpecInScope` 在保存/预览路径追加同一函数；前端 `web/src/utils/formValidation.ts` 复用 `web/src/utils/fieldPath.ts` 与 `web/src/utils/widgetCapabilities.ts`，在 `savePageDraft` 与 `CompositeEditor.save()` 前拦截非法配置，使其根本不到服务端。错误沿用既有 `{field, message}` details 契约。回归用例包含一份完全合法、仍可发布的 PageSpec。

### 日期时间值序列化（SUI-022）

日期类表单字段提交的是日历值，不是折算到 UTC 的瞬时值。antd 主题内置 `DateWidget` 用 `toISOString()` 序列化 `date-time`——picker 所在的本地偏移被丢弃，换成 UTC `Z` 结尾；清空时透传 `null`，而 null 会滞留在 rjsf v6 的 `formData`（只有 `undefined` 才移除键），随后撞上 AJV `type: string`。`@rjsf/core` 的 `TimeWidget`（antd 主题未提供 time widget）用 `${value}:00` 字符串拼接产值，对 antd TimePicker 的 Dayjs 值会产出非法串。因此渲染器在 `web/src/components/SchemaFormRenderer/widgets.tsx` 按 rjsf widgetMap 的注册名覆盖三个实现：`DateWidget` / `DateTimeWidget` / `TimeWidget`；一次注册同时覆盖两条路径——`format` 缺省路径（`format: date` / `date-time` / `time` 解析到同名 registry key）与显式 `x-widget` 路径（`DatePicker` → `date`、`TimePicker` → `time`，经 `widgetToRjsf`）。

序列化契约（`web/src/utils/datetime.ts`）：

- `date` → 本地日历日期 `YYYY-MM-DD`（无时间、无偏移）；
- `date-time` → 带显式本地偏移的 RFC3339 `YYYY-MM-DDTHH:mm:ssZ`（dayjs 的 `Z` token 输出 `±HH:MM`，如 `2026-01-31T08:30:45+08:00`）；与旧 `toISOString()` 形态的差异是保留操作者的墙钟与偏移，瞬时值本身不变、可无损换算（按 UTC 设计的消费方解析后是同一时刻），精度到秒；
- `time` → `HH:mm:ss`；
- 清空提交 `undefined`，键从提交值移除——SchemaFormRenderer 归一层在折叠 `formData` 前过滤 `undefined` 条目（旧行为是滞留 `null` 折回 payload）。

显示种子（`initialValues` / `defaultValue` / 上次提交值）进入 picker 前在同一模块归一：不带时区指示的 naive 串（DB 风格 `2026-01-01T08:00:00`）按本地墙钟解析，绝不被静默平移；携带 `Z`/偏移的种子保持瞬时值（dayjs 默认语义）——非 UTC 浏览器时区下显示的日历日可能是相邻一天，这是文档化行为（保持瞬时，而非原日历日），由按进程时区守卫的用例断言。`normalizeDateTimeSeed` 是未来改动种子解释的唯一挂钩点。time 种子只接受 `HH:mm`/`HH:mm:ss`；越界值显示为空，不静默进位。

边界：序列化契约跟随 widget，而非独立选项——`format: date-time` 字段配 `x-widget: DatePicker` 会提交纯日期形态（`TimePicker` 反之）；按 UTC 设计的消费方解析后得到同一瞬时；非 UTC 时区下带 `Z`/偏移种子的跨日显示是有意行为；`DateRange` 仍仅声明（SUI-020）。验证：真实 rjsf 引擎 e2e（`datetime-widgets.test.tsx`）在 UTC 与非 UTC 进程时区下经实际 RJSF 提交路径断言提交串与清空行为。

### 枚举标签按值关联（SUI-023）

枚举下拉标签按值关联，绝不按位置关联。GUX-017 的渲染链路不变（schema `x-enum-options` / `FormFieldSpec.enumOptions` → `ui:options.enumNames`，因 rjsf v5 的 `optionsList` 只消费 `ui:options.enumNames`），但标签表现在按 `String(value)` 键与 schema 的 `enum` 数组匹配构建，不再依赖声明顺序：

- **标量值同权**：hint/option 的 `value` 接受字符串、数值与布尔（旧实现静默丢弃非字符串 hint 值，数值枚举只能回退裸值）；`false` 正常关联——它是值，不是空态。
- **重排选项**：`x-enum-options` 或 `enumOptions` 条目可按任意顺序声明；标签跟随 schema `enum` 顺序，因为关联以值为键。
- **缺标签/缺翻译**：没有匹配条目的枚举值渲染原始值字符串（绝不伪造标签）；value 不在 `enum` 内的 hint 被忽略，不会使标签表错位；输出的 `enumNames` 数组与 `enum` 同长同序（rjsf 按下标取用，长度对齐是正确性前提）。只声明 `zh-CN` 的标签照常渲染（GUX-017 的 zh 优先取值链不变；全表单文案跟随 locale 见 SUI-024）。
- **提交值不受影响**：标签只存在于 `ui:options.enumNames`/schema 级 `enumNames`；`schema.enum` 绝不重写。真实 rjsf 提交路径 e2e（`enum-labels.test.tsx`）按可见标签点选选项，断言 payload 携带原始枚举值（数值 `2`，而非标签文本）。

无 schema `enum` 时 `fields` 路径按声明顺序兜底（旧契约，行为不变）。

### 表单文案跟随当前语言（SUI-024）

表单呈现文案——字段名称、说明、占位、分组标题、枚举下拉标签、提交按钮文字——全部跟随平台当前 locale 渲染。渲染器呈现路径中原先硬编码的 `zh-CN` locale 实参替换为单一 locale 来源（`web/src/utils/uiLocale.ts` 的 `currentUiLocale()`）：umi 运行时内返回当前平台 locale，非 umi 上下文（单测替身/异常）回退 `zh-CN`。统一回退链不变（`localizedText`：当前 locale → zh/en 互备 → 遗留短 key → 任一非空 → 显式 fallback 实参，如缺标签时的字段 key、缺枚举标签时的原始值）。

不刷新页面的语言切换（umi `setLocale(lang, false)`）经 window `languagechange` 事件广播；SchemaFormRenderer 订阅该事件并重算派生 schema/uiSchema，可见文案实时换语言。切换语言**不会重置已填写数据**：渲染器的表单态就是实时值镜像，重算只改呈现元数据；initialValues 回种逻辑以 initialValues JSON 为键（与 locale 无关）。默认的刷新式切换整页重载，不经过本机制。

边界：只声明 `zh-CN` 的文案在 en 界面渲染 zh 文本（互备回退，不伪造翻译）；生成器直接写进 `jsonSchema` 的静态 `title`/`description` 原样输出——跟随 locale 的是叠加其上的呈现层（`FormPresentationSpec` 的 label、placeholder、分组标题、枚举 hints）；自带文案的页面模板经同一回退链在各自渲染处跟随平台 locale。

### 默认表单派生对齐（SUI-025）

函数契约不带表单呈现配置时，表单不会丢——它是派生出来的，而派生现在是两侧同一份契约。Go 生成器（`internal/dashboard/generator` 的 `deriveFormPresentation`）与 Web 调试页（`web/src/utils/schemaHints.ts` 的 `derivePresentationSpec`）共用一份预期结果 fixture（`testdata/form_presentation_parity.json`），两侧测试读同一文件、逐 case 断言输出完全一致，覆盖顶层字段顺序、控件、分组与 layout hints。

派生规则（两侧一致）：

- **字段顺序**：顶层 properties 按 JSON 声明序产出，再按声明的 `x-order` 稳定排序（任意有限数值；未声明排最后，并列回声明序）。嵌套 object/数组项仅在携带 hints 时产出，以 JSON Pointer 定位（`/address/city`、`/tags/-/name`，`~0`/`~1` 转义，深度封顶）。携带任一 hint 的 schema 产出全部顶层字段加带 hints 的嵌套子字段；完全无 hints 的 schema 不产出任何字段。
- **控件**：合法的 `x-widget` 覆盖；否则类型缺省表补位，且不改变「是否有 hints」的判定——`integer`/`number` → `InputNumber`（integer 带 `enum` 仍派生 `InputNumber`，要下拉须显式 `x-widget: Select`）、`boolean` → `Switch`、`format: date`/`date-time` → `DatePicker`、`format: time` → `TimePicker`、`maxLength > 120` → `TextArea`、数组 `items.enum` → `MultiSelect`、`enum` → `Select`。
- **分组**：`x-ui-groups` 声明（本地化标题、`collapsible`/`collapsed`），字段用 `x-group` 归组；`x-group` 指向未声明组时自动追加一个组（标题为人性化 key）；落空（无字段）的组丢弃。
- **布局**：`x-ui-layout` 校验为 `vertical`（默认）/`horizontal`/`inline`/`grid`；非法值回退 `vertical`。
- **无配置 = 标准纵向表单**：完全无 hints 的 schema 不产出 `fields`/`groups`，wire 上只有 `layout: vertical`。渲染端补位（`SchemaFormRenderer` → `deriveDefaultPresentationSpec`）：无 fields 的 spec 合并派生的标准字段（含类型缺省控件），`layout`/`groups` 仅在 spec 未声明时回填。调试预览与发布渲染一致，因为两者走同一个兜底。渲染器把派生 `DatePicker` 按 schema `format: date-time` 解析到 `date-time` 注册控件，保持 SUI-022 序列化。

派生不再下发渲染端已有的显示兜底：人性化标签（`x-label` > `schema.title` > 人性化 key 已覆盖显示）与 `请输入X` 占位从派生输出移除——占位现在必须显式 `x-placeholder`。

边界：数字相邻的 key 两侧人性化结果不同（Go `Auto 1` vs Web `Auto1`）——只影响自动分组标题；声明的 `x-order: 0` 两侧都排最前，但 Go wire 省略 `order: 0`（`omitempty` int）；非整数 `x-order` 只存在于 Web 编辑器态（Go 存整数）；携带显式 `fields` 的存量发布 spec 不受影响——兜底只作用于无 fields 的 spec。验证：共享 fixture 驱动 `TestFormPresentationParityFixture`（Go）与 `form-presentation-parity.test.ts`（Web）；渲染端兜底由真实 rjsf e2e（`form-default-fallback.test.tsx`）覆盖。

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

### 非对象根请求的 JSON 编辑（SUI-026）

结构化表单控件只能表达对象根。函数契约声明非对象根类型（`array`、裸标量或 `null`——SUI-011 请求兼容性分级的 `degraded` 形态）时，`SchemaFormRenderer` 切换到 JSON 编辑形态（`JsonRootEditor`）而非无法渲染：种子来自 SUI-017/018 默认值层（根数组 `[]`/items 骨架、标量类型占位）或显式初值，整段文本即完整请求 payload，提交与对象表单走同一份 handle 契约（`validate`/`getSubmitValues`/`submit`）——同一 `@rjsf/validator-ajv8` 校验器把 payload 按契约 schema 校验，解析值原样发出（数组/标量不套对象壳，保持 SUI-017 往返语义）。界面同时给出不能渲染结构化表单的具体原因（声明的根类型）与实际可用入口（本编辑器）。非对象根形态下普通对象初值被忽略（调用方常用 `|| {}` 兜底，那里永远不可能是合法种子），改用 schema 派生种子。

组合页区块同样适用：`form.jsonSchema` 声明非对象根的区块计入有表单（`sectionHasForm`），渲染 JSON 编辑器而非裸执行按钮；提交路径把数组/标量原样送入执行链，page_state 合并只记 `{ values: <payload> }`（数组无法对象展开成扁平键）。

边界：JSON 编辑器只覆盖已声明的非对象根——`degraded` 的对象根（条件关键字、元组、联合）仍走结构化表单，变体渲染属于 SUI-042/043；JSON 形态下 `externalErrors` 字段级注入不适用（整体校验一次失败）；manual 层级（缺少请求映射）仍产出 `needs_review` 提案——具体原因作为按 `functionId` 关联的 `openapi_schema_compatibility*` 诊断展示在 OpenAPI Sources operations 表（`requestCompatibility` 列，层级 Tag + 至多三条原因行）与 Functions 详情诊断中。

### 每实例字段 ID（SUI-027）

渲染器按 rjsf `idPrefix`（`{前缀}_{路径}`，默认 `root`）生成每个字段控件的 id、label `htmlFor` 与错误/说明 `aria` 关联。同页挂载两个同 schema 表单（一个契约两个区块的组合页或双预览）时，若 id 前缀不变，两实例产出相同 id、label/错误/说明跨实例串扰。`SchemaFormRenderer` 按存活实例分配前缀：首个实例保持默认 `root`（既有 `#root_*` 页面选择器与用例不受影响），并发实例依次领取 `root-2`/`root-3`…；实例卸载即归还，串行重渲染重新领回 `root` 而不漂移。前缀在实例生命周期内领取一次（`useRef`），同一实例重渲染时稳定，只有重新挂载才可能换号。

### 表单宽度与响应式布局（SUI-028）

`FormFieldSpec.width` 采用 12 栏协议栅格（合法范围 1–12）；渲染器把显式声明的宽度乘 2 换算为 Ant Design 24 栏 `Col span`（`width: 6` → `ant-col-12`）。换算只作用于该协议字段：`formContext.colSpan`（`layout: grid` 下为 12）本身已是 antd 栏数、保持原样；类型派生缺省同样不变（单字段、`object`/`array`/长文本或字段数不足两个时整行，否则半行）。历史配置里的 `0`（未设置）与越界值（`>12`、负数）一律忽略并按上述优先级回退——不迁移、不静默缩放旧配置；这类配置在管理员于 Page Studio 编辑前按标准布局渲染。

每个模板列同时携带响应式跨度：`xs=24`（768px 以下所有字段独占整行）、`md=max(12, ceil(span/2))`（768px 起至少半行）、`lg=span`（992px 起恢复完整声明宽度）。整行跨度（24）在所有断点保持 24，避免溢出。分组与长文本字段在两种视口下都可读可操作；字段输入、校验与提交绑定链不变。浏览器用例在 mock 操作页以桌面与 375px 宽度分别覆盖。

### 在 Page Studio 编辑表单分组（SUI-029）

Page Studio 的 `FormPresentationEditor` 对所有含表单的页面（操作页、任务页、报表、资源页与组合页区块表单）暴露完整 `FormGroupSpec` 列表：可创建分组、改名（key 与多语言标题）、拖拽排序、删除；每组字段归属是对已声明 `form.fields` 的多选（按选择顺序），已被其他分组认领的字段置灰以防 `form_group_field_conflict`；可折叠/默认折叠开关设置两个可选行为标记。编辑结果作为同一草稿的 `FormPresentationSpec.groups` 写回，草稿重开原样（含顺序）回读——不存在编辑器专属的旁路状态。保存/发布仍走 SUI-021 校验（key 唯一、字段已知、字段只属一组），非法编辑以定位诊断暴露，不会静默进入已发布页面。边界：编辑器只改呈现（schema、binding 与 payload 不变）；分组字段选择限于 `form.fields` 已声明的字段——嵌套字段需先有声明指针 key 的字段项。

### 展示与调用参数的边界（SUI-045）

操作者看到的表单与最终发出的请求是两个不同对象，中间由三层正交职责连接：

| 层                     | 内容                                                              | 职责                                                                                                 |
| ---------------------- | ----------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| 基础表单（jsonSchema） | `FormPresentationSpec.jsonSchema`（由函数契约 input schema 派生） | 调用契约的真值：字段集合、类型、`required`、约束                                                     |
| 展示覆盖               | `fields[]` 逐字段覆盖 + `layout`/`groups`/按钮                    | 操作者输入面：如何呈现、哪些字段可见/禁用                                                            |
| Binding                | `PageBinding.selectors.input.assignments`（Selector AST）         | 调用取值映射：把 form/row/selection/detail/page_state/literal 来源映射到 payload target（`$.` 指针） |

展示覆盖不修改 jsonSchema，Binding 不修改展示。`SchemaFormRenderer` 消费前两者生成输入面；服务端 `buildBindingPayloadFromSelectors`（`internal/api/console/console_execute.go:523`）消费 Binding 与执行上下文生成请求 payload——payload **只**由 selector assignments 构造，未声明 assignment 的字段不进 payload。

**FormFieldSpec 字段两分**：

- **纯展示（不影响调用）**：`label`/`placeholder`/`description`/`role`/`unit`/`perTarget`/`widget`/`width`/`order`/`enumOptions`/`remoteOptions`/`widgetProps`，以及页面级 `layout`/`groups`/`submitButton`/`cancelButton`。`layout` 特例说明边界：`deriveRuntimeSchema` 只把它映射为 `formContext` 布局配置（`web/src/components/SchemaFormRenderer/index.tsx:480-500`），不触碰 schema、校验与 payload——**同一 schema、同一 Binding 下标准与紧凑布局生成相同请求**（SUI-031/036 的设计前提）。
- **调用相关（影响取值或校验）**：
  - `defaultValue` —— 表单初值种入：仅补 `undefined` 的字段，绑定注入/`initialValues` 优先，`null`/空串视为已提供不覆盖（`applySpecDefaults`，`index.tsx:412-424`）；
  - `visible`/`visibleWhen`/`disabled` —— 可见性（见下）；
  - `required`/`validationRules` —— 并入运行时 schema：`required: true` 加入、`required: false` 撤销 schema 原必填、`min`/`max` 按类型映射 ajv 关键字、`pattern` 直写、`custom` 显式告警忽略（`mergeFieldValidity`，`index.tsx:352-408`）。

**隐藏字段的三条出口**（SUI-015 的边界）：

1. **值保留**：隐藏（`visibleWhen` 不满足或 `visible: false`，统一谓词 `isFieldHidden`）字段以 `ui:widget: "hidden"` 保留在 schema 中，值保留在表单状态；提交 payload 经唯一投影 `projectSubmitValues`（SUI-014 统一提交取值，`index.tsx:445-450`）生成——隐藏键按当前值剔除、其余键原样保留，原生提交与命令式 `getSubmitValues()`（`index.tsx:661-665`）共用同一条规则；`getValues()` 返回含隐藏字段的全部值，仅用于状态镜像/回显（`index.tsx:680-688`）。
2. **绑定可达**：隐藏字段的值可经绑定 `{ kind: "form", path }` 进入 payload——服务端从执行上下文 `context.form` 按 path 取值（`resolveSourceValue`，`console_execute.go:593-594`）；浏览器端 `projectBindingContext` 把上下文裁剪到发布 selector 实际引用的字段（`web/src/components/PageRenderer/runtime.ts:78-120`），隐藏字段若被 selector 引用仍随裁剪结果送达。**隐藏且由绑定提供的合法值可执行。**
3. **服务端兜底**：隐藏必填的豁免只发生在**客户端**运行时 schema（从 `required` 摘除，`index.tsx:538-540`）；服务端在绑定应用之后校验最终 payload——`validateBindingExecutePayload` 对不匹配 input schema 的 payload 返回结构化 422（`console_execute.go:512-544`）。**隐藏且无任何来源（无 assignment、无 literal/default transform 兜底）的必填项因此在服务端得到字段级结构化错误**，不会被静默丢弃后发出错请求。自 SUI-016 起判定引擎与未知字段策略与函数直接调用入口共享：缺少 schema 放行、无法编译的 schema（或非 JSON payload）按缺陷拒绝而非当作无契约，未知字段遵循标准 JSON Schema 语义（未声明 `additionalProperties: false` 即放行）——同一组对照 fixture 锁定两个入口。

**最终校验顺序**（已按此序实现；SUI-014 统一提交取值后维持）：

- 客户端：绑定注入（弹窗参数/行注入/级联，作为 `initialValues`）→ `applySpecDefaults` 种入 → `deriveRuntimeSchema`（visibleWhen 豁免 required → `mergeFieldValidity` 覆盖合并 → required 字符串补 `minLength: 1`）→ AJV 提交校验 → 唯一投影 `projectSubmitValues` 剔除隐藏键 → `onFinish`。注入是**覆盖**而非补空：注入 JSON 变化时整表重置为种入值（`index.tsx:633-639`）。
- 服务端（`executeBindingFromSnapshot`，`console_execute.go:186-202`）：executor bound → binding fresh → governance → **先** `buildBindingPayloadFromSelectors` 应用全部绑定与 default transform → **后** `validateBindingExecutePayload` 校验最终请求 → 幂等占位 → 审批闸 → 下发。即「绑定及明确默认值应用完再校验最终请求」。

**旧页面兼容**：`required`（含 `required: false` 撤销）、`defaultValue`、`visible`/`visibleWhen`、`validationRules` 四类覆盖继续按现语义生效（`web/src/components/SchemaFormRenderer/__tests__/field-validity.test.tsx` 锁定），本节不新增 wire 字段、纯语义澄清。分界同时约束展示变体协议（SUI-031）：变体只允许覆盖纯展示项；调用相关项属于调用契约的一部分，任何变体不得改动。

**示例**（同一函数 `player.ban`，schema 含 `playerId *`、`env *`、`reason *`、`notifyDays`）：

- **标准 vs 紧凑布局**：变体仅在 `layout`（`vertical` vs 紧凑栅格）与 `width` 上不同，schema、fields 调用相关项、Binding 完全一致 → 两种布局下同一输入生成**相同**请求 `{ "playerId": ..., "env": ..., "reason": ..., "notifyDays": ... }`。
- **手填 vs 选中行注入**：同一 target `$/playerId` 两种来源——手填 `{ kind: "form", path: "/playerId" }`（表单字段，用户输入）；行注入 `{ kind: "row", path: "/id" }`（行操作弹窗把选中行 `id` 注入表单初值，发布运行时 `mapRowParams` → `openDialog`，`web/src/components/PageRenderer/compositeRuntime.tsx:1063/865`；编辑器预览对齐为 `openDialogPrefill`，`PreviewRuntime.tsx:104`）。两者取值路径一致：注入值作为 `initialValues` 种入、用户随后修改以表单值为准。
- **缺少必填来源**：`env` 为 schema 必填，展示覆盖 `visible: false` 隐藏，且无任何 assignment 提供它。客户端豁免并剔除 → payload 缺 `env` → 服务端 422：

```json
{
  "error": "validation_failed",
  "message": "binding payload does not match input schema",
  "details": {
    "bindingId": "ban-1",
    "functionId": "player.ban",
    "schema": "inputSchema",
    "error": "payload does not match schema: jsonschema validation failed with 'file:///...schema.json#' - at '': missing property 'env'"
  }
}
```

### 命名展示变体（SUI-031）

一个表单可以携带多个命名的呈现形态——典型的一对是标准纵向表单与紧凑栅格——而不分叉调用契约。本节定义协议：存储（SUI-032——变体随表单节点经既有草稿/快照 `SpecJSON` blob 落库，零表结构变更）、发布前校验（SUI-033——变体形状规则与当前契约新鲜度，spec 级与服务级用例同证，含回滚预检）、Page Studio 管理（SUI-034——操作/任务/报表/资源页表单页签管理命名风格：新增/改名/删除、设默认并在改名/删除时同步、逐字段列宽/控件覆盖、逐风格预览；按钮/分组/label 覆盖与变体独有字段仍走 API 编写——见使用文档）与默认解析（SUI-035——渲染端生效 spec 解析应用发布的 `defaultVariant`，经共享合并规则回退 基础→派生纵向；旧页面与解析不到的 key 原样渲染基础）与已发布表单切换（SUI-036——声明了多于一个风格的已发布表单渲染会话内切换器；显式选择在解析链头部替换发布默认项，切换后输入值与校验错误保留，且仅在会话内有效）均已落地，以下是它们共同实现的契约。

**稳定表单标识。** PageSpec 中每个携带表单的位置都由稳定段 key 组成的规范路径寻址，绝不使用数组下标：`resource.createForm`、`resource.updateForm`、`resource.actions.<actionKey>.form`（ActionSpec 的 `key`）、`operation.form`、`task.form`、`report.queryForm`、`composite.sections.<sectionKey>.form`（CompositeSection 的 `key`）。区块与动作重排不改变标识。数组下标路径（如 `composite.sections[2].form.fields[0].width`）仍是校验错误的定位惯例，不是标识。变体存放在它所覆盖的表单节点内部，因此变体继承表单标识，在草稿、快照与回滚之间永不悬空。

**wire 形态（additive；`page-spec:1` 不变）。** `FormPresentationSpec` 新增两个可选字段：

```ts
interface FormPresentationSpec {
  // …既有字段（jsonSchema/layout/groups/fields/submitButton/cancelButton）…
  variants?: FormVariantSpec[]; // 命名展示变体（SUI-031）；数组顺序 = 管理列表顺序
  defaultVariant?: string;      // 生效默认变体 key；未声明 = 基础呈现（原表单）生效
}

interface FormVariantSpec {
  key: string;            // 变体 key：本表单内唯一
  title?: LocalizedText;  // 操作者可见名称（风格列表 / 切换 UI）
  layout?: "vertical" | "horizontal" | "inline" | "grid";
  groups?: FormGroupSpec[];
  fields?: FormVariantFieldSpec[]; // 逐字段纯展示覆盖（键同 SUI-019）
  submitButton?: FormButtonSpec;
  cancelButton?: FormButtonSpec;
}

interface FormVariantFieldSpec {
  key: string;
  label?: LocalizedText;
  description?: LocalizedText;
  placeholder?: LocalizedText;
  role?: "identity" | "status" | "reason" | "expiry" | "amount" | "quantity";
  unit?: LocalizedText;
  perTarget?: boolean;
  widget?: FormWidget;
  width?: number; // 1-12，同 SUI-028
  order?: number;
  enumOptions?: EnumOption[];
  remoteOptions?: RemoteOptionsSpec;
  widgetProps?: Record<string, JSONValue>;
}
```

变体字段类型在结构上不含 `required`/`defaultValue`/`visible`/`visibleWhen`/`disabled`/`validationRules`——调用相关覆盖无法在变体中表达（见下）。先于这些字段的渲染器忽略它们、按基础呈现渲染（additive 可选字段；运行时绝不降级、猜测或转换——ABI 规则不变）。

**变体允许覆盖什么。** 恰好是 SUI-045 的纯展示集合：逐字段 `label`/`description`/`placeholder`/`role`/`unit`/`perTarget`/`widget`/`width`/`order`/`enumOptions`/`remoteOptions`/`widgetProps`；页面级 `layout`/`groups`/`submitButton`/`cancelButton`。调用契约不可被任何变体覆盖：`jsonSchema`、`required`（两个方向）、`validationRules`、`defaultValue`、`visible`/`visibleWhen`、`disabled`，以及表单的 `PageBinding` assignments。发布校验对变体内出现的调用相关键返回定位错误（`form_variant_illegal_override`），变体因此永远无法改变提交内容、校验方式或调用治理。

**覆盖合并语义。** 基础呈现即现有 `FormPresentationSpec`，保持不变、始终生效。变体 V 的生效呈现 = 基础 + V 的覆盖：

- `jsonSchema` 与全部调用相关覆盖永远来自基础。
- `layout`/`groups`/`submitButton`/`cancelButton`：V 声明则整体替换基础值；未声明则继承基础。
- `fields`：按 key 合并。V 的条目覆盖基础同 key 条目的纯展示属性（逐属性取胜；V 未声明的属性保留基础值）。V 独有的 key 合法，只要能在 `jsonSchema` 中解析（SUI-019 规则），其余呈现走 schema 默认。仅基础声明的条目原样透传。
- 不支持「撤销」：变体不能收回基础的展示覆盖（v1 无 null 删除语义）。要对某个变体「取消」基础覆盖，请修正基础——这让合并规则保持可预测。

只声明 `key`（可加 `title`）的变体合法，渲染与基础完全一致——这是给「标准」风格命名、与紧凑风格并列的推荐方式。没有保留变体 key；验收标准中的 "standard"/"compact" 是普通用户自选 key。

**默认项。** `defaultVariant` 声明操作者未选择时生效的变体。声明时必须解析到已声明的变体 key，否则发布失败并定位（`form_variant_default_unknown`，SUI-033）；未声明——包括 `variants` 为空或缺失时——基础呈现即生效默认；无变体的节点上声明 `defaultVariant` 本身也是定位错误。基础呈现因此始终可用且无名：它是零配置地板。

**共用一切调用面。** 同一表单的所有变体共用一份 `jsonSchema`、一套 Binding、一套调用治理（权限、风险、审批、危险确认、幂等——全部挂在绑定/函数层，无一挂在变体层）。同 schema、同 Binding、同输入下，一个表单的每个变体产出完全相同的请求 payload；切换变体按构造就是展示操作（SUI-045 的前提，现在由允许覆盖集合强制）。这也是切换能保留操作者输入与错误状态的原因：表单状态按 schema 路径键控，变体从不触碰路径。

**解析语义（SUI-035 落地，SUI-036 补全链头）。** 渲染表单的生效变体依次为：操作者的会话内显式选择（运行时 UI 态，SUI-036）→ 发布的 `defaultVariant` → 基础呈现 → `deriveDefaultPresentationSpec` 的标准纵向表单（基础不带 `fields` 的表单，SUI-025）。SUI-035 把 `defaultVariant` 以下的链路落在 `SchemaFormRenderer` 的生效 spec 步骤（`web/src/components/SchemaFormRenderer/index.tsx`）：所有消费方——操作/任务/报表/资源页渲染器与组合页渲染器——经 `resolveVariantPresentation`（与编辑器预览同一合并规则）解析发布默认项；运行时解析失败的 key（陈旧缓存）沿同一链回退，无变体配置的页面走恒等路径。编辑器逐风格预览传入显式选择的风格并剥离 `variants`/`defaultVariant`，渲染器解析链不会在已合并呈现上二次解析。发布校验使解析不到的默认项在实践中不可达；渲染端仍做优雅降级（防御边界）。SUI-036 补全链头：`SchemaFormRenderer` 在已发布表单声明多于一个变体时渲染会话内风格切换器——操作者的显式选择在同一调用侧替换 `defaultVariant` 进入同一解析。显式选择只存于组件态：组件重挂载即复位、不持久化，每次新加载仍跟随发布默认项。切换不重挂崩溃边界（以基础 spec 为键）也不触碰 schema，因此输入值与校验错误跨风格保留，两种风格提交相同 payload；确认条目按构造不随风格变化。

**发布快照与回滚。** 变体与 `defaultVariant` 位于表单节点内部，因此随同一份发布快照落库——一份 digest、沿用现有表、无旁表（SUI-032）。回滚原子恢复上一快照：其变体与默认项按发布时原样回来，绝不做跨版本合并。回滚预检已在跑 `ValidatePublishablePageShape`（SUI-021）；SUI-033 把校验扩展到变体并对照**当前**函数契约，因此回滚一个变体引用了已删除/已改名契约字段的快照会在预检失败并定位到受影响变体——陈旧快照被暴露，而不是被静默发布。

**契约新鲜度（SUI-033）。** 上述形状校验把变体 key 对照表单自身 `jsonSchema` 解析——其新鲜度只与草稿相同。发布路径用两道检查补齐与**当前**函数契约的差距（`ValidateVariantContractFreshness`，凡 `ValidatePageSpecInScope` 以 `publish=true` 运行处均生效，含版本化回滚预检）：契约字段删除或改名后，草稿冻结的 schema 仍能解析过期变体 key，因此发布时把每个变体字段与分组字段 key 对照承载表单所执行函数的当前 input schema 复解析，失败即 `form_variant_contract_stale`，点名受影响变体 key 与函数。载体→函数映射沿页面模型的结构性引用：resource 动作表单经 `actions[i].bindingId`，composite 表单区块经 `sections[i].bindingId`（static 区块跳过），`operation.form`/`task.form`/`report.queryForm` 经页面唯一的 `action`/`task`/`report` usage 绑定；映射缺失或歧义时跳过而不猜测。已知边界：`resource.createForm`/`updateForm` 在 spec 上没有结构性函数引用，其变体只对照表单 schema 校验（形状 + 重新生成），不对照当前契约——它们的提交接线在生成器内部。作用域无需新机制：变体在页面快照内部，快照已按 (game, env) + pageKey 作用域隔离——同 pageKey 在不同游戏/环境下变体互相独立。

**旧页面与新页面。** 本协议之前发布的页面不带 `variants`，原呈现逐字渲染——不为它们发明伪变体。无变体配置的新页面同样不产出变体字段；其不带 `fields` 的基础表单即派生的标准纵向表单（SUI-025），也是本协议的零配置默认。操作者所选变体是运行时展示状态；协议只持久化发布默认——按用户/会话持久化选择不在范围内。

**校验规则（SUI-033 已实现）。** `ValidateFormPresentations` 在所有校验路径（保存/预览、发布、accept-and-publish、提案、版本化回滚预检）上按既有 `{field, message}` details 契约执行变体检查：变体 key 非空（`form_variant_key_missing`）且表单内唯一（`form_variant_key_conflict`）；`defaultVariant` 解析如上（`form_variant_default_unknown`）；变体字段 key 非空（`form_variant_field_key_missing`）、不重复（`form_variant_field_duplicate`）且能在表单 `jsonSchema` 中解析（`form_variant_field_unknown`）；变体字段上的调用相关键即使 `encoding/json` 会静默丢弃也能定位——`FormVariantFieldSpec.UnmarshalJSON` 把展示白名单外的每个键审计进 `UnknownKeys`，校验以 `form_variant_illegal_override` 报出并点名违规键；`widget` 合法性/类型兼容与 `width` 范围沿用基础字段的 SUI-020/SUI-021/SUI-028 规则，变体分组规则与基础对齐（`form_group_key_missing`/`form_group_key_duplicate`/`form_group_field_unknown`/`form_group_field_duplicate`/`form_group_field_conflict`，变体分组字段允许声明在基础 fields 或本变体 fields）。Web 端在 `web/src/utils/formValidation.ts` 镜像同一规则（`unknownVariantOverrideKeys` 承担 Go `UnmarshalJSON` 审计的角色），草稿保存前即被拦截。

### 多态输入分支（SUI-042）

OpenAPI 多态请求 schema（`oneOf`/`anyOf` + `discriminator`）在渲染结构化表单前必须选定分支：只有知道 discriminator 值，分支字段与 required 集合才有意义。SUI-042 在 `SchemaFormRenderer` 渲染期解决该问题——PageSpec 无新字段、wire 无变更：表单 `jsonSchema` 仍携带原始多态声明，已发布呈现、Binding 与调用治理不动。

**解析为三态**（`web/src/components/SchemaFormRenderer/branchSelection.ts`）：根无 `discriminator` 返回 `null` 并走既有路径（无 discriminator 的普通 `oneOf`/`anyOf` 也在此列，rjsf 保持原生行为）；可解析声明（分支值来自 `discriminator.mapping` 引用或分支单一 `const`/`enum` 值，支持 `#/` 本地 `$ref` 解析）产出分支列表；`propertyName` 缺失、无变体、分支无法定值（缺声明、悬空引用）、分支值冲突则产出带具体原因的 `unresolved`。

**可解析分支。** `SchemaFormRenderer` 在表单上方渲染分支选择器。所选分支的 subschema 合并进基础 schema（分支 properties 覆盖基础、required 取并集并加入 discriminator 属性，渲染/校验 schema 移除 `oneOf`/`anyOf`/`discriminator`），无配置呈现按合并后 schema 重新派生字段元数据，使分支独有字段享有 SUI-025 标签/控件地板；显式配置的 `fields` 原样保留。切换分支保留共享键值、预置 discriminator 值，但丢弃目标分支未声明的键。payload 只含所选分支允许的键与 discriminator——校验与提交使用合并 schema（单一投影源 `specRef`）；分支专属必填字段未填则拦截提交。`initialValues` 携带已声明 discriminator 值时首屏选中对应分支。选择仅为会话态：换页面/换函数（分支键集变化）复位到首个声明分支，重挂载则依次跟随 `initialValues` 与声明顺序。崩溃边界仍以基础 spec 为键（SUI-036 规则），分支切换不重挂表单。

**不可解析分支。** 渲染器不猜语义：降级到共享 `JsonRootEditor`（与 SUI-026 非对象根同一 handle 契约、同一 AJV 校验器、同一 Binding 执行链），并以说明解析失败原因的提示告知操作者。客户端仍按原始多态 schema 校验完整请求 JSON，提交路径与其它表单一致。

**已知边界。** 本层只覆盖带显式 `discriminator` 的对象根；嵌套 discriminator 与普通组合式 `oneOf`/`anyOf` 保持既有行为。分支标签取分支 `title` 或 discriminator 值（wire 未提供 i18n 映射）。编辑器（Page Studio）无分支编写 UI——分支来自发布函数契约的 schema，呈现配置不得改动 discriminator 或分支选择规则。

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

- `table.columns[].unit` / `columns[].enum` / `columns[].render` / **显式字段语义**（D5）——
  `x-*` 呈现 hints 的输出侧。游戏 SDK 在契约输出 schema 的 items 属性上声明语义（机械解析、
  不靠字段名猜测）：`x-field-role`（`identity|status|reason|expiry|amount|quantity`）与
  `x-field-unit`（裸字符串或 `{zh-CN,en-US}` 对象），配合既有 `x-enum-options`。生成器坍缩为
  wire 字段：`role=identity` → 列 `fixed:"left"` + `render:"copy"`（固定左列可复制身份列）；
  `x-enum-options` → 列 `enum` + `render:"tag"`（状态枚举标签列）；`x-field-unit` → 列 `unit`。
  输入侧进入 `FormFieldSpec.role`/`unit`/`perTarget`（`form_hints.go` 与前端
  `derivePresentationSpec` 同步解析，非法 role 静默忽略；`x-field-per-target` 仅当恰为
  `true` 时声明该字段按目标逐项计，其余值一律视为未声明）。渲染：组合页与资源页渲染器把单位追加到列头（`标题 (单位)`）
  与 `number` 单元格值；`render:"copy"` 渲染可复制单元格；`dataType:"datetime"` 单元格带显式
  浏览器时区标签（`UTC±HH:MM`——平台不声明游戏服务器时区，故显式标注偏移而非暗示 UTC）；
  `enum`+`tag` 渲染选项文案为彩色 antd Tag。确认框（D2 危险确认）从目标表单的
  `FormFieldSpec.role`/`unit` 取语义：身份条目置前加粗、到期条目红字、amount/quantity 值带
  单位后缀，操作者确认前即读到谁/什么/多少/多久。边界：单位仅展示（不做金额缩放或换算——
  声明的单位原样追加，仅 `x-field-per-target` 声明的字段在 D2 危险确认多选目标时另示总量）；
  `identity` 之外的 role 不
  改变列布局（status/reason/expiry 只驱动确认强调与未来缺省呈现）；时区标签为浏览器本地偏移
  （无游戏级 tz 配置）；组合页列语义走同一生成器派生（绑定带 hints 契约的组合表格区块自动
  获得同款列），弹窗内表格保留语义单元格渲染但不加表头排序 UI；枚举文案与单位跟随当前界面
  locale（部署面为 zh-CN/en-US 两套，其余回落 zh-CN），不再固定 zh-CN；声明集合外的枚举值
  渲染显式未知标注（默认色 Tag 或警示文案，保留原值、非仅靠颜色区分），无法解析的时间渲染
  显式无效标注而非漏出 `Invalid Date (UTC±xx)` 拼接（共享格式化函数同样原值透传）
  （GUX-005/006/007）

- `danger` + `chain`（rowActions / toolbar.actions）：**危险操作确认契约**（D2）——`danger`
  动作在用户显式确认前绝不执行。带 `targetSection` 时确认框列出目标弹窗的预填参数与动作链
  步骤，确认后才打开弹窗并执行动作链；不带 `targetSection`（纯动作链按钮）时确认框列出动作
  链步骤。确认内容按目标表单的字段标题渲染参数、枚举值映射为选项文案、参数名人性化展示
  （不显示原始字段 key），所见即实际将提交的内容。非危险动作带 `targetSection` 时直接开弹窗
  并执行动作链（详情弹窗用动作链预载自身 fields/table 数据）；非危险纯链动作直接执行动作链。
  边界：字段语义强调覆盖 `x-field-role` 声明的 identity/reason/expiry/amount/quantity
  （未声明 role 的字段照旧渲染）；`openModal` 链步骤无 danger 标志位（wire 无该
  字段），链内打开弹窗不触发确认；确认发生在弹窗打开之前，弹窗内改动导致提交参数
  变化时提交前按最终值重新确认（参数未变不重复弹确认）；声明 `x-field-per-target` 的
  字段在源表格多选目标 >1 时于危险确认框另示总量条目（`值 × 目标数`，单位与语义
  复用该字段声明）——仅显式声明的字段聚合，未声明字段绝不乘总量（不做语义猜测）
- **执行结果反馈**（D4，无新增 wire 字段）：渲染器按 `PageExecutionResult.kind` 分类
  每次执行结果，不再把任何非 error 返回都当作成功。`sync`（或无 kind）为业务执行成功——
  触发表单 `success` 事件绑定、重跑 `onSuccessRefresh` 目标并关闭弹窗。`approval` 保持
  弹窗开启，显示「已提交审批」提示、审批单号（`approvalId`）与审批中心深链入口（`/approvals?approvalId=...`），且不发
  success 事件、不触发刷新（审批通过前业务侧尚未变更）。`task` 保持弹窗开启，显示
  「任务已提交」提示与任务号（`taskId`）及任务中心入口（`/ops/jobs`），可沿用任务与执行日志链。抛出的异常保持弹窗与已填参数开启，并在弹窗内
  渲染失败反馈：响应体稳定 `error` 码、服务端 `message`，以及 `details` 的字段级条目
  （按表单字段标题映射，不显示原始字段 key）；结果体自带顶层 `error` 字段（未抛异常）
  同样按失败处理，不发 success 事件。服务端权限不足统一为 `403 permission_denied` 并透出机器可读 `details`（`required` 权限、`allowedRoles`、`functionId` 或任务 scope），渲染端把 `details` 收敛为原因行（`缺少权限：...`/`需要角色：...`）而不只显示通用状态文案。`details` 中字段存在于区块表单 schema 的条目经共享
  表单渲染器的 `externalErrors` prop 注入对应控件下方（RJSF `extraErrors`），反馈条只汇总
  表单外字段的错误。边界：inline（非弹窗）表单区块提交期间整卡进 loading 骨架，失败后
  输入值不保留（弹窗表单会保留）；操作页任务结果面板带显式「前往任务中心」入口（`/ops/jobs`）
  与「查询执行状态」入口；审批入口固定跳 `/approvals`；仅文档化的权限 details 键渲染为原因行（其余 `details` 仍按字段级处理）

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

`query` usage（资源页列表绑定）的 input assignments 中 form 来源路径按派生的**列表查询 schema** 校验 = `listView.filters` 键集 ∪ `{current, pageSize}`（后两项仅在 `pagination.enabled` 时纳入）；越界 form 路径在保存/发布结构校验即失败（GUX-017：强制查询栏真实接线——列表绑定缺 form assignments 时查询栏只是装饰、永不过滤）。资源页查询栏本身只渲染声明的 `listView.filters` 加分页字段，校验集合与可见查询表单保持一致。

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
- 表单展示变化不允许静默改变已发布页面；发布快照必须包含 FormPresentationSpec 及其命名展示变体（`variants`/`defaultVariant`，SUI-031）——回滚原子恢复并对照当前契约重新校验。
- 运行控制台只消费 PublishedPageSpec；未发布草稿不得暴露到执行路径，人工草稿领先发布时仍以发布快照执行。
