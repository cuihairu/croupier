# Croupier Schema 驱动 UI 架构审查

审查日期 2026-11-21，代码基线 `6b0e10ba6`。主范围是 Web 控制台、OpenAPI Source 导入、OpenAPI Provider、Dashboard 生成器、Page Studio、发布存储与调用链。移动端仅检查 schema 表单的入口与能力边界。

## 结论

| 问题                         | 当前结论                                                                                                                                                                                                                             |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 组件封装是否合理             | **分层合理，适配层需要收紧。** FunctionContract、PageSpec、RJSF/AJV 和页面运行时各有职责，适合继续演进。当前表单适配器同时改校验、管理状态、适配控件、处理布局和产出参数，多入口表现已有差异。                                       |
| 常见 OpenAPI 能否通用生成 UI | **简单 JSON 对象适用。** 对象内的标量、常规枚举、简单嵌套和数组有可用基础。带组合约束、引用、非对象请求体、复杂参数序列化或多媒体类型的常见接口，需要明确诊断和回退。现阶段无法承诺任意 OpenAPI 3.x 都能自动生成可正确提交的操作页。 |
| 同一 schema 能否有多种展示   | **能按不同 PageSpec 配置并发布。** 同一 scope 内，不同 `pageKey` 的页面可以绑定同一函数，并分别配置字段与界面。**同一页面、同一表单的命名风格和运行时选择尚未接通。**                                                                |
| 配置能否落库并有默认界面     | **现有 PageSpec 已满足按页面落库与发布版本管理。** 服务端生成默认表单，函数调试页也能即时生成默认表单。两条默认派生路径存在差异。建议复用 PageSpec 加入命名展示和默认指针，先修正契约和提交一致性。                                  |

## 1. 从接口到界面的实际链路

1. **导入与绑定。** OpenAPI Source 上传经 kin-openapi 加载与校验，按 operation 派生 FunctionContract，可在执行器绑定前生成提案。OpenAPI Provider 发现运行时方法时也会派生 schema，有独立实现和兼容回退。最终进入页面生成的是函数输入和输出契约，以及资源、能力和操作语义。 [Source 解析](../../internal/api/openapi/openapi_parser.go#L102) · [绑定与待绑定契约](../../internal/api/openapi/openapi_binding.go#L97) · [Provider 发现](../../internal/drivers/openapi/provider_discover.go#L221)
2. **生成默认页面。** 生成器根据函数语义生成资源、操作、任务或报表提案。资源页可组合列表、详情和操作，报表需要查询及结果映射。默认表单的核心是 `FormPresentationSpec`，包含 `jsonSchema`、布局、分组、字段和按钮配置。生成器默认 `vertical`，并从顶层属性推导控件和中文名称兜底。完全缺少输入 schema 时，操作表单有单一 payload 编辑入口。 [操作表单生成](../../internal/dashboard/generator/generator_operation.go#L145) · [默认表单](../../internal/dashboard/spec/form_presentation.go#L207) · [缺少 schema 的回退](../../internal/dashboard/generator/form_payload.go#L13)
3. **编辑、保存和发布。** Page Studio 支持修改页面草稿。发布事务校验草稿修订、保存完整 PageSpec 和绑定契约快照、更新当前发布指针并写入历史。业务控制台按游戏、环境读取生效页面。页面预览和线上页面可共享表单组件。 [表单编辑器](../../web/src/components/PageEditor/FormPresentationEditor.tsx#L28) · [发布事务](../../internal/api/page/service_publish.go#L90) · [控制台读取发布页](../../internal/api/console/service.go#L61)
4. **渲染与调用。** `SchemaFormRenderer` 把平台配置编译为 RJSF 的运行时 schema、`uiSchema` 和 Ant Design 布局。资源、操作、任务、报表、组合页和函数调试页都有调用点。控制台按发布绑定组装请求，执行前再次按函数输入 schema 校验。函数调试页从 descriptor 派生表单和参数初值，可选择原始 JSON。 [公共渲染器](../../web/src/components/SchemaFormRenderer/index.tsx#L423) · [控制台执行校验](../../internal/api/console/console_execute.go#L507) · [调试页 schema 派生](../../web/src/pages/Functions/Invoke/index.tsx#L161)
5. **结果展示。** 输出 schema 能辅助生成字段、列表和结果摘要。资源表格、详情、报表和任务状态还依赖 PageSpec 的列、结果映射和绑定语义。一个返回 `items` 的 schema 只能描述数据形状，分页参数、行标识和哪些操作应挂在行上仍需要平台约定或管理员配置。 [资源页面规格](../../internal/dashboard/spec/page_spec_v2.go#L18) · [结果渲染](../../web/src/components/PageRenderer/ResultViewRenderer.tsx#L79) · [调试页结果](../../web/src/pages/Functions/Invoke/InvocationResponse.tsx#L147)

## 2. 组件封装与实际差距

### 应保留的边界

- **保留一个通用 Web 表单引擎。** RJSF 负责对象、数组、字段状态和递归渲染，AJV 提供前端校验，平台适配 Ant Design。已有控件、错误边界、分组、远程选项及字段错误注入可复用。主题库语法 `ui:*` 留在编译产物中，有利于配置长期兼容。 [渲染入口](../../web/src/components/SchemaFormRenderer/index.tsx#L687) · [RJSF 的 uiSchema 定义](https://rjsf-team.github.io/react-jsonschema-form/docs/api-reference/uiSchema/)
- **保留页面和表单两级抽象。** 资源页负责表格和行操作，操作页负责确认与执行，任务页负责异步状态，报表页负责查询与结果，组合页负责布局和区块联动。表单控件只负责数据填写。现有资源、操作、任务、报表和组合页的拆分符合这一职责分配。 [PageSpec 模型](../../internal/dashboard/spec/types.go#L537) · [页面分发](../../web/src/components/PageRenderer/index.tsx)
- **保留发布快照与契约新鲜度检查。** 管理员配置和函数契约分别演进，发布版本能固定当时的展示及绑定依据。运行时能识别已发布绑定与当前 schema 的差异。 [发布快照](../../internal/api/page/service_publish.go#L104) · [契约新鲜度](../../internal/dashboard/freshness/freshness.go#L16)

### 优先修正的缺口

| 优先级 | 发现                                                                                                                                                                                                                                                                                                                                         | 影响与处理方向                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| P0     | **OpenAPI 导入的请求契约存在语义损失和双路差异。** Source 只收集 path/query，有这些参数时只合并 body 的 `properties` 和 `required`，同名键会被 body 覆盖，数组或标量 body 也没有保留位置。Provider 另行平铺参数，非对象 JSON body 放在 `body`。表单输入目前也缺少对 HTTP 参数位置、`style`、`explode`、媒体类型的统一表达。                  | 两条路径可为同一个 operation 生成不同表单和 payload。建议先定义可执行的请求中间表示，保留位置、类型和序列化信息。派生 legacy 平铺输入时检查冲突、约束丢失和不兼容形状，给出迁移及回退诊断。 [Source 输入派生](../../internal/api/openapi/openapi_parser.go#L581) · [Provider 输入派生](../../internal/drivers/openapi/input_schema_kin.go#L55) · [HTTP 请求构造](../../internal/drivers/openapi/provider_call.go#L132)                                                                                                                                                          |
| P0     | **提交存在两条不同的数据语义。** 表单内建提交会剔除隐藏字段，`getValues()` 返回含隐藏字段的实时值。调试页以及资源的创建、编辑和部分行操作用 `validate() + getValues()`。同时呈现规则允许改运行时 `required/min/max/pattern`，隐藏字段会从顶层 `required` 摘除，服务端仍按函数契约判定。                                                      | 切换展示可能改变 payload，或出现前端通过而服务端拒绝。隐藏时仍保留属性定义，已有值还可能被格式和范围校验拦下。应统一读取、校验、裁剪和序列化的接口，先完成绑定和明确的默认值填充，再校验最终请求。布局变体应沿用相同字段与 payload 语义。旧规则依赖应逐页核对。 [运行时校验与隐藏](../../web/src/components/SchemaFormRenderer/index.tsx#L347) · [两条提交路径](../../web/src/components/SchemaFormRenderer/index.tsx#L622) · [资源调用](../../web/src/components/PageRenderer/ResourcePageRenderer.tsx#L804) · [调试调用](../../web/src/pages/Functions/Invoke/index.tsx#L216) |
| P1     | **字段路径和控件声明范围超出实际运行时。** Go 协议允许类似 `address.city` 的路径。适配层用 `properties[field.key]` 和 `uiSchema[field.key]` 写入顶层，因此 RJSF 可以渲染嵌套对象，但平台配置无法用该路径控制其子字段。DateRange、RichText 无专门映射，Switch 映射 checkbox，Code/JSON 映射 textarea。                                        | 建立控件能力表，声明允许的 schema 类型、输出形状和回退。以 JSON Pointer 表达字段路径，兼容已有顶层键，并在发布时拒绝未知字段、冲突分组、类型不兼容的控件。编辑器的控件选项从同一能力表生成。 [字段协议](../../internal/dashboard/spec/form_presentation.go#L46) · [控件与字段映射](../../web/src/components/SchemaFormRenderer/index.tsx#L224) · [编辑器控件菜单](../../web/src/components/PageEditor/FormPresentationEditor.tsx#L36)                                                                                                                                           |
| P1     | **默认推导和语言未统一。** Web 调试入口读取 `x-ui-layout/x-ui-groups/x-group`，无 hints 时依赖 RJSF。Go 默认页面是纵向、顶层字段排序并主动指定部分 widget。Go 把 `date` 和 `date-time` 都推导为 DatePicker，适配器将其映射为 `date`，随库安装的 Ant Design 日期控件默认仅输出 YYYY-MM-DD。标签、说明、占位和组名的部分路径使用固定 `zh-CN`。 | 同源表单从调试到发布可能换布局、顺序、时间控件与文案。先定义统一的归一化与默认派生规则，用共享 fixture 对齐 Go/TS。DateTime 要保留时分秒和明确的时区语义。 [调试派生](../../web/src/utils/schemaHints.ts#L273) · [Go 控件推导](../../internal/dashboard/generator/form_hints.go#L260) · [字段呈现](../../web/src/components/SchemaFormRenderer/index.tsx#L281) · [分组模板](../../web/src/components/SchemaFormRenderer/templates.tsx#L115)                                                                                                                                     |
| P1     | **未知 schema 特性的诊断不完整。** 生成器递归检查 `oneOf/anyOf/discriminator` 和 HTTP(S) 引用，但缺少对根类型、悬空引用、dialect、媒体类型和入参丢失的系统性判断。错误边界会提示重试或 JSON，但业务页要由调用方提供可用的 JSON 入口。                                                                                                        | 用统一兼容性报告区分可直接生成、可降级编辑和需人工补绑定。保留 JSON 输入只能补足表现层能力，HTTP 序列化缺失仍需明确阻止错误执行。 [生成诊断](../../internal/dashboard/generator/schema_subset.go#L11) · [表单错误边界](../../web/src/components/SchemaFormRenderer/ErrorBoundary.tsx#L48) · [调试页 JSON 分页](../../web/src/pages/Functions/Invoke/RequestBodyEditor.tsx#L48)                                                                                                                                                                                                  |
| P2     | **布局与编辑器覆盖不完整。** `horizontal/inline` 主要调整标签列宽，`grid` 设 12 列。分组模板使用固定 `span`，协议定义 width 为 1–12，代码直接传给 Ant Design 的 24 栏栅格。编辑器当前主要暴露布局、字段顺序、组件、可见、禁用、标签和占位。                                                                                                  | 让编辑器声明真实支持的能力。以响应式断点为布局基线，统一 width 单位并验证旧数据。将分组编辑、移动端折叠和布局快照测试纳入首批风格功能。 [布局适配](../../web/src/components/SchemaFormRenderer/index.tsx#L423) · [栅格模板](../../web/src/components/SchemaFormRenderer/templates.tsx#L50) · [编辑器](../../web/src/components/PageEditor/FormPresentationEditor.tsx#L135)                                                                                                                                                                                                      |

同页多表单还需要稳定的实例 ID。局部 DOM 验证中，两个表单的首个字段同时使用 `root_first`，第二个作用域的标签查询无法正确关联。组合页并排显示多个表单、Page Studio 并排预览或变体暂时共存都会依赖这一层。应通过 RJSF 的 `idPrefix` 为每个挂载实例分配唯一、稳定的前缀，并验证 label、错误和说明的关联。 [RJSF Form 调用](../../web/src/components/SchemaFormRenderer/index.tsx#L687)

校验入口也有分歧。Console 执行用独立校验器，schema 编译失败会返回错误，并给部分未声明开放性的对象补严格属性规则。函数直接调用使用的 descriptor 校验默认允许缺失或无法编译的 schema 通过。两者默认 Draft 7。建议导入时统一报告方言与可编译性，并对通过两条执行链的相同输入做对照。 [Console 校验器](../../internal/validation/jsonschema.go#L44) · [函数直接调用校验器](../../internal/platform/functioncontract/validate.go#L19)

另一个和默认体验相关的决策是初值。函数调试页会给属性填入 schema 的 `default`、`example`、枚举首项或类型占位，再同步到原始 JSON。用于调试可以减少初次录入，但业务操作应明确区分示例、真实默认值和用户填写值。尤其是可选金额、开关和执行范围，建议缺省时保持未提供。 [初值推导](../../web/src/utils/json.ts#L163) · [调试页写入初值](../../web/src/pages/Functions/Invoke/index.tsx#L176)

## 3. OpenAPI 兼容性矩阵

这里的“支持”指从导入到提交均能保持语义。渲染库能画出控件只覆盖其中一步。OpenAPI 3.0 的 Schema Object 与 JSON Schema 有差异，3.1 使用基于 JSON Schema 2020-12 的方言。入口检查版本前缀 `3.` 本身无法证明方言在整个链路中保真。 [OpenAPI 3.0.4 Schema Object](https://spec.openapis.org/oas/v3.0.4.html#schema-object) · [OpenAPI 3.1.1 Schema Object](https://spec.openapis.org/oas/v3.1.1.html#schema-object) · [Source 校验](../../internal/api/openapi/openapi_parser.go#L102)

| 输入类型或场景                                          | 当前判定                               | 需要补齐的契约或验证                                                                                                                                                                 |
| ------------------------------------------------------- | -------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| JSON 对象，标量、字符串枚举、普通边界约束               | 基础可用                               | 对齐 `required` 与空串；覆盖数字枚举、`false/0`、枚举标签按值匹配。                                                                                                                  |
| 嵌套对象、普通数组、对象数组                            | 基础编辑可用，复杂呈现部分支持         | 验证递归必填、增删、稳定 ID、数组项级展示覆盖。                                                                                                                                      |
| 普通 path/query 参数                                    | 简单场景可用                           | 验证 path-item 继承、body 同名参数、query 数组、对象及序列化。                                                                                                                       |
| `date`、`date-time`、`time`                             | 具备时间控件，发布链有日期时间语义缺口 | 以完整返回值测试时区、秒精度及运行时校验。                                                                                                                                           |
| 本地 `$ref/$defs`、嵌套及递归引用                       | 依导入路径和引用形态                   | Source 序列化 `ref.Value` 的外层对象，嵌套引用可能仍指向源文档的 `#/components/...`。独立 schema 需带可解析定义或内联并正确处理递归。Provider 使用有深度上限的内联器，需单列测试。   |
| `allOf`、`oneOf`、`anyOf`、`discriminator`              | 组合类型部分支持，需要人工检查         | `allOf` 原样保留和合并语义需对拍。上传路径在与 path/query 合并时可能先丢掉根级组合约束，导致生成器看不到它。依据 discriminator 提供明确的分支选择和验证，用 fixture 验证切换后的值。 |
| OpenAPI 3.0 `nullable`、3.1 联合类型、条件 schema、元组 | 未建立端到端支持基线                   | 验证字段缺失、JSON `null`、空字符串的区分，覆盖 2020-12 关键字在导入、浏览器与服务端的行为。                                                                                         |
| `additionalProperties`、字典、读写方向                  | 部分支持                               | 动态键需测试提交和保留。发布页服务端校验会为部分未声明 `additionalProperties` 的对象施加严格规则。接入时要显式验证读写方向和可写 payload。                                           |
| 根数组、标量或空 body                                   | 依入口，缺少通用表单提交               | 当前共享表单值是 `Record<string, JSONValue>`。局部渲染探针中，给根数组设置默认值 `["one"]`，通过 `getValues()` 得到 `{"0":"one"}`。应完整支持根值，或明确包裹并可逆映射。            |
| header/cookie、form-urlencoded、multipart、binary       | 未接通通用表单到传输                   | Source 跳过 header/cookie，Provider 请求映射的 body 主要走 JSON。Upload 控件产出 URL 或 URL 数组，与 HTTP 文件内容是两种不同数据。                                                   |
| 外部引用与缺失引用                                      | Source 禁止外部读取；失效引用需诊断    | 发布前检查引用可解析。优先使用导入前打包好的可解析文档。                                                                                                                             |

矩阵依据与边界见 [Source 输入输出抽取](../../internal/api/openapi/openapi_parser.go#L581) · [Provider 引用解析](../../internal/drivers/openapi/input_schema.go#L147) · [Generator 子集诊断](../../internal/dashboard/generator/schema_subset.go#L43) · [前端根值归一](../../web/src/components/SchemaFormRenderer/index.tsx#L65) · [服务端严格对象校验](../../internal/validation/jsonschema.go#L82) · [文件控件](../../web/src/components/SchemaFormRenderer/widgets-upload.tsx#L68)。

## 4. 同一 schema 的多种展示与数据库设计

### 当前可直接实现的形态

| 需求                                           | 用现有实现能做到的程度                                                                                                 |
| ---------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| 同一个函数有两张业务页面，各用一种布局         | 可用两个 `pageKey`，分别绑定同一函数并发布，菜单可提供两个入口。                                                       |
| 一张页面里对同一函数使用两种界面               | 组合页可以按区块提供独立的表单和绑定，支持行内、弹窗、标签页、卡片等展示位置。配置是区块级的，使用者选择的是页面区块。 |
| 同一张页面发布前后使用不同风格                 | 可保存新草稿并发布，保留历史版本和回滚能力。发布版本是变更历史。                                                       |
| 同一个表单让操作者现场切换“标准 / 紧凑 / 分组” | 缺命名变体、有效默认、运行时切换和选择记忆。                                                                           |
| 调试页和业务页复用管理员配置的风格             | 调试页现行入口直接从 descriptor 派生；业务页读取 PageSpec。缺少有明确 scope 和优先级的解析机制。                       |

现有持久化键以 `gameId + env + pageKey` 为核心。`page_specs.spec_json` 保存草稿，`published_page_specs.spec_json` 保存发布快照，`page_versions.spec_json` 保存历史。此前 `function_ui_schema*` 系列表进入遗留清理，新的呈现配置适合继续沿 PageSpec 发布链演进。 [PageSpec 数据表](../../internal/model/page_spec.go#L16) · [旧表清理](../../internal/model/migration_legacy_cleanup.go#L9) · [组合区块规格](../../internal/dashboard/spec/types.go#L582)

### 建议的数据模型与选择规则

新增多风格能力时，在 PageSpec 的**表单位点**记录命名展示变体。位点可为操作表单、资源的创建/编辑表单、报表过滤表单或组合页中有稳定标识的表单区块。既有 `form` 继续作为基础。示意结构如下，字段名为设计提案。

```json
{
  "defaultPresentationKey": "standard",
  "presentationVariants": [
    {
      "key": "standard",
      "title": { "zh-CN": "标准", "en-US": "Standard" },
      "layout": "vertical"
    },
    {
      "key": "compact",
      "title": { "zh-CN": "紧凑", "en-US": "Compact" },
      "layout": "grid",
      "fields": [
        { "path": "/playerId", "width": 6 },
        { "path": "/reason", "width": 12 }
      ]
    }
  ]
}
```

- **风格可覆盖内容。** 行列布局、分组、顺序、标签、帮助文案、字段宽度及同值域控件。表单可见性、默认值、验证条件与取值映射有请求语义，应作为基础表单或绑定配置进行审核。设计稿若包含一个快速入口只填写常用字段，应在既有绑定能可靠提供其余值时发布。
- **默认解析。** 取当前用户显式选择且仍存在的已发布变体，其次取管理员发布时指定的默认变体，其次取原有表单展示。新表单缺少展示配置时由标准派生器生成纵向界面。无变体的旧页面自动走原有表单。用户切换时保留当前表单值和错误状态。
- **落库和版本。** 将变体与默认键写入现有草稿及发布快照。发布验证检查唯一 key、默认键存在、字段路径可解析、控件兼容和契约版本。契约更新后可提示字段改名或删除，并对变体和绑定分别做迁移校验。回滚使用对应版本的完整变体配置。
- **用户偏好及跨页面复用。** 若要跨设备记住选择，可单独保存 `userId + gameId + env + pageKey + formKey → variantKey`。若多页面需要共用管理员维护的模板，再提供带版本的 scope 级模板库，让 PageSpec 固定模板版本或在发布时展开到快照。发布接口可用现有草稿修订号防止多人编辑覆盖。共用模板的身份应与页面和函数绑定关联，结构相同的函数可能有不同操作者、标签及管控要求。
- **外观与布局。** “标准、紧凑、分组”属于同引擎内的表单展示。页面布局由 PageSpec 或组合区块承载，颜色、密度、字体等外观由受控主题 token 承载。三者可以组合。首批实现只需保证同一字段通过不同呈现得到同一 JSON 值。

例如，`playerId`、`reason`、`expiresAt` 可在标准界面中单列显示，在紧凑界面中排为两列，在审核工作台按“玩家与原因”“生效时间”分组。三种界面应经同一转换和校验后提交相同的 JSON，且共用该函数的权限、审批、确认和审计。

## 5. 演进顺序与验收

| 阶段                | 交付                                                                                                                   | 验收依据                                                                                                                                                        |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1. 契约与提交一致性 | 对齐 Source 和 Provider 的兼容输入转换，统一表单提交接口。建立 schema 方言、引用、参数位置、媒体类型和可执行能力诊断。 | 同一 OpenAPI fixture 通过上传、绑定及运行时发现后，可比较生成输入和最终 HTTP 请求。覆盖 path/query 冲突、根数组、嵌套引用、`nullable`、`date-time` 和隐藏字段。 |
| 2. 默认 UI 一致性   | 共用默认推导规则和控件能力表，补充路径、枚举值、日期精度、locale、响应式布局和 JSON 降级策略。                         | 同一契约在调试、预览和发布链有一致的默认控件与值语义，特殊场景有可解释的诊断。                                                                                  |
| 3. 命名展示与落库   | Page Studio 可维护标准与紧凑变体，配置默认，预览、发布、切换。                                                         | 同一 `pageKey` 下两种布局确实不同、输入保留并生成完全相同的请求。保存重开、发布、重启读取、跨游戏环境隔离、回滚均用集成测试验证。                               |
| 4. 扩展展示词汇     | 按实际使用反馈加入分组、弹窗或逐步操作等界面。需要跨页面复用时，再加入模板库和用户偏好。                               | 复杂操作的确认、必填与提交状态在不同展示下一致。发布前可准确指出所有失效的配置路径。                                                                            |

建议把验收 fixture 放在同一条测试链。至少覆盖“源 OpenAPI → 规范化契约 → 默认 PageSpec → 保存与发布 → 表单交互 → 服务端校验 → 实际 HTTP 请求”。同一组 fixture 可以证明渲染层和传输层对同一 schema 的理解一致。现有测试主要覆盖公共适配器、控件、生成器、Provider 兼容派生和页面发布各自的行为。

## 6. 审查范围与证据

本文依据仓库源码、现有测试、局部行为探针和 OpenAPI 官方规范。Web 为主。Flutter 的函数调用页使用独立的 `SchemaFormSpec` 分类与控件集，复杂节点可退化为只读 JSON，与 Web 的 PageSpec 布局配置没有同一消费链。实际生产数据库数据、线上发布、视觉易用性及浏览器响应式布局未纳入本次验证。 [Flutter 输入入口](../../mobile/lib/features/functions/function_invoke_page.dart#L27) · [Flutter 字段分类](../../mobile/lib/features/functions/schema_form_model.dart#L107)

### 定向复现

| 探针                                                                                   | 结果                                                                                                                                                             |
| -------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| OpenAPI Source 同时包含 query `mode:string`、header 必填参数和对象 body `mode:integer` | 实际独立输入 schema 只保留 body 的 `mode:integer`，丢失 header 参数和根级 `additionalProperties:false`。一个子字段仍引用源文档的 `#/components/schemas/Nested`。 |
| OpenAPI Source 有一个 query 参数和根数组 body                                          | 实际独立输入 schema 只包含 query 参数。                                                                                                                          |
| Web 同一 schema 的纵向与网格                                                           | 实际 DOM 的列宽分别是 24 和 12。同页挂载时字段 ID 重复。                                                                                                         |
| Web 表单隐藏字段、嵌套路径和根数组                                                     | `getValues()` 返回隐藏字段值。点分路径仅出现在顶层 `uiSchema` 中。根数组的共享表单值变成数字键对象。                                                             |

OpenAPI 探针和 4 个 Web 探针均复现预期中的当前行为。临时测试源码已清理。

### 现有测试

- Web 公共表单、函数调试页与表单编辑器共 22 套、314 个测试通过。另一次四套定向复跑中，隐藏字段切换测试超过 Jest 的 5 秒限制，其余 27 项通过。
- Go 的 OpenAPI Source、Dashboard 生成器、页面规格、页面 API 和 JSON Schema 校验包通过。OpenAPI Provider 首次运行受沙箱套接字限制，允许监听本地临时端口后整包通过。
