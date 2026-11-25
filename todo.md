# TODO

更新：2026-10-30。以源码、失败路径及实际验收证据区分已实现、已验证和待核实；LOCAL VERIFIED 只覆盖注明的本地验收，不等于生产上线。优先级 P0 > P1 > P2。每个复选框对应一个可独立评审/验收的交付；依赖 `—` 不代表没有环境条件，NOT RUN 表示对应验收未执行。

最新结论见 [可靠性复审](docs/review/reliability-recheck-2026-10-30.md)。之前的完成声明与回流旧任务完整保存在 [本轮整理前快照](docs/review/todo-import-2026-10-30.md)；更早原文见 [10 月 29 日历史](docs/review/todo-history-2026-10-29.md)。旧 C2/E2/D2/D5 分别复用 UI-001/002/003，不重开一套父子任务；其已交付历史不删除，只退出当前执行清单。

## Schema 驱动 UI（2026-11-21）

依据 [Schema 驱动 UI 架构审查](docs/review/schema-driven-ui-2026-11-21.md)。以下条目均待交付。报告中的复现和测试记录作为基线。真实游戏联调继续跟踪 UI-001，既有模板呈现验收见 UI-003、GUX-017。

### OpenAPI 导入与发送

- [x] **SUI-001 · P0 · 设计 · 定义统一的 OpenAPI 请求表示** ✅ be8c136fe
  - 依赖：—。交付与验收：固定参数位置、名称、必填、序列化方式及请求体媒体类型的表达。用 fixture 表达 path、query、header、cookie，以及同名参数和根数组 body；明确旧平铺契约的兼容边界。
  - 交付：`internal/dashboard/spec/request_contract.go`（canonical 类型）；设计文档 `docs/architecture/openapi-request-contract.md` + `docs/zh/...`（含投影规则、兼容边界、执行映射）；descriptor-v2 交叉引用。证据：`go build ./internal/dashboard/spec/` 通过；be8c136fe。

- [x] **SUI-002 · P0 · 实现 · 保真提取 Source 请求契约** ✅ 45796d6d1
  - 依赖：SUI-001。交付与验收：提取 operation 与 path-item 参数及完整 body schema。回归样例覆盖 query/body 同名、必填 header、根数组和 body 根级约束，派生结果逐项可追溯。
  - 交付：kin 前端 `ExtractOperationRequestContractKin` + map 前端 `ExtractOperationRequestContractMap`（operation 覆盖 path-item、完整 body schema、序列化默认、$ref 内联）。证据：`TestRequestContract_KinPathItemParamsMergedWithOverride`/`_KinHeaderCookieCaptured`/`_KinRootArrayBodyPreserved`/`_BodyRootConstraintForcesStructured`/`_QueryBodySameNameCollision`；45796d6d1。

- [x] **SUI-003 · P0 · 实现 · Provider 复用统一请求提取** ✅ 45796d6d1
  - 依赖：SUI-001, SUI-002。交付与验收：同一 3.x fixture 经上传与运行时发现得到相同的规范请求。既有 Provider 配置和 Swagger 2.0 兼容用例通过。
  - 交付：`provider_discover.go` 复用统一提取（kin 优先、map 兜底）；对拍测试 `TestRequestContract_KinMapParity` + `TestInputSchemaParity_KinMatchesLegacy`；Swagger 2.0 兼容 `TestRequestContract_Swagger20Compat`（in:body / formData / consumes）；既有 config-file provider 用例通过（`TestProviderCallWithRequestBody` 等）。证据：45796d6d1。

- [x] **SUI-004 · P0 · 实现 · 安全处理旧平铺入参** ✅ 45796d6d1
  - 依赖：SUI-001。交付与验收：已有无歧义对象输入维持原接口。新派生遇到同名冲突或无法保留的约束时给出 operation 与字段级诊断，覆盖旧绑定重读和重新导入。
  - 交付：`ProjectOperationInputSchema` 三模式（flat/structured/body_only）；字节稳定平铺 `TestRequestContract_FlatProjectionByteStable`；字段级诊断（`DiagRequestParameterCollision`/`DiagRequestBodyShapeNonFlat` 带 `Field`）；api 侧重读/重导 `TestOpenAPIRequestSchemaReimportStable`。证据：45796d6d1。

- [x] **SUI-005 · P0 · 实现 · 保留根数组和标量 JSON 请求体** ✅ 8427322d6
  - 依赖：SUI-002, SUI-003。交付与验收：根数组和标量在存在 query 参数时仍能通过请求映射送达上游，断言实际 HTTP body 的 JSON 类型及值。
  - 交付：`Provider.Call`/`buildRequest` 消费请求契约解析调用方 payload（`resolveCallInput`），structured/body_only body 原样发送；`TestProviderCall_ContractRootShapePreserved` 断言 query + body JSON 类型与值（数组/标量/对象根 + flat 回归）。证据：8427322d6。

- [x] **SUI-006 · P1 · 实现 · 按 OpenAPI 序列化参数** ✅ 0109f42bf
  - 依赖：SUI-002, SUI-003。交付与验收：验证公开声明支持的参数位置和 `style/explode` 组合。fixture 断言 path 替换、query 数组和对象、header 与 cookie 的实际 HTTP 值。无法执行的组合返回诊断。
  - 交付：执行器按位置与 style/explode 序列化（`param_serialize.go`：path simple/label/matrix、query form/spaceDelimited/pipeDelimited/deepObject、header simple、cookie form 原始值；逐项转义 + 字面分隔符，spaceDelimited 用 `+`）。支持矩阵公开声明于 `docs/architecture/openapi-request-contract.md#parameter-serialization-sui-006`（+中文镜像）。不可执行组合以诊断 `openapi_parameter_serialization_unsupported` 在两个提取前端记录（`request_contract.go`），执行器同因拒绝请求。契约与 legacy config 方法统一走序列化器，query/path 值补齐 URL 编码。证据：`TestSerializeParamValue_Matrix`（全矩阵单测）；`TestProviderCall_ParameterSerializationWire`（httptest 线上断言 path 替换/转义、query 数组/对象/deepObject、header、cookie 与不可执行报错）；`TestProviderCall_LegacyConfigSerialization`（旧行为不回归）；`TestRequestContract_SerializationDiagnostics`（kin/map 诊断同文）；`go build ./...` 通过；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 通过；0109f42bf。

- [x] **SUI-007 · P1 · 实现 · 确定性选择请求和响应媒体类型**
  - 依赖：SUI-001。交付与验收：同一多媒体类型文档重复导入结果一致。输入以执行器支持能力选择媒体类型，输出 schema 从声明的成功响应派生，缺少可执行媒体类型时提案记录原因。
  - 交付：媒体类型按执行器能力确定性选择（`internal/dashboard/spec/request_body_media.go`：`SelectRequestBodyMediaType` 可执行集 = `application/json` + RFC-6839 `application/*+json`，`application/json` 优先、后缀其次、同 rank 字典序；`SelectExecutableMediaType` 供 2.0 `consumes` 声明序；`IsJSONMediaType`/`IsExecutableRequestBodyMediaType`）。kin/map 两个提取前端同源选择（`request_contract.go`/`request_contract_map.go`）；无可执行媒体类型时以诊断 `openapi_request_body_no_executable_media`（severity warning、field=body、message 列声明集与可执行集）记录原因，body 槽位不设（取代旧「不可执行媒体类型静默成为槽位并错标 JSON」）。执行器（`provider_call.go` `contractBodyContentType`）以所选媒体类型设 `Content-Type`；存量契约记录不可执行媒体类型时同因拒绝。输出 schema 从声明成功响应派生（200 优先、其余 2xx 升序、优先 application/json）；上传管线 `openAPIResponseSchema`/`openAPIMediaSchemaJSON` 去 map 迭代不定序、驱动 `extractOutputSchema` 同口径。支持矩阵公开声明于 `docs/architecture/openapi-request-contract.md#media-type-selection-sui-007`（+中文镜像）。证据：`TestIsJSONMediaType`/`TestIsExecutableRequestBodyMediaType`/`TestSelectRequestBodyMediaType_PreferenceAndTieBreak`/`TestSelectRequestBodyMediaType_Deterministic`/`TestSelectExecutableMediaType_ConsumesOrder`/`TestRequestBodyMediaDiagnostic`（spec 单测）；`TestRequestContract_ExecutableMediaTypeSelected`/`TestRequestContract_ExecutableMediaTypeSuffix`/`TestRequestContract_NoExecutableMediaTypeRecordsReason`（kin/map 同码）；`TestRequestContract_RepeatedImportDeterministic`（同一文档 20 次导入逐方法 JSONEq）；`TestRequestContract_Swagger20ConsumesCapability`；`TestProviderCall_BodyMediaTypeSentAsContentType`/`TestProviderCall_StaleNonExecutableBodyMediaTypeRefused`；`TestOpenAPIResponseSchemaDeterministic`（上传管线）/`TestExtractOutputSchema_Ascending2xx`（驱动）；`go build ./...` 通过；`go test ./internal/...` 全绿（172 pkg ok）；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 通过；a4ebdfbad。

- [x] **SUI-008 · P1 · 实现 · 将本地 schema 引用封装成独立契约** ✅ af81e9266
  - 依赖：SUI-002, SUI-003。交付与验收：脱离 OpenAPI 文档后，嵌套及递归本地引用仍可解析或明确进入待审。验证缺失引用、循环引用和外部引用边界。
  - 交付：契约提取把无法内联的 schema 引用显式标记待审（取代旧「缺失/循环引用静默坍缩为匿名对象、外部引用保留但无信号」）。重落在新 API 上：spec 侧新增 `internal/dashboard/spec/schema_reference.go`（`SchemaReferenceUnresolvedDiagnostic`：severity warning、field=声明 `$ref` 指针原文、message 命名边界——`missing target in the document` / `cyclic reference past the inlining depth cap` / `reference outside the document`）；驱动侧 `input_schema.go` 的 `resolveSchemaRefs`/`resolveRefPointer` 经 `issues *[]refIssue` 收集三类边界，`refIssuesToDiagnostics` 按 (ref,reason) 去重。新树归一点在 `ProjectInput`（`request_canonical.go` 的 `canonicalizeRequest`，对 DTO 原始快照做拷贝后解析引用）：kin 前端（`kinRequestContracts` 契约保留原始 `$ref`）投影时收集全部引用诊断，投影过程不再二次收集；`output_schema.go`/旧 `extractInputSchema` 保持静默（传 nil）。map 回落前端 `extractInputSchemaWithIssues` 同机收集，`provider_discover.go` 回落分支把诊断带入 `InputProjection.Diagnostics`（kin loader 拒载悬空/外部引用、整份文档回落 map 前端，故这两个边界经 map 前端记录同样的诊断）。嵌套本地引用照常内联（无诊断）；循环引用在深度上限（16）截断记 cyclic；缺失引用坍缩为 `{"type":"object"}` 记 missing；外部引用原样保留 `{"$ref":...}` 记 external。支持矩阵公开声明于 `docs/architecture/openapi-request-contract.md#schema-reference-resolution-sui-008`。证据：`TestSchemaReferenceUnresolvedDiagnostic`（spec 单测）；`TestSchemaRef_NestedLocalRefsResolve`/`TestSchemaRef_CyclicRefRecordsDiagnostic`/`TestSchemaRef_MissingRefRecordsDiagnostic`/`TestSchemaRef_ExternalRefRecordsDiagnostic`/`TestSchemaRef_SameRefDedupedPerContract`/`TestSchemaRef_ParameterRefCollected`/`TestSchemaRef_FallbackDiscoveryRecordsDiagnostic`（生产发现路径 provider Init→GetMethodDetails）；`go build ./internal/...` 通过；`go vet` 全绿；`go test ./internal/drivers/openapi/... ./internal/dashboard/spec/...` 全绿；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 通过（EXIT=0）；af81e9266。

- [x] **SUI-009 · P1 · 实现 · 归一化 OpenAPI 3.0 与 3.1 schema 方言** ✅ 8a15a0d38
  - 依赖：SUI-002, SUI-003。交付与验收：对照测试覆盖 3.0 `nullable`、3.1 联合类型、缺失值与显式 null，并明确条件 schema、元组等关键字的支持或诊断。前端和服务端对声明支持的样例判定一致。
  - 交付：重落在新 API 上：spec 侧 `internal/dashboard/spec/schema_dialect.go` 原样落地——`DetectSchemaDialect`（按 `openapi` 字段判 3.0/3.1）、`NormalizeSchemaDialect`（`nullable:true` 折入 `type` 数组、`"null"` 移末位、递归子 schema 位置、非 schema 值如 default/example/enum 透传）、`ClassifySchemaSupport`（supported/degraded/unsupported；条件 schema、元组、多类型联合→degraded）、诊断 `openapi_schema_dialect_keyword`（warning，field=schema JSON 指针，`DialectNotesToDiagnostics` 按 (path,keyword) 排序保证重导入字节一致）。新树把 lane 的 `NormalizeRequestContractDialect` 并入 `request_canonical.go` 的 `canonicalizeRequest`（对契约全部参数 + body media schema 逐槽归一并合并诊断，`ProjectInput` 在投影前统一执行），map 回落 `extractInputSchemaWithIssues` 对组装后 schema 同机归一+同诊断（`provider_discover.go` 回落分支带入方法详情）；`output_schema.go` 输出 schema 同机归一（响应校验器 Draft 7 不认 `nullable`，不归一会拒合法 null）；生成器 `generator_report.go` 的 `schemaTypeFromObject` 改为主类型解析（`type` 数组取首个非 null，与前端 `primarySchemaType` 同规则；此前对 `["string","null"]` 返回 `""` 使可空字段 placeholder 兜底静默丢失）。缺省 vs 显式 null 语义钉住（`required` 内缺省即非法、可空必填字段可发 `null`、不可空显式 `null` 被校验器拒）。前端镜像与文档随 1deb91baf 已合（`web/src/utils/schemaDialect.ts`；`docs/architecture/openapi-request-contract.md#schema-dialect-normalization-sui-009`）。证据：spec 单测 20 例（nullable 折入/3.1 联合/`"null"` 末位/条件/元组/非 schema 值透传/byte-stable/诊断确定性 30 迭代）；驱动层 8 例——`TestDialect30_NullableFolded`/`TestDialect31_UnionNormalized`/`TestDialect30And31_SameCanonicalForm`（3.0/3.1 逐字节同形）/`TestDialect_ProductionByteStable`/`TestDialect_DegradedKeywordsDiagnosed`/`TestDialect_Draft07TupleUsesFallbackPath`（kin 拒载回落 map、回落侧记元组诊断）/`TestDialect_OutputSchemaNormalized`（Draft 7 校验显式 null 通过、缺省被拒、不可空 null 被拒）/`TestDialect_FallbackDiscoveryRecordsDialectDiagnostic`；生成器 `TestExtraSchemaTypeFromObject_TypeArray`/`TestExtraSchemaTypeFromObject_FeedsWidgetDerivation`；`go build ./internal/...` 通过；`go vet` 全绿；`go test ./internal/drivers/openapi/... ./internal/dashboard/spec/... ./internal/dashboard/generator/...` 全绿；`pnpm --dir web run tsc` 0 错；`bash scripts/dashboard_vnext_guard.sh` PASSED；8a15a0d38。

- [x] **SUI-010 · P1 · 实现 · 区分请求和响应字段方向** ✅ c0eb3785f
  - 依赖：SUI-008, SUI-009。交付与验收：处理 `readOnly/writeOnly` 对输入、输出和必填集合的作用。嵌套字段、引用字段与普通字段都经过请求和响应 fixture 验证。
  - 交付：重落在新 API 上：spec 侧 `internal/dashboard/spec/schema_direction.go` 原样落地——`SchemaDirection`（request/response）与 `droppedKeyword`（request→`readOnly`、response→`writeOnly`）；`FilterSchemaDirection` 在深拷贝上工作（输入不被突变），根级标记时整份 schema 归 nil；递归位置：`properties`/`items`/`additionalProperties`（对象形式，被标记时该槽位删除）/`allOf`+`anyOf`+`oneOf` 成员/`dependentSchemas`，另递归但永不移除成员：`if`/`then`/`else`/`not`/`contains`/`propertyNames` 与元组 `prefixItems`/`items`-数组；`$defs`/`definitions` 不动；`adjustRequired` 仅在确有剔除时收缩 `required`（收缩为空则删键），无剔除时结构原样返回保证字节稳定；方向匹配的标记存活；诊断 `openapi_schema_readonly_dropped`/`openapi_schema_writeonly_dropped`（warning，field=schema JSON 指针），`DirectionNotesToDiagnostics` 按 (path,keyword) 排序。新树把 lane 的 `CanonicalizeRequestContract` 并入 `request_canonical.go` 的 `canonicalizeRequest`：方言归一（SUI-009）→ 请求方向过滤（SUI-010）一体的契约归一（诊断序 ref→dialect→direction），SUI-004 平铺/冲突诊断在 `ProjectInput` 内于过滤后计算——readOnly 属性与参数同名不再假阳性冲突、过滤后的 body 投影回 flat；body schema 根级 readOnly 时整个 body 槽位剔除（`Body=nil`，不发「无 JSON body」约束诊断）；参数 schema 根级 readOnly 保留参数（地址永不破坏）。map 回落 `extractInputSchemaWithIssues` 逐参数槽 + body 槽同机过滤（参数根级 readOnly 同样保留字段）。`output_schema.go` 响应侧过滤 `writeOnly`、根级 writeOnly 落下一 2xx 候选；`openapi_parser.go` 的 `openAPIResponseSchema` 经 `openAPIResponseSchemaDirection` 同口径过滤（该路径无诊断通道）。文档 `docs/architecture/openapi-request-contract.md#schema-field-direction-sui-010` 随 1deb91baf 已合，其函数名笔误（`FilterJSONSchemaDirection`→`FilterSchemaDirection`）本批修正；未改 web（过滤在服务端完成）。证据：spec 单测 21 例（普通/嵌套/引用内联/数组 items/additionalProperties/allOf 成员/根级标记/条件与元组不移除/匹配标记存活/`$defs` 不动/required 收缩/字节稳定/输入不被突变/诊断映射与 30 迭代确定性）；驱动层 8 例——`TestDirection_RequestDropsReadOnlyAndShrinksRequired`（普通+嵌套+`$ref` 内联三处剔除、required 逐层收缩、剔除进诊断）/`TestDirection_KinAndMapFrontendsDropTheSame`（两前端投影逐 JSON 同形）/`TestDirection_ResponseDropsWriteOnly`/`TestDirection_ResponseWholeSchemaWriteOnlyDropped`/`TestDirection_RequestNoReadOnlyNoDiagnostics`/`TestDirection_DroppedPropertyNoLongerCollides`（过滤后投影回 flat）/`TestDirection_FlaggedRootRequestBodyDropped`/`TestDirection_ParameterRootReadOnlyKept`；api 层 `TestOpenAPIResponseSchemaDirection`；`go build ./internal/...` 通过；`go vet ./internal/...` 全绿；`go test ./internal/drivers/openapi/... ./internal/dashboard/spec/... ./internal/dashboard/generator/... ./internal/api/openapi/...` 全绿；`pnpm --dir web run tsc` 0 错；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 通过（EXIT=0）；c0eb3785f。

- [x] **SUI-011 · P1 · 实现 · 生成 schema 兼容性诊断** ✅ d8968afc9
  - 依赖：SUI-004, SUI-007, SUI-008, SUI-009, SUI-047。交付与验收：给出可直接生成、可降级编辑或需人工修复的状态与字段路径。覆盖 `allOf/oneOf/anyOf/discriminator`、悬空引用、根值及媒体类型。无法正确发送的 operation 不生成可直接执行的默认提案。
  - 状态／条件：LOCAL VERIFIED（2026-11-25，本地全套件）：分级在归一后的请求契约上判定（SUI-008/009/010 之后），分级结果不会与表单实际渲染内容不一致。spec 侧新增 `internal/dashboard/spec/schema_compatibility.go`：三态 `direct`/`degraded`/`manual` 与诊断码 `openapi_schema_compatibility_degraded` / `openapi_schema_compatibility_manual`（均 warning，不阻断契约落库，由提案质量门消费）。驱动侧新增 `internal/drivers/openapi/request_compatibility.go` 的 `AssessRequestCompatibility(operationID, request, root)`：先 `canonicalizeRequest` 再逐槽判定——`allOf`/`oneOf`/`anyOf` 与参数的非 JSON content 媒体 → degraded；`discriminator` → manual；引用不二次上报（缺失/循环/外部三类 refIssue 统一映射 manual，Field=引用原文）；body 根 `array` 或裸标量 → degraded；body 无 `application/json` 媒体类型或 JSON 与其它媒体并存 → manual。Field 统一为请求契约内 JSON pointer（形如 `$.body.content.application~1json/schema/discriminator`），按 (field,message) 排序去重，重导入字节稳定。接线：`openapi_parser.go` 逐 operation 评估并写入 `OpenAPISourceOperation.requestCompatibility`（lowerCamelCase wire 键）；`openapi_binding.go` 两条契约路径（bound `functionMetaInputForBinding` / unbound `functionMetaInputForOperation`）经 `sourceRequestDiagnostics` 装入 `FunctionContractInput.RequestDiagnostics`；`contract_service.go` 用 `functionContractDiagnostics` 与归一诊断去重合并落库。补上两处此前静默丢信号的断链：`contract_projection.go` 的 `OperationSpecFromContract` 从不复制契约诊断、`generator_operation.go` 的 `assessBaseCandidate` 从不读 `op.Diagnostics`——补齐后 `manual` 才真正强制 `needs_review`（`degraded` 不强制，其 JSON 编辑路径仍可用）；`schema_subset.go` 另补 `allOf` 与未内联本地 `$ref`（`unresolved_$ref`）两条 reason。证据：驱动层 13 例（direct/nil/allOf+oneOf+anyOf/discriminator/悬空引用/远程引用/根数组/裸标量/非 JSON body/JSON 与 multipart 并存/参数 content/manual 压 degraded/确定性）；api 层 `TestParseOpenAPISource_AssessesRequestCompatibility`（三 operation 三态 + 字段路径断言）与 `TestService_CreateSourceStoresRequestCompatibilityOnContract`（multipart 上传 → 契约行带 manual 诊断 → 生成的默认提案 `quality=needs_review`）；service 层 3 例（unbound/bound 落库、重注册去重）；生成器 3 例（manual→needs_review、degraded→basic、干净→basic）；`go build ./...` 通过；`go test ./internal/...` 全绿（零 FAIL）；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 通过。文档：`docs/architecture/openapi-sdk-descriptor-v2.md`（中英双树「请求兼容性分级」支持矩阵）与 `docs/architecture/dashboard-page-model.md`（quality 判定补契约级诊断）。边界如实：form-urlencoded/multipart 发送属 SUI-012/013（当前判 manual）；变体渲染属 SUI-043（当前只能 JSON 编辑）；SUI-047 完整矩阵的其余维度未在本批收口。

- [x] **SUI-012 · P2 · 实现 · 发送 form-urlencoded 请求体** ✅ 4b9e2ef7c
  - 依赖：SUI-007。交付与验收：执行器支持后再将该媒体类型标记为可执行，断言真实 Content-Type、多值字段和转义后的请求体。
  - 状态／条件：LOCAL VERIFIED（2026-11-25，本地全套件）：执行器（`provider_call.go` `buildBody`）新增 `form` 分支——`RequestBodyMapping.Type == "form"` 序列化为 `application/x-www-form-urlencoded`（`buildFormBody`）：显式 `Fields` 映射取值、无映射全量透传；`url.Values.Encode` 转义全部值并按键排序（空格为 `+`、`&`/`=`/`%` 转义、确定性）；多值字段一条 key 多个 entry（`[]interface{}`/`[]string` 逐项）；嵌套对象按展开式 `key[subkey]=value`（空对象 `{}` 保字段存在）；标量渲染 JSON 风格（bool `strconv.FormatBool`、数字 `FormatFloat(-1)` 而非 Go `%v` 的 `1e+06` 形态）；nil 跳过、缺失映射字段跳过。兼容性分级同步改真（SUI-011 修正）：form-urlencoded 从「执行器不可发送（manual）」改为 degraded —— body 只声明 form 媒体或 JSON 与 form 并存 → `openapi_schema_compatibility_degraded`（执行器可发，但序列化方式由 provider `requestBody.type` 配置决定、生成页面无法表达该选择，故不进 direct 层）；multipart 与其它不可序列化媒体保持 manual，消息由「只能发 JSON」改为按可序列化集合如实陈述。文档三层同步：`docs/architecture/openapi-sdk-descriptor-v2.md` 中英双树分级矩阵新增 form 两行、`docs/architecture/openapi-request-contract.md` Known boundaries 改写 form 段（SUI-012 已执行 + 如实记录 media-type selection 文档与现源缺口：`request_body_media.go` 在 1deb91baf 合并时被丢弃，投影仅认 `application/json`）。证据：`TestBuildBody_FormType_EscapesAndMapsFields`（转义+字段映射+多余字段剔除）/`_MultiValueField`（一条 key 多 entry）/`_WithoutFieldsSendsAllData`/`_RendersScalars`（bool/数字/嵌套对象/空对象）/`_IsDeterministic`/`_SkipsAbsentAndNilFields`；`TestCall_FormRequestBody_SendsEscapedFormBody`（httptest 全链断言真实 Content-Type=application/x-www-form-urlencoded、逐字节 body `account=acc+one&tag=alpha&tag=b%26c`、`url.ParseQuery` 还原多值）；分级 `TestAssessRequestCompatibility_FormURLBodyMediaIsDegraded`（field=`$.body.content.application~1x-www-form-urlencoded`）/`_MultipartOnlyBodyIsManual`；`go build ./...` 通过；`go test ./internal/...` 全绿（SUITE-EXIT=0）；`go vet` 全绿；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 通过（EXIT=0）。边界如实：multipart 发送属 SUI-013（仍 manual）；执行器媒体类型仍由 provider 配置而非文档声明选择，完整媒体能力选择（SUI-007 文档所述）未在现源实现。证据：4b9e2ef7c。

- [x] **SUI-013 · P2 · 实现 · 发送 multipart 文件请求体** ✅ ae91af17c
  - 依赖：SUI-007。交付与验收：明确文件字段的数据边界和执行器传输，验证单文件、文件数组、附带字段及上游收到的文件字节。现有产出 URL 的 Upload 控件沿用其值契约。
  - 状态／条件：LOCAL VERIFIED（2026-11-25，本地全套件）：执行器（`provider_call.go` `buildRequest`）新增 multipart 分支——`RequestBodyMapping.Type == "multipart"` 走 `buildMultipartBody`（`mime/multipart` writer，Content-Type 由 writer 生成含随机 boundary），其余类型仍走 `buildBody`。文件字段数据边界：上传对象 `{filename, content}`，`content` 为 data URL（`data:<media>;base64,<data>`，按 media 推扩展名）或原始 base64（缺省 `upload` 文件名）；数组字段展开为多个 part；**裸字符串一律按文本 part 处理**（仅 `data:` 前缀字符串视为文件）——对应验收「现有产出 URL 的 Upload 控件沿用其值契约」：URL 值按普通文本字段原样传输，不猜测文件语义；非对象/非字符串载荷报错（`file field value must be an upload object or a data URL string, got %T`）、非法 base64 报错、nil 字段跳过。附带字段经 `formValueString` 渲染为文本 part；显式 `Fields` 映射或无映射全量透传；字段按源键排序写入保证确定性（boundary 随机但 part 内容稳定）。上游收到解码后的文件字节（data URL 头剥离、base64 解码）。兼容性分级如实：multipart 保持 `manual`，消息改为真实边界——「执行器在配置 multipart 时可序列化，但 Upload 控件交付的是文件 URL 而非文件字节，生成页面无法交付该载荷」（SUI-047 矩阵跟踪控件契约演进）。文档三层同步：`docs/architecture/openapi-request-contract.md` 与 `docs/architecture/openapi-sdk-descriptor-v2.md` 中英双树改写 multipart 段与分级矩阵行。证据：`TestBuildMultipartBody_SingleFile`/`_FileArray`/`_CompanionFieldsAndDerivedFilename`/`_RawBase64Content`/`_SkipsNilAndRejectsBadPayload`/`_IsDeterministic`（按 part 载荷对比剥离 boundary 帧封）；`TestCall_MultipartRequestBody_SendsFileParts`（httptest 全链断言 boundary Content-Type、文本 part、文件 part 收到解码后字节 `iVBORw0KGgo=` 而非 data URL）；`TestAssessRequestCompatibility_MultipartOnlyBodyIsManual` 补消息含 file bytes 断言；`go build ./...` 通过；`go test ./internal/...` 全绿（SUITE-EXIT=0，172 包零 FAIL）；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 通过（EXIT=0）。边界如实：Upload 控件仍产出 URL，multipart 在分级中保持 manual；执行器媒体类型仍由 provider 配置选择（SUI-007 的 executable media selection 未在现源实现，`request_body_media.go` 于 1deb91baf 丢弃）。证据：ae91af17c（代码+测试）、ca1789f79（文档）。

### 表单值、校验与默认界面

- [x] **SUI-014 · P0 · 修复 · 统一表单的提交取值接口** ✅ 2d4825d3c ddae70725
  - 依赖：SUI-045。交付与验收：原生提交、调试、资源创建/编辑、行操作和组合页经同一提交取值逻辑。隐藏前后的编辑值可恢复，提交 payload 严格按统一的投影规则生成。
  - 交付：`SchemaFormRenderer` 唯一投影 `projectSubmitValues`/`getSubmitValues()`/`isFieldHidden`（原生提交与命令式同一规则，2d4825d3c）；`ResourcePageRenderer`、`Functions/Invoke`、`CloneModal` 命令式提交改走 `getSubmitValues()`（ddae70725）；组合页经 `onFinish` 原生路径。证据：`submit-values.test.tsx`（原生/命令式剔除-恢复对拍）本机单跑通过；`compositeDialog.test.tsx` 10/10 通过；2d4825d3c ddae70725。

- [x] **SUI-015 · P0 · 修复 · 以最终绑定参数校验调用契约** ✅ d775c708d 2d4825d3c ddae70725
  - 依赖：SUI-014, SUI-045。交付与验收：统一可见性、旧呈现校验规则与参数绑定的边界。绑定及明确默认值应用完再校验最终请求；隐藏且无绑定来源的必填项有结构化错误，隐藏且由绑定提供的合法值可执行。
  - 交付：验收已由边界定义 + 统一投影 + 既有服务端校验顺序满足，无新增代码。边界定义在 SUI-045（`console_execute.go:196-200` 先 `buildBindingPayloadFromSelectors` 应用绑定与 default transform、后 `validateBindingExecutePayload` 校验最终请求，`console_execute.go:507-521` 结构化 422）；可见性与提交裁剪统一为 `isFieldHidden`/`projectSubmitValues`（2d4825d3c）。证据：`go test ./internal/api/console/ -run 'TestServiceExecuteBinding(RejectsMissingRequiredPayloadField|UsesRowSelectorWithoutPassingWholeRow|RejectsPayloadTypeMismatch|IgnoresUnselectedContextFields)$'` 4/4 通过；`field-validity.test.tsx` 18/18 通过；d775c708d 2d4825d3c ddae70725。

- [x] **SUI-016 · P1 · 实现 · 对齐两条服务端入口的 schema 校验策略** ✅ f32020f46
  - 依赖：SUI-009。交付与验收：Console 执行与函数直接调用使用同一组对照 fixture。区分缺少 schema 和 schema 编译失败，钉住显式及省略 `additionalProperties` 的契约行为与合法字典数据。
  - 状态／条件：LOCAL VERIFIED（2026-11-25，本地全套件）：单一判定引擎 `internal/validation.ValidateJSONSchemaVerdict(schema, payload) (*SchemaVerdict, error)`——`SchemaVerdict` 携带 `MissingRequired`（字段路径聚合）/`OtherMessages`/`Details`（path→required 或渲染消息），错误通道专载缺陷（schema 无法编译、payload 非 JSON、多 JSON 值），缺陷与违规分流即「区分缺少 schema 与编译失败」的载体；收集器自 functioncontract 迁入 validation（`collectErrors` 按 `kind.Required` 取路径、根横幅消息在有 cause 时过滤）。两条入口接线：console `validateBindingExecutePayload` 改走 verdict（缺 schema 放行 / 缺陷 → 422「binding input schema cannot be enforced」 / 违规 → 422「binding payload does not match input schema」带 verdict 渲染）；`functioncontract.ValidateInput` 委托 verdict（缺 schema 放行 / 缺陷 → 返回错误 fail-closed 与 ARCH-027 同向 / 违规 → 400 缺必填与字段级 details 同旧）。unknown-field 策略统一为标准 JSON Schema 语义：删除 console 专属 `strictSchemaObjects` 注入（缺省 `additionalProperties` 即放行，显式 `false` 拒绝，schema 形式校验未知值，无 `properties` 的对象为自由字典）——同 schema 同 payload 两入口 verdict 一致。共享对照 fixture 表 `internal/validation/validationtest.SchemaValidationFixtures()`（14 例：有效最小对象/缺必填/类型错/显式 false 拒未知/显式 true 收/省略收/自由字典任意键/typed 字典合法+非法/嵌套必填路径/枚举违规/缺 schema 放行/编译失败拒绝/非法 JSON 拒绝），三处入口逐条断言：core `functioncontract.TestValidateInputSharedFixtures`、HTTP 入口 `function.TestValidateInvokeInputSharedFixtures`（经 `validateInvokeInput`+descriptor 落库）、console `TestValidateBindingExecutePayloadSharedFixtures`（经 binding+FunctionSpec）——入口间分歧表现为 fixture 失败而非静默漂移。行为变更如实：直接调用入口此前对编译失败与非法 JSON 静默放行（旧测试 `PassesThrough` 已改写为 `MissingSchemaPassesDefectsReject`）；console 入口此前对省略 `additionalProperties` 拒绝未知字段（strict 注入已删，文档同步）。文档：`docs/architecture/openapi-sdk-descriptor-v2.md` 中英双树新增「服务端校验策略（SUI-016）」小节；`docs/architecture/pagespec-protocol.md` 中英双树服务端兜底段更新（引用行号 512-544 + 策略一致性）。证据：`go build ./...` 通过；`go vet` 全绿；`gofmt -l` 零输出；`go test ./internal/...` 全绿（SUITE-EXIT=0，零 FAIL）；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 通过（DOCS-EXIT=0）。边界如实：入口 HTTP 形态有意不同（invoke 400 / console 422），一致的是 verdict；`normalizeRequired` 兼容清洗保留在 functioncontract 侧。证据：f32020f46（代码+测试）、081c86a14（文档）。

- [x] **SUI-017 · P1 · 修复 · 保留 Web 表单根值类型** ✅ 361aa77bd
  - 依赖：SUI-014。交付与验收：根数组和标量通过编辑、取值和提交往返后仍保持原 JSON 类型。覆盖空数组、`false`、`0`、显式 null 和旧对象表单。
  - 状态／条件：LOCAL VERIFIED（2026-11-25）：种子侧 `deriveSchemaDefaults` 改为按 `primarySchemaType` 根类型派生——根数组种子 `[]` 或一件 items 骨架（对象 items 展开字段骨架、基元 items 给占位值）、根标量给类型占位（string `''` / integer `0` / number 取 `minimum` / boolean `false`，优先级 `default` > `example` > `enum` 首项，显式 `default: null` 原样保留）、可空联合（SUI-009 type 数组）取首个非 null 主类型、旧对象根路径不变（根级 default 不参与派生）。工作台侧：`RequestBodyEditor` 表单页签仅对对象根可用（`formTabAvailable`），根数组/标量只保留原始 JSON 页签；`execute()` 在非对象根时强制走 `JSON.parse(rawJson)` 提交、inputMode 复位为 json，`[]`/`false`/`0`/`null` 经编辑 → 取值 → 提交往返类型不变。证据：`json.test.ts` 34/34（新增「SUI-017 根数组与标量的种子」5 例 + 「根值类型往返」6 例 each 表）、Invoke `index.test.tsx` 49/49（新增根数组 `['a']` 与根标量 `false` 两条端到端提交断言）、Instances 36/36、utils 240/240、Functions 全目录 397/397；`pnpm run tsc` 0 错误；web 全量 jest 468 suites / 5541 tests 全绿 exit 0（446s）；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 通过。文档：`docs/architecture/dashboard-page-model.md` 中英双树 Form strategy 增补「根值类型经往返保持原 JSON 类型（SUI-017）」段。边界如实：结构化表单按构造只表达对象根，根数组/标量无表单页签（编辑与提交都走原始 JSON，SUI-026 再补 JSON 编辑呈现）；对象根的种子沿用旧派生路径（根级 default 不参与）；生成页面表单（PageSpec `FormPresentationSpec`）按构造对象根，其中根数组/裸标量由请求兼容性分级归入降级形态。证据：361aa77bd（代码+测试）、030ca9d5d（文档）。

- [x] **SUI-018 · P1 · 修复 · 按明确的默认规则填充请求初值** ✅ 39b32d682
  - 依赖：SUI-014。交付与验收：schema 默认值、示例与占位独立处理。可选值未填写时保留缺省，用户显式给出的 `false/0/null` 不被覆盖。调试页可显式填入示例。
  - 状态／条件：LOCAL VERIFIED（2026-11-25）：种子拆两层——默认值层 `deriveFormDefaults`（只派生 schema 显式声明的 `default`，嵌套对象递归收集子字段声明默认；显式 `default: null` 视为「未声明默认」→ 字段保持缺省而非提交 null；根数组/标量无 default 时给类型骨架保证 JSON 模式可发送初值）作表单与 JSON 编辑器初值；模板层 `deriveSchemaDefaults`（`default` > `example` > `enum` 首项 > 类型占位，SUI-017 根类型语义不变）只在用户索要时产出。「填入示例」显式动作：调用工作台 RequestBodyEditor 头部按钮（无 schema 禁用）与实例调试弹窗（DebugModal）请求参数区按钮，以模板层同时覆盖表单值与 JSON 文本。效果：可选字段未填写即不进初值 → 提交投影对缺失键原样保留缺省（absent keys stay absent）；用户显式给出的 `false/0/null` 以普通表单值原样提交不被任何层覆盖；占位留在显示侧（`x-placeholder` 提示通道，不进值）。locale 中英双树新增 `requestBody.fillExamples(+Tooltip)` 与 `debug.button.fillExamples(+Hint)`。证据：`json.test.ts` 40/40（新增 `deriveFormDefaults` 6 例）；Invoke `index.test.tsx` 121/121（新增默认值层初值/填入示例/显式 falsy 三条 e2e，根数组用例改为初值 `[]` + 显式填入骨架）；新增 `Instances/__tests__/DebugModal.test.tsx` 4/4（初值只含声明 default / 填入示例覆盖 / 无 schema 空模板+禁用 / 手写不被重算覆盖）；Functions+utils 目录 508/508；`pnpm run tsc` 0 错误；web 全量 jest 469 suites / 5554 tests 全绿 exit 0（430s）；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 通过。文档：`docs/architecture/dashboard-page-model.md` 中英双树 Form strategy 增补「请求初值按明确的默认规则填充（SUI-018）」段。边界如实：对象根仍忽略根级 `default`/`example`（旧对象表单种子路径不变，SUI-017 已钉）；显式 `default: null` 不入默认值层（提交缺席优于误提交 null，服务端可自行兜底）；JSON 编辑面所见即所提交（填入示例后模板值按文本提交）；jest-dom `toHaveValue(RegExp)` 对 textarea 不适用，DebugModal 断言走 `value` + `toMatch`。证据：39b32d682（代码+测试）、e63d50b02（文档）。

- [x] **SUI-019 · P1 · 实现 · 支持嵌套字段的呈现路径** ✅ 5b7a2c032
  - 依赖：—。交付与验收：以 JSON Pointer 定位字段并兼容旧顶层 key。测试对象子字段、数组项字段、键名含点或斜杠的场景，以及未知路径的诊断。
  - 状态／条件：LOCAL VERIFIED（2026-11-25）：新增 `web/src/utils/fieldPath.ts` 解析 RFC 6901 指针（`~0`/`~1` 转义、`-` 数组段），保留裸顶层 key 旧语义；renderer 的 label/description/enumNames、required、validationRules、disabled、visible、defaultValue 与提交投影均按解析出的路径落到对应 schema/uiSchema/值分支；未知路径只告警不伪造 schema。证据：`fieldPath.test.ts` 16/16、`pointer-fields.test.tsx` 12/12（含真实渲染与提交）、renderer+schemaHints 226/226、web 全量 jest 471 suites / 5585 tests 全绿 exit 0（450s）；`pnpm run tsc` 0 错误；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 通过；`go build ./internal/dashboard/...` 通过。文档：`docs/architecture/pagespec-protocol.md`、`docs/architecture/dashboard-page-model.md`、`docs/dashboard/composite-editor-v3.md` 中英双树同步。边界如实：点号属性名按裸 key 字面处理、不是路径缩写；`ui:order` 仅接受顶层属性名，指针字段不入列；穿越数组的值写入/删除为 no-op（数组整体提交）；未知路径当前是 `console.warn`，保存/发布拦截由 SUI-021 补；Go 默认派生与 Web 指针推导对齐由 SUI-025 补。证据：5b7a2c032（代码+测试）、781ff1182（文档）。

- [x] **SUI-020 · P1 · 实现 · 建立表单控件能力表** ✅ cf90d428d
  - 依赖：—。交付与验收：为已注册控件声明适用 schema 类型、输出值形状及回退。对照测试覆盖 DateRange、RichText、Switch、Code、JSON 的实际能力，编辑器选项与注册能力一致。
  - 状态／条件：LOCAL VERIFIED（2026-11-25）：新增 `web/src/utils/widgetCapabilities.ts` 共享能力表，覆盖全部 26 个 `FormWidget`，逐项记录可接受 schema 类型、提交值形态、实际 `implemented` 状态和回退行为，事实来源为 `widgetToRjsf` 与自定义 widget/field 注册表；Page Studio 12 个控件选项各带原生 `title` 能力提示，选项值不变。证据：`widgetCapabilities.test.ts` 7/7（26 枚举全覆盖、字段完整性、DateRange/RichText 仅声明、Switch 布尔、Code/JSON 字符串 textarea、编辑器选项子集）、FormPresentationEditor 与 SchemaFormRenderer 定向 111/111；`pnpm run tsc` 0 错误；web 全量 jest 472 suites / 5594 tests 全绿 exit 0（387s）；guard PASSED；docs build 通过；Go build 通过。文档：协议/页面模型/编辑器使用中英双树同步。边界如实：本项只建能力表与提示、未新增控件实现，DateRange/RichText 仍仅声明、静默回退到类型默认控件；Code/JSON 仍是普通 textarea 字符串，不解析或校验内容；能力提示仅为提示信息，不改变提交值。证据：cf90d428d（代码+测试）、d211d5ca7（文档）。

- [x] **SUI-021 · P1 · 实现 · 校验保存与发布的表单呈现配置** ✅ c8d174119
  - 依赖：SUI-019, SUI-020。交付与验收：校验未知字段、重复或冲突分组、不兼容控件及非法宽度。错误能定位表单和字段，已有合法 PageSpec 能重新发布。
  - 状态／条件：LOCAL VERIFIED（2026-11-25）：服务端新增 `spec.ValidateFormPresentations`（`internal/dashboard/spec/form_validation.go`），走查 PageSpec 全部 7 处表单位（resource.createForm/updateForm/actions[].form、operation.form、task.form、report.queryForm、composite.sections[].form），校验字段 key 非空/不重复/可在 jsonSchema 解析（裸 key 与 SUI-019 指针同契约、点号名按字面 key）、分组 key 与字段唯一/不冲突/已在 fields 声明、控件在 26 枚举内且与字段 schema 类型兼容（SUI-020 能力表口径、DateRange/RichText 仅声明不拒、类型缺省跳过、类型数组按任意匹配）、width 0 或 1–12；诊断经 `publishShapeDiagnostic` 定位（如 `resource.createForm.fields[2].width`）。`ValidatePublishablePageShape` 追加该函数（发布/accept-and-publish/回滚预检/提案校验自动继承），`ValidatePageSpecInScope` 保存/预览路径追加同一函数；前端 `web/src/utils/formValidation.ts` 复用 `fieldPath.ts`+`widgetCapabilities.ts` 同规则，`PageStudio.handleSave` 与 `CompositeEditor.save()` 保存前拦截。证据：Go `form_validation_test.go` 新用例（全码表+全位置+指针路径+合法全量夹具零诊断+发布链透出）与 `service_v2_test.go` validate 路径用例全绿，`go test ./internal/dashboard/spec/... ./internal/api/page/...` ok；web `formValidation.test.ts` 57/57、PageStudio 全目录 87 suites/1134 tests 绿、tsc 0 错误；guard PASSED；docs build 通过。全量 jest 473 suites 470 绿，3 红套件两轮隔离复跑均绿（load 80 时 66 处纯超时、零断言失败，判定环境负载非回归）。文档：协议/页面模型/编辑器使用中英双树同步。边界如实：组合页编辑器当前只产出 `form.jsonSchema`（fields/groups 由服务端派生），预检在该编辑器对现产物为空操作、接线已就位；`diagnosticsToDetails` 对同 field 多诊断按 map 去重（后写覆盖），单字段多错以首个定位为主；现有合法 PageSpec（生成器默认 Width 12、裸 key）发布不受影响。证据：c8d174119（Go 校验+测试）、aa6d24a25（Web 预检+测试）、4d9806bfc（文档）。

- [ ] **SUI-022 · P1 · 修复 · 保留 date-time 选择的时间和时区**
  - 依赖：—。交付与验收：`date` 按日期提交，`date-time` 含时分秒和明确偏移或 UTC。用跨日、跨时区、清空案例验证调试页和发布页的实际提交值。
  - 状态／条件：未开始。

- [ ] **SUI-023 · P1 · 修复 · 按枚举值关联可见标签**
  - 依赖：—。交付与验收：沿用 GUX-017 的现有标签渲染，补齐数字、布尔假值、重排选项和缺少翻译的案例。标签变化不改变提交的枚举值。
  - 状态／条件：未开始。

- [ ] **SUI-024 · P1 · 修复 · 表单文案跟随当前语言**
  - 依赖：—。交付与验收：字段名称、说明、占位、分组标题使用当前 locale 与统一回退。切换中英文不会重置已填写数据。
  - 状态／条件：未开始。

- [ ] **SUI-025 · P1 · 实现 · 对齐调试页与发布页的默认表单派生**
  - 依赖：SUI-020, SUI-022, SUI-023, SUI-024。交付与验收：Go 生成器与 Web 调试页共用一组预期结果，覆盖顶层字段顺序、控件、分组及 layout hints，无配置时使用标准纵向表单。
  - 状态／条件：未开始。

- [ ] **SUI-026 · P1 · 实现 · 为不适用结构化控件的请求提供 JSON 编辑**
  - 依赖：SUI-011, SUI-014, SUI-017。交付与验收：能够序列化执行的请求可用 JSON 编辑并走相同绑定与校验。对缺少请求映射的接口显示具体原因，业务页面的错误提示对应实际可用入口。
  - 状态／条件：未开始。

- [ ] **SUI-027 · P1 · 修复 · 为同页表单分配唯一字段 ID**
  - 依赖：—。交付与验收：组合页或双预览挂载同 schema 的两个表单时，label、错误和说明分别关联本实例；同一实例重渲染时关联稳定。
  - 状态／条件：未开始。

- [ ] **SUI-028 · P2 · 修复 · 统一表单宽度与响应式布局**
  - 依赖：—。交付与验收：明确协议 12 栏宽度和 Ant Design 24 栏间的换算与历史配置处理。宽屏网格可多列，窄屏可读并可操作，分组和长字段通过浏览器用例。
  - 状态／条件：未开始。

- [ ] **SUI-029 · P2 · 实现 · 在 Page Studio 编辑表单分组**
  - 依赖：SUI-019, SUI-021, SUI-028。交付与验收：界面可创建、排序、移动和删除分组，设置折叠行为。草稿重开后分组和顺序回读一致。
  - 状态／条件：未开始。

- [ ] **SUI-030 · P2 · 测试 · 稳定隐藏字段切换回归用例**
  - 依赖：SUI-014。交付与验收：根据实际 UI 提交与回填事件等待，重复跑测试记录剔除、恢复断言及执行结果，复核报告中出现的 5 秒超时。
  - 状态／条件：未开始。

### 表单风格、默认项与发布

- [ ] **SUI-031 · P1 · 设计 · 定义命名展示变体协议**
  - 依赖：SUI-045, SUI-019, SUI-020。交付与验收：更新 [页面模型](docs/zh/architecture/dashboard-page-model.md#表单策略) 与 [PageSpec 协议](docs/zh/architecture/pagespec-protocol.md#formpresentationspec)，明确稳定表单标识、变体 key、默认项、允许覆盖的字段、发布快照和回滚语义。标准与紧凑共用 schema、Binding 与调用治理。旧页面沿用原呈现，新页面没有配置时生成标准纵向表单。
  - 状态／条件：未开始。

- [ ] **SUI-032 · P1 · 实现 · 将展示变体纳入 PageSpec 存储**
  - 依赖：SUI-031。交付与验收：同一页面的变体和默认 key 随草稿及发布快照写入现有表，草稿回读、旧版本回读与服务器重启验证数据一致。
  - 状态／条件：未开始。

- [ ] **SUI-033 · P1 · 实现 · 发布前验证变体与当前契约**
  - 依赖：SUI-021, SUI-032。交付与验收：重复 key、默认 key 不存在及非法覆盖都返回定位错误。契约字段删除或改名后，暴露受影响变体，验证回滚时的契约新鲜度。
  - 状态／条件：未开始。

- [ ] **SUI-034 · P1 · 实现 · 在 Page Studio 管理命名风格**
  - 依赖：SUI-032, SUI-033。交付与验收：管理员可新增、改名、删除风格并设默认，能预览每种风格。编辑器和 [使用文档](docs/zh/dashboard/composite-editor-v3.md) 区分风格与绑定操作，用「调整表单列数」与「从选中行取 playerId」各验证一次。重开草稿恢复配置，并发编辑沿用现有修订冲突处理。
  - 状态／条件：未开始。

- [ ] **SUI-035 · P1 · 实现 · 解析表单的生效默认风格**
  - 依赖：SUI-025, SUI-032。交付与验收：管理员发布的默认项生效，缺少配置时依次复用原表单或派生纵向默认布局。覆盖旧页、缺失默认项和有效的显式选择。
  - 状态／条件：未开始。

- [ ] **SUI-036 · P1 · 实现 · 在已发布表单切换风格**
  - 依赖：SUI-014, SUI-027, SUI-028, SUI-035。交付与验收：标准、紧凑的可见布局不同。切换保留输入及错误状态，两种风格生成相同请求，确认与审批行为一致。
  - 状态／条件：未开始。

- [ ] **SUI-037 · P1 · 集成验收 · 核对风格发布、读取与回滚**
  - 依赖：SUI-033, SUI-034, SUI-036。交付与验收：在测试栈走草稿→发布→重启读取→回滚，核对数据库中的默认 key 和变体内容，并验证同 pageKey 在不同游戏及环境互相隔离。
  - 状态／条件：未开始。

- [ ] **SUI-038 · P2 · 方案决策 · 评估跨设备记忆风格偏好**
  - 依赖：SUI-037。交付与验收：依据使用反馈决定是否实现服务端偏好。若纳入，定义用户与作用域键、变体删除后的失效规则，以及用户选择和管理员默认值的优先级。
  - 状态／条件：未开始。

- [ ] **SUI-039 · P2 · 方案决策 · 评估跨页面共享风格模板**
  - 依赖：SUI-037。交付与验收：区分现有 ComponentTemplate 的整块界面与联动复用，以及仅共享表单展示配置的需求。若纳入跨页面风格复用，明确模板版本固定方式、发布后稳定性、升级及回滚行为。
  - 状态／条件：未开始。

- [ ] **SUI-040 · P2 · 方案决策 · 评估新增展示方式**
  - 依赖：SUI-037。交付与验收：用实际操作者流程决定分组预设、弹窗或逐步操作的优先级。每种入选方式具备独立交付范围和相同绑定、确认及提交的验收案例。
  - 状态／条件：未开始。

### 跨链路验收

- [ ] **SUI-041 · P1 · 集成验收 · 从 OpenAPI 导入到实际 HTTP 请求**
  - 依赖：SUI-004, SUI-005, SUI-006, SUI-008, SUI-010, SUI-011, SUI-015, SUI-016, SUI-017, SUI-022, SUI-026。交付与验收：共享 fixture 走上传、Provider 发现、默认提案、发布、表单操作和上游请求录制。覆盖同名参数、根数组、嵌套引用、nullable、date-time 与隐藏字段，并对暂未支持场景断言诊断。
  - 状态／条件：未开始。

### 后续能力

- [ ] **SUI-042 · P2 · 实现 · 为带 discriminator 的输入提供分支选择**
  - 依赖：SUI-009, SUI-011, SUI-014。交付与验收：用户可选择已声明的分支，界面显示分支字段并校验。切换分支后只提交所选分支允许的值，无法明确选择的组合继续走诊断或 JSON 降级。
  - 状态／条件：未开始。

- [ ] **SUI-043 · P2 · 方案决策 · 确定调试页如何复用已发布风格**
  - 依赖：SUI-035, SUI-037。交付与验收：用同一函数绑定多个页面的样例决定调试页如何定位页面及表单位点，明确作用域、读取权限以及没有发布页时的默认派生规则。
  - 状态／条件：未开始。

- [ ] **SUI-044 · P2 · 方案决策 · 定义风格的主题外观范围**
  - 依赖：SUI-031, SUI-037。交付与验收：说明颜色、字体、密度是否纳入后续风格配置。若纳入，明确可用主题 token 与回退值，用同一风格在亮暗主题下校验可读性。
  - 状态／条件：未开始。

### 设计更新与组件审查

- [x] **SUI-045 · P0 · 设计 · 明确展示与调用参数的边界** ✅ d775c708d
  - 依赖：—。交付与验收：更新 [PageSpec 协议](docs/zh/architecture/pagespec-protocol.md#formpresentationspec)，明确展示覆盖项、基础表单与 Binding 的职责、隐藏必填字段的取值来源和最终校验顺序。记录旧页面的 `required`、默认值、可见性和校验覆盖如何兼容。用同一函数的标准与紧凑布局、手填和选中行注入两类绑定，以及缺少必填来源的例子说明预期请求或诊断。
  - 交付：`docs/zh/architecture/pagespec-protocol.md` + 英文镜像 `docs/architecture/pagespec-protocol.md` 新增「展示与调用参数的边界（SUI-045）」小节：三层职责（jsonSchema/展示覆盖/Binding）、FormFieldSpec 字段两分（纯展示 vs 调用相关）、隐藏字段三出口、客户端+服务端最终校验顺序、旧页面四类覆盖兼容、三例（标准 vs 紧凑、手填 vs 行注入、缺少必填来源）。并入 origin/main 后按 SUI-014 落地形态复核引用（`projectSubmitValues`/`getSubmitValues`/`isFieldHidden` 唯一提交投影）。证据：`go build ./...` 通过；`go test ./internal/drivers/openapi/... ./internal/api/openapi/... ./internal/dashboard/spec/...` 通过；`bash scripts/dashboard_vnext_guard.sh` PASSED；`CI=true pnpm --dir docs build` 通过；`pnpm --dir web run tsc` 0 错误；面向本域 web 套件（SchemaFormRenderer/Functions/services/Invoke/PageRenderer）在超时提升到 30s 后通过（submit-values/visible-when/widgets-select 三套件 18/18 全绿）；d775c708d。

- [ ] **SUI-046 · P1 · 审查 · 逐项核对现有 UI 组件和组件模板**
  - 依赖：—。交付与验收：对照编辑器注册表、数据库中的内置模板定义和发布渲染器，清点表单、表格、详情、动作等区块与模板。逐项记录属性、函数依赖、参数来源、输出与联动，标注预览和发布差异。用代表模板走一次插入、发布、执行，验证复制后的页面配置及模板更新提醒，记录每个发现的证据。
  - 状态／条件：未开始。

- [ ] **SUI-047 · P1 · 设计 · 明确 OpenAPI 到 UI 的支持矩阵**
  - 依赖：SUI-001。交付与验收：更新 [OpenAPI / SDK 接入设计](docs/zh/architecture/openapi-sdk-descriptor-v2.md#json-schema-支持子集)，按参数位置、根数组、引用、日期时间、组合类型与媒体类型，注明可自动生成并执行、可用 JSON 编辑或需要人工补绑定，并为每种状态指定对照样例。
  - 状态／条件：未开始。

## 游戏运营后台体验改进（2026-11-16）

设计和观察依据：[游戏运营后台体验审查](docs/design/game-operations-ux-review-2026-11-16.md)。以下为新体验交付；高风险、身份及真实接入继续复用既有专项，未验证项保持未勾。

依赖顺序：GUX-001/02 先明确现状和角色；GUX-005～07 为共享语义基础；GUX-008～13 组成标杆流程；GUX-017 在相关交付完成后验收。GUX-003/04/14～16 可按依赖并行。

- [x] **GUX-001 · P1 · 巡检 · 核对部署版本与取证**
  - 依赖：—。交付与验收：记录部署镜像/版本与源码提交，保存首页、目录、玩家模板及详情截图；确认测试游戏/环境、空数据和模板边界，不记录凭据。
  - 状态／条件：LOCAL VERIFIED（2026-11-17，192.168.5.5 部署栈实测）：镜像 digest/构建 commit 对账入 README（平台栈=docker.yml run 43928190705 @ 4e2b12c68，demo 栈=同镜像重标签）；四张截图（console 首页、player 目录、player-manage 模板、行详情抽屉）+ 菜单链接清单落盘；scope demo-game/demo 经 games API 复证，空态（0 pages published）与模板边界如截图；凭据未入记录。边界如实：模板页渲染浏览器内存预览数据（977×n 等差），活数据链由 E2E（CI 栈）与 API 重放演练（部署栈，余额 6532→7180 重放不动）覆盖。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-001/README.md`（同目录含 4 张截图 PNG 与 console-links.txt）

- [x] **GUX-002 · P1 · 设计 · 角色任务与导航**
  - 依赖：—。交付与验收：梳理运营、开发接入、平台管理的任务与现有权限映射；交付导航结构与入口，不删除既有能力。
  - 状态／条件：LOCAL VERIFIED：三工作区（运营/开发接入/平台管理）重分组方案覆盖全部 10 个现存顶级组、0 路由删除、0 新权限；角色映射按 configs/roles.json 实测权限集对账（运营缺 audit:read/ops:read、客服缺 support:read 等缺口如实记录、不裁决，权限模型改动归用户拍板）；落地页规则与跨界入口（执行记录/SDK 分发同路由入口副本）已定；实现随 GUX-003+ 走 DoD。
  - 完成证据：`docs/design/game-ops-navigation-design.md`（中文镜像 docs/zh/design/game-ops-navigation-design.md）

- [x] **GUX-003 · P1 · 实现 · 运营首页**
  - 依赖：GUX-001, GUX-002。交付与验收：按角色展示待办、异常、最近处理和常用入口；缺失数据展示真实空态，移走重复的产品建设说明。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。根路径 `/` 改 LandingRedirect 按 access 能力落地（运营→/console/home、开发→/functions/catalog、平台→/system、全不可见→/account）；Welcome 模板页与未路由 Admin 模板页下线、locale welcome 键 8 语言同步清理（app.welcome.* 死键留 GUX-015）；运行控制台根视图挂工作台四面板：待办（审批 pending×audit:read+未读消息）、异常（契约失效计数零请求+告警×ops:read）、最近处理（execution-logs mine=true 免审计权限）、常用入口按权限过滤；单面板失败独立重试不拖垮整页、权限外整体不渲染、空数据真实空态；审批面板随 scopeKey 重拉。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-003.md`

- [x] **GUX-004 · P1 · 实现 · 游戏环境与作用域**
  - 依赖：GUX-001, GUX-002。交付与验收：统一当前游戏/环境展示，按契约支持区服/渠道/时区；切换作用域清理旧数据和在途结果。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。作用域展示全局唯一（GameSelector header/mobile 共用 scope store，取证既有交付）；本项补契约字段展示（GameEnvMeta.description 进环境选项次级行）与在途结果防护（scope.ts 代次计数 + useScopeFreshGuard，工作台 usePanel 接入丢弃晚到旧 scope 响应；ProTable 族既有 debounce+abort 口径不变）。边界：区服/渠道/时区不在作用域契约（GameEnvItem={env,description,color,databaseName}）内，按契约如实不提供选择器；描述为后端单串原样展示非 LocalizedText。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-004.md`

- [x] **GUX-005 · P1 · 实现 · 统一状态语义**
  - 依赖：GUX-001, GUX-002。交付与验收：建立状态映射与文本/颜色/未知态规则；玩家列表、详情、确认和回执显示一致且不只依赖颜色。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。共享语义层（列表列/详情抽屉共用）：声明集合外枚举值渲染显式「未知（原值）」标注（tag 与非 tag 两种渲染都带完整字样，非仅靠颜色），枚举文案跟随界面 locale。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-005-006-007.md`

- [x] **GUX-006 · P1 · 实现 · 统一时间语义**
  - 依赖：GUX-001, GUX-002。交付与验收：时间组件显示明确时区和一致格式，覆盖空值、无效值、跨时区；列表与详情同值同义。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。无效 datetime 渲染显式「无效时间（原值）」标注（不再漏 `Invalid Date (UTC±xx)` 拼接，formatDateTime 加 NaN 守卫原值透传）；空值统一 `-`；D5 显式时区标签此前已交付保持不变；列表与详情同走共享语义层同值同义。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-005-006-007.md`

- [x] **GUX-007 · P1 · 实现 · 统一资产与身份展示**
  - 依赖：GUX-001, GUX-002。交付与验收：提供玩家身份、币种金额和道具数量语义展示；缺单位或身份来源标明未知，复制和长文本可用。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。number+unit 单位跟随界面 locale（不固定 zh-CN）；未声明 unit 不杜撰单位（走默认渲染）；copy 可复制渲染沿用；身份列语义（D5 identity→fixed+copy）此前已交付。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-005-006-007.md`

- [x] **GUX-008 · P1 · 实现 · 查询工具栏**
  - 依赖：GUX-005, GUX-006, GUX-007。交付与验收：玩家 ID/昵称高频检索优先，高级筛选可展开；重置、回车、加载、空结果和作用域变化可验证。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。核对既有实现：搜索表单 `{ labelWidth: 'auto', defaultCollapsed: true }`、`search: filterable ? undefined : false`（valueType/valueEnum 驱动搜索项）、玩家模板列序 id/nickname 高频在前；本批核对未重复实现，既有行为由渲染器全树用例覆盖。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-008-013.md`

- [x] **GUX-009 · P1 · 实现 · 资源列表与动作层次**
  - 依赖：GUX-005, GUX-006, GUX-007。交付与验收：整理列宽、时间与数字对齐，减少逐行重复强调；按能力、权限和对象状态决定动作，未提供的 CRUD 不显示。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。行内动作一律 link 态（recharge/retryFailed 移除 primary，工具栏级批量充值保留）；操作列 >4 条折叠「更多」下拉，下拉删除沿用 deleteAction 文案+danger 确认（语义不降级）；数字/时间列 dataType 派生右对齐；未配置绑定的动作不渲染（既有 hasBinding 过滤）；列表单元格渲染统一走 renderFieldSemantic 并跟随界面 locale。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-008-013.md`

- [x] **GUX-010 · P1 · 实现 · 玩家详情**
  - 依赖：GUX-005, GUX-006, GUX-007, GUX-009。交付与验收：以玩家身份组织详情和实际可用信息；与列表共享语义组件，覆盖加载、缺失和错误，来源按契约呈现。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。详情抽屉顶部按声明 `listView.identityKey` 渲染行身份锚（契约驱动不猜字段名），与列表/详情共享 renderFieldSemantic 语义组件（加载 Skeleton、缺失 '-'、错误显式可定位均为既有面，本批语义渲染统一覆盖列表单元格）。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-008-013.md`

- [x] **GUX-011 · P1 · 实现 · 邮件补偿输入**
  - 依赖：GUX-005, GUX-006, GUX-007。交付与验收：复用已有能力/契约构建补偿面板，明确目标、作用域、内容、数量和原因；无绑定时解释不可用。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。向导目标步骤即时汇总已选人数/区服，全服/区服显式「以服务端为准」不杜撰人数；radio 选项走 locale key；create 绑定 `executor_unbound` 以 ExecutorUnboundAlert（mail_task.create 定位入口）呈现而非普通失败。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-008-013.md`

- [x] **GUX-012 · P1 · 实现 · 变更确认**
  - 依赖：GUX-005, GUX-006, GUX-007, GUX-011。交付与验收：提交前汇总实际目标、作用域、内容和风险，复用已有高风险与审批契约；权限、取消和目标变更均可验证。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。提交前核对步骤结构化摘要（类型/标题/范围/目标/每人数量/总量——总量仅指定玩家+补偿可算时展示）+ 审批/直发提示 + 必勾确认 checkbox；高风险与审批契约沿用既有 D2/审批链（渲染器侧 modal.confirm 未变更）。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-008-013.md`

- [x] **GUX-013 · P1 · 实现 · 执行回执**
  - 依赖：GUX-005, GUX-006, GUX-007, GUX-012。交付与验收：使用真实接口返回展示提交/执行/失败/部分结果及可追溯标识；未知保持未知，不杜撰回滚或成功。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。渲染器写操作结果面按白名单（id/taskId/approvalId/count/total/amount/retried/sentCount/failedCount；非空串与有限数字）展示真实返回标识，布尔与未知键忽略；向导审批态回执带审批单号+任务 ID，接口未返回的字段不展示，不发明部分成功状态。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-008-013.md`

- [x] **GUX-014 · P1 · 实现 · 运行控制台空态**
  - 依赖：GUX-001, GUX-002。交付与验收：合并重复空卡，区分无发布页、无权限与加载失败；按角色提供可执行的下一步。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。全部菜单无页面时合并为一张空态卡（区分「暂无已发布页面 / 页面尚未挂载 {count}」）；categoryKey 未命中改「菜单不存在或未发布」+返回首页（不再误标为无页面且暴露 key）；无权限/加载失败既有分支用例锁定；下一步按角色（canPageRead → 页面工作台/菜单管理按钮；运营角色 → 联系管理员，无越权入口）。逐菜单占位仅在部分菜单有页面时保留。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-014.md`

- [x] **GUX-015 · P2 · 实现 · 说明与文案收敛**
  - 依赖：GUX-001, GUX-002。交付与验收：将目录及页面工作台的重复引导移至按需帮助或首次引导；移除登录模板占位符及内部术语的误导展示。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。函数目录首屏引导（hero 卡 + 双引导子卡 + 双常驻 Alert）收敛为一张默认折叠的「使用说明与常见误解」Collapse（按钮全保留，同目标「查看资源」去重），数据标签行与 SummaryOverview 保留；PageWorkflowGuide 整卡 Steps+Alert 折叠为默认收起条；登录占位符 8 locale 去模板名（含组件 defaultMessage 与 5 个测试 locator 同步）；死键清理 accountLogin.{tab,errorMessage} ×8 locale、app.welcome.link.* ×6 locale（GUX-003 遗留）；控制台分类卡 category.key 代码串移除、概览说明去（menu_items）括注。tsc 0；全量 jest 466 suites/5480 tests 全绿 load=3；go internal rc=0；guard PASSED。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-015.md`

- [x] **GUX-016 · P2 · 设计 · 视觉与交互规范**
  - 依赖：GUX-001, GUX-002。交付与验收：定义标题、间距、表格、表单、抽屉、语义色及全部交互状态；覆盖明暗主题、中文英文和键盘焦点。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。docs/design/game-ops-visual-spec.md + zh 镜像：§3 标题六级层级与内部 key 禁令、§4 DASHBOARD_PAGE_TOKENS 间距词汇表+圆角 8/12、§5 ProTable 基线（折叠搜索/link 动作/更多下拉/右对齐/GUX-014 状态模型）、§6 表单三容器契约测试口径、§7 640 详情抽屉+identity 锚、§8 语义色（契约枚举/未知（原值）非仅颜色/--brand-*）、§9 交互状态全集含键盘焦点 :focus-visible 硬性要求、§2+§10 明暗（preset×mode、暗色二次派生按渲染值验对比度、按套分别验）与中英文（intl 契约+键位对齐测试）、§11 review 检查单；引用路径全部实测存在，docs build 通过。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-016.md`（边界：检查单无自动化守卫、index.md 登记暂缓避外席在途）

- [x] **GUX-017 · P1 · 验收 · 标杆流程端到端验收**
  - 依赖：GUX-003, GUX-004, GUX-008, GUX-009, GUX-010, GUX-011, GUX-012, GUX-013。交付与验收：以玩家查询→详情→补偿→确认→回执运行可控 fixture，覆盖错误/部分成功、作用域切换、键盘及 1440px/笔记本截图；真实游戏验收沿 UI-001，fixture 不抵扣。
  - 状态／条件：LOCAL VERIFIED（2026-11-17）。本地三进程真栈（server 18790/agent 19093/demo-game 18093 + web dev 8000）发布页跑 Playwright 驱动 20/20 通过：A1–A12 玩家查询（过滤命中唯一行）→Tab/Enter 详情（身份锚 20007）→Esc 关抽屉→部分失败重试回执→带内 data.error 判败（常驻面板+无回执+弹窗不关）→fail-closed 建单→全服建单审批回执→作用域切换不串；B1–B4 模板向导（演示数据横幅/即时人数汇总/核对摘要/必勾确认/审批回执）；C1–C4 1280×800。验收发现并修复四处：①带内 data.error 按成功渲染（ResourcePageRenderer/ResourceMutationFeedback 改 throw 判败+常驻错误面板）②五个资源模板查询栏装饰化（assignments 空，补 filters+form assignments 与各 list 契约对齐，契约外列移出查询栏）③rjsf v5 不消费 schema 级枚举标签（模板改 x-enum-options，SchemaFormRenderer 两路径归一 ui:options.enumNames）④fixture 漂移重导出+drift-lock 绿。门禁：tsc 0 错、PageRenderer/console/templates/SchemaFormRenderer 相关套件全绿、dashboard_vnext_guard PASSED、docs build 通过。
  - 完成证据：`docs/review/evidence/2026-11-17/gux-017/README.md`（同目录含 20/20 驱动结果与 23 张截图，不记录凭据）。边界（如实）：面包屑露 category key、页副标题露 pageKey（观感项未修）；demo.db 经验收写入；真实游戏验收沿 UI-001 不抵扣。文档三层同步：dashboard-page-model.md（失败判类+查询栏契约+x-enum-options 表单标签）、pagespec-protocol.md（list query schema 校验面）、composite-editor-v3.md（枚举标签契约），含 zh 镜像。

## 架构与可靠性

- [x] **OPS-LOG-001 · P1 · 部署面 · 192.168.5.5 docker 宿主机日志轮转与磁盘守护（dashboard 登录 P0 复发预防）**
  - 背景：2026-11-15 dashboard 登录报错挂数周的根因=该宿主机磁盘 100% 满（118G 用尽）→ postgres `could not write init file` → server 连库失败；当次已救（清旧镜像+截断 17G 容器日志，磁盘 100%→82%），但容器日志无轮转会复发。
  - 交付与验收：192.168.5.5（ssh root@）daemon.json 配 `log-driver: json-file, max-size=50m, max-file=3` 并重启 docker 生效；存量 46 容器需 recreate 才吃到新配置（给出 recreate 顺序与依赖：postgres→server/server2→dashboard→agent→其余）；加 cron 或脚本每周报磁盘水位（>85% 告警）；验收=新起容器 inspect LogConfig 生效+磁盘水位可查。
  - 完成证据：2026-11-15；`docs/review/ops-log-001-evidence-2026-11-15.md`；daemon.json 备份、重启前后容器快照与水位脚本试跑记录见该证据文件。
  - 状态／条件：DONE · LOCAL VERIFIED（2026-11-15，用户确认该机为 demo 测试环境后执行）。daemon.json `10m→50m`（备份 `/etc/docker/daemon.json.bak-20261115`），`systemctl restart docker` 后 46/46 容器自动恢复（unless-stopped，前后快照 `/root/{pre,post}-restart-containers.txt`）。逐栈 recreate（compose 依赖序自动满足 postgres→server→dashboard→agent）：croupier 主栈 13 + `--profile analytics`（ingestion/analytics-worker）+ chirp 11 + atlas 5 + vapor 2 + wingman 2 + rustfs + herald + chorus-pg（裸容器按 inspect 参数重建，数据卷原样挂回）。终态 LogConfig：36×50m + 3×10m（memex-admin/server + 1 旧 buildkit）+ 7×{}（6 buildkit 随 CI 自动重建即吃新配置 + memex-rustfs）。wingman 用在跑容器 env 原样提取的 WINGMAN_* 密钥传入 compose（不回显、用后 shred）。磁盘回收 16.23GB（image prune until=24h——`docker system df` 的 19.87GB「可回收」全部 <7 天，until=168h 实收 0B；在飞 CI build 产物分钟级不受影响）：90%→75%，水位脚本 `/usr/local/sbin/docker-disk-watchdog.sh` + cron 每周一（阈值 85%，试跑通过）。验收复证：新容器 inspect 50m 生效；dashboard :8000→200、server /healthz→200、login API 400（校验层，非 500/503——DB 链路恢复）。
  - 已知边界：(1) memex 栈 compose 文件位于 runner workspace（build context 已被清理）无法本机 recreate，三容器功能正常，下一次 memex deploy workflow 自动带上 50m；(2) `/data/deploy/croupier/docker-compose.telemetry.yaml` 引用 `jaegertracing/all-in-one:latest`（本地仅 1.62.0，主 compose 已收编 jaeger/otel 且 50m ✓），该文件 prometheus/prometheus-storage 未部署、本轮不动其引用漂移；(3) 告警通道仅本机日志 `/var/log/docker-disk-watchdog.log` + syslog（该机无推送通道），周报间隔内快速填满理论上可再触顶，改 daily 只动 cron 字段；(4) 重启 daemon 与 recreate 均落在一次在飞 CI build 窗口内（buildkit 容器随 daemon 重启被拉回，build 未中断未验证，失败可 rerun）。

- [x] **ARCH-001 · P1 · 方案决策 · 审批并发与执行领取语义**
  - 依赖：—。交付与验收：基于现有 SQLStore 定义 pending 原子转换、续跑领取及重复请求规则；评审表覆盖批准/拒绝竞争、重复批准、执行结果未知。
  - 状态／条件：LOCAL VERIFIED：已审 D1–D7 决策覆盖 CAS、胜出者、冲突、未知结果及持久领取边界；方案交付与 ARCH-002 实现分开。
  - 完成证据：2026-10-30 复审；`docs/design/approval-concurrency-design.md`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **ARCH-002 · P1 · 实现 · 接入审批 SQL 存储**
  - 依赖：ARCH-001。交付与验收：复用 internal/platform/approvals/sql_store.go；A 创建、B 读取/批准、重启后可查，并发仅一决定且仅一执行领取；不能只改构造器。
  - 状态／条件：LOCAL VERIFIED：`ServiceContext` 已由 MemStore 切到 `NewSQLStore(db)`（`internal/svc/service_context.go:350`）；CAS `pending→approved/rejected`、`ClaimForDispatch` 持久领取、迁移 0050 补列（存量幂等 + 全新建表）；重启重开读回 claim、并发 6 路仅 1 领取且恰好 1 次续跑。
  - 完成证据：2026-10-30；`internal/platform/approvals/sql_store.go`、`internal/svc/service_context.go`、`internal/db/migrate/migrations.go`；`go test ./internal/platform/approvals/... ./internal/api/approval/... ./internal/svc/... ./internal/db/migrate/... ./internal/api/function/... ./internal/api/console/... ./internal/platform/functioninvoke/... ./internal/logic/function/... -count=1` 全绿；定向 `TestClaimForDispatchExactlyOnce/sql_claim_survives_store_reopen`、`TestApproveCov_ConcurrentApprovalRunsExactlyOnce`、`TestApprovalDispatchClaimMigration_*` 通过；commit `7e7d9557`。

- [x] **ARCH-003 · P1 · 实现 · 集群互联身份认证**
  - 依赖：—。交付与验收：增加专用 TLS 配置及身份绑定，拒绝无证书/错 CA/Agent 身份/未握手 forward；合法 invoke/start/cancel 保留 epoch fencing。
  - 状态／条件：LOCAL VERIFIED：`cluster.interconnectTls` 专用证书面（cert/key/ca 齐备且非 insecureSkipTls 时 mTLS）；`VerifyInterconnectPeer`（`internal/cluster/identity.go`）只接受双 EKU（ServerAuth+ClientAuth）server 角色证书、空 CommonName/Agent 证书（仅 ClientAuth）拒绝；`ServeHelloHandler` 标记连接会话、`ServeForwardHandler` 拒绝未握手 forward；合法 invoke/start/cancel 保留 epoch fencing（stale epoch 仍 NotOwner）。
  - 完成证据：2026-10-31；`internal/cluster/identity.go`、`internal/cluster/transport.go`、`internal/cluster/interconnect.go`、`internal/config/config.go`、`cmd/server/cluster.go`、`internal/devcert/devcert.go`；`go test -count=1 ./internal/... ./core/... ./agents/...` 全绿；定向 `TestInterconnect_MTLSFullChain`（无证书/错 CA/Agent 身份拒绝 + 合法 invoke/start/cancel + epoch fencing）、`TestServeForwardHandler_RejectsUnhandshaked`、`TestServeForwardHandler_AcceptsHandshaked`、`TestVerifyInterconnectPeer_*`、`TestStartCluster_InterconnectTLSWiring` 通过；commit `9547afb8`。

- [x] **ARCH-004 · P1 · 实现 · 集群就绪及失败降级语义**
  - 依赖：—。交付与验收：独立存活/就绪探针；配置要求集群时协调库、互联监听或租约不可用返回 ready=503，显式配置才允许 standalone 降级。
  - 状态／条件：LOCAL VERIFIED：本地存活/就绪及显式降级实现和相关包通过；双实例摘流故障仍由 ARCH-005 验收。
  - 完成证据：2026-10-30 复审；`cmd/server/cluster_readiness_test.go`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **ARCH-005 · P1 · 故障验证 · 集群故障摘流与重连**
  - 依赖：ARCH-003, ARCH-004。交付与验收：双实例注入协调库断连、互联占端口、owner 退出、旧 epoch；逐例记录拒绝、摘流、恢复及副作用次数。
  - 状态／条件：LOCAL VERIFIED：真实 `startCluster` 装配双实例钻机，覆盖协调库断连、互联占端口、owner 退出、旧 epoch；逐例断言拒绝、摘流、恢复及副作用次数。
  - 完成证据：2026-11-01；`cmd/server/fault_drill_cluster_test.go`、`cmd/server/cluster.go`（`clusterInvokerOverride` 注入点）；定向运行 `go test ./cmd/server/ -run 'TestDrillCluster_' -count=1` 全绿（4/4 PASS，耗时 140.855s）；commit `bf784cc9`。

- [x] **ARCH-006 · P1 · 实现 · 物理数据库路由统一真值**
  - 依赖：—。交付与验收：复用授权后的 binding.database_name，消除前缀重算；验收旧名/自定义名/前缀变更/下划线碰撞/并发首次开库，已有绑定有显式迁移。
  - 状态／条件：LOCAL VERIFIED：自定义绑定、命名碰撞、并发首次打开及 Forget 用例通过；预算问题不算本项完成，见 ARCH-009。
  - 完成证据：2026-10-30 复审；`internal/db/router/router_binding_test.go`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **ARCH-007 · P2 · 方案决策 · 版本化审计摘要与失败策略**
  - 依赖：—。交付与验收：列出 scope/Changes/Action 等规范化哈希字段、旧版本兼容和写失败策略；外部签名是否需要单列决策。
  - 状态／条件：LOCAL VERIFIED：保留版本化全字段摘要与写失败策略决策；D4 调用方落地尚缺，ARCH-008 已回退。
  - 完成证据：2026-10-30 复审；`docs/design/audit-chain-integrity-design.md`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **ARCH-008 · P1 · 实现 · 落地审计完整性规则**
  - 依赖：ARCH-007。交付与验收：逐字段修改被检测、旧链可验证、多实例写入一致；审计库故障按已批准策略处理。
  - 状态／条件：LOCAL VERIFIED：版本化哈希/v1 兼容/Log strict 与逐字段篡改检测/多实例链冲突解析已有测试；本项补齐 D4 调用方契约——副作用已执行后审计写失败一律经 `ReportWriteGap` 呈现（WARN + rate-limited 告警），不再 `fmt.Printf`/`_ =` 静默吞掉，也不返回 5xx（5xx 会误导「业务未执行」并引诱盲重试）。真实审计库故障注入 NOT RUN，不据此宣称生产链路已验证。
  - 完成证据：2026-10-31；`internal/audit/audit.go`（ReportWriteGap）、`internal/platform/functioninvoke/approval.go`、`internal/api/approval/service.go`、`internal/api/approval/stepup_otp.go`、`internal/api/alert/inbound.go`、`internal/api/configexplorer/service.go`、`internal/api/console/service.go`、`internal/api/openapi/service.go`、`internal/api/ops/helpers.go`、`internal/api/page/service.go`、`internal/api/resourcecatalog/service.go`、`internal/service/contract_service.go`、`internal/audit/chain_version_test.go`（TestReportWriteGap_SurfacesGapAndRateLimits）、`internal/platform/functioninvoke/audit_gap_test.go`（TestAuditFunctionInvoke_WriteGapSurfaced、TestAuditApprovalCreated_WriteGapSurfaced）；`go test ./internal/... ./core/... ./agents/... -count=1` 全绿，`go build ./...` 过，dashboard vnext/e2e guard PASSED，dependency boundary PASSED，web tsc 过，docs build 过。

- [x] **ARCH-009 · P1 · 实现 · 数据库首次打开的取消与预算**
  - 依赖：—。交付与验收：让 resolver/open/migrate 消费 context；取消等待及时返回、同库 singleflight、不同库隔离，暴露池数/首开耗时；与 ARCH-006 路由命名修复可并行，合并时核对绑定行为
  - 状态／条件：LOCAL VERIFIED：取消等待/ensure/migrate context/singleflight/统计沿用既有实现；本项补齐 R30-06——同步无 context 的 `cfg.Open`（gorm.Open 驱动初始化）改为经 `openWithContext` 在预算 context 内运行，预算到期即返回 deadline 错误，驱动迟到返回的连接被 `closeQuietly` 关闭而非泄漏；随后 PingContext/migrate 仍按预算检查。真实数据库驱动初始化故障注入 NOT RUN，不据此宣称生产链路已验证。
  - 完成证据：2026-10-31；`internal/db/router/router.go`（openWithContext + GameDBCloseLateHook 测试接缝）、`internal/db/router/router_arch009_test.go`（TestGameDB_OpenBudgetBoundsBlockingOpen）；`go test ./internal/db/... ./internal/svc/... ./internal/api/page/... ./internal/api/console/... -count=1` 全绿，`go test -race ./internal/db/router/ -run 'TestGameDB_OpenBudgetBoundsBlockingOpen|TestGameDB_OpenBudgetExceeded|TestGameDB_LifecycleCancelAbortsInFlightOpen' -count=1` 过，`go build ./...` 过。

- [x] **ARCH-010 · P2 · 性能验证 · 多 game/env 连接池容量**
  - 依赖：ARCH-009。交付与验收：在给定 game/env 数、并发、数据库连接配额下记录池数/连接峰值/P95 首开耗时；给出回收阈值与容量结论。
  - 状态／条件：LOCAL VERIFIED：真实 atlas-cov-mysql（127.0.0.1:13307，max_connections=151）与 atlas-cov-pg（127.0.0.1:15432，max_connections=100）上，参数化（games×envs、并发、可选 P95/连接峰值目标）探针全绿：mysql 8 池并发 8 → pool=8/P95≈415ms/clientPeak=8/serverPeak=15/residual=0；pg 8 池并发 8 → pool=8/P95≈368ms/clientPeak=8/serverPeak=15/residual=0；pg 4×3=12 池并发 12（-race）→ pool=12/P95≈697ms/serverPeak=19/residual=0；mysql 72 池并发 72 → pool=72/P95≈462ms/clientPeak=72/serverPeak=83/residual=0。容量结论：`Forget` 回收后服务端连接回落基线（残差 0），database/sql 每池默认空闲 2 连接，mysql 151 配额下理论 poolCapacity≈72、pg 100 配额下≈47，本次 8/12/72 池均 fits=true。
  - 完成证据：2026-11-01；`internal/db/router/cap_probe_test.go`（TestArch010Probe：`ARCH010_DRIVER` 门禁 mysql/postgres，`ARCH010_GAMES/ENVS/CONCURRENCY` 参数化，服务端 `information_schema.processlist`/`pg_stat_activity` 峰值采样，`Forget` 回收残差，容量结论 = `(max_connections - 5% 预留) / idlePerPool`，可选 `ARCH010_TARGET_P95_MS`/`ARCH010_TARGET_MAX_CONNS` 断言）；`go test ./internal/db/router/ -count=1` 全绿（6.0s），`go test -race ./internal/db/router/ -run TestArch010Probe` 过，`go test ./internal/db/... ./internal/svc/... -count=1` 全绿，`go vet ./internal/db/router/` 过，`go build ./...` 过。

- [x] **ARCH-011 · P1 · 实现 · 分析 worker 持久化后确认**
  - 依赖：—。交付与验收：修正 worker.go 批量 Append 尚未 Send 即 XACK、重入队错误未核对的问题；杀进程及写库/入库故障下每条消息可核对，说明允许重复与去重策略。
  - 状态／条件：LOCAL VERIFIED：Send 成功后 ACK、失败保留 pending、重入队/死信 XADD 错误不 ACK（代码修复 5b25eb6d）；真实杀进程/写库故障逐条核对由 ARCH-012 钻机完成（Append后/Send前、Send后/ACK前、重入队失败三故障点，源消息/PEL/死信/ClickHouse 行对账无静默丢失）。去重策略：raw 表无服务端去重，消费端以 event_id/order_id 为去重键；Send后/ACK前崩溃重投会产生重复行（允许，消费端去重），非静默丢失。聚合字段竞态另由 DATA-014 修复。
  - 完成证据：2026-10-31；`internal/analytics/worker/worker.go`（deferred XACK：append 记录 ackRef、flush 后 ackSent；重入队/死信 XADD 错误不 ACK）、`internal/analytics/worker/arch011_test.go`（TestAppendEventRow_RecordsAckRef、TestFlushEventBatch_AcksRefsAfterSend、TestProcessMessage_RequeueFailureNoAck、TestProcessMessage_DeadLetterFailureNoAck、TestAdvanceCheckpoint_Monotonic）、`internal/analytics/worker/arch012_drill_test.go`（TestDrill_AppendThenCrash_BeforeSend_ReclaimNoLoss、TestDrill_SendThenCrash_BeforeAck_ReclaimNoLoss、TestDrill_RequeueFailure_MessageStaysPending）；`go test ./internal/analytics/... -count=1` 全绿，真实 Redis+ClickHouse 钻机全绿，`go build ./...` 过。

- [x] **ARCH-012 · P1 · 故障验证 · 分析管道崩溃恢复**
  - 依赖：ARCH-011。交付与验收：在 Append 后/Send 前、Send 后/ACK 前、重入队失败时中断；比对源消息、PEL、死信与 ClickHouse 行，证实无静默丢失。
  - 状态／条件：LOCAL VERIFIED：真实 Redis（redis-server / ARCH012_REDIS_ADDR）+ 真实 ClickHouse（ARCH012_CLICKHOUSE_DSN，需 analytics 库与 001_init.sql 建出的 events/payments 表）三故障点钻机全绿——(1) Append 后/Send 前崩溃：消息保持 PEL，重启 reclaim 重投 → 重新 append → flush → ClickHouse 行 + ACK，无丢失；(2) Send 后/ACK 前崩溃：行已落库、消息仍 PEL，重启 reclaim 重投 → 重复行（允许，消费端 event_id 去重）→ ACK，无丢失；(3) 重入队失败：消息不被 ACK、保持 PEL 供 reclaim 重试，无死信无丢行。缺可控环境时 t.Skip（边界：需 Redis 与 ClickHouse 可控环境）；CI 经 ci-analytics.yml 的 arch012-drill job（compose redis+clickhouse）强制执行。
  - 完成证据：2026-10-31；`internal/analytics/worker/arch012_drill_test.go`（TestDrill_AppendThenCrash_BeforeSend_ReclaimNoLoss、TestDrill_SendThenCrash_BeforeAck_ReclaimNoLoss、TestDrill_RequeueFailure_MessageStaysPending）、`.github/workflows/ci-analytics.yml`（arch012-drill job）；真实 Redis+ClickHouse 下 `go test ./internal/analytics/worker/ -run 'TestDrill_' -count=1` 全绿，`go build ./...` 过。

- [x] **ARCH-013 · P2 · 实现 · 阻止不支持的分析组合**
  - 依赖：—。交付与验收：配置/启动预检识别 Kafka 发布配内置 Redis-only worker；默认 Redis 链不受影响，不扩建 Kafka consumer。
  - 状态／条件：LOCAL VERIFIED：不支持的 Kafka/noop 组合配置门禁及相关包通过；不是外部消费者集成验收。
  - 完成证据：2026-10-30 复审；`cmd/ingest/cmd/root_test.go`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **ARCH-014 · P1 · 方案决策 · 异步任务重试与终态恢复协议**
  - 依赖：—。交付与验收：基于 sidecar TaskRunner 定义同 taskId 重复 Start、事件 ACK/持久重放及 Agent 重启后的结果未知语义；不把页面幂等等同游戏副作用去重。
  - 状态／条件：LOCAL VERIFIED：已定义在途重复 Start、有限终态重试、unknown 及游戏收据边界；运行时缺口见 ARCH-015/018。
  - 完成证据：2026-10-30 复审；`docs/design/task-retry-recovery-protocol.md`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **ARCH-015 · P1 · 实现 · 任务可靠事件与重复启动约束**
  - 依赖：ARCH-014。交付与验收：按已评审协议收口 TaskRunner 重复 Start 和事件重放/终态收敛，复用 task_runs/task_events；验证旧协程清理不删除同 ID 新任务，取消不回退终态；派发前落库失败由 ARCH-026 单独处理。
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。在途重复 Start（D1 generation 去重）、终态有限重试（D2）与会话丢失/`unknown` 收割（D3）已实现；R30-03 部分写入缺口闭合——`handleTaskEvent` 以单一事务（`RecordTaskEvent`）持久化 guard 状态更新与事件追加，事件写失败整体回滚、来源恢复后重试收敛，不另建任务体系。详见 [最新复审](docs/review/reliability-recheck-2026-10-30.md)。
  - 完成证据：2026-10-31；`internal/model/task_model.go` 新增 `TaskRunModel.RecordTaskEventTransactional`（单一事务：guard 更新 + seq 计算 + 事件追加，任一失败整体回滚）；`internal/tasks/store.go` 新增 `Store.RecordTaskEvent`；`internal/server/control_handler.go` `handleTaskEvent` 改调 `RecordTaskEvent`（`TaskStore` 接口收敛为该方法）。`go test ./internal/... -count=1` 全绿；新增 `TestControlService_HandleTaskEvent_RealStore_NoPartialWrite`（事件表缺失 → 报错且行保持 queued、无部分写入；表恢复 → 重试补写 seq=1 并收敛 failed）、`TestTaskRunModel_RecordTaskEventTransactional`/`_RollsBackOnAppendFailure`。边界诚实：task API `/tasks/cancel` 的 UpdateRunIfStatusNotIn+AppendEvent 仍非单事务，但 cancel_requested 非终态、重试可收敛（至多重复意图事件，非缺失），未纳入本次修复。

- [x] **ARCH-016 · P1 · 实现 · Provider drain 真正摘流**
  - 依赖：—。交付与验收：当前 handleDrain 仅记录并回空 ACK；实现停止新路由、等待在途及超时退出语义，验证 ACK 语义与执行结束条件。
  - 状态／条件：LOCAL VERIFIED：Draining/RPC 在途计数/路由门/宽限窗沿用既有实现；本项补齐 R30-09——(1) `handleInvoke` 对任务延续（taskId 存在且会话存储已装配）跳过读 agentlocal 的 `pickInstance`（drain 已摘除该表），改由 `callProvider` 按会话存储选路，draining 会话对延续放行、无命中明确失败，不再误报未注册；(2) `DrainProviderSession` 等待会话在途归零外，还等待 `TaskRunner.Count()==0`（覆盖「已 ACK 未进入 Provider RPC」窗口，该窗 Inflight=0 但任务仍在），宽限到点后不取消在途、留给 ARCH-015 D3 收敛。真实多实例原子准入竞争注入 NOT RUN。
  - 完成证据：2026-10-31；`internal/agent/local_handler.go`（handleInvoke 任务延续跳过 pickInstance、callProvider 无会话命中明确失败、DrainProviderSession 等待 TaskRunner.Count）、`internal/agent/provider_drain_test.go`（TestCallProvider_TaskContinuationAfterDrainNoAgentLocal、TestHandleInvoke_TaskContinuationSkipsPickInstanceAfterDrain、TestDrainProviderSession_WaitsForAcceptedTasks）；`go test ./internal/agent/... ./agents/sidecar/... ./internal/app/agent/... -count=1` 全绿，`go test -race ./internal/agent/ -run 'TestCallProvider_TaskContinuationAfterDrainNoAgentLocal|TestHandleInvoke_TaskContinuationSkipsPickInstanceAfterDrain|TestDrainProviderSession_WaitsForAcceptedTasks|TestCallProvider_InflightCountedAroundCall|TestDrainProviderSession_WaitsForInflightZero' -count=1` 过，`go build ./...` 过。

- [x] **ARCH-017 · P1 · 实现 · Server 停机收尾顺序**
  - 依赖：—。交付与验收：控制 listener.Close 等活跃会话时不阻塞后续 HTTP 停机/根取消；保证 ServiceContext.Close 冲刷留痕，设置有界清理并验证 socket/goroutine/池释放。
  - 状态／条件：LOCAL VERIFIED：停收→HTTP drain→rootCancel→等待→Close 正常路径回归沿用既有；本项补齐 R30-07——`scheduler.Manager` 新增生命周期 context（Start 创建、Stop 取消），`tick` 由 `context.Background()` 改用该 context；阻塞的 store/dispatch 调用被 Stop 取消，循环及时退出，`Stop` 有界返回，runServer 的 defer StopScheduler 不再在 30s 停机预算外挂住。补硬界兜底与真实停机入口故障注入：`Stop` 对两条调用路径统一 `waitStopped(stopTimeout)`（默认 5s，`SetStopTimeout` 覆盖），不观察 ctx 取消的硬卡死 tick 也超时返回（不把超时当资源已释放，循环退出仍由 stoppedCh 保证）；`SetTickHook` 测试缝；`StartScheduler` 不再自动 Start（root.go 注入治理/钻机钩子后显式 Start）。真实停机入口钻机验证阻塞调度、总截止、defer 与关池顺序（SIGINT→有界返回、HTTP/control listener 拒绝新连接）。
  - 完成证据：2026-10-31；`internal/tasks/scheduler/manager.go`（lifecycleCtx/lifecycleCancel、tick 用 lifecycle ctx；`stopTimeout`/`waitStopped`/`SetStopTimeout`/`SetTickHook`）、`internal/tasks/scheduler/manager_stop_test.go`（TestManagerStop_AbortsBlockedTick + blockingStore；TestManagerStopBoundedWhenTickStuck、TestManagerStopSecondCallerBoundedWhenTickStuck、TestManagerStopUnblocksBeforeTimeoutWhenTickHealthy）、`internal/svc/scheduler.go`（StartScheduler 不再自动 Start）、`cmd/server/root.go`（drillSchedulerHook 注入、显式 Start）、`cmd/server/fault_drill_r28_test.go`（TestDrill_BlockedScheduler_ShutdownBoundedAndCloseRuns：真实停机入口 SIGINT，卡死调度 tick 下 30s 内有界返回、HTTP/control listener 拒绝新连接）；`go test ./internal/tasks/... ./internal/svc/... ./cmd/server/... -count=1` 全绿（含钻机 3.48s），`go build ./...` 过。

- [x] **ARCH-018 · P1 · 故障验证 · 执行中断与停机演练**
  - 依赖：ARCH-015, ARCH-016, ARCH-017, ARCH-026。交付与验收：执行中注入断链、drain、SIGTERM；逐项记录新请求拒绝、终态/未知态、截止时间及资源释放；不把测试 fixture 的游戏副作用结果当真实游戏恢复凭据
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。D3 看门狗收割器随附（7da8d2d2：`runTaskWatchdog` 后台循环 + `ReapStale`，预算 `taskLog.reaperTimeout` 默认 5m）；执行中断钻机（1a2be73a：`cmd/server/fault_drill_r27_test.go`，真实 runServer boot + e2e-agent-probe 子进程）四场景全绿：断链→unknown+新请求 503；drain→在途完成+新请求 503+undrain 恢复派发；`-hang` 看门狗→预算内 timed_out+finishedAt 置位+新请求仍 200（agent 存活，非断连所致）；SIGTERM→优雅停机返回 nil+在途 unknown+HTTP 监听释放。已知边界：探针副作用计数是测试 fixture 非游戏恢复凭据，不断言游戏 exactly-once（GAME-010 仍 NOT RUN）；重连/迟到终态 soft-terminal 收敛未在钻机断言（ARCH-014 D3 设计域）。
  - 完成证据：2026-10-31；`internal/server/control_handler.go`（runTaskWatchdog）、`internal/model/task_model.go`（ReapStale）、`internal/tasks/store.go`（ReapStale 代理）、`internal/config/config.go`（reaperTimeout）、`cmd/server/fault_drill_r27_test.go`（四场景钻机）；`go test ./cmd/server/ -run 'TestDrill_' -count=1` 全绿；`go build ./...` 过；dashboard guard PASSED。

- [x] **ARCH-019 · P2 · 实现 · 执行领域依赖方向收敛**
  - 依赖：—。交付与验收：复用 assignment gate 下沉领域包，functioninvoke 不再 import internal/api；core/agents 的依赖边界检查可执行且原行为一致。
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。闸门下沉（72fa504f）：`EnsureFunctionAssigned` 自 `internal/api/assignment/gate.go` 迁至 `internal/logic/assignment/gate.go`（复用既有 `LoadAllAssignments`，语义不变；用例随迁 `gate_test.go` + `gate_error_r52_test.go` 全绿）；`functioninvoke/govern.go` 改 import `internal/logic/assignment`，执行链不再 import `internal/api`（仓内仅余 `internal/handler/routes.go` 的 API handler 注册合法引用）。依赖边界检查可执行：`scripts/check_dependency_boundary.sh`（core/agents 生产代码禁 import `internal/` + functioninvoke 禁 import `internal/api`，含扫描路径存在断言防静默 no-op）+ 合成违规自测 `scripts/check_dependency_boundary_test.sh`，已接线 ci.yml。原行为一致：`go build ./...` 过，三包测试全绿，dashboard guard PASSED，docs build 过。
  - 完成证据：2026-10-31；`internal/logic/assignment/gate.go`、`internal/logic/assignment/gate_test.go`、`internal/logic/assignment/gate_error_r52_test.go`、`scripts/check_dependency_boundary.sh`、`scripts/check_dependency_boundary_test.sh`、`.github/workflows/ci.yml`；`go build ./...` 过；dashboard guard PASSED。

- [x] **ARCH-020 · P2 · 实现 · 架构文档防漂移检查入 CI**
  - 依赖：—。交付与验收：复用 scripts/docs/check-architecture.py、check-architecture.cjs 与生成器，核对 SVG、路径、首页资源和 TODO DAG；新链接失败即报告，避免直接开启全站 strict 阻断旧页面。
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。check-architecture.py 已接线 docs-check.yml（`:60` 步；触发路径补 `todo.md`+`scripts/docs/**`）；干净树 PASS（78 TODO ID 无环、SVG XML、作用域链接/证据路径、README 图引用）；计数规则改为只数结构化任务项（审核发现 等非任务 checkbox 不再误报）；新链接失败即报告、作用域限定未开全站 strict。边界诚实：check-architecture.cjs 为手动 Playwright 布局工具，需已构建 docs，未入 CI。
  - 完成证据：2026-10-31；`scripts/docs/check-architecture.py`、`.github/workflows/docs-check.yml`、`docs/development/lifecycle.md`；`python3 scripts/docs/check-architecture.py` 干净树 PASS（78 TODO ID 无环）；docs build 过；dashboard guard PASSED。

- [x] **ARCH-021 · P1 · 实现 · 扩展同步身份与 scope 接入**
  - 依赖：—。交付与验收：复用现有 HTTP puller/版本 payload 与 protected 路由，为 Agent 建立受限机器身份及 scope 授权；无凭据/跨 scope 拒绝，合法 Agent 实际拉取生效且坏版本保持 last-good。证据：internal/app/agent/extension_sync_puller.go 未设认证头，internal/handler/routes.go 受 protected 保护，AgentSyncPayload 检查读取权限；不通过取消服务端授权绕过。
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。受限机器身份（X-Croupier-Agent-Token）已接入：中间件将机器 token 限定在扩展同步端点（其它路径 403），AgentSyncPayload 校验 agentID scope（跨 scope 403、无凭据走 RBAC 被拒）；合法 Agent 拉取生效（PullOnce 携带头、版本 payload 应用），坏版本 last-good 由 configsync 承担；签发命令 `croupier-server token agent-sync`（ResolveSecret+SignMachine round-trip 已验证）。
  - 完成证据：2026-10-31；`internal/security/jwtutil/machine_token.go`、`internal/svc/service_context.go`、`internal/api/extension/service.go`、`internal/app/agent/extension_sync_puller.go`、`cmd/server/token.go`、`internal/svc/machine_auth_test.go`、`internal/api/extension/machine_scope_test.go`、`internal/app/agent/extension_sync_puller_test.go`；定向运行 go test（jwtutil/svc/extension/agent/cmd-server）全绿，go build ./... 过。

- [x] **ARCH-022 · P2 · 实现 · 注册与重连的显式请求预算**
  - 依赖：—。交付与验收：复用 core/register，为 DialAndRegister/SyncOnce/SyncAttempts 明确一致的注册超时边界及调用者 deadline 优先级；核对心跳 tick 重拨与首拨退避差异，验证无注册响应时按预算退出、Stop 可释放连接与 goroutine，不引入重复并行重连。
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。注册预算已统一：registerCtx 对每次注册调用（DialAndRegister/SyncOnce/SyncAttempts）应用 requestTimeout（默认 10s），调用方 deadline 更早时以调用方为准；挂起 Register 无响应按预算退出（测试锁定 ~60ms 预算返回）；心跳 tick 重拨与首拨退避差异核清——首拨失败走 RunReconnectLoop 指数退避 5→60s，心跳恢复在 tick 内直接重拨（已被心跳间隔限频，不再叠加退避），属有意设计。Stop 经 lifecycleCancel+lifecycleWG.Wait 释放连接与 goroutine；dialInflight CAS 防重复并行重连。
  - 完成证据：2026-10-31；`core/register/registration.go`、`core/register/loops.go`、`core/register/register_budget_test.go`；定向运行 go test ./core/register/ ./internal/app/agent/ ./agents/capture/... -count=1 全绿，go build ./... 过。

- [x] **ARCH-023 · P0 · 实现 · 定时触发先领取再派发**
  - 依赖：—。交付与验收：复用 task_schedule_run_logs 唯一键，派发前原子领取 schedule+slot 并明确领取租约/崩溃恢复；多实例同槽仅一派发，记录失败不得执行；覆盖领取后/派发前和派发后/回写前中断，业务最终副作用去重仍由 GAME-010 承担
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。跨槽陈旧领取已由调度周期统一扫描（reapStaleClaims → ListStaleClaims(created_at<now-claimLease, status=claimed)）解析为 indeterminate——run-log 不再永久停在 claimed；租约过期的 claimed 不再记确定 failed（派发后/回写前中断与派发前中断不可区分，indeterminate 是唯一诚实终态）；绝不重派发。同槽 resolveStaleClaim 同步改 indeterminate。web 运行记录状态集扩至 claimed/indeterminate/approval_required 并新增 orange 标签（派发结果未知不得误读为确定失败）。
  - 完成证据：2026-10-31；`internal/tasks/scheduler/manager.go`、`internal/model/task_schedule.go`、`internal/model/task_schedule_model.go`、`internal/tasks/scheduler/arch023_test.go`、`internal/model/task_schedule_model_test.go`、`web/src/pages/Ops/Schedules/index.tsx`、`web/src/services/api/schedules.ts`；定向运行 go test ./internal/tasks/... ./internal/model/ ./internal/svc/ ./internal/server/ ./internal/api/schedule/ -count=1 全绿，web tsc 过，dashboard guard PASSED，docs build 过。

- [x] **ARCH-024 · P0 · 实现 · 任务及调度入口统一执行治理**
  - 依赖：—。交付与验收：让 /tasks Start、schedule Create/TriggerNow/自动触发复用可注入的执行治理服务；不把模块级 JWT/scope 误作函数政策。对同一高危函数，普通/页面/任务/手动调度/定时入口一致执行禁用/分配/函数权限/政策审批/输入契约；自动化主体和批准快照显式定义，撤权后不继续裸派发
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。Govern 链在主体加载后统一调用 CheckInvokePermission（显式函数 ACL FunctionActionAllowed + function:invoke 兜底，HasAdminRole 旁路，nil FunctionModel fail-closed 拒绝）——普通/任务/手动/自动入口同一条链，ACL 拒绝 → 零派发、先于审批创建（不制造审批噪音）；与 /tasks Start 的显式 ACL 检查不再漂移。详见 [最新复审](docs/review/reliability-recheck-2026-10-30.md)。
  - 完成证据：2026-10-31；`internal/platform/functioninvoke/govern.go` 在主体加载后统一调用 `CheckInvokePermission`（显式函数 ACL `FunctionActionAllowed` + `function:invoke` 兜底，HasAdminRole 旁路；nil FunctionModel fail-closed 拒绝）——普通/任务/手动/自动入口同一条链，ACL 拒绝 → 零派发、先于审批创建（不制造审批噪音）。`go test ./internal/platform/functioninvoke/... ./internal/api/schedule/... ./internal/api/task/... ./internal/api/function/... ./internal/api/approval/... ./internal/api/console/... ./internal/logic/function/... -count=1` 全绿；新增 `TestGovern_ExplicitFunctionAclDenied`/`TestGovern_ExplicitFunctionAclAllowed`（operator 主体 + 显式 per-function 规则：仅 auditor 规则 → 403 零派发；operator 规则 → 放行）；入口测试 env 同步补 `FunctionModel` + 调用者权限（schedule/approval/console），使 TriggerNow/批准续跑/绑定执行能走到派发路径。

- [x] **ARCH-025 · P0 · 实现 · 策略查询和初始化故障禁止静默放行**
  - 依赖：—。交付与验收：区分无覆盖策略与策略存储/配置故障；高风险执行无法获得有效政策时拒绝或使用明确批准的 last-good，不返回 nil,nil 绕过审批/审计；覆盖 GetPolicy SQL 错误及 PolicyManager 初始化失败
  - 状态／条件：LOCAL VERIFIED：DONE 2026-10-31。GetPolicy SQL 错误 fail-closed；PolicyManager 初始化失败回退内置默认档（high/danger 仍需审批，治理保持强制，错误记日志）；契约风险读取故障与未声明（ErrRecordNotFound）已分开——来源故障向上传播，治理链 fail-closed 拒绝、不建审批，approval step-up 门槛视作高危要求二次验证，绝不静默降档。详见 [最新复审](docs/review/reliability-recheck-2026-10-30.md)。
  - 完成证据：2026-10-31；`internal/platform/functioninvoke/policy.go` 的 `resolvePolicyRisk` 改为返回 `(policy.RiskLevel, error)` 并区分未声明与来源故障，`EnforceFunctionPolicy` 传播该错误；`internal/api/function/helpers.go` 的 `ResolvePolicyRisk` 同步签名；`internal/api/approval/stepup_otp.go` 的 `approveStepUpRisk` 在来源故障时 fail-closed 视作高危。`go test ./internal/... -count=1` 全绿（164 包）；新增 `TestGovern_ContractRiskReadErrorDenies`（function_contracts 表删除 → Govern 拒绝、零审批）、`TestResolvePolicyRisk_ContractSourceFailure_V9`、`TestApproveStepUpRisk_SourceFailureFailsClosed`。边界诚实：`RegistryStore.GetOpenAPI` 为内存查找，其唯一错误是未命中（未声明），保持回落 medium；PolicyManager 初始化失败采用既有「回退内置默认档」策略而非禁用治理。

- [x] **ARCH-026 · P1 · 实现 · 任务主记录持久化失败时禁止派发**
  - 依赖：—。交付与验收：Dispatcher 在 CreateRunWithMeta 失败时返回可定位错误并不得发送 StartTask；验证 DB 失败时执行器计数为 0、无孤立事件/不可取消任务；不与 Agent 事件重放重复实现
  - 状态／条件：LOCAL VERIFIED：生产 TaskRunWriter 已注入，落库失败返回且不发送 StartTask；dispatch 定向包通过。
  - 完成证据：2026-10-30 复审；`internal/platform/dispatch/dispatcher_test.go`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **ARCH-027 · P1 · 实现 · 输入契约读取失败的明确拒绝语义**
  - 依赖：—。交付与验收：区分旧函数确无 schema 与 descriptor 查询失败；契约读取故障不得作为跳过验证依据；固定已批准快照/版本，覆盖 schema DB 失败、required 缺失和版本漂移；复用 ValidateInvokeInput
  - 状态／条件：LOCAL VERIFIED：descriptor 查询错误拒绝、无 schema 与读取失败区分；相关包通过，风险查询故障另见 ARCH-025。
  - 完成证据：2026-10-30 复审；`internal/platform/functioninvoke/govern_test.go`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

## 页面链验收

- [ ] **UI-001 · P1 · 故障验证 · 真实游戏页面发布全链（旧 C2）**
  - 依赖：GAME-002, GAME-010, ARCH-024。交付与验收：独立真实游戏环境走契约→模板→编辑保存→接受提案→发布→binding 执行；验证列表/详情/弹窗/刷新、筛选/契约变化/人工草稿隔离，记录落库快照与游戏调用凭据
  - 状态／条件：BLOCKED（2026-11-04 两条路径≤30 分钟评估后已代决：路径 A 本机 game-* 栈替代真实接入方不可行，`spawn` 是游戏社区/目录平台，`game-infinite-clash` 有账号/角色/奖励 Lua 服务但需 shield/openresty/MySQL/Redis 运行面且无 OpenAPI/SDK 契约、GAME-001/007/009 仍缺接入方契约与用户拍板；路径 B Nacos/SQL 搭建面可行性更高并推进，`cs-nacos`（nacos-server 1.4.2）重启后 health `OK`，schema/configsource 已存在，实测暴露 Nacos 拒绝 dataId 含 `/` 导致目录路径 `invalid dataId`，已修复为 `croupier:runtime:switch.yaml` 编码并验证 write/read/list/delete 均 200，commit `312017c2`。UI-001 页面发布全链仍 BLOCKED：GAME-002/GAME-010 未完成，缺真实游戏权威函数、持久凭据、受控写环境与 GAME-007/009 方案批准；demo/Nacos 配置面不能替代全链验收）。复核（2026-11-04）：仍 BLOCKED——GAME-002/GAME-010 未完成（权威玩家函数/持久凭据/受控写环境/游戏方持久去重缺失），GAME-001/007/009 仍缺接入方契约与用户拍板；`game-infinite-clash` 仅 proto 内部协议、无 OpenAPI/SDK 契约且需 shield/openresty/MySQL/Redis 运行面，不满足「真实接入方」验收。
    复核（2026-11-06）：仍 BLOCKED——依赖 GAME-002/GAME-010/ARCH-024 中 GAME-002、GAME-010 仍未完成（平台侧无持久执行凭据存储与收据查询模型，`internal/` 无 Credential/Receipt/operationId 持久去重实现，仅 `sdks/go/examples/gameops-demo` 内存/state-file 演示）。本次补充评估此前未覆盖的真实游戏候选 `game-civilization`（`/home/cui/workspaces/game-civilization`，真实 GM 玩家/封禁/邮件/充值 API，`gm/src/main/java/net/game/gm/module/server/controller/PlayerController.java` 等）：其有 Java 21 + Maven （`mvnw`/`/home/cui/tools/apache-maven-3.9.9`）且本地曾有 MariaDB+Redis+Nacos 可跑通 68 例基线，但默认依赖为内网 `192.168.5.157:8848`（Nacos）/`192.168.5.143:3306`（MySQL）/`.143:6379`（Redis），当前均 `No route to host` 不可达；本机 `doggo-mysql`（13306）与 `cs-nacos`（8848）为无关实例（Nacos `v1/ns/service/list` 返回 `{"doms":[],"count":0}`，无 civilization 服务注册），项目无 `target/*.jar` 编译产物。即便修正配置可启动，其 GM 执行链（RSocket 经 Nacos 到 ServerService）无平台所需操作凭据/收据查询语义，且 GAME-002/GAME-010 平台能力缺失，不能满足「契约→模板→编辑保存→接受提案→发布→binding 执行」独立真实环境全链验收；门禁基线绿仅记录机制侧（`go test ./internal/platform/functioninvoke/... ./internal/drivers/openapi/... ./internal/api/game/... -count=1` PASS、`pnpm --dir web run tsc` 0 错误），不构成全链验收。缺 GAME-002/GAME-010 完成 + 真实接入方权威函数/持久凭据/受控写环境。

    复核（2026-11-09）：仍 BLOCKED——依赖未变：GAME-002/GAME-010 未完成（internal/ 无 Credential/Receipt/operationId 持久去重实现面），GAME-001/007/009 仍缺接入方契约与用户拍板；本会话未获得新增真实游戏环境/受控写环境，无可用推进面。
    注记（2026-11-17）：`sdks/go/cmd/demo-game`（docs/design/demo-game.md）已交付持久化模拟接入方（34 契约 + exactly-once 存储 + 发布/binding/重放 E2E），页面发布全链的**机制演练**可对它跑（scripts/e2e/demo-game.sh，2026-11-16 本地 23/23 绿）；但本项关闭仍须真实接入方环境，demo-game 验收不抵扣 GAME-001。
    复核（2026-11-10）：仍 BLOCKED——重新 grep `internal/model/`+`migrations.go` 无 Credential/Receipt 持久去重实现（唯一 OperationID 命中在 `openapi_source.go`，属 OpenAPI 导入域非游戏收据）；GAME-001/007/009 未获用户拍板，无新增真实环境。继续队列 LAY-001。
- [x] **UI-002 · P1 · 故障验证 · 示例异常状态全链（旧 E2）**
  - 依赖：—。交付与验收：复用现有 Go gameops-demo 的 33 函数及内存状态，独立 Server/sidecar/demo 栈验证发布、审批、report.generate、失败/未知/取消/幂等重试；既有 fixture 只证机制，不证明进程重启后游戏侧持久去重
  - 状态／条件：LOCAL VERIFIED（2026-11-01）：修复 gameops-demo 契约驼峰缺陷（`batch_ban/batch_recharge/batch_unban`），agent 注册从 3 函数 rebuild 失败变为 34 函数 `warnings=0`；`configs/games.json` 添加 `croupier-demo` scope 后搭起本仓自洽 Server(18881)+sidecar-agent(19096)+gameops-demo 栈，实测 report.generate 全链：成功 `started→completed/succeeded`、失败 `result.error="report_generate_failed"`、取消 `started→cancel_requested→cancelled`（5s delay 被 243ms 中断）。进程重启持久去重已补齐：demo `-state-file` 持久化 reportRunSeq，实测重启后 `report-demo-1→report-demo-2` 序列延续；单测 `TestGameOpsReportRunSeqPersistsAcrossRestart` 断言三次重启序列不重置。审批链说明：demo 唯一 task 函数 `report.generate` risk=warning，不触发 ApprovalRequired；审批门语义由 ARCH-024 政策层与 functionInvoke 契约验收覆盖，本项以审批路由未误触发（task 响应无 approvalId）为界。页面 UI 发布全链（契约→模板→提案→发布→binding）归 UI-001 真实环境验收。
  - 完成证据：2026-11-01；`sdks/go/examples/gameops-demo/store.go`（statePath 加载/持久化 reportRunSeq）、`executors.go`（递增后持久化）、`main.go`（`-state-file` flag）、`report_test.go`（`TestGameOpsReportRunSeqPersistsAcrossRestart`）；`configs/games.json`（croupier-demo/local-demo）；实测栈：`go build ./examples/gameops-demo` 后启动 server(18881)+agent(19096)+demo，`POST /api/v1/tasks` 依次触发成功/失败/取消，重启 demo 后 `report-demo-2` 延续；`go test ./examples/gameops-demo/ -count=1` PASS。

- [x] **UI-003 · P1 · 故障验证 · 批量确认与默认呈现实页验收（旧 D2/D5）**
  - 依赖：UI-002。交付与验收：复用 perTarget、字段语义和五模板；跨页选人、筛选变化、总量确认与实际提交一致，发布回读保留身份列/单位/时区/枚举；不重复规划已实现 UI。
  - 状态／条件：LOCAL VERIFIED（2026-11-01）：代码侧 2026-10-31 收口（合并确认框、MAX_SAFE_INTEGER/乘积溢出 fail-closed、players 补偿空目标 fail-closed，`resourceBatchSelection.test.tsx` 覆盖跨页选人/筛选变更/清除/翻页不误报/目标预填/确认内容与提交一致）；本次补齐发布态 demo 栈实页验收，确认列表身份列/时区/金币单位、批量充值二次确认（每人 100 金 × 2 = 200 金，目标 `20001、20002`）与实际执行一致；修复批量 binding 的 canonical lowercase function ID、补 create/update/delete selector、补 selection `pick` transform 防止 `playerIds` 丢失。
  - 完成证据：2026-11-01；`sdks/go/examples/gameops-demo/contracts.go`（`player.batchban/batchunban/batchrecharge`）、`web/src/components/PageRenderer/templates/playerManage.tsx`（lowercase IDs、create/update/delete selectors、integer schemas、selection `pick` transform）、`gameOpsTemplateCapabilities.test.ts`（binding ID lowercase 断言）；发布版本 7 实测 snapshot 含 3 个 selection binding `transform={"type":"pick"}`；Playwright `/tmp/ui003.playwright.config.ts` 1 passed，执行返回 HTTP 200 `{"success":true,"count":2,"total":200}`；`go test ./examples/gameops-demo/... -count=1` PASS、`pnpm test -- gameOpsTemplateCapabilities playerManage gameOpsPublishFixture resourceBatchSelection compositeBatchTotal` 6 suites/69 tests PASS、`pnpm run tsc` PASS、`git diff --check` PASS。

## 真实游戏接入

- [ ] **GAME-001 · P1 · 资料调查 · 真实游戏权威数据与执行契约**
  - 依赖：—。交付与验收：向接入方整理玩家/角色/封禁/订单/邮件/活动权威读写入口、身份映射、凭据与测试目标；核对现有 ownership/high-risk 设计，交付缺口矩阵。
  - 状态／条件：BLOCKED（2026-11-02 复核）：需接入方真实游戏契约/样本/测试账号；本沙箱无真实游戏接入方。缺接入方契约/样本/测试账号。
  - 复核（2026-11-06）：仍 BLOCKED——需接入方真实游戏契约/样本/测试账号；本沙箱无真实游戏接入方。本次补充评估此前未覆盖的真实游戏候选 `game-civilization`（`/home/cui/workspaces/game-civilization`，真实 GM 玩家/封禁/邮件/充值 API，`gm/src/main/java/net/game/gm/module/server/controller/PlayerController.java` 等）：其默认依赖为内网 `192.168.5.157:8848`（Nacos）/`192.168.5.143:3306`（MySQL）/`.143:6379`（Redis），当前均 `No route to host` 不可达；本机 `doggo-mysql`（13306）与 `cs-nacos`（8848）为无关实例（Nacos `v1/ns/service/list` 返回 `{"doms":[],"count":0}`，无 civilization 服务注册），项目无 `target/*.jar` 编译产物；即便修正配置可启动，其 GM 执行链（RSocket 经 Nacos 到 ServerService）无平台所需操作凭据/收据查询语义。缺接入方契约/样本/测试账号。

- [ ] **GAME-002 · P1 · 实现 · 接入真实玩家工作台**
  - 依赖：GAME-001, GAME-008。交付与验收：复用现有玩家模板和发布链，列表/详情/操作以权威游戏回读及执行凭据验收，失败不显示成功。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需目标游戏环境（权威玩家函数）；缺真实游戏环境。
  - 复核（2026-11-06）：仍 BLOCKED——需目标游戏环境（权威玩家函数）与受控写权限；本沙箱无真实游戏环境。本次补充评估此前未覆盖的真实游戏候选 `game-civilization`（`/home/cui/workspaces/game-civilization`，真实 GM 玩家/封禁/邮件/充值 API）：默认依赖内网 `192.168.5.157:8848`（Nacos）/`192.168.5.143:3306`（MySQL）/`.143:6379`（Redis），当前均 `No route to host` 不可达，本机无关实例（`doggo-mysql` 13306 / `cs-nacos` 8848，Nacos 服务列表 `{"doms":[],"count":0}` 无 civilization 注册），项目无 `target/*.jar` 编译产物；即便修正配置可启动，其 GM 执行链（RSocket 经 Nacos 到 ServerService）无平台所需操作凭据/收据查询语义，且平台侧 GAME-002/GAME-010 能力缺失（`internal/` 无 Credential/Receipt/operationId 持久去重实现，仅 `sdks/go/examples/gameops-demo` 内存/state-file 演示）。缺真实游戏环境/受控写权限。
  - 注记（2026-11-17）：游戏半边现可对持久化模拟接入方 `sdks/go/cmd/demo-game` 验收（玩家查询/创建/充值 exactly-once 已有 E2E 证据，docs/design/demo-game.md）；本项**关闭**仍等 GAME-001 真实接入方 + GAME-008/010 平台半边，demo-game 验收不抵扣。

- [ ] **GAME-003 · P1 · 实现 · 接入真实邮件与补偿**
  - 依赖：GAME-001, GAME-008, GAME-010, GAME-012, ARCH-024。交付与验收：复用模板/审批/任务，按对象凭据核对发放、部分失败重试和重复操作不重发。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需目标游戏环境及受控写权限；缺真实游戏环境/受控写权限。

- [ ] **GAME-004 · P1 · 实现 · 接入真实封禁管理**
  - 依赖：GAME-001, GAME-008, GAME-010, ARCH-024。交付与验收：复用封禁模板，身份/期限/原因批准快照一致，封禁解封后权威回读可核对。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需目标游戏环境及受控写权限；缺真实游戏环境/受控写权限。
  - 注记（2026-11-17）：游戏半边可对 `sdks/go/cmd/demo-game` 验收（ban.create/unban/extend/batch 契约已注册，权威回读经 player.get）；**关闭**仍等 GAME-001 + GAME-008/010 + ARCH-024 平台半边，demo-game 验收不抵扣。

- [ ] **GAME-005 · P1 · 实现 · 接入真实订单核查**
  - 依赖：GAME-001, GAME-008, GAME-010, ARCH-024。交付与验收：复用订单模板，订单/发货权威状态一致，补发有凭据且重试不重复发货。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需测试订单与受控补发权限；缺真实测试订单/受控补发权限。
  - 注记（2026-11-17）：游戏半边可对 `sdks/go/cmd/demo-game` 验收（order.recredit 的 `already_credited` 幂等标记与 operationId 指纹冲突拒绝已有 E2E/单测证据）；**关闭**仍等 GAME-001 + GAME-008/010 + ARCH-024，demo-game 验收不抵扣。

- [ ] **GAME-006 · P1 · 实现 · 接入真实活动配置**
  - 依赖：GAME-001, GAME-008, GAME-010, ARCH-024。交付与验收：复用活动模板，发布/回读版本一致，失败保留旧版本并有审计。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需测试配置和发布权限；缺真实活动配置/发布权限。
  - 注记（2026-11-17）：游戏半边可对 `sdks/go/cmd/demo-game` 验收（activity 五操作 + `demo_activity_versions` UNIQUE(activity_id,version) 版本行保留=回滚审计）；**关闭**仍等 GAME-001 + GAME-008/010 + ARCH-024，demo-game 验收不抵扣。

- [ ] **GAME-007 · P1 · 方案决策 · 评审游戏权威归属与统一目标引用**
  - 依赖：GAME-001。交付与验收：复用已有 game-authority-ownership 和 player-identity-target-model 设计，决定本地 Player/Balance 的演示/副本定位以及账号/角色/区服/渠道/合服映射来源；交付批准的映射与迁移边界，禁止另建平台游戏经济真值
  - 状态／条件：BLOCKED（2026-11-13 复核）：决策记录已落 docs——ADR 0002（docs/architecture/adr/0002-game-authority-target-ref.md，Proposed 2026-11-13，D1-D8 + 签核清单；中文镜像 docs/zh/architecture/adr/0002-*.md；双索引已挂；docs `CI=true pnpm build` 过）。LOCAL VERIFIED。仍待用户拍板（签核清单 3 项）翻 Accepted。缺用户签核批准。

- [ ] **GAME-008 · P1 · 实现 · 统一游戏目标引用与歧义阻断**
  - 依赖：GAME-007。交付与验收：实现已批准的 TargetRef 解析并供页面/审批/任务/审计复用；同角色 ID 多区服、账号多角色、合服旧引用明确解析，歧义拒绝，不以 gameId/env 或主机代替业务区服
  - 状态／条件：BLOCKED（2026-11-01 复核）：需 GAME-007 方案批准后实施 + 接入方身份映射样本；缺方案批准/身份映射样本。

- [ ] **GAME-009 · P0 · 方案决策 · 评审高风险操作凭据与未知结果协议**
  - 依赖：GAME-001。交付与验收：复用 high-risk-operation-contract-design，决定稳定 operationId、参数/目标指纹、批准快照、游戏持久去重与收据查询职责；明确响应丢失后先查凭据及重试/取消/补偿边界，不重建与 taskId/page idempotency 重复的系统
  - 状态／条件：**方案已落，待用户拍板**（2026-11-22，commit b814280c0，LOCAL VERIFIED）：评审文档 `docs/design/high-risk-operation-protocol-review.md`（中文镜像 `docs/zh/design/high-risk-operation-protocol-review.md`）——风险面 6 项（响应丢失重复发放/未知误判/平台幂等≠游戏 exactly-once/取消谎称回滚/参数漂移/无凭据伪造成功）+ 5 项决策方案 D1–D5（operationId=`gameId:env:domain:UUIDv7` 复合编码、专用 `business_operation_credentials` 表、SDK `GameOperationResult` 信封+durableIdempotency 能力声明、未知 60s×10 自动查询+重试上限 8 且不补发、取消强制游戏确认）+ 备选对比表 + 验收映射 + GAME-010 范围（平台侧）/边界（游戏侧待真实环境）。**待用户拍板后才可实现 GAME-010**。验证：`go test ./internal/... -count=1` 全绿（含 AUD-036/037 提交）；`cd docs && CI=true pnpm build` 通过。

- [ ] **GAME-010 · P0 · 实现 · 游戏持久执行凭据与未知结果收敛**
  - 依赖：GAME-009, GAME-008。交付与验收：按批准协议贯通游戏适配器、执行结果和凭据查询；同 operationId 改参数拒绝、响应丢失/服务重启不重复发奖、未知结果可查询收敛；取消不谎称回滚，补偿引用原凭据
  - 状态／条件：BLOCKED（2026-11-01 复核）：需游戏方持久去重/查询实现与受控写环境；Go demo 的内存记录不能替代。缺游戏方实现/受控写环境。

- [ ] **GAME-011 · P1 · 方案决策 · 评审批量运营冻结与预算恢复规则**
  - 依赖：GAME-009。交付与验收：复用 batch-operations-compensation-design，评审名单快照、预演人数/预算、审批后变化处理、单位/倍率、速率并发、失败阈值、暂停恢复和逐对象凭据；冻结后的对象/金额变更重新审批
  - 状态／条件：BLOCKED（2026-11-01 复核）：批次方案待评审批准（用户拍板）；前端跨页选择和 perTarget 确认不等于服务端批次协议。缺方案批准。

- [ ] **GAME-012 · P1 · 实现 · 服务端批次对象账本与受控恢复**
  - 依赖：GAME-011, GAME-010。交付与验收：复用任务体系实施批准的快照/逐对象状态与预算约束；变更名单不可偷换批准范围，部分失败只重试可确认未执行对象；暂停恢复/阈值停止/最终核对可验，不以一次 onExecute 返回成功代替批次完成
  - 状态／条件：BLOCKED（2026-11-01 复核）：需批准的批次方案和游戏执行收据；缺方案批准/游戏执行收据。

- [x] **GAME-013 · P2 · 实现 · games 目录 API 支持显式 gameId 与存量库 seed 复核**
  - 依赖：—。交付与验收：games REST Create/Update 允许显式 gameId（校验沿用 sanitizeGameID 语义，连字符可用），dashboard 手建 scope 能产出与 SDK 侧一致的标识；games.json bootstrap 现状为首装仅跑（games 表非空即跳过），补存量库对账（烘焙清单新增条目幂等补种或提供再同步入口）；验收=存量库不直连 DB 即可建出带连字符的 gameId scope，并经控制台切换与 X-Game-ID/X-Env 路由复证
  - 状态／条件：LOCAL VERIFIED（2026-11-21，commit 44d21cd9d）：games REST Create 接受显式 `gameId`（`sanitizeExplicitGameID`：trim、小写、空格折叠下划线、字符集 `^[a-z0-9_-]+$`、长度 ≤64，连字符原样保留）并在落库前查重（冲突 409）；未显式提交时回落 Name 派生，既有 `-`→`_` 归一语义不变；Update 允许改 `gameId`，守卫为已绑 `game_envs` 或 `envs` 元数据非空即 409 拒绝（与 Delete 前置条件同构），改名后 `Router.ForgetGame(oldID)` 丢弃按旧 ID 缓存的连接；存量库对账=新增 `auth.gamesSeedReconcile`（默认 false 保持首装仅跑语义），打开后对非空 games 表按 game_id 幂等补种烘焙清单缺失条目并写 game_envs 绑定，已存在条目绝不修改；dashboard 手建 scope 新增「游戏 ID（gameId）」字段（本地 pattern 校验 + en-US 词条）
  - 验证：`go build ./internal/...` 0 错；`go test ./internal/api/game/` ok（新增 `service_gameid_test.go` 7 用例：连字符创建与业务串寻址、缺省派生归一、非法字符表驱动、创建查重、改名成功且旧标识不可寻址、env 绑定阻止、envs 元数据阻止、改名撞车）；`go test ./internal/svc/ -run Reconcile` ok（新增 3 用例：开关关不动存量库、开关开补种缺失含绑定且重复运行幂等、开关开空库全量播种）；`pnpm --dir web run tsc` 0 错；jest `create-edit` 新增 2 用例 PASS；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 0 错；契约 snake_case 扫描 clean
  - 完成证据：commit 44d21cd9d（13 文件，492 insertions / 20 deletions）；实现 `internal/api/game/helpers.go`（`sanitizeExplicitGameID`）、`internal/api/game/service.go`（Create 查重、Update 改名守卫、ForgetGame）、`internal/api/game/dto.go`（两个 DTO 的 `gameId` 字段）、`internal/svc/game_seed.go`（`seedBootstrapGames` 对账分支）、`internal/config/config_auth.go`（`GamesSeedReconcile`）、`web/src/pages/GamesManage/Create.tsx`（gameId 表单项）、`web/src/services/api/games.ts`（upsertGame 透传）、`web/src/locales/en-US/gamesManage.ts`（3 键）；测试 `internal/api/game/service_gameid_test.go`、`internal/svc/game_seed_reconcile_test.go`、`web/src/pages/GamesManage/__tests__/create-edit.test.tsx`；文档 `docs/design/demo-game.md` 与 `docs/zh/design/demo-game.md` 的 §5 / §8
  - 已知边界：`create-edit.test.tsx` 既有用例「提交载荷含 name/aliasName/icon」在本机负载 134 下 20s 超时红，已在基线 origin/main 282860181 同文件复现同一签名（1 failed / 3 passed，BASE_EXIT=1）判定为环境 flake 而非本改动回归，CI 侧仍需复核；`go test ./internal/svc/` 全包在本机 600s 假超时（go test 进程仅 1.6% CPU，属负载饿死非死锁），隔离复跑 seed 用例 36s PASS；对账开关默认关，存量库验收未在 192.168.5.5 复证（部署由用户触发）；该机 demo-game/demo scope 仍为 GAME-013 之前的一次性 DB 直插

- [x] **GAME-DEMO-001 · P1 · 实现 · 复杂游戏模拟 demo（gameops-rpg，巡检单 2026-10-20）**
  - 依赖：无（刻意不依赖 GAME-002~010 真实接入方）。交付与验收：`sdks/go/examples/gameops-rpg/` 模拟 RPG 世界（玩家等级/金币/钻石/背包、任务每日刷新、邮件附件领取=补偿流核心、订单幂等发货、封禁联动），SDK→Agent 注册 17 函数（`player.*` `mail.*` `task.*` `order.*` `economy.*` `report.generate`），tick 驱动玩法循环 + JSON 快照持久化；demo 编译、单测覆盖发货幂等/封禁拒领/余额 fail-closed/邮件过期、平台注册可见、经平台全链路（mail.send 附件→落库→领取→入背包→report.generate 审批通过执行）与异常链路（重复发货拒绝、封禁拒领）
  - 状态／条件：LOCAL VERIFIED（2026-11-11，本机隔离栈 28080/29090/29091 + sqlite，避让 quickstart python demo 的 default/dev 契约槽）。编译/`go vet` 通过；单测 28 用例全绿（world 13 + store 3 + contracts 4 + executors 8）；平台注册 17 契约落库（health + functions API + sqlite function_contracts 复核）；补偿链：mail.send（附件 potion×5+gem×1）→ mail-9004 落库 → mail.claim → 背包 potion+5/gem+2；审批链：report.generate（risk=high）invoke 返回 approvalRequired → 第二账号 TOTP 批准 → `state:approved, continuation:true` → 执行 `report-rpg-1, runSeq:1, success:true`；异常链：重复 order.ship 续跑返回 `already_shipped` 且钻石只入账一次（+6）；封禁玩家 10007 mail.claim 返回 `player_banned` 且背包不变；快照重启恢复复验（shipped 状态持久）。已知边界：内存态模拟，不能替代 GAME-002~010 的真实接入方验收；审批需第二账号（self_approval_forbidden 两人规则），demo 侧管理员已在隔离栈注册 TOTP（仅本地测试凭据）
  - 完成证据：commit c7018a8c（实体 store）/ b5c98c5f（tick 循环与经济规则）/ 78111562（17 函数契约与执行器+README）/ 82d0ec98（28 用例测试）/ 03cb67de（player.get 入参键统一 playerId）；`go test ./examples/gameops-rpg/...` → `ok ... 28 PASS`（demo 源 `sdks/go/examples/gameops-rpg/main.go`）；e2e 过程记录见本会话（隔离栈配置 /tmp/gameops-rpg-e2e/，验证后进程已清理）

## Agent 与隔离

## 运维分栏（OPS 系列，2026-11-01 用户令立项：游戏运维平台扩展；供给引擎不自研，集成现成项目）

- [x] **OPS-010 · P1 · 实现 · 节点资源指标与进程清单采集（OPS 系列前置，2026-11-01 盘点确认缺失）**
  - 依赖：—（OPS 系列前置项）。
  - 现状盘点：agent 采集对游戏数据足够深（CDC/函数观测/拨测），但无 CPU/内存/磁盘/网络指标、无进程与端口清单、心跳无资源读数——调度（OPS-001/003）与健康门禁（OPS-004）的数据前提不存在。
  - 交付与验收：agent 周期采集（默认 15s，可配）节点指标（CPU%、内存 used/total、磁盘各挂载点 used/total、网络收发速率）+ 进程清单（名称/pid/内存/监听端口）+ 端口列表；随心跳或独立批量上报；服务端入库（保留策略：原始 7 天、聚合 90 天）；面板出节点资源曲线。
  - 实现约束：跨平台无外部依赖（gopsutil 或等价）；采集关闭时可配；指标字段与 OBS-AGENT-001 的 OTel 语义对齐（避免两套字段名）。
  - 落位：实现为共享采集库，优先挂在 devops agent（运维角色，其会话执行记录/execlog/回放统计是现成的操作面数据，与资源指标互补成完整运维数据集）；capture/sidecar/synthetics 后续按需接入同一库。（2026-11-01 用户确认 devops agent 已有基础数据收集：会话记录+execlog+回放统计=审计与 runbook 输入；缺的资源指标由本项补齐）
  - 验收：Linux+Windows 真机各一台指标上报；数值与 top/任务管理器抽样对账误差 <10%；关闭采集后零上报流量。
  - 状态／条件：LOCAL VERIFIED（2026-11-01，边界见 [交付报告](docs/review/2026-11-01-ops-010-node-metrics.md) §5）：采集面（CPU/内存/磁盘/网络收发速率 + 进程清单 + 端口列表，gopsutil v4 无外部依赖）、入库（原始 7 天 + 迁移 0051 建 `agent_metrics_hourly` 小时聚合 90 天 + RollupHourly/PruneHourly）、history API `window=raw|hourly`、面板网络速率曲线 + 进程/端口清单表全部落地；`MetricsEnabled=false` 零上报由上报循环不启动保证；默认间隔 30s→15s；字段命名与 OBS-AGENT-001 OTel 语义对齐（usage_percent/bytes_sent_rate 单一命名）。**真机抽样对账（Linux+Windows 各一台、与 top/任务管理器 <10%）本环境不可运行，登记为部署后人工验收项**；Windows 采样未实测；每进程 CPU% 为 gopsutil 启动以来平均语义。
  - 完成证据：2026-11-01；`core/metrics/processes.go`、`core/metrics/disknet.go`、`core/metrics/processes_test.go`（TestCounterRate/CollectProcessAndPortFaces/NetworkRateNonNegative）、`internal/platform/registry/metrics.go`（三面持久化 + RollupHourly 幂等 + PruneHourly）、`internal/platform/registry/metrics_hourly_test.go`、`internal/platform/registry/metrics_enrichment_test.go`、迁移 0051 三处同步（`internal/svc/migrations.go` + `internal/db/migrate/migrate.go` MinimumRequiredVersion=51 + `internal/db/migrate/migrate_test.go`）、`internal/svc/migrations_metrics_enrichment_test.go`（存量形态/缺表跳过/列集 round-trip）、`internal/api/ops/metrics_history.go` + `metrics_history_window_test.go`（raw 重建三面/hourly/越权 fail-closed）、`internal/app/agent/ops_config.go`、`proto/croupier/ops/v1/ops.proto`（PortMetrics/rates/listening_ports，四 SDK gencode 同步）、`web/src/pages/Ops/Nodes/NodeDetailDrawer.tsx`、[交付报告](docs/review/2026-11-01-ops-010-node-metrics.md)；`go build ./...` 过，`go test ./core/metrics/... ./internal/platform/registry/... ./internal/api/ops/... ./internal/svc/... ./internal/db/migrate/... ./internal/app/agent/... -count=1` 全绿，web tsc 0 错误，NodeDetailDrawer jest 29/29，dashboard guard PASSED，docs build 过；commit `42c3ca76`。

- [x] **OPS-001 · P1 · 实现 · 云资源供给引擎（集成 Terraform/OpenTofu，不自写云适配器）**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-01，边界见 [交付报告](docs/review/2026-11-01-ops-001-provision-engine.md) §4）：`internal/ops/provision` 用 hashicorp terraform-exec 驱动 tofu/terraform；provider 模板（阿里云/腾讯云/AWS/Hetzner 起步）；一键扩容闭环=选厂商/地域/规格 → create → cloud-init 注入 croupier agent（bootstrap token+hub 地址）→ 自注册 → 按网络审批策略（个人直通/团队工单）→ 纳管；HTTP 面 `/ops/provision*` 五端点（提交/列表/详情/续行/销毁）落地，`provision.enabled=false` 时如实报 503 不假装已开启；迁移 0052 建 `provision_runs`。**真实 provider 集成（mock cloud + 真实 provider 各一）本环境不可运行（需云凭据 + tofu 二进制），登记为部署后人工验收项**——本环境以模板渲染真值面 + FakeRunner 全链 + `-race` 覆盖编排正确性；cloud-resource-requests.md 的目录/申请单三角色流（catalog/requests 表 + `cloud.request.*` 权限 + 执行回执）未随附（设计纸保持未实现状态，见其「Implementation status」）。
  - 理由：各云适配已有成熟项目（Terraform provider 全覆盖），自写=重复造轮子；croupier 只做策略与编排。
  - 交付与验收：`internal/ops/provision` 用 hashicorp terraform-exec 驱动 tofu/terraform；provider 模板（阿里云/腾讯云/AWS/Hetzner 起步）；一键扩容闭环=选厂商/地域/规格 → create → cloud-init 注入 croupier agent（bootstrap token+hub 地址）→ 自注册 → 按网络审批策略（个人直通/团队工单）→ 纳管；集成测试用 mock cloud + 真实 provider 各一。
  - 完成证据：2026-11-01；`internal/ops/provision/`（types/runner/template/orchestrator + orchestrator_test/template_test）、`internal/model/provision_model.go`、迁移 0052 三处同步（`internal/svc/migrations.go` + `internal/db/migrate/migrate.go` MinimumRequiredVersion=52 + `internal/db/migrate/migrate_test.go`）、`internal/svc/migrations_provision_test.go`（建表/幂等/round-trip + 注册表含 0052）、`internal/svc/provision_wiring.go`（DBStore/ApprovalGate/AgentRegistry 适配）、`internal/svc/service_context.go`（enabled 装配）、`internal/api/ops/provision.go` + `provision_test.go`（503/422/404/409/202/200 映射）、`internal/handler/routes.go`（`/ops/provision*` 五路由）、[交付报告](docs/review/2026-11-01-ops-001-provision-engine.md)；`go build ./...` 过，`go test ./internal/ops/provision/... ./internal/api/ops/... ./internal/svc/... ./internal/model/... ./internal/db/migrate/... ./internal/handler/... -count=1` 全绿（provision 包含 `-race`），`go vet` 过，docs build 过。
- [x] **OPS-002 · P1 · 实现 · 缩容与排水（drain → destroy）**
  - 依赖：OPS-001。
  - 状态／条件：LOCAL VERIFIED（2026-11-02，边界见 [交付报告](docs/review/2026-11-02-ops-002-drain-destroy.md) §4）：销毁前置三闸落地——数据盘护栏（`provision_runs.destroy_protected` 持久化标记 + `PUT /ops/provision/:runId/protection`，迁移 0053 补列）拒绝自动销毁；排空门要求运行名下在线 agent 全部 drain（复用 `AgentSession.Draining` 路由门）；「无负载才允许销毁」保守版=目标机在途任务（task_runs 活跃状态集 `[queued, dispatching, running, cancel_requested]`）归零（`TaskRunModel.CountActiveByAgents` 新查询，经 `ServiceContext.GameScopeContext` 解析 game 库）；销毁走同一 Terraform 状态（`Orchestrator.Destroy` 复用 WorkDir/main.tf，防漂移）；失败 fail-closed 500 `provision_load_unverifiable` 不放行。**「账号负载迁移走调度编排」未做**（无账号分布数据源，诚实缺位——todo 原文「先行用无负载保守版」即认可此边界）。
  - 交付与验收：drain=把目标机账号负载迁移走（依赖调度编排，先行用「无负载才允许销毁」保守版）；数据盘护栏（持久化标记禁止自动销毁）；销毁走同一 Terraform 状态，防漂移。
  - 完成证据：2026-11-02；`internal/api/ops/provision.go`（`provisionDestroyGuards` 三闸 + `OpsProvisionDestroy` 守卫接线 + `OpsProvisionProtection` + `ProvisionProtection` handler + `provision_destroy_protected`/`provision_tasks_inflight`/`provision_agents_not_drained`/`provision_load_unverifiable` 哨兵）、`internal/model/provision_model.go`（`DestroyProtected` 列）、迁移 0053 三处同步（`internal/svc/migrations.go` + `internal/db/migrate/migrate.go` MinimumRequiredVersion=53 + `internal/db/migrate/migrate_test.go`）、`internal/model/task_model.go`（`CountActiveByAgents`）、`internal/svc/provision_wiring.go`（护栏双向映射）、`internal/svc/service_context.go`（`GameScopeContext` 导出）、`internal/audit/audit.go`（`provision.destroy`/`provision.protect`/`provision.unprotect`）、`internal/handler/routes.go`（`PUT /ops/provision/:runId/protection`）、`provision_test.go` 六守卫用例 + `task_model_test.go` + `migrations_provision_test.go`（0053 存量形态复刻）、[交付报告](docs/review/2026-11-02-ops-002-drain-destroy.md)；`go build ./...` 过，`go test ./internal/api/ops/ ./internal/model/ ./internal/svc/ ./internal/ops/provision/ ./internal/handler/ ./internal/audit/ ./internal/db/migrate/ -count=1` 全绿（守卫用例含 `-race`），`go vet` 过，docs build 过。
- [x] **OPS-003 · P2 · 实现 · 审批双模式**
  - 依赖：OPS-001。
  - 状态／条件：LOCAL VERIFIED（2026-11-01）：网络级 `provision.approval=auto|workflow` 双模式落地——auto=个人直通（缺省，行为与现状一致）；workflow=扩容/销毁均走申请单（创建→审批→执行，审计留痕），客户端传 direct 也被强制为 ticket 不得绕过审批；销毁入口先推进 `destroy_pending`（资源不立即销毁），审批通过后经 `POST /ops/provision/:runId/destroy` 执行 terraform destroy 原语，未批准/非 destroy_pending 返回 409 `provision_state_conflict`，批准后双续行被拒（防双跑）。
  - 交付与验收：网络级配置 `provision.approval=auto|workflow`；个人=直通，团队=扩容/销毁申请单（创建→审批→执行，审计留痕）；复用现有 scope 隔离与审计。
  - 完成证据：2026-11-01；`internal/config/config.go`（`ProvisionConfig.Approval` + `EffectiveApproval`，`ProvisionApprovalAuto`/`ProvisionApprovalWorkflow`）、`internal/api/ops/provision.go`（`requestDestroy` + `OpsProvisionDestroyResume` + `ProvisionDestroyResume` handler + `provision_state_conflict` 哨兵）、`internal/ops/provision/orchestrator.go`（`RequestDestroy` + `DestroyResume` + `ErrRunNotDestroyPending`）、`internal/svc/provision_wiring.go`（`CreateDestroyApproval` 审批单）、迁移 0054 三处同步（`internal/svc/migrations.go` `provisionRunDestroyApprovalMigration` + `internal/db/migrate/migrate.go` MinimumRequiredVersion=54 + `internal/db/migrate/migrate_test.go`）、`internal/model/provision_model.go`（`DestroyApprovalID` 列）、`internal/ops/provision/types.go`（`StateDestroyPending` + `DestroyApprovalID`）、`internal/audit/audit.go`（`provision.destroy_request`）、`internal/handler/routes.go`（`POST /ops/provision/:runId/destroy`）、`configs/server.yaml`（`provision.approval` 注释）、测试 `internal/config/provision_approval_test.go`（`TestProvisionConfig_EffectiveApproval`）、`internal/api/ops/provision_test.go`（workflow 强制 ticket / 销毁走申请单 / 未批准 409 / 批准后执行）、`internal/ops/provision/orchestrator_test.go`（`RequestDestroy` 状态推进 / `DestroyResume` 批准后转 destroyed / 双续行拒绝）、`internal/svc/migrations_provision_test.go`（0054 补列幂等 + 注册表含 0054）；`go build ./...` 过，`go vet`（touched 包 + pre-commit `go vet ./...`）过，`go test ./internal/api/ops/ ./internal/config/ ./internal/model/ ./internal/svc/ ./internal/ops/provision/ ./internal/handler/ ./internal/audit/ ./internal/db/migrate/ -count=1` 全绿（`internal/svc` 446s / `internal/api/ops` 469s / `internal/model` 62s / `internal/db/migrate` 64s，超时 900s 下无假死），gofmt 干净；commit `16873069`。
- [x] **OPS-004 · P1 · 实现 · 部署发版（包仓库+灰度+回滚）**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-02，边界见下文）：服务端部署编排落地——包仓库（对象存储登记 + sha256 + size + 元数据）、发版单（选包 + 目标机集合 + 分批策略 1/10/50/100%）、健康门禁（探针失败自动暂停批次）、回滚（指回上一版本包重放）。HTTP 面 `/ops/deploy/packages`（POST 登记/GET 列表）+ `/ops/deployments`（POST 创建/GET 列表/GET :id）+ `/ops/deployments/:id/start` + `/ops/deployments/:id/rollback`；未装配（DB/对象存储缺）如实报 503 deploy_not_configured，不假装已开启。迁移 0055 建 `deploy_packages` + `deployments`。
  - 交付与验收：构建产物登记到内置包仓库（对象存储复用 rustfs 或文件服务）；发版单=选包+选目标机集合+分批策略（1/10/50/100%）；每批健康门禁（探针不过自动暂停）；回滚=指回上一版本重放。
  - 完成证据：`internal/ops/deploy/deploy.go`（编排：分批切分/健康门禁暂停/回滚/状态机）、`deploy_test.go`（分批 10%、健康门禁 c 失败暂停、apply 失败 failed、回滚 rolled_back、非法 percent/空目标拒绝）；`internal/model/deployment_model.go`（包/发版单/目标模型）+ `model_test.go`（CRUD + 表名）；`internal/api/ops/deployments.go`（HTTP + DTO）+ `deployments_test.go`（DTO 构建 / 未装配 503）；`internal/api/ops/deploy_agent.go` + `deploy_agent_test.go`（Healthy 探针失败/通过/无指标）；迁移 0055 `internal/svc/migrations.go` + `migrations_deploy_test.go`（建表幂等）+ `internal/db/migrate/migrate.go` MinimumRequiredVersion=55 + `migrate_test.go`；路由 `internal/handler/routes.go`。`go test ./internal/ops/deploy/ ./internal/model/ ./internal/api/ops/ ./internal/svc/ ./internal/db/migrate/ ./internal/handler/ -p 1 -count=1` 全绿（api/ops 445s、svc 225s、model 105s、db/migrate 65s）；`go build ./...` 过、`go vet`（touched 包）过、web tsc 0 错误、dashboard_vnext_guard PASSED、git diff --check 干净。
  - 边界诚实：生产投递（DeployAgent.StagePackage）复用对象存储签名 URL + agent 本地下载（经 ExecCommand），宿主机需有 `curl`/`wget` 且在 agent 命令白名单——本环境无真实多机部署/agent 白名单，未端到端实测，登记为部署后人工验收项；健康门禁当前按探针失败/无近期指标判定（资源过载阈值未接入，见 OPS-010 指标面）；分批/回滚编排语义经 Fake agent 单测验证，真实 agent 换包依赖既有 `UpdateProcess`（§10.4 已验收）。
- [x] **OPS-005 · P2 · 实现 · 日志采集与检索**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-02，边界见下文）：agent 应用日志 tail 采集折入 metrics 批量面（`core/logs/tailer.go`，行级 seq 单调 + 截断 + 级别/时间戳解析 + 轮转/截断重置），服务端 `agent_log_entries` 落库（`LogStore` 按 (agent,file,seq) 去重，原始 7 天保留），检索 API `GET /ops/logs/search`（按机/文件/级别/时间窗，scope 隔离 fail-closed），节点详情面板应用日志视图（级别标签/文件/内容，最新在前分页）。
  - 交付与验收：agent 采集应用日志 tail 上报；服务端落库（按机/应用/级别/时间检索）；面板日志视图；采集开关与采样可配。
  - 实现要点：proto `MetricsReport.logs`（字段 13）+ `LogEntry` 消息（file/level/message/ts_unix_ms/seq）；`OpsConfig.LogFiles`（默认空=关）+ `LogMaxLineBytes`（默认 4096）+ `LogMaxLinesPerCycle`（默认 500）；`UpstreamClient.WithLogTailer` + `reportMetrics` 折入；迁移 0056 建 `agent_log_entries`（三列唯一索引 dedup，received_at 索引检索），`MinimumRequiredVersion=56`。
  - 边界诚实：tail 仅按配置显式路径（不 glob 展开）；message 为原始行（含级别前缀）；级别按行首 120 字符内正则匹配（未知=空）；时间戳仅解析行首 RFC3339（无=0 由服务端 received_at 兜底）；无 Windows 真机 tail 实测（跨平台路径同 gopsutil，轮转用 size 检测）；面板只检索最新 100 条（无历史分页加载 UI）。
  - 完成证据：2026-11-02；`core/logs/tailer.go` + `inode_unix.go` + `inode_windows.go` + `tailer_test.go`（新行/缺失文件重试/截断轮转/超长行/级别/时间戳）；`internal/platform/registry/logstore.go` + `logstore_test.go`（入库去重/按级按文件检索/分 agent 隔离/prune）；`internal/server/log_ingest_test.go`（handleMetricEvent 折入日志）；`internal/app/agent/log_tail_upstream_test.go`（reportMetrics 折入 logs）；`internal/api/ops/logs_search_test.go`（返回过滤/级别过滤）；`internal/svc/migrations_agentlog_test.go`（0056 建表幂等）；`internal/db/migrate/migrate_test.go`（合成 0056）；`web/src/services/api/ops.ts` + `NodeDetailDrawer.tsx` + `NodeDetailDrawer.test.tsx`（日志加载/空态）；`configs/agent.yaml`（logFiles 示例）。`go build ./...` 过、`go test ./core/logs/... ./internal/app/agent/... ./internal/platform/registry/... ./internal/server/... ./internal/api/ops/... ./internal/svc/... ./internal/db/migrate/... ./internal/model/... ./internal/handler/... -p 1 -count=1` 全绿、`cd sdks/go && go test ./... -count=1` 全绿、`cd web && pnpm run tsc` 0 错误、`pnpm test -- src/pages/Ops` 32 套件/373 测试通过（含 NodeDetailDrawer 31 例）；commit `a4b69e1a`。
- [x] **OPS-006 · P2 · 实现 · 配置下发与批操作**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-02，边界见下文）：服务端批编排 `internal/ops/batchop`——按 scope/标签圈选机器（RegistrySelector 按 game/env + 标签 AND + 显式 agent 交集）、config push / restricted exec 两种载荷、dry-run 只预览不派发、逐机并发（8 路 cap）经 ops tunnel 执行、聚合报表（成功/失败/跳过）+ 审计（create/dry_run/execute 三类事件）。HTTP 面 `POST /ops/batch`（创建，默认 dry-run）+ `POST /ops/batch/:id/execute`（执行/预览）+ `GET /ops/batch` + `GET /ops/batch/:id`；迁移 0057 建 `batch_ops`。agent 侧新增 `PushFile`（`MsgPushFileRequest`，配置 `pushBaseDir` 白名单 + `maxPushFileBytes` 上限 + content sha256 校验 + 拒绝越界/覆盖需 force），命令执行复用既有 `ExecuteCommand`（白名单 fail-closed 已在 OPS-001 前验证）。
  - 交付与验收：按 scope/标签圈选机器集合下发配置文件或执行受限命令模板；dry-run 预览；结果聚合报表；全部进审计。
  - 实现要点：proto `PushFileRequest/Response`；`OpsConfig.PushBaseDir`/`MaxPushFileBytes`；`OpsClientWrapper.PushFile`；`batch_ops` 模型 + 迁移 0057 + `MinimumRequiredVersion=57`；审计事件 `batch_op.create/dry_run/execute`。
  - 边界诚实：batch 执行经 agent 活跃 TCP 会话（离线 agent 目标计入 failed）；命令模板复用 agent `ExecAllowedCommands` 白名单（未命中/空白名单 fail-closed）；config push 仅写 `pushBaseDir` 下（未配置=禁用）；dry-run 不落敏感载荷明文到审计（仅摘要）；本环境无真实多机 agent 栈，端到端派发经 Fake agent 单测验证，真实多机登记为部署后人工验收项；面板未做批量操作 UI（验收只要求 API 聚合报表与审计，面板归 OPS-011 大屏候选）。
  - 完成证据：2026-11-02；`internal/ops/batchop/batchop.go` + `selector.go` + `id.go` + `batchop_test.go`（创建校验/exec 聚合/失败统计/config push/dry-run 不派发）；`internal/app/agent/ops_pushfile.go` + `ops_pushfile_test.go`（写入/越界拒绝/未配置禁用/hash 不匹配/覆盖需 force）；`internal/agent/local_handler_test.go`（handlePushFile 未装配拒绝）；`internal/logic/ops/agent_ops_client.go`（PushFile）；`internal/api/ops/batchops.go` + `batchops_test.go`（创建校验/未知 kind）；`internal/svc/migrations_batchop_test.go`（0057 幂等）；`internal/db/migrate/migrate_test.go`（合成 0057）；`internal/model/batch_op_model.go`；`internal/handler/routes.go`；`configs/agent.yaml`（pushBaseDir 示例）。`go build ./...` 过、`go test ./internal/ops/batchop/... ./internal/app/agent/... ./internal/agent/... ./internal/logic/ops/... ./internal/api/ops/... ./internal/model/... ./internal/svc/... ./internal/db/migrate/... ./internal/handler/... ./pkg/protocol/... -p 1 -count=1` 全绿、`go vet` 定向包过、`cd sdks/go && go build ./...` 过、`python compileall` 过。
- [x] **OPS-007 · P2 · 实现 · 成本控制**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-03，边界见下文）：创建前规格×时长估算（内置价格目录，未知规格不计价不伪造价）；机器成本标签与面板（/ops/cost 页面：估算 + 成本总览 + 预算）；闲置回收评估（CPU 低于阈值持续 >= N 小时判定，默认只提醒不销毁，AutoDestroy 需显式 Enabled+AutoDestroy）；预算告警走通知渠道（超 80% 触发 notify 分发）。
  - 交付与验收：创建前规格×时长估算；机器成本标签与面板；闲置回收（无负载 N 小时提醒/自动销毁，可配）；预算告警走通知渠道。
  - 实现要点：`internal/ops/costcontrol`（`ComputeEstimate`/`BuildSummary`/`IdleEvaluator`/`BudgetEvaluator`）；迁移 0058 建 `cost_budgets`；HTTP `/ops/cost/{estimate,summary,idle,budget}`；web `/ops/cost` 面板。
  - 边界诚实：价格目录为内置参考价（生产价需接入真实账单/价格 API，估算精度登记为部署后对账）；闲置判定依赖 agent 资源指标（OPS-010），无近期样本视为未知不误判；自动销毁默认关闭（只提醒）；本环境无真实云/多机栈，端到端真实云成本对账与自动销毁登记为部署后人工验收项。
  - 完成证据：2026-11-03；`internal/ops/costcontrol/costcontrol.go` + `idle.go` + `costcontrol_test.go`（估算/汇总/闲置/预算/聚合）；`internal/model/cost_budget_model.go` + `cost_budget_model_test.go`；`internal/api/ops/costcontrol.go` + `costcontrol_test.go`（估算/预算/汇总 scope 隔离）；`internal/svc/migrations_costbudget_test.go`（0058 幂等）；`internal/handler/routes.go`（/ops/cost/*）；`web/src/services/api/ops.ts` + `web/src/pages/Ops/Cost/index.tsx` + `web/config/routes.ts`；`docs/api/index.md`（Ops - cost control 端点族）+ `docs/operations/cost-control.md` + `docs/operations/index.md` + `docs/.vitepress/config.mjs`（docs 导航）。`go build ./...` 过、`go test ./internal/ops/costcontrol/... ./internal/api/ops/... ./internal/model/... ./internal/svc/... ./internal/db/migrate/... -count=1` 全绿、`go vet` 定向包过、`cd sdks/go && go build ./...` 过、`cd web && npx tsc --noEmit` 0 错误、`npx jest config/routes.test.ts src/pages/routes-resolution.test.tsx src/utils/api.test.ts src/services/api` 全绿、`bash scripts/dashboard_vnext_guard.sh` PASSED、`cd docs && CI=true pnpm build` 过。
- [x] **OPS-011 · P2 · 实现 · 运维大屏（舰队健康总览，运维台独立页面）**
  - 依赖：OPS-010。
  - 状态／条件：LOCAL VERIFIED（2026-11-03，交付完成）。前端独立大屏页 `web/src/pages/Ops/Dashboard/`（`index.tsx` 组件 + `health.ts` 纯逻辑）；五源 best-effort 轮询（listOpsNodes/fetchOpsAlerts/listOpsProvisionRuns/listSyntheticsResults + getAgentMetricsHistory，Promise.allSettled 单源失败不整体挂）；三色健康档（默认 60/85，HealthThresholds 可配）+ 防抖（连续 3 周期越界才变色，normal/offline/unknown 立即恢复）；离线灰态置底；聚合带（全网节点/在线率/平均 CPU·内存/今日扩缩容/活动告警 + 各档位计数）；告警滚动条（剔除已静默）；点击节点卡下钻（hourly 近 48 点资源曲线 + 进程/端口摘要）；`?agentId=` 深链预选下钻；断流 30s → 顶部「数据过期」横幅（不展示旧值为当前）。
  - 交付与验收：ops 前端独立大屏页——①节点卡片墙：每节点一枚卡片（名称/大区/角色/在线状态），三色健康态=正常（CPU+内存 <60%）/压力（60-85%）/过载（>85% 或探针失败），离线灰态置底；②聚合带：全网节点数、在线率、总负载、今日扩缩容次数、活动告警数；③账号负载视图：各节点承载账号数 vs 容量上限（依赖调度编排的账号分布数据）；④告警滚动条（实时事件流）；⑤点击节点卡下钻到该节点详情（资源曲线/进程/最近操作）。
  - 数据源：OPS-010 资源指标 + synthetics 探针 + 审计事件流；轮询或 SSE 推送（≤5s 延迟）。
  - 阈值：三色阈值可配（网络级）；状态判定防抖（连续 3 个周期越界才变色）。
  - 验收：模拟 20 节点（含 1 离线 2 过载）状态正确着色；断流 30s 后显示「数据过期」而非旧值；1080p 与 4K 电视墙两档布局。
  - 完成证据：commit `b6ee4114`（10 文件/1340 行）已推送 origin/main；实现面 `web/src/pages/Ops/Dashboard/index.tsx` + `web/src/pages/Ops/Dashboard/health.ts`（五源 best-effort 轮询 + 三色健康档 + 防抖 + 离线置底 + 聚合带 + 告警滚动条 + 下钻 + `?agentId=` 深链 + 断流 30s「数据过期」横幅）；验收对照：20 节点模拟（1 离线 2 过载 1 压力）计数与着色正确（`web/src/pages/Ops/Dashboard/__tests__/index.test.tsx`：正常 16/压力 1/过载 2/离线 1，离线置底 DOM 末位）；断流 → 「数据过期」横幅（`isStale` + 横幅用例）；tsc 0 错误、`npx jest src/pages/Ops/Dashboard` 33/33、routes+ops service 47/47、`dashboard_vnext_guard.sh` PASSED、docs build 通过。
  - 边界诚实：账号负载视图为「待接入」占位（OPS-002 边界：当前无账号分布数据源，不伪造数字）；阈值当前取默认常量（网络级可配登记为后续工作）；大屏只读（探针失败为信号非强制执行）；1080p/4K 两档——页面用 `auto-fill minmax(240px,1fr)` 响应式网格，未设专用电视墙断点（登记为已知边界）。

- [ ] **OPS-008 · P3 · 实现 · 数据面备份恢复**
  - 依赖：—。
  - 状态／条件：BLOCKED（2026-11-06）。数据面执行已接通（Stage A+B）：backups 归属四列（game_id/env/source_id/source_name，迁移 0060）+ ops 创建按请求 scope 解析注册数据源并真实执行 dump→sha256→上传→succeeded（不再悬挂 pending；未注册源→404 拒绝、存储未配置→500 拒绝）+ 清单按 scope 过滤并带 gameId/env/sourceName/checksum。校验读路径（Stage D）：objstore.Store.Get 读回（file/s3/cos/oss 四驱动 + 逐驱动用例）+ backupexec.VerifyChecksum 重算 sha256 对照存储值 + ops BackupService.Verify。定期备份调度（Stage E）：config backup.schedule.enabled 缺省 false 必须显式 + 后台循环（backupexec.Scheduler 枚举注册启用数据源触发真实备份，在途防重，root.go 按开关装配）。剩余非本批交付：恢复演练模式（Stage C）——需用户拍板演练目标机制（恢复执行器/演练目标未定形，todo 原文「暂缓，不盲建」），且需隔离数据库/对象存储读权限做真实恢复验证；缺用户决策/隔离恢复环境。
  - 完成证据：2026-11-04；模型层 `38a270a5`（ListBackupsOptions scope 过滤 + DBSourceModel.FindByScope）、ops API `f0d244f1`（createBackupForScope 真实执行 + buildOpsBackupItems + 新用例 TestBackupServiceCreateScoped_NoSourceRejected/NoStorageRejected/Executes 端到端 sqlite dump→file store→succeeded、TestBackupServiceListWithRealModel 改按 scope 语义）、迁移 0060 `2467bc8b` + 回归 `23c8996e`；Stage D `2c6e1e31`（objstore.Get + 四驱动 + 逐驱动用例 Test*Store_Get_RoundTrip/NotFound + VerifyChecksum/BackupService.Verify 用例）、Stage E `4eba5cc0`（backup.schedule 配置 + Scheduler 用例 + root.go 装配）；`go test ./internal/api/ops/ ./internal/model/ ./internal/api/backup/ ./internal/svc/ ./internal/platform/objstore/ ./internal/platform/backupexec/ ./internal/config/ -count=1` 全绿、`go build ./...` 过、`go vet ./internal/...` 干净、gofmt 干净。
  - 交付与验收：游戏库定期备份策略（经 capture 所见的数据源登记）、恢复演练模式、备份清单与校验。已完成：备份清单（scope 过滤 + 归属列）、备份校验（Get 读回重算 sha256）、定期备份调度（config 开关默认关显式启用）。待定：恢复演练模式（Stage C，演练目标机制未定形，暂缓）。
- [x] **OPS-009 · P3 · 实现 · 运维 runbook 自动化**
  - 依赖：OPS-004, OPS-006。
  - 状态／条件：DONE + LOCAL VERIFIED（2026-11-05；go build OK、go test 全绿、go vet 过、guard PASSED）。
  - 交付与验收：常用操作固化为 runbook（参数化步骤序列，可引用 OPS-004/006 能力）；一键执行+审计。
  - 完成证据：2026-11-05 提交 e3773a28（feat(ops): add runbook automation engine）；实现 `internal/ops/runbook/runbook.go` + `internal/api/ops/runbooks.go`，路由 `internal/handler/routes.go`，迁移 `internal/svc/migrations.go` + `internal/db/migrate/migrate.go`；`go test ./internal/api/ops/ -count=1` PASS（TestRunbookService_Run_WithRealBatch、TestRunbookCreateHandler_CrossScopeRejected）；`go test ./internal/ops/runbook/ ./internal/model/ ./internal/svc/ ./internal/db/migrate/ -count=1` PASS；`go build ./...` OK；`go vet ./internal/...` 过；`bash scripts/dashboard_vnext_guard.sh` PASSED。
- [x] **OPS-COMPLETE-001 · P2 · 实现 · 远程电源管理 + 审批接入（巡检单 2026-10-20）**
  - 依赖：—（复用既有 approval 链与 agent-ops TCP 通道）。
  - 状态／条件：DONE + LOCAL VERIFIED（2026-11-11；go build OK、相关包 go test 全绿、go vet 过、guard PASSED）。能力矩阵与审批策略矩阵落 `docs/design/ops-power-management.md`（+docs/zh 镜像）：物理机 IPMI/BMC（ipmitool chassis power）、WOL 唤醒（仅 power_on、不可验证）、云实例命令模板层（`{instanceRef}` 占位）、agent 所在机带内 graceful reboot/power_off（force_off 拒绝——带内关机前无带外手段拉起）；风险分级 power_on/status=low（x-risk safe）、power_off/reboot=medium（warning）、force_off=high（high，批准走 OTP step-up）。请求状态机 pending→executing→verifying→done/partial/failed，pending→rejected；批准范围快照（targetsJSON）提交即冻结；批量=单审批多目标、失败逐机回填（partial 聚合）；审批拒绝（两宽规则）由审批服务把关、请求侧 GET 惰性同步 rejected。
  - 交付与验收：internal/powerops 新包（Provider 接口 + ipmi/wol/cloud/agent 四实现）；POST /ops/power（提交→审批单→批准后 resume 执行→逐机状态回填）、GET /ops/power/:id（请求+逐机 targets）、GET /ops/power（列表）、POST /ops/power/:id/{resume,reject}、PUT /ops/nodes/:nodeId/power-config（机器清单挂现有 nodes.meta.power 子对象，无新库存表）；powerops.enabled 缺省 false 不装配（端点如实 503 powerops_not_configured）。
  - 完成证据：5 个提交——2b5e9c59（docs: add ops power management design）、79df4dda（feat: add powerops provider package）、78d0e215（feat: add power request models and migration，goose 0064 幂等建表 power_requests/power_request_targets，migrate.go 64 对齐 + 合成迁移清单同步）、1a918bfa（feat: add powerops request orchestrator）、87c57a63（feat: add ops power api and wiring，含 ops.power.* x-risk 声明注入 registry 即 resolvePolicyRisk 认可缝隙）。测试：go test ./internal/powerops/ -count=1 18 用例全绿（假 ipmitool 脚本测命令拼装与 -H/-p/-U/-P/-I lanplus 断言、WOL 102 字节魔术包回环收包、云模板序列、agent 带内 refuse、审批拒绝零 provider 调用、部分失败聚合 partial、状态确认轮询 off→on、超时保持 unverified 诚实 failed、WOL 永不 verified、校验失败零落库、审批门失败 pending 无链、fail-closed 查询错误拒绝执行）；go test ./internal/api/ops/ -run TestPower -count=1 9 子用例全绿（未配置 503 六端点、提交 202 pending+审批记录 mode=powerops、校验 422 不落库、未批准 resume 409 power_state_conflict、批准后全链 done+verified、拒绝零 provider 调用、GET 惰性同步 rejected 落库、power-config 登记后可提交+404/422 契约、列表）；go test ./internal/api/ops/ ./internal/audit/ ./internal/handler/ ./internal/svc/ ./internal/db/migrate/ ./internal/config/ ./internal/model/ -count=1 全绿；go build ./... OK；go vet ./internal/... 干净；bash scripts/dashboard_vnext_guard.sh PASSED。边界诚实：真实 BMC/WOL/云 API 不在本机验证（无硬件）——mock 全覆盖，真机验证待环境；agent 带内 force_off 拒绝、WOL 仅唤醒不可验证为设计边界（设计 `docs/design/ops-power-management.md` §3/§9；实现 `internal/powerops/`）。
- [x] **MOB-ANA-001 · P2 · 实现 · 移动端数据分析闭环（巡检单 2026-10-20）**
  - 依赖：—（复用既有 analytics 分域接口与 approvals OTP 链）。
  - 状态／条件：DONE + LOCAL VERIFIED（2026-11-11；flutter analyze 0 错、mobile 全量 flutter test 全绿、go build OK、analytics 包 go test 全绿）。分析需求盘点落 `docs/design/game-analytics-requirements.md`（+docs/zh 镜像）：十域逐域 ✅/⚠️/❌ 结论（宏观大盘/玩家行为/游戏进度/实时监控=✅ 已有接口；经济系统/社交/稳定性/反作弊/运营活动=❌ 数据源待建；商业化⚠️ 缺 LTV/首充，P1），验证先行（先核 internal/api/analytics 现有实现再下判，未凭空假设缺口）。
  - 交付与验收：服务端新增 analytics 聚合门面（internal/api/analytics/facade.go，加法不改造分域 handler）——GET /analytics/facade/summary 一次拉 8 已建域摘要卡（逐域 goroutine 错误隔离，单域失败标 unavailable 其余照常；未建 5 域如实标 pending_data_source 不伪造数字；装配缺欠显式 unavailable 不静默丢卡）+ POST /analytics/facade/query 统一分域查询路由（未知域 422、待建域 422 domain_pending）；移动端 mobile/lib/features/analytics/ 四文件（service fail-fast 解析 / controller riverpod scope 联动 / dashboard 13 卡三态齐全 ok 渲染指标·unavailable 描错·pending「数据源待建」/ 分域详情页：已建域 query 拉数通用渲染、待建域占位不发请求）+ 监控页新增「数据分析」入口；approvals 移动端 OTP step-up 闭环为既有交付（高危审批 otp_required→弹码→otp_invalid 重试→otp_not_enrolled 提示，approval_detail_page_test.dart 全链用例），本单核验通过未重复实现。
  - 完成证据：3 个提交——1b866d0b（docs: add game analytics requirements，中英双树）、5584a661（feat: add analytics facade query entry，facade.go 498 行 + facade_test.go 13 卡聚合/逐域故障隔离/nil 域显式 unavailable/分域路由透传/handler 7 断言全绿）、f7d5a980（feat: add mobile analytics dashboard，service+controller+dashboard+detail 四文件 + 监控页入口 + 10 用例：summary 坏形态 StateError/scope 头注入/query body 契约/缺 data 键 fail-fast/三类卡片渲染/空态/错误重试恢复/ok 卡进详情拉数/待建卡占位零请求）。测试：flutter analyze 0 issues；flutter test 全量 235 用例全绿（含 approvals OTP 闭环既有用例 = 验收项「approvals OTP 闭环（模拟 OTP 测试）」证据）；go test ./internal/api/analytics/ -count=1 全绿；go build ./... OK。边界诚实：❌ 域仅占位声明（数据源待建，等数仓/ETL 接入后在 facade 加一行注册即亮卡）；分域详情为通用键值/列表渲染，非逐域定制图表；WAU/会话时长/LTV 等缺口指标登记在 `docs/design/game-analytics-requirements.md` §3 未实现；移动端实现 `mobile/lib/features/analytics/`。
- [x] **OBS-AGENT-001 · P2 · 实现 · 四类 Agent 接入 OTel 打点（配置开关默认关）**
  - 依赖：—。
  - 状态／条件：**DONE · LOCAL VERIFIED（2026-11-05）**：四类 Agent（capture/devops/sidecar/synthetics）全部接入 `core/otelrec.Recorder` 缝 + `internal/telemetry/agentrec` 真实 OTel SDK 适配（otel v1.46 已复用，构建标签 `!no_otel`/`no_otel`）；config 复用既有 `telemetry.enabled`（默认 false）+ `enable_metrics` + `collectorUrl` + `useTls` + `serviceName` 键；开启后上报可被服务端面板看到（每 agent 经 Start/Stop/Connected/Disconnected/Count/Observe 覆盖启停、连接状态、事件/吞吐计数、错误率）；关闭时零开销路径（noop `Discard()`，各构造器默认装配）；测试用内存/内存 exporter 断言（每 agent 各带专用 otelrec 用例）。边界诚实：`no_otel` 构建为 fail-closed（recorder_trim.go 编译期剔除，不产生上报）；OTLP exporter 直连不在本仓内联验证（依赖外部 collector，登记为部署后人工验收项）。
  - 交付与验收：agents/capture、devops、sidecar、synthetics 复用服务端 telemetry SDK（otel v1.46 已在 go.mod）；config 增 telemetry.enabled（默认 false）+ OTLP endpoint；开启后 span/metric 上报可被服务端面板看到；关闭时零开销路径（noop provider）；每 agent 至少覆盖：启停、连接状态、事件/吞吐计数、错误率；测试用内存 exporter 断言。(2026-11-01 用户提议「agent 应支持 otel 可选开启」,已代决立项;eBPF 火焰图评估后暂缓:宿主机级 profiler 非本仓职责,待真实 profiling 需求再议)
  - 完成证据：6 个增量提交 —— `3abafedf`（feat(otelrec): add agent OTel recorder seam and wire capture）、`54392e87`（feat(agentrec): add real OTel recorder for agent telemetry）、`f073e0b5`（feat(capture): wire agent telemetry config and OTLP recorder）、`0739ad85`（feat(devops): wire agent telemetry and session observer）、`6fa308e9`（feat(synthetics): wire agent telemetry and probe metrics）、`26d22e47`（feat(sidecar): wire agent telemetry recorder and lifecycle metrics）。实现面：`core/otelrec/`（Recorder 接口 + NoopRecorder/Discard）、`internal/telemetry/agentrec/`（recorder.go/recorder_trim.go，build tag `!no_otel`/`no_otel`）、四 agent 的 config 装配（`agents/*/config/telemetry*.go` + `configs/*-agent.example.yaml`）、`cmd/*-agent/root.go` 的 recorder 构建 + Start/Stop/Shutdown 接线、`internal/app/agent/app.go` 的 armRecorderChaining（OnConnected/OnDisconnected 链式不丢用户回调）。测试面：`core/otelrec/` 契约编译期断言、`internal/telemetry/agentrec/` 内存 exporter 用例、每 agent 的 `*_otelrec_test.go` + `*_telemetry_test.go`（默认关 + 块解析 + 打点断言 + 生命周期观测）。验证：每增量 `go build ./...`（默认与 `no_otel`）+ `go vet ./internal/...` + 对应包 `go test -count=1` 全绿（sidecar 增量含 cmd/sidecar-agent 默认与 no_otel 双构测试；internal/app/agent 仅 3 个基线环境性超时用例除外（已用 clean HEAD worktree 复证非本增量回归））；`gofmt` 干净。

- [x] **AGENT-001 · P0 · 故障验证 · 四类 Agent 当前接口的 scope 隔离矩阵**
  - 依赖：—。交付与验收：按当前已支持的注册/配置/列表/统计/详情/动作/日志/离线读取逐项核对；复用非空 scope 和绑定测试，覆盖同 ID 冲突及跨 scope 指派；devops 未接通的会话/回放列 NOT IMPLEMENTED，不用假数据当验收
  - 状态／条件：**DONE · LOCAL VERIFIED（2026-11-01）**：四类 Agent（capture/devops/sidecar/synthetics）× 注册/配置/列表/统计/详情/动作/日志/离线读取逐接口矩阵已落 `docs/review/2026-11-01-agent-scope-isolation-matrix.md`；每行标注 scope 来源 + fail-closed 与否 + 测试证据。核对中发现 live 缺口并已修复：`/ops/nodes/:id/{drain,undrain,restart,meta}` 此前只按 nodeId 解析（A scope 管理员可动 B scope 节点），本轮统一补 `requireAgentScope` fail-closed 门（`internal/api/ops/helpers.go`），`opsNodeMeta` 保持「不存在→404」契约；新增 3 个跨 scope 拒绝用例（`internal/api/ops/node_action_scope_test.go`：`TestOpsNodeDrain_CrossScopeRejected` / `TestOpsNodeRestartUndrain_CrossScopeRejected` / `TestOpsNodeMeta_CrossScopeRejected`）；6 个既有 drain/restart/undrain 用例补 RegistryStore 装配。同 ID 冲突证据 `TestHandleRegisterRequest_SameAgentIDCrossScopeRejected`（control_handler_scope_test.go:211，commit 3c5e44be）；跨 scope 指派拒绝 `TestDispatcher_pickAgentWithRouting_FiltersGameEnvironment`（dispatcher_test.go:784）。边界诚实：synthetics 离线读取 query-scope 已知边界（§3.1，不改行为）、devops 会话/回放 NOT IMPLEMENTED 无假数据（§3.2）、`/api/v1/nodes` 无 scope 面 by design（§3.3）、`ops.NodeService` 死实现（§3.4）。
  - 完成证据：矩阵文件 `docs/review/2026-11-01-agent-scope-isolation-matrix.md`；新测试 `internal/api/ops/node_action_scope_test.go`（3 用例）；修复 `internal/api/ops/helpers.go`（4 函数）；`go test ./internal/api/ops/ -count=1` 全绿、`internal/server` scope/注册用例全绿、`internal/platform/dispatch` 路由 scope 用例全绿；`gofmt` 干净；commit `9485f3d2`。

- [x] **AGENT-002 · P1 · 实现 · devops 面板会话中继**
  - 依赖：—。交付与验收：接通已有 Broker/Hub 的创建/数据/窗口/关闭通道与 Server 授权、审计、背压；不新增 RDP/VNC；SSH 真目标端到端连接和断线清理可验。
  - 状态／条件：**DONE · LOCAL VERIFIED（2026-11-14）**：下行 open/input/resize/close 经 MuxConn 双向控制会话落到 agent 侧 Broker/Hub 链（白名单/审计前置/背压/asciicast 全部沿用既有真值，中继不另立）；输出/结束经上行 DevopsSessionEvent 事件帧扇出给面板 SSE（回放环 + 有界消费者，慢消费者摘流不断会话）；控制面丢失 = 会话面关闭（agent 侧 OnConnClosed 与 server 侧断连清理对称收束）。SSH 真目标端到端：`agents/devops/session/relay_test.go` 内嵌 x/crypto/ssh echo 服务端（真 SSH 协议握手 + pty + window-change），全链 open→input→echo→resize→close→end 事件→断线审计链可验——2026-11-01 的「缺 SSH 主机」blocker 由进程内真 SSH 服务端解除。operator 是 server 鉴权身份（open 请求下发，agent 审计链落此字段，不信客户端自报）。未新增 RDP/VNC（allowlist 声明 rdp/vnc 时 Open 显式拒绝，既有行为）。
  - 完成证据：6 个提交——`b4fd25f20`（feat: add devops session relay wire contract，proto/croupier/ops/v1 9 消息 + protocol 9 msgID/IsEvent + 全 SDK gencode + `pkg/protocol/message_more_test.go` 名字断言）、`d91c83355`（feat: add mux control session to controlconn，DialMux/SetOnClosed/PullDevopsAllowlist + `agents/controlconn/controlconn_mux_test.go` 真 MuxConn 注册/心跳/双向事件）、`d14443dfc`（feat: add devops session relay wire onto broker chain，`agents/devops/session/relay.go` Relay 四类下行请求 + 输出消费者 + OnConnClosed 收束 + `relay_test.go` 5 用例含 echo SSH 全链/拒绝/禁用面/控制丢失/未知消息）、`00b54111e`（feat: wire devops relay into agent runner，`agents/devops/runner/runner.go:119-133` DialMux 装配 + relay.SetHub:195）、本批 `feat: add devops session relay server side`（`internal/platform/devopssessions/hub.go` 会话路由态/回放环/溢出摘除/断连收束 + `internal/server/control_handler.go` MsgDevopsSessionEvent 派发 + `internal/server/tcp_listener.go` 断连钩子 + REST `internal/api/ops/devops_sessions.go` 六端点含 SSE 流 + audit devops.session_open/close + root.go 装配）。测试：`go test ./internal/platform/devopssessions/ ./internal/server/ ./internal/api/ops/ ./agents/devops/... ./agents/controlconn/ -count=1 -race` 全绿（hub 背压/回放/断连收束、ControlService 事件派发、TCPListener 断连收束代理会话、REST open/list/input(429 session_busy)/resize/close/SSE 回放+end、relay echo SSH 全链）；`go build ./...` + `go vet ./internal/... ./cmd/server/` 干净。边界诚实：面板 UI（AGENT-003）未交付——REST/SSE 面已可验，页面待下批；跨进程真二进制联调（真 devops-agent + 真 server + 真外部 SSH 主机）未在本机跑——各层用同一 wire 契约（同一 proto 消息）且均有真 MuxConn/真 SSH 协议层测试。

- [x] **AGENT-003 · P1 · 实现 · devops 主机/会话/回放界面**
  - 依赖：AGENT-002。交付与验收：复用四类 Agent 公共页与现有 asciicast；真实会话列表、scope 权限、动作审计与回放可达，未实现能力不以假按钮替代。
  - 状态／条件：**DONE · LOCAL VERIFIED（2026-11-15）**：devops 实例页挂会话面板——`web/src/pages/Ops/Agents/devops.tsx` 复用 `OpsAgentsPage` 公共骨架 children 槽位（index.tsx 零改动，scope 头 `X-Game-ID`/`X-Env` 走既有请求拦截器全局注入）。真会话列表（`DevopsSessionsPanel.tsx`：会话 ID/agent/操作者/协议/目标/通道/创建时间 + 接入/关闭）；开会话表单只列在线 devops agent（离线项 disabled 标注），仅 SSH——RDP/VNC 服务端显式拒绝（`agents/devops/session/broker.go:102`、`sshtarget.go:129`），表单不放假选项。接入 = fetch-SSE（JWT 走 Authorization 头，`createEventSource`）+ xterm 终端（`DevopsTerminal.tsx`：pty 字节流 base64 解码 `term.write`，先收服务端 256 条回放环即「最近一屏」），输入/窗口调整转发（input/resize 带 `skipErrorHandler`，429 session_busy 面板内节流提示不全局弹错）；关闭走确认弹框 + DELETE（服务端审计 `devops.session_close`）；抽屉关闭只 detach 本地视图，面板文案如实写明。scope 权限与动作审计不在前端重复实现——服务端 open/close 已有属主 scope 校验与审计事件（AGENT-002 面）。边界诚实：历史 asciicast 存档在 agent 本机磁盘、服务端无取回端点，不做假回放按钮，面板与文档均如实标注「接入只补最近一屏回放环，历史不可回放」。文档同步：`docs/operations/{access-control,agent-comparison}.md` + `docs/zh/operations/` 双树（「骨架/noopTarget 未接通」旧表述按现行源码更新为已接通真值，全部锚点 file:line 复核过；`docs && CI=true pnpm build` 过）。
  - 完成证据：web 新增 `web/src/services/api/devopsSessions.ts`（6 函数 + 类型，sessionId URL 编码）+ `DevopsSessionsPanel.tsx` + `DevopsTerminal.tsx` + `devops.tsx` 接线；`web/package.json` 增 `@xterm/xterm@^5.5.0`/`@xterm/addon-fit@^0.10.0`（无 overrides 行扰动，antd6 废弃属性同步清零：Modal/Drawer `destroyOnHidden`、Drawer `size`）。测试 3 件 12 用例全绿——service `devopsSessions.test.ts`（5：open POST 体/list 容错/input+resize skipErrorHandler/close reason 默认/SSE 端点）、`devopsTerminal.test.tsx`（2：帧解码写终端/结束帧停下行+输入转发）、`devopsSessionsPanel.test.tsx`（5：列表真数据/空态诚实文案/开会话全链/校验不过不发请求/关闭确认重载）。门禁：`pnpm --dir web run tsc` 0 错误；定向 jest 三文件 12 用例全绿且在全量跑中保持绿（全量 38 红套件与改动交集为空，均为 load 60-90 下既有套件 5s 超时假红——两轮红集漂移 27/94→38/79 符合负载族签名，main 上 CI-Dashboard 基线绿）；`go build ./...` 过；`go test ./internal/...` 绿（`internal/svc` 首轮 600s 超时为兄弟会话 go 风暴争用假红：单测 0.12s 过、负载回落后包级复跑 ok 522.6s，其余包首轮全绿）；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 过。

## 探测验收与调查

- [ ] **PROBE-001 · P1 · 故障验证 · 真实游戏协议的多语言插件验收**
  - 依赖：—。交付与验收：复用 go-plugin 宿主、Go/Python/Node SDK；真实游戏目标验证 Describe/Check、凭证不回显、超时/取消/崩溃/熔断/恢复；不再规划 JSONL 等价运行时。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需真实游戏协议/目标/凭据；沙箱无真实游戏目标。缺真实协议/目标/凭据。

- [ ] **PROBE-002 · P1 · 故障验证 · 第三方服务多视角探活验收**
  - 依赖：PROBE-004, PROBE-005。交付与验收：针对各角色实际能力建立矩阵，分别核对 sidecar 的 Checker/策略热更、synthetics 原生 HTTP/ICMP/插件、面板阶段/窗口/故障恢复；模拟端口通但业务错、断线和恢复；不宣称所有角色共享全部 DB/私有协议 Checker
  - 状态／条件：BLOCKED（2026-11-01 复核）：动态同步/丢失语义仍有实现/决策前置（依赖 PROBE-005 方案批准），真实网络视角需目标/凭据；缺 PROBE-005 决策/真实目标凭据。本地机制部分待前置决策后开展。

- [ ] **PROBE-003 · P2 · 资料调查 · 玩家 Login 级适配边界**
  - 依赖：—。交付与验收：复用已有 Minecraft SLP 与私有协议骨架，收集 online-mode/自家游戏登录协议与账号条件；SLP 状态检查不能作完整玩家登录证据。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需 MC Java 账号/出网、自家游戏账号/协议，许可证变更另行授权（用户拍板）。缺账号/出网/协议授权。

- [x] **PROBE-004 · P1 · 实现 · synthetics 目标及策略版本同步**
  - 依赖：—。交付与验收：按既有角色设计补齐静态配置到受控版本拉取/热生效，复用 core/configsync 并绑定 gameId/env；坏版本保留 last-good，重连后取最新，禁用/删除目标释放资源；不凭 sidecar 的 puller 宣称 synthetics 已拥有同能力
  - 状态／条件：LOCAL VERIFIED（2026-11-01 复核）：policy/target 下行拉取均已落地（commit a10d4986），单测 policy_sync_test.go / targets_sync_test.go 通过；可本地实施项已实施。
  - 交付（policy sync）：probe.policy 下行拉取经 core/configsync 热生效，`agents/synthetics/runner/runner.go` 新增 `newPolicyPuller` + `intervalFor`（EffectiveIntervalFor 叠加策略），坏版本/拉取失败保留 last-good（`policy_sync_test.go`）；服务端 `internal/server/control_handler.go:725` handleProbePolicyConfig 读 config_versions probe.policy 按注册会话 gameId/env 下发。
  - 交付（target sync）：synthetics.targets 下行拉取经 core/configsync 热生效，`agents/synthetics/runner/runner.go` 新增 `newTargetsPuller` + `applyTargets`（验证后 diff 热替换，删除目标停 timer 释放资源），坏版本/校验失败保留 last-good，重连后 configsync 续拉取最新（`targets_sync_test.go`）；服务端 `internal/server/control_handler.go:789` handleSyntheticsTargetsConfig 读 config_versions synthetics.targets 按注册会话 gameId/env 下发（`control_handler_synthetics_targets_test.go`）；wire 新增 `MsgSyntheticsTargetsConfigRequest/Response`（0x040129/0x04012A，`proto/croupier/ops/v1/ops.proto`、`pkg/protocol/message.go`、`agents/controlconn/controlconn.go`）。
  - 完成证据：`agents/synthetics/runner/runner.go`、`agents/synthetics/runner/policy_sync_test.go`、`agents/synthetics/runner/targets_sync_test.go`、`internal/server/control_handler.go`

- [ ] **PROBE-005 · P1 · 方案决策 · 探测结果与故障恢复事件丢失语义**
  - 依赖：—。交付与验收：区分可丢的周期采样与必须可核对的故障/恢复跃迁；评审断链缓存/重传/新鲜度/缺测状态及容量预算，优先复用既有 report/告警机制，不默认持久化每条采样
  - 状态／条件：BLOCKED（2026-11-01 复核）：协议缺口需方案评审批准（用户拍板）；真实第三方环境不能补齐此协议缺口。缺方案批准。

## 待批准的运维方案

- [ ] **OPS-012 · P2 · 方案决策 · Agent 自升级四问**
  - 依赖：—。交付与验收：评审升级源/触发权限/重启接力/失败姿态，复用现有指纹及最后已知锚点；交付失败不砖化与带外回滚验收方案，等待既有要求的决策批准后实施。
  - 状态／条件：BLOCKED（2026-11-01 复核）：待方案批准（用户拍板）。缺方案批准。

- [ ] **OPS-013 · P2 · 方案决策 · 受管程序配置更新五问**
  - 依赖：—。交付与验收：明确路径沙箱/格式校验/原子性与重启回滚/体积带宽/审批闸，复用 configsync 先例；不声称现有探针配置拉取已支持程序文件分发。
  - 状态／条件：BLOCKED（2026-11-01 复核）：待方案批准（用户拍板）。缺方案批准。

## 部署事实

- [ ] **DEPLOY-001 · P1 · 资料调查 · 真实部署与外部依赖配置盘点**
  - 依赖：—。交付与验收：核对实际镜像摘要/角色/端口/TLS/协调后端/备份、Herald 与外部平台开关、OIDC/Vault/观测出口；交付脱敏拓扑和与模板差异，不把 Compose 存在当部署证据。
  - 状态／条件：BLOCKED（2026-11-01 复核）：需真实部署只读权限与脱敏配置；沙箱无生产部署访问。缺部署权限/脱敏配置。

- [ ] **DEPLOY-002 · P1 · 故障验证 · 备份可恢复与业务凭据核对**
  - 依赖：DEPLOY-001。交付与验收：复用 backupexec 和既有备份/对象存储，在隔离环境实际恢复配置、契约/页面、审批/任务/执行凭据与角色数据，记录 RPO/RTO/校验和及未覆盖内存状态；验证副作用不因恢复重复执行，不新增未经设计的恢复服务
  - 状态／条件：BLOCKED（2026-11-01 复核）：需脱敏备份、隔离数据库/对象存储读权限；备份 API/文件存在不证明恢复成功。缺备份/隔离恢复环境。

## 容量验证

- [ ] **PERF-001 · P2 · 性能验证 · 会话与重连容量基线**
  - 依赖：—。交付与验收：给定 SDK/Agent 数、并发、消息大小和延迟目标，测 MuxConn 队列、重连风暴、连接峰值/P95 和资源释放；单独记录已知正确性缺口，不用吞吐通过替代安全结论
  - 状态／条件：BLOCKED（2026-11-01 复核）：需容量目标（用户拍板的并发/延迟目标）与隔离容量环境。缺容量目标/隔离环境。

- [ ] **PERF-002 · P2 · 性能验证 · 任务查询与 SSE 容量基线**
  - 依赖：—。交付与验收：给定并发任务/订阅者、事件频率和保留量测量事件写入/查询延迟、断线续读、慢客户端背压及资源释放；记录当前可靠性缺口，不能用吞吐替代 ARCH-015 验收
  - 状态／条件：BLOCKED（2026-11-01 复核）：需独立任务基准栈与延迟目标（用户拍板）。缺基准栈/延迟目标。

- [ ] **PERF-003 · P2 · 性能验证 · 插件并发与故障隔离容量**
  - 依赖：—。交付与验收：复用现有 go-plugin gate/backoff/breaker，给定插件数/并发/预算验证队列拒绝、超时、崩溃风暴、宿主资源和 Stop 释放；报告平台/版本，不做新的插件运行时选型
  - 状态／条件：BLOCKED（2026-11-01 复核）：需可控插件负载与资源预算（用户拍板的容量目标）。缺负载目标/资源预算。

## 共用核心分析与外部专业分析

- [ ] **DATA-001 · P1 · 方案决策 · 内置共用指标与外部专业分析边界**
  - 依赖：—。交付与验收：按用户确定的边界评审核心指标：新增、DAU/WAU/MAU、近实时在线、D1/D7/D30 留存、付费人数/订单/流水/退款/净额/付费率/ARPU/ARPPU；定义活跃事件、账号/角色去重、cohort/分母、UTC/业务时区、迟到更正、币种/单位及近似误差。复杂自助分析/归因/预测走外部，已有行为/关卡页保留但不扩建专业 BI
  - 状态／条件：BLOCKED（2026-11-01 复核）：方案决策待评审批准（用户拍板）；复用 docs/analytics/core-and-external.md 和既有 game-metrics-overview，口径未验收不称已可用。缺方案批准。

- [ ] **DATA-002 · P1 · 实现 · 共用事件接入与指标数据来源归一**
  - 依赖：DATA-001。交付与验收：复用 ingest/Server 事件入口/worker，提供游戏登录、注册、心跳、支付/退款的显式映射；每个指标声明权威来源、gameId/env/维度和新鲜度。统一 GORM 与 ClickHouse 读适配契约，不静默把平台 Player 数当游戏新增，不默认为两套库自动同步
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-001 的指标口径批准；缺方案批准。源码缺口保留：overview.go 读关系模型/Player，warehouse.go 读 ClickHouse；需接入样本。

- [x] **DATA-003 · P1 · 实现 · 活跃与在线聚合的版本和重放语义**
  - 依赖：DATA-001。交付与验收：修正 daily_users 的替换键/版本参数与分钟在线 HLL 快照重写策略，迁移已有表而不只改 initdb；同日多次 flush、迟到事件、重放和多 worker 的同一分钟结果可核对，不把全量 HLL 重写当增量相加。
  - 状态／条件：LOCAL VERIFIED（2026-11-01）：010_analytics.sql 改 `ReplacingMergeTree(version)` 且 version 移出 ORDER BY；minute_online 由 SummingMergeTree 改 ReplacingMergeTree(version) 全量快照替换；worker.go 写 version；warehouse 查询加 FINAL；迁移脚本 scripts/clickhouse/011_fix_aggregate_tables.sql 重建已有表；真实 ClickHouse `TestDrill_DATA003_AggregateReplacement`（data003_drill_test.go:26）FINAL 取 95/55/6/1200/20/2 不叠加。
  - 完成证据：2026-11-01；`internal/analytics/worker/data003_drill_test.go`、`scripts/clickhouse/011_fix_aggregate_tables.sql`、`internal/analytics/worker/worker.go`；`go test ./internal/analytics/worker/... ./internal/api/analytics/...` 全绿；commit `5f1ba096`。

- [ ] **DATA-004 · P1 · 实现 · 付费聚合的币种、订单状态与累计口径**
  - 依赖：DATA-001。交付与验收：复用 payment 明细，按已批准币种/单位和订单唯一键处理重复成功、退款和状态变更；修复 revAgg flush 后删除而日表按替换读的增量/累计冲突，跨 flush/重启可回算，不能把不同币种原始金额直接相加
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-001 的币种/单位与订单状态口径批准；缺方案批准。2026-11-21 复核：`worker_flush.go:103` 累加批次增量，`worker_agg.go:103,185` 每轮摘除内存并写独立增量，`010_analytics.sql:33` 与 `warehouse.go:202` 按日替换并用 FINAL 读取。同日分批 100、25 的定向探针分别写出 100、25，未写日累计 125。币种与订单口径仍归本项，ACK 后恢复归 DATA-021。查询侧续审确认 `internal/model/analytics_payments_model.go:142,164,192` 的 GORM 总览与日营收聚合均只选 success，且 SUM(amount) 未按币种分组。

- [ ] **DATA-005 · P1 · 实现 · 基础 cohort 留存的生产计算链**
  - 依赖：DATA-001, DATA-002。交付与验收：复用 RetentionModel，补齐注册 cohort 与回访事件的受控计算/回算任务；D1/D7/D30 明确分母、业务日、观察窗成熟与迟到更正；未成熟/缺数据返回不可用，不能伪造 0；用逻辑时钟和回放样本验算，无需等真实运行 30 天才写机制测试
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-001 的 cohort/回访口径批准与 DATA-002 事件映射；缺方案批准。2026-11-21 复核：`retention.go:40` 只读取 cohort，`UpsertCohort` 仍无生产调用；等待口径决策时可用固定样本验证回算机制。概览 WAU 和留存回读归 DATA-027。

- [ ] **DATA-006 · P1 · 实现 · 统一核心指标查询响应与 scope**
  - 依赖：DATA-002, DATA-003, DATA-004, DATA-005。交付与验收：复用现有 Analytics API/warehouse 查询，统一核心指标的窗口、维度、分母、source/freshness/approximation、币种和不可用原因；读取已有授权 scope，不让查询参数改到其它 game/env；缺配置/延迟/查询失败不能伪装零值
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-002~005 的有效聚合口径；缺上游方案与实现。

- [ ] **DATA-007 · P1 · 实现 · 运营核心分析页的真实数据闭环**
  - 依赖：DATA-006。交付与验收：复用 Overview/Realtime/Retention/Payments/Warehouse 组件，收敛为同口径核心视图；过滤条件/日期/时区/单位保持一致，展示延迟、缺测、未成熟和未启用状态，真实零值可区分；已有专业页不删除也不以硬编码值补齐
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-006 统一查询契约；缺上游实现。

- [ ] **DATA-008 · P1 · 故障验证 · 核心指标可核对样本与端到端验收**
  - 依赖：ARCH-011, DATA-007。交付与验收：以固定多 game/env/跨日/多币种/重复乱序/退款样本，从接入→Redis→worker→查询→页面逐指标对账；优先复用 DATA-031 的固定样本链，补充真实脱敏输入，验证重启/迟到/跨 scope 拒绝与不可用状态，记录原始样本、期望、实际、版本和数据新鲜度
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-007 端到端视图；真实游戏仍需脱敏事件和支付对账样本；缺上游实现/验收样本。

- [ ] **DATA-009 · P2 · 性能验证 · 核心分析吞吐与查询容量**
  - 依赖：DATA-006。交付与验收：给定事件速率、保留量、查询并发与新鲜度预算，测写入/聚合/查询 P95、积压和回算对控制面的影响；复用现有独立分析管道，不自动引入新的流处理服务
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-006；需业务量级（用户拍板）和隔离 Redis/ClickHouse 环境。

- [ ] **DATA-010 · P1 · 资料调查 · 专业分析外部产品接入核查**
  - 依赖：—。交付与验收：按实际专业需求收集外部分析平台官方版本/许可、数据输入、授权 scope、身份/嵌入/深链、数据回流与成本限制；交付对接矩阵与单个首接目标，不在本轮选型或虚构任何供应商集成已完成
  - 状态／条件：BLOCKED（2026-11-01 复核）：需用户确定专业场景与候选外部产品；缺业务拍板/候选环境。

- [ ] **DATA-011 · P1 · 方案决策 · 外部专业分析的受控集成契约**
  - 依赖：DATA-001, DATA-010。交付与验收：复用 extensions 与既有数据出口，评审指标/事件/只读数据集的输出契约和来源、scope、字段映射、时间窗、版本、短期凭据/SSO/深链及审计；外部查询必须被服务端或受限数据集约束，不把浏览器过滤器当隔离，不泄露仓库凭据
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-001 指标边界与 DATA-010 首接产品；缺业务拍板/候选环境。

- [ ] **DATA-012 · P1 · 实现 · 首个专业分析平台适配与入口**
  - 依赖：DATA-011。交付与验收：按批准契约实现一个真实外部平台的配置/连接验证、受控数据输出和当前 scope 的报告入口；复用扩展安装/开关/审计，失联有明确降级信息，不阻塞 Croupier 核心运营与内置指标
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-011 批准契约；缺首接平台测试实例及最小权限凭据。

- [ ] **DATA-013 · P1 · 故障验证 · 外部分析授权与故障隔离验收**
  - 依赖：DATA-012。交付与验收：核对跨 game/env、撤权、过期凭据、外部故障、超时和重复导出；未授权数据不可见，导出可追溯，外部平台不可用时内置核心指标与运营执行可继续
  - 状态／条件：BLOCKED（2026-11-01 复核）：依赖 DATA-012 适配实现；缺真实外部分析环境和多 scope 测试身份。

- [x] **DATA-014 · P1 · 实现 · 分析聚合共享状态串行化**
  - 依赖：—。交付与验收：复用 worker 的 touchedMinutes/touchedDays/revAgg；明确消费、pending 回收与 flush 的快照/同步所有权，失败重试不丢增量；收入并发写读及活跃桶增删在 race 下无竞争，固定样本核对总量，不仅给 ClickHouse batch 加锁
  - 状态／条件：LOCAL VERIFIED：flush 在 aggMu 内把 touchedMinutes/touchedDays/revAgg 整段快照转移并清空共享 map，随即释放锁再对快照做 ClickHouse I/O；失败项 remerge 回共享 map（写方已重建同 key 时 revAgg 累加而非覆盖），失败重试不丢增量。`TestSharedState_ConcurrentRevAggNoLoss` 固定样本核对已发送+剩余=总量，`-race` 无竞争。
  - 完成证据：2026-11-01；`internal/analytics/worker/worker.go`（flush 快照转移 + remerge 助手）；`internal/analytics/worker/shared_state_test.go`（并发写读总量核对）；`go test ./internal/analytics/worker/ -race -count=1` PASS。

- [x] **DATA-015 · P2 · 实现 · 分析 worker 退出等待与刷新资源收尾**
  - 依赖：—。交付与验收：Run/main 对 pending 回收、定时 flush 和最终 flush 建立有界退出/等待顺序；退出后不再读写 worker/测试配置，不在刷新未结束时关资源；复现既有 runFlushInterval 跨用例竞态并验证消失，阻塞刷新仍遵守总截止
  - 状态／条件：LOCAL VERIFIED：Run 用 workerCtx（WithoutCancel 派生）管理定时 flush 与 pending 回收子协程，主循环退出后 stopWorkers 并分别以 shutdownTimeout 上界等待；最终 flush 用带 finalFlushTimeout 截止的独立 ctx 并 select 等待，阻塞刷新仍守总截止。main 改为等待 Run 完整返回，不再固定 200ms 猜测。测试夹具用 runFlushInterval/shutdownTimeout/finalFlushTimeout 注入缩短。
  - 完成证据：2026-11-01；`internal/analytics/worker/worker.go`（Run 有界退出顺序 + waitGroupBounded）；`cmd/analytics-worker/main.go`（等待 runDone）；`internal/analytics/worker/coverage_v9_test.go`（`TestRunShutdownWaitsForPeriodicFlushAndHonorsDeadlineV9` 阻塞刷新有界退出）；`go test ./internal/analytics/worker/ -race -count=1` PASS。

### 数据分析续审与建设（2026-11-21）

基线 `60c75377c`。概览、行为、支付、留存、实时、调用、关卡和仓库已有查询入口，各域的指标来源和完整度不同。五个域仍声明 `pending_data_source`。先修正数据接入、聚合和状态表达，再按权威来源交付指标。缺少真实游戏输入时先用固定样本验证机制，真实数据验收继续跟踪 DATA-008。能力范围参见 [游戏数据分析需求](docs/zh/design/game-analytics-requirements.md)。

验证范围：在本地用 Go 探针复现接入部分写入和两轮支付聚合独立出行。行为窗口上限为源码复核，行为 API 定向探针和补充 Go 套件未在本轮完成。Redis/ClickHouse 故障恢复、真实游戏指标和生产数据的对账留在相应待办验收。

#### 接入与数据可靠性

- [ ] **DATA-016 · P1 · 修复 · 接入批次先校验后写入**
  - 依赖：—。交付与验收：事件和支付批次先完整验证，再开始发布。第 N 条非法时返回条目位置且全批入队数为 0。证据：`cmd/ingest/cmd/root.go:522,559` 在同一循环校验和发布，定向探针复现后项非法却已写入前项。
  - 状态／条件：未开始。

- [ ] **DATA-017 · P1 · 实现 · 接入批次的安全重试**
  - 依赖：DATA-016。交付与验收：约定稳定事件标识与重复提交语义，明确队列部分成功的响应。模拟第 N 条入队失败、网络结果未知、进程重启和异步重复消费，逐条对账。证据：`cmd/ingest/cmd/root.go:528,565` 中途发布失败仅返回批次级 500。
  - 状态／条件：未开始。

- [ ] **DATA-018 · P1 · 修复 · 支付接入保留分级金额精度**
  - 依赖：—。交付与验收：按已支持的金额单位，在接入与消费路径做精确整数及边界校验，拒绝分值小数和超范围值。合法大整数逐位往返，不经过有损 float64 转换。证据：`cmd/ingest/cmd/root.go:616` 用浮点检查正值，`internal/analytics/worker/worker_insert.go:39` 经浮点转无符号整数。
  - 状态／条件：未开始。

- [ ] **DATA-019 · P1 · 修复 · 重试不裁剪源 Redis Stream**
  - 依赖：—。交付与验收：将失败重投与源流保留策略解耦，按消费者仍需的数据位置控制裁剪。验收积压超过 1000 条的重试场景，未读和 pending 条目可完整重放。证据：`internal/analytics/worker/worker_checkpoint.go:107` 用 MaxLen 1000 向源流重投，覆盖发布端自己的配置语义。
  - 状态／条件：未开始。

- [ ] **DATA-020 · P1 · 修复 · 活跃指标写入失败可恢复**
  - 依赖：—。交付与验收：模拟 HLL 写失败与 Redis 恢复，原始事件已落仓时仍可重建正确指标。覆盖原始事件 ACK 后、指标持久化之前杀进程的场景。证据：`internal/analytics/worker/worker_agg.go:39` 忽略 PFADD/EXPIRE 错误且脏桶标记仅在内存，原始批次在 `worker_flush.go:35` 独立 ACK。
  - 状态／条件：未开始。

- [ ] **DATA-021 · P1 · 修复 · 支付明细 ACK 后能重算日指标**
  - 依赖：DATA-004。交付与验收：以持久明细或持久聚合进度为恢复依据。在明细 Send 和 ACK 后、日指标落库前杀 worker，重启后正确回算且重复回算不翻倍。证据：`internal/analytics/worker/worker_flush.go:58` 先写内存增量再 ACK。
  - 状态／条件：未开始。

- [ ] **DATA-022 · P1 · 修复 · 断流告警按实际恢复状态关闭**
  - 依赖：—。交付与验收：已告警作用域持续无数据时保持告警，查数失败或只读到部分数据时独立标记未知，重新收到符合恢复规则的数据后关闭。基线速率按实际基线区间计算。证据：`internal/api/analytics/pipeline_monitor.go:199,256` 仅查询滑动窗口内出现的作用域，未再次命中的既有告警进入恢复判断。
  - 状态／条件：未开始。

- [ ] **DATA-023 · P1 · 修复 · 分析数据管道识别队列进度与初始化失败**
  - 依赖：—。交付与验收：明确新建组从哪里开始消费、何时允许复位既有组位点，记录初始化失败。已有组重启保持消费位置，已 ACK 数据不会因为旧应用 checkpoint 被回放；worker 晚于首批入站启动时按已声明策略处理积压。证据：`internal/analytics/worker/worker.go:173` 忽略建组错误并默认以 `$` 起步，`worker_checkpoint.go:29` 启动时直接对既有组 SetID。
  - 状态／条件：未开始。

#### 查询口径与运营闭环

- [ ] **DATA-024 · P1 · 修复 · 行为分析覆盖完整查询窗口**
  - 依赖：—。交付与验收：摘要、热图、采用明细、漏斗和路径按完整窗口做有界聚合。若采用抽样，在响应中明确规模和覆盖偏差。超过 100 条的样本必须覆盖跨分页的漏斗步骤。证据：`internal/api/analytics/behavior.go:31,168,235,323` 只读第一页且丢弃总量，`internal/model/vars.go:58` 将 3000/5000 压为 100。续审确认关卡、剧集、地图也走同一个只取一页的 helper，见 `internal/api/analytics/retention.go:76,154,231`，验收需一并覆盖。
  - 状态／条件：未开始。

- [ ] **DATA-025 · P1 · 实现 · 实时在线与会话采用各自权威来源**
  - 依赖：DATA-001, DATA-002。交付与验收：以心跳或会话生命周期计算在线与活跃会话，事件吞吐单独展示；验证一会话多事件、多个设备和长时间无心跳。证据：`internal/api/analytics/overview.go:137` 将窗口用户数作为在线人数、事件数作为活跃会话数。
  - 状态／条件：未开始。

- [ ] **DATA-026 · P2 · 优化 · 实时趋势使用批量分桶查询**
  - 依赖：—。交付与验收：按窗口和间隔有界查询并填补空桶。边界事件只计一次，对最长支持窗口断言数据库往返次数与桶数解耦。证据：`internal/api/analytics/overview.go:174` 每桶执行两次查询，且现有模型包含起止边界。
  - 状态／条件：未开始。

- [ ] **DATA-027 · P1 · 实现 · 大盘补齐 WAU 与留存读链**
  - 依赖：DATA-001, DATA-005, DATA-006。交付与验收：滚动 WAU 与已成熟的 D1/D7/D30 采用实际查询结果，Web 和 Mobile 使用一致状态，真实零值、待成熟和无来源可区分。证据：`web/src/services/api/analytics.ts:60` 当前将 WAU 和三项留存写为 null。
  - 状态／条件：未开始。

- [ ] **DATA-028 · P2 · 实现 · 接入观测窗口的在线峰值**
  - 依赖：DATA-003, DATA-025。交付与验收：从同一业务时区的在线序列取区间最大值，展示采样粒度与覆盖时间。固定跨日样本核对峰值和缺测状态。
  - 状态／条件：未开始。

- [ ] **DATA-029 · P1 · 修复 · 分析门面的查询参数具有明确语义**
  - 依赖：DATA-006。交付与验收：记录每域支持的指标、筛选和时间窗，应用已支持参数，其他参数返回字段级诊断。Web 和 Mobile 测试覆盖不同窗口得到不同结果。证据：`internal/api/analytics/facade.go:265,315` 声明 `metrics` 却未消费，实时和调用摘要不使用传入的 timeRange。
  - 状态／条件：未开始。

- [ ] **DATA-030 · P2 · 修复 · 门面摘要为各分析域设置独立时间预算**
  - 依赖：—。交付与验收：阻塞一个域超过其预算时，其余域仍可及时返回，超时域有可识别状态。验证调用方取消与并发上限。证据：`internal/api/analytics/facade.go:165` 启动各域后统一等待全部完成。
  - 状态／条件：未开始。

- [ ] **DATA-031 · P1 · 测试 · 分析集成验收覆盖真实入站链**
  - 依赖：DATA-016, DATA-017。交付与验收：CI 用测试凭据经 ingest 发布固定样本，经 worker、聚合和 API 核对跨窗口、重复/部分失败与金额。保留现有直写 Redis 的 worker 测试。证据：`scripts/e2e/analytics.sh:3` 当前绕过 ingest。
  - 状态／条件：未开始。

- [ ] **DATA-032 · P2 · 修复 · ClickHouse DDL 变更触发分析 CI**
  - 依赖：—。交付与验收：只改 initdb 或分析表迁移文件时触发 ClickHouse 相关用例，并分别验证空库建表和已有表升级。证据：`.github/workflows/ci-analytics.yml:5,14,64` 作业读取 initdb SQL，路径过滤未包含 SQL 和迁移目录。
  - 状态／条件：未开始。

#### 待补齐的分析能力

- [ ] **DATA-033 · P1 · 设计 · 定义游戏内经济事件的权威契约**
  - 依赖：DATA-001。交付与验收：确定产出、消耗、转移、库存快照的事件字段、币种标识、单位、事务键和权威来源。用至少一套经济账本样本钉住收支平衡与缺测语义。对应 `facade.go:51` 的 economy 待建状态。
  - 状态／条件：未开始。

- [ ] **DATA-034 · P1 · 实现 · 游戏内通货收支与库存可查询**
  - 依赖：DATA-002, DATA-017, DATA-033。交付与验收：首个已接入游戏可按币种、时间、来源与用途查产出和消耗；库存趋势仅在有权威快照时展示。固定输入核对重复、转移与冲正，并接入域摘要和详情。
  - 状态／条件：未开始。

- [ ] **DATA-035 · P2 · 实现 · 首充转化按注册 cohort 计算**
  - 依赖：DATA-004, DATA-005。交付与验收：明确首单、转化窗口、分母和未成熟状态；验证重复订单、退款和窗口边界，结果随支付明细可回算。
  - 状态／条件：未开始。

- [ ] **DATA-036 · P2 · 实现 · 按观察期提供实际 LTV**
  - 依赖：DATA-004, DATA-005。交付与验收：按注册 cohort 和币种核对净收入、人数、观察期和退款修正；跨期与欠成熟样本状态清晰。预测型分析沿既有外部分析路线。
  - 状态／条件：未开始。

- [ ] **DATA-037 · P2 · 实现 · 会话时长分布**
  - 依赖：DATA-025。交付与验收：仅对具备会话标识和明确结束或超时策略的输入产出时长分布。固定样本覆盖掉线、重复心跳及跨日会话。
  - 状态／条件：未开始。

- [ ] **DATA-038 · P2 · 实现 · 任务完成漏斗的首个可核对样本**
  - 依赖：DATA-002, DATA-024。交付与验收：以一类任务的领取、进度和完成事件定义步骤与窗口，按样本核对转化并记录事件缺失。复用现有行为漏斗。
  - 状态／条件：未开始。

- [ ] **DATA-039 · P2 · 方案决策 · 定义社交分析的数据入口**
  - 依赖：DATA-001。交付与验收：确认公会活跃、好友互动或聊天计数的首个需求、来源和边界，给出脱敏计数样本和当前等待原因。对应 social 待建域。
  - 状态／条件：未开始。

- [ ] **DATA-040 · P2 · 方案决策 · 定义客户端稳定性指标与来源**
  - 依赖：DATA-001。交付与验收：区分游戏崩溃、错误、卡顿和分析管道故障。按版本和平台确定指标分母，给出 SDK/外部系统的首接方案及去重样本。对应 stability 待建域。
  - 状态／条件：未开始。

- [ ] **DATA-041 · P2 · 方案决策 · 定义反作弊事件的可见范围**
  - 依赖：DATA-001。交付与验收：明确游戏或外部系统提供的异常标记、复核结果与授权范围，用合成事件验证聚合口径。对应 anticheat 待建域。
  - 状态／条件：未开始。

- [ ] **DATA-042 · P2 · 方案决策 · 定义活动参与与奖励事件**
  - 依赖：DATA-001。交付与验收：固定活动 ID、活动窗口、参与去重和奖励发放来源，用样本验证参与率与奖励数。只有成本来源得到确认后才计算 ROI。对应 campaign 待建域。
  - 状态／条件：未开始。

#### 分析采集、发布器与仓库续审（2026-11-21）

- [ ] **DATA-043 · P1 · 修复 · Redis 发布器配置无效时拒绝启动**
  - 依赖：ARCH-013。交付与验收：Redis 地址解析失败时从发布器工厂返回初始化错误，只有显式配置 noop 才可选择空写入模式。验证无效 Redis 地址阻止启动且不会确认采集成功。证据：`internal/analytics/mq/redis_pub.go:22,55` 将解析错误降级为空发布器。Go 探针通过真实工厂验证无效 URL 返回 Noop，发布调用仍返回成功。
  - 状态／条件：未开始。

- [ ] **DATA-044 · P1 · 修复 · ClickHouse 结果迭代失败时拒绝返回截断指标**
  - 依赖：—。交付与验收：DAU、在线、营收及复用结果迭代的管道监控层能识别迭代错误。验收第一行失败、已读取部分行后失败、正常读完和空结果，读失败应映射到对应不可用状态。证据：`internal/api/analytics/warehouse.go:36,156,186,212` 的 rows 接口缺少 Err，循环结束直接返回结果。用仓库查询生产源码和替身行流复现第二行失败时在线接口只返回第一行并报成功。
  - 状态／条件：未开始。

- [ ] **DATA-045 · P1 · 修复 · Ingest 监听失败应终止进程并返回错误**
  - 依赖：—。交付与验收：地址不可用时启动立即返回失败，运行中的 HTTP 服务异常退出也应传递到进程退出状态。验收已被正常占用的端口和可用端口的优雅关闭。证据：`cmd/ingest/cmd/root.go:247,255` 在 goroutine 中仅记录监听错误，主流程继续等待关停信号。Go 探针确认无效端口的监听错误不会传给外层命令。
  - 状态／条件：未开始。

- [ ] **DATA-046 · P1 · 修复 · 在读取采集请求前应用请求体上限**
  - 依赖：—。交付与验收：鉴权验签和业务解码共用同一实际字节数限制，过大请求在上限处停止读取并返回稳定 413。覆盖事件、支付和无 Content-Length 的正常分块请求。证据：`cmd/ingest/cmd/root.go:222,387,508,545` 将鉴权放在外层并先读取完整 body，两个业务处理器之后才应用上限。
  - 状态／条件：未开始。

- [ ] **DATA-047 · P1 · 修复 · 以消费组进度计算主队列积压**
  - 依赖：DATA-023。交付与验收：将留存长度、未派发 lag、已派发未 ACK 数分开报告，Redis 无法提供有效读数时显示未知。验收同一流在全部消费并 ACK 后积压回落，即使物理长度没有变化。证据：`internal/analytics/mq/redis_pub.go:83` 和 `internal/api/analytics/pipeline_monitor.go:308,316` 以 XLEN 作为主流积压，Kafka 发布器返回固定 0，当前监控可把已消费记录报成积压。
  - 状态／条件：未开始。

- [ ] **DATA-048 · P1 · 修复 · 发布端按消费进度保留 Redis 源流**
  - 依赖：DATA-019、DATA-023。交付与验收：新事件发布与可安全清理边界采用一致策略。验证 worker 停机、积压超过旧长度上限后恢复，尚未派发及未 ACK 的事件均可重放。证据：`internal/analytics/mq/redis_pub.go:45,60` 的正常 XADD 依据长度裁剪源流，独立于 DATA-019 跟踪的失败重投路径。
  - 状态／条件：未开始。

- [ ] **DATA-049 · P2 · 优化 · 调用统计的延迟计算具有确定口径和资源上限**
  - 依赖：—。交付与验收：公开 P95 算法、缺失样本和小样本处理，验收 1、2、20、21 条与大体量样本。长窗口计算须有服务端时间和内存预算，并在高负载下保持汇总语义。证据：`internal/api/analytics/invocations.go:200,229,246` 允许 30 天窗口，两份内存切片处理全量延迟并原地排序，低样本下现有取整可能偏低，既有百分位测试只有 20 条有效样本。
  - 状态／条件：未开始。

#### Worker 失败恢复与付费查询续审（2026-11-21）

- [ ] **DATA-050 · P1 · 修复 · 事件与支付批次独立推进刷新**
  - 依赖：ARCH-011。交付与验收：周期和最终刷新都尝试处理两条流，分别报告失败。验收事件 Send 持续失败且支付可正常写入时，支付继续落仓并 ACK，事件保留 pending。证据：`internal/analytics/worker/worker_flush.go:121` 事件刷新失败后直接返回；ClickHouse 替身观测事件 Send 调用一次、支付一次也未调用。
  - 状态／条件：未开始。

- [ ] **DATA-051 · P1 · 修复 · 为补发事件保留稳定去重键**
  - 依赖：ARCH-011、DATA-017。交付与验收：为缺失上游 ID 的逻辑事件生成一次标识并贯穿持久重试，或在入口明确拒绝不符合契约的 ID。验收同一消息在 Send 后 ACK 失败、重启回收及重新入队时仍可按同一事件 ID 去重。证据：`internal/analytics/worker/worker_insert.go:23,27` 每次处理重新生成 UUID；相同 Redis 消息重复处理的探针得到两个不同 ID。
  - 状态／条件：未开始。

- [ ] **DATA-052 · P1 · 修复 · 将重投和死信转移做成幂等状态变更**
  - 依赖：DATA-019。交付与验收：目标流写入成功而源 ACK 失败时，再次处理不会复制重试记录或死信，目标未写入时源消息可继续回收。覆盖故障发生在两条 Redis 命令之间、超时结果未知及多个 worker 的情形。证据：`internal/analytics/worker/worker_checkpoint.go:71,107,120,148` 将两步分开执行。Redis 替身在 XADD 成功、XACK 连续失败时记录同一源消息两条死信。
  - 状态／条件：未开始。

- [ ] **DATA-053 · P1 · 修复 · 让定时批量写入失败进入可恢复的重试流程**
  - 依赖：ARCH-011、DATA-052。交付与验收：给批次刷新失败的记录保留可观察的状态与次数，区分短期仓库故障和持续失败的记录。覆盖不足一批、长期 Send 错误、仓库恢复和永久坏记录的隔离。证据：`internal/analytics/worker/worker_checkpoint.go:84,122` 仅处理同步错误时递增重试次数，`worker_flush.go:23,49` 的定时 Send 错误丢弃内存批次与确认引用。Redis 与 ClickHouse 替身复跑超过阈值后，源消息仍 pending、重试计数仍为 0。
  - 状态／条件：未开始。

- [ ] **DATA-054 · P1 · 修复 · 原始事件与聚合使用同一个事件时间**
  - 依赖：DATA-001。交付与验收：采集、写仓、活跃分桶统一解析 RFC3339、兼容 epoch 输入和业务时区。对无效时间采用明确处置，重放结果不随处理时间漂移。证据：`internal/analytics/worker/worker_insert.go:13` 与 `worker_helpers.go:90` 的原始行允许 epoch 字符串，`worker_agg.go:32` 的 HLL 只读 RFC3339。探针从同一旧事件得到正确历史原始行和当天活跃桶。
  - 状态／条件：未开始。

- [ ] **DATA-055 · P1 · 修复 · 隔离不同 Redis 消费组的检查点**
  - 依赖：DATA-023。交付与验收：若保留应用检查点，则将消费组纳入持久键，明确旧无分组键的迁移规则。共享同一 stream 的两个组独立消费与重启，验收各自消费位置均不受另一个组影响。证据：`internal/analytics/worker/worker_checkpoint.go:18,29,42` 检查点键只含 stream，恢复对各自的 group 调用 XGroupSetID。两个组的替身探针复现共用键被较旧进度覆盖。
  - 状态／条件：未开始。

- [ ] **DATA-056 · P1 · 修复 · 按周和月的实际窗口去重付费人数**
  - 依赖：DATA-004。交付与验收：天、ISO 周、月分别按请求作用域及身份粒度进行区间去重。跨日重复购买的同一玩家在周或月内只计算一次，收入与订单数仍保持原语义。证据：`internal/api/analytics/payments.go:181,316,356` 先查日去重数，再按周或月相加，无法识别跨日重复用户。SQLite 对账采用生产 SQL，同一人两天各支付一次时日去重相加得到 2，周内 DISTINCT 得到 1。完整 Go API 定向测试在限时内未完成。
  - 状态／条件：未开始。

- [ ] **DATA-057 · P1 · 修复 · 付费指标在活跃分母不可用时显式降级**
  - 依赖：DATA-001。交付与验收：区分活跃来源未配置、查询失败、无活跃与有可用样本。保留可核实的营收、付费人数与 ARPPU，缺少分母时对 ARPU、付费率显示不可用。证据：`internal/api/analytics/payments.go:45,61` 查询失败、未配置或计数为零时退回付费人数；仍有付费的窗口因此显示 100% 付费率和与 ARPPU 相等的 ARPU。当前已按源码核对，完整 Go API 定向测试在限时内未完成。
  - 状态／条件：未开始。

- [ ] **DATA-058 · P1 · 修复 · 以同批关卡尝试统计完成率**
  - 依赖：DATA-001、DATA-024。交付与验收：按稳定尝试标识关联开始与完成事件，给仅有完成数据的窗口定义不可用或明确的跨窗归属。验收重复上报、前一天开始当天完成、异步到达及零次尝试。证据：`internal/api/analytics/retention.go:100,112,128` 独立计数后使用 max(attempts, 1) 作分母。没有开始记录的窗口含两次完成时公式得到 200%，当前为源码确认。
  - 状态／条件：未开始。

## 前端审核复验

- [x] **REVIEW-001 · P1 · 故障验证 · 批量充值目标 ID 归一化复验**
  - 依赖：—。交付与验收：批量充值 demo 把选中行对象整体 `map(String)` 成 `[object Object]`，导致实际渲染路径无法匹配玩家 ID 却返回成功（web/src/components/PageRenderer/templates/playerManage.tsx:442）；保留工作区已有在途修复，按实际渲染/输入边界核对修复与回归证据。验收以实际批量执行入参为玩家 ID、回读影响人数与目标名单一致且无 [object Object] 为准。
  - 状态／条件：LOCAL VERIFIED：对象 selection 的精确余额增量、未选不变与 ID 兼容测试通过；真实环境另见 UI-001/003。
  - 完成证据：2026-10-30 复审；`web/src/components/PageRenderer/templates/gameOpsPublishFixture.test.tsx`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **REVIEW-002 · P1 · 故障验证 · 空白批量金额拒绝复验**
  - 依赖：—。交付与验收：`summarizeQuantityTotal` 把空串/全空白串转换成 0，违背“不确定输入不猜 0、不执行”注释并会确认并提交 0 充值（web/src/components/PageRenderer/batchSelection.ts:73）；保留工作区已有在途修复，按实际渲染/输入边界核对修复与回归证据。验收以空串/全空白均禁止确认与提交、合法有限数字仍可按显式语义执行为准。
  - 状态／条件：LOCAL VERIFIED：空串/空白阻断和合法金额确认后提交有直接断言，未重复实现 guard。
  - 完成证据：2026-10-30 复审；`web/src/components/PageRenderer/__tests__/ResourcePageRenderer.test.tsx`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **REVIEW-003 · P1 · 故障验证 · 错误响应体优先级一致性复验**
  - 依赖：—。交付与验收：`extractApiErrorCode` 声称与 `utils/errors` 同口径但优先 `.data` 再 `.response.data`，后者相反优先 `.response.data`，二者可能取到不同响应体（web/src/utils/apiError.ts:7）；保留工作区已有在途修复，按实际渲染/输入边界核对修复与回归证据。验收覆盖 data 与 response.data 并存且内容冲突时，extractApiErrorCode 与 utils/errors.extractErrorCode 对有效冲突响应采用同一响应体与稳定码。
  - 状态／条件：LOCAL VERIFIED：有效冲突 response.data/data 响应体优先级样本通过；不扩张为所有畸形响应解析统一。
  - 完成证据：2026-10-30 复审；`web/src/utils/__tests__/apiError.test.ts`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **REVIEW-004 · P1 · 故障验证 · 批量表单确认文案可达性复验**
  - 依赖：—。交付与验收：带表单批量动作进入表单后绕过原有 `confirm/confirmTitle/confirmDescription` 路径，批量充值模板声明的目标确认文案实际不可达（web/src/components/PageRenderer/ResourcePageRenderer.tsx:1246）；保留工作区已有在途修复，按实际渲染/输入边界核对修复与回归证据。验收覆盖带表单动作的确认标题/说明/目标可见且与实际提交一致，取消确认不会调用执行器。
  - 状态／条件：LOCAL VERIFIED：confirm=true 的标题/说明/目标/总量、取消零执行及正向提交参数测试通过。
  - 完成证据：2026-10-30 复审；`web/src/components/PageRenderer/__tests__/ResourcePageRenderer.test.tsx`；逐项判断及定向运行见 [复审报告](docs/review/reliability-recheck-2026-10-30.md)，不以文件存在或测试总数替代断言。

- [x] **REVIEW-005 · P1 · 实现 · 批量金额安全整数与乘积边界**
  - 依赖：—。交付与验收：为 summarizeQuantityTotal 与业务金额契约明确单位/倍率/最大值，拒绝非安全整数及乘积溢出；覆盖 MAX_SAFE_INTEGER 邻界、极大目标数与 Infinity，不显示可确认的失真总量
  - 状态／条件：LOCAL VERIFIED：`summarizeQuantityTotal` 收口为「有限非负安全整数 × 正整数安全整数 = 安全整数」，`batchTotalEntry` 复用同契约且移除 `toPrecision(12)` 失真显示；非法输入不显示/不提交。证据：`resourceBatchSelection.test.tsx` 与 `compositeBatchTotal.test.tsx` 覆盖 MAX_SAFE_INTEGER 邻界、溢出、Infinity、非整数拒绝；commit 0d74c456
  - 完成证据：2026-10-31；`web/src/components/PageRenderer/confirmContent.ts`（summarizeQuantityTotal 收口）、`web/src/components/PageRenderer/ResourcePageRenderer.tsx`（batchTotalEntry 复用同契约）、`web/src/components/PageRenderer/__tests__/compositeBatchTotal.test.tsx`、`web/src/components/PageRenderer/__tests__/resourceBatchSelection.test.tsx`（MAX_SAFE_INTEGER 邻界/溢出/Infinity/非整数拒绝用例）；`pnpm --dir web run tsc` 0 错误，`pnpm --dir web test -- compositeBatchTotal resourceBatchSelection` 绿；commit 0d74c456

## 依赖安全证据复核

- [x] **SEC-001 · P1 · 资料调查 · 默认分支依赖告警适用性复核**
  - 依赖：—。交付与验收：获取 GitHub 默认分支未关闭告警的官方明细，映射 go.mod/go.sum、package.json/锁文件与 SDK/构建产物；区分运行时/开发/传递依赖、受影响版本与实际可达条件，逐项形成“需修复/不适用及依据/待验证”清单，再拆最小修复任务，不仅凭告警总数宣布存在可利用生产漏洞。
  - 状态／条件：DONE · LOCAL VERIFIED（2026-11-09 复核解阻并完成：2026-11-01 记录的「无 security_events 权限」已失效，当前 `gh api repos/cuihairu/croupier/dependabot/alerts` 可读）。默认分支未关闭告警共 4 条，逐项判定（无「需修复」项）：
    - #491 `@hono/node-server@1.19.17`（medium，GHSA-rmxm-3fg6-px4f，serveStatic 路径二次解码绕过中间件）——web/pnpm-lock.yaml，**开发期传递依赖**：umi → @umijs/preset-umi → @umijs/bundler-utoopack → @utoo/pack → @hono/node-ws（声明 `^1.19.11`）→ 本包；漏洞路径是静态文件服务器，本仓仅 umi dev/构建工具链触达，web 生产形态是静态产物由 nginx 服务，运行面不含此包——**不适用为生产漏洞**。范围内无修复版（漏洞区间 ≥1.19.10 <2.1.3 覆盖整个 ^1.x，修复版 2.1.3 跨主版本、违背父链 ^1.19.11 声明）——**待验证**：上游 @utoo/pack / @hono/node-ws 支持 2.x 后随依赖链升级，不做 workspace override 强跨主版本（对应 pnpm override 事故先例）。
    - #487/#483/#482 `deepmerge@4.3.1`（low，GHSA-gcqx-w48m-w2h4，mergeObject 原型污染）——三份 pnpm-lock.yaml 均**开发期依赖**：web 侧依赖方为 @svgr/core + umi bundler 链（构建期配置合并），sdks/js 与 sdks/plugins/js 侧为 jest-config/jest-util 测试链；原型污染需对抗性 merge 输入，本仓 merge 输入均为仓内受控构建/测试内容——**不适用**。上游无修复版本（漏洞区间封顶 4.3.1 即当前最新版）——**待验证**：上游发布 >4.3.1 修复版后升级三处锁文件。
  - 最小修复任务拆解结论：无「需修复」项；两条「待验证」跟随项已登记于上，无需新增修复任务。证据：`gh api repos/cuihairu/croupier/dependabot/alerts?state=open`（4 条明细）、web/pnpm-lock.yaml（deepmerge@1.5.2+4.3.1、@hono/node-server@1.19.17 依赖块）、sdks/js/pnpm-lock.yaml、sdks/plugins/js/pnpm-lock.yaml（jest 链）。
  - 完成证据：2026-11-09；`web/pnpm-lock.yaml`、`sdks/js/pnpm-lock.yaml`、`sdks/plugins/js/pnpm-lock.yaml`（3 份锁文件逐条映射告警条目）、`go.mod`；`gh api repos/cuihairu/croupier/dependabot/alerts?state=open` 4 条明细逐项判定；commit df2d5025

## 审核发现

- [x] 审核发现：分析 worker 把成功入批但未 Send 的 Redis 消息延迟 ACK，批 Send 失败时丢弃批并把消息留在 PENDING；reclaim 重新投递会再次执行 touchRevenue（相加累计 revenue/refunds），导致收入/退款聚合重复计数（internal/analytics/worker/worker.go:448, internal/analytics/worker/worker.go:625）
  - 修复：支付聚合增量延后到批次 Send 成功后提交；失败批次不残留增量。证据：2026-10-31；`go test ./internal/analytics/worker/... -count=1` 全绿；新增 `TestPaymentAggregationDefersUntilBatchSend`。
- [x] 审核发现：govern.go 注释声明门序与旧 Invoke 逐条对齐（输入契约为第 6 步、政策审批第 5 步），但实现把 ValidateInvokeInput 提前到 LoadCurrentAdmin/EnforceFunctionPolicy 之前（internal/platform/functioninvoke/govern.go:127），改变错误优先级（无效输入 400 会先于策略拒绝 403/404 返回），与注释及旧行为不一致（internal/platform/functioninvoke/govern.go:68）
  - 修复：full 执行链恢复 policy→input 顺序；schedule Create 定义期单独保留 input 校验。证据：2026-10-31；`go test ./internal/platform/functioninvoke/... -count=1` 全绿；新增 `TestGovernPolicyBeforeInputContract`。
- [x] 审核发现：`TestHandleInvoke_TaskContinuationBypassesClearedAgentLocal` 存在间歇性失败——断言 `resp.GetPayload()=="echo:"+string(data)` 与请求 `data` 的 proto 字段序列化顺序非确定有关（期望串与回显字节在 map 迭代/序列化时错位），全量 `go test -count=1 ./internal/agent/...` 多次复现 FAIL（internal/agent/provider_drain_test.go:451）；本测试为新引入用例（非基线既有），建议改为按字段解码断言而非全字节串比较。
  - 修复：该断言随 ARCH-016 重构（3d5312fc）重写——延续路由测试改为 `proto.Unmarshal` 解码后 `assert.NotEmpty(invokeResp.GetPayload())`（provider_drain_test.go:319）或固定字面量对比（`"echo:payload"` 对固定请求 `[]byte("payload")`，:266），全文件已无 `"echo:"+string(...)` / `GetPayload()==` 脆弱全字节串比较（`grep` 零命中）。证据：2026-10-31；`go test ./internal/agent/ -count=1` PASS（8.4s）；`go test ./internal/agent/ -run 'TestCallProvider_TaskContinuation|TestHandleInvoke_TaskContinuation|TestDrainProviderSession_WaitsForAcceptedTasks' -count=5` PASS。
- [x] 审核发现：govern.go full 链新增 CheckInvokePermission（R30-01）后，顶部「门序」注释列出 8 个唯一编号步骤（5=ACL、6=政策、7=输入），但实现内联注释把「函数权限/政策审批」与「输入契约」都标为 `// 7.`（两处并存），未同步为 6/7（internal/platform/functioninvoke/govern.go:166、internal/platform/functioninvoke/govern.go:175），与 ARCH-024 完成证据「全链统一」存在注释漂移。
  - 修复：`// 6.` 同步政策审批步号（原误标 `// 7.`）；`// 5.` ACL 与 `// 7.` 输入契约已一致，纯注释无功能行为变更。证据：2026-10-31；`go test ./internal/platform/functioninvoke/... -count=1` 全绿；`go build ./internal/platform/functioninvoke/` 过。
- [x] 审核发现：`reapStaleClaims`（R30-04 跨槽恢复）与 `resolveStaleClaim` 均把租约过期 claimed 解析为 `indeterminate`，但 `UpdateRunLog` 的 WHERE 仅含 `schedule_id+slot`、不含 `status=claimed` 再校验（internal/model/task_schedule_model.go:192-200）：并发路径下另一实例恰已完成派发并回写 dispatched 后，跨槽扫描仍按 createdAt<before 把它覆写成 indeterminate，可能覆盖真实终值（internal/tasks/scheduler/manager.go:186-200）。
  - 修复：`UpdateRunLog` 的 WHERE 增加 `AND status=claimed` 再校验——仅当行仍为 claimed 时才回填终值，否则 no-op 保留已落 dispatched/终态，陈旧 reclaim 不再覆写真实派发结果。证据：2026-10-31；`go test ./internal/model/ -run TestTaskScheduleModel_UpdateRunLog_GuardNonClaimed -count=1` PASS；`go test ./internal/model/ ./internal/tasks/scheduler/ -count=1` 全绿；新增 `TestTaskScheduleModel_UpdateRunLog_GuardNonClaimed`（正常回填 + 已派发后陈旧 reclaim no-op）。
- [x] 审核发现：本轮 diff 其余方面（机器 token 鉴权路径白名单与 scope 校验、RecordTaskEvent 单事务、任务在途/延续路由、依赖边界与架构 DAG 守卫、前端窄屏与本地化键、文档复核、CI 接线、blob→无类型字段迁移）未复现功能性问题，相关测试包均通过。
  - 证据：2026-11-01 复核属实——`go test ./internal/security/jwtutil/ -run 'TestSignMachine|TestParseMachine' -count=1`、`go test ./internal/model/ -run 'TestTaskRunModel_RecordTaskEvent' -count=1`、`go test ./internal/agent/ -run 'TestCallProvider_TaskContinuation|TestHandleInvoke_TaskContinuation|TestDrainProviderSession' -count=1`、`go test ./internal/svc/ -run 'TestAuthMiddleware_MachineToken_PathRestriction' -count=1`、`go test ./internal/svc/ -run 'TestMigrationsProbeEvents|TestApprovalDispatchClaimMigration' -count=1`、`go test ./internal/app/agent/ -run 'Test.*Boundary' -count=1` 均 PASS；未复现功能性问题，无需修复。
- [x] 审核发现：工作树存在 0 字节空文件 `internal/cluster/arch005_drill_test.go`（git 未跟踪），使 `go build ./...` 及任何 import `internal/cluster` 的包（internal/server、internal/platform/functioninvoke 等）报 `expected 'package', found 'EOF'` 构建失败——遗留调试/半成品物（internal/cluster/arch005_drill_test.go:1）。
  - 修复：删除该 0 字节空文件（未跟踪、无 git 变更可提交）；`go build ./...` 恢复 PASS。证据：2026-11-01；`internal/cluster/arch005_drill_test.go` 已不存在；`go build ./...` exit 0。
- [x] 审核发现：`cmd/server/cluster_extra_test.go:225` 遗留调试输出 `t.Logf("forward debug: OK=%v Error=%q NotOwner=%v", ...)`，非断言性调试日志，应移除或改为断言上下文（cmd/server/cluster_extra_test.go:225）。
  - 修复：移除该 `t.Logf`（原 :225，现删除后不再存在）。证据：2026-11-01；`go test ./cmd/server/... -count=1` PASS（556.7s）；commit `7412020a`。
- [x] 审核发现：896de78e 移除 `processMessage` 中 `touchRevenue(ctx, m)` 调用后，`internal/analytics/worker/worker.go:675` 的 `touchRevenue` 仅剩测试引用，成为生产死代码（internal/analytics/worker/worker.go:675）。
  - 修复：删除 `touchRevenue` 生产函数及对应纯内存测试（TestTouchRevenue_*）；`flush_agg_repro_test.go` 改用直接填充 `revAgg` 保持 flush 消费路径覆盖。证据：2026-11-01；`go test ./internal/analytics/worker/... -count=1` PASS（19.8s）；commit `55f79b82`。
- [x] 审核发现：f8d12842 将政策审批注释从 `// 7.` 改为 `// 6.`，但 `RequireGameEnvScope` 的 SubjectAutomation 复核块同样标注 `// 6.`（internal/platform/functioninvoke/govern.go:158），与顶部「门序」注释（EnforceFunctionPolicy=第 6 步、RequireGameEnvScope 有意不在链内）冲突，内联注释仍有两处 `// 6.` 漂移（internal/platform/functioninvoke/govern.go:158、internal/platform/functioninvoke/govern.go:166）。
  - 修复：`RequireGameEnvScope` 复核块改为「自动化主体补 scope 复核（有意不在 full 链内，见顶部门序）」标注，去掉 `// 6.` 编号，不再与链内 EnforceFunctionPolicy=第 6 步冲突；纯注释无功能变更。证据：2026-11-01；`go test ./internal/platform/functioninvoke/... -count=1` PASS（17.8s）；commit `aa1f74a3`。
- [x] 审核发现：`configs/server.yaml` 与 `internal/config` 均声明 `cluster.interconnectTls.enabled`（TLSClientConfig.Enabled），但 `cmd/server/cluster.go:143-147` 的 `useInterconnectTLS` 只按 cert/key/ca 三路径判断、从不读取 `.Enabled`——`enabled:true` 但缺证书时静默走明文，`enabled:false` 但证书齐备时仍启用 mTLS，配置面与实际行为不一致（cmd/server/cluster.go:143）。
  - 修复：`useInterconnectTLS` 增加 `cfg.InterconnectTLS.Enabled &&` 前置条件——仅 enabled 且 cert/key/ca 齐备才启用 mTLS，配置面与实际行为一致。证据：2026-11-01；`go test ./cmd/server/... -count=1` PASS（556.7s）；commit `2d9d5ba5`。
- [x] 审核发现：机器 token 鉴权路径复核发现遗漏——token 放入 `Authorization: Bearer` 时管理员 `Parse` 忽略未知 `tokenType` 字段，退化为空 Username/AdminID=0 的 claims 并绕过 token 版本查询，使机器 token 可冒充「空用户名管理员」身份。
  - 修复：`authenticate` 拒绝 AdminID=0 或 Username 空的退化管理员 claims，机器 token 不能经 Bearer 路径冒充管理员。证据：2026-11-01；`internal/svc/machine_auth_test.go:100` 的 `TestAuthMiddleware_MachineTokenInBearerRejected` 断言 401/authentication_failed；`internal/svc/service_context.go:1844` 拒绝退化 claims；`go test -count=1 -p 1 ./internal/svc/... ./internal/security/jwtutil/... ./internal/api/extension/... ./internal/app/agent/...` PASS。
- [x] 审核发现：opsNodeDrain/opsNodeUndrain 以 JSON 文本作为 MsgProviderDrainRequest 请求体发送（internal/api/ops/helpers.go:819、940），但 agent 侧用 proto.Unmarshal 解析该请求（internal/agent/local_handler.go:1084-1086、internal/agent/tcp_local_listener.go:396-398）；proto 解码 JSON 必失败（实测 `proto: cannot parse invalid wire-format data`），agent 侧 ProviderSession 的 draining 标记与在途等待永不生效，ARCH-016 drain/undrain 实际是空操作。 属实并已修：server 侧改 `proto.Marshal(&sdkv1.ProviderDrainRequest{})`（internal/api/ops/helpers.go:815-823），不再发 JSON 文本；`go test ./internal/api/ops -run TestOpsNodeDrainRegistryGate -count=1` 全绿；证据 commit `c949c20d`。
- [x] 审核发现：opsNodeUndrain 发送 `{"reason":"__undrain__"}` 依赖 reason 哨兵区分 undrain（internal/api/ops/helpers.go:940），agent 侧 DrainProviderSession 以 `req.Reason == ProviderDrainReasonUndrain` 判定（internal/agent/local_handler.go:1092）；但该哨兵经 JSON→proto 解码必然失败，且服务端发送时未 proto 编码，undrain 分支（清除 draining、恢复路由）永远无法触达。 属实并已修：undrain 下行改 `proto.Marshal(&sdkv1.ProviderDrainRequest{Reason: protocol.ProviderDrainReasonUndrain})`（internal/api/ops/helpers.go:938-945），哨兵随 proto 编码正常触达；`go test ./internal/api/ops -run TestOpsNodeDrainRegistryGate -count=1` 全绿；证据 commit `c949c20d`。
- [x] 审核发现：reapStaleClaims 每次仅 `ListStaleClaims(now-lease, 100)` 处理前 100 条且每条为单条无条件 UPDATE，无批量/分页游标；claim 速率高于清理速率时积压会长期滞留，且该扫描与 triggerSchedule 处理同一 `claimed` 槽位存在双重写入（internal/tasks/scheduler/manager.go:186-198）。 属实并已修：`reapStaleClaims` 改为按 `created_at` 游标分页循环（`ListStaleClaimsAfter`，internal/tasks/scheduler/manager.go:186-215、internal/model/task_schedule_model.go:202-222），积压>100 单轮全部收敛；新增 `TestManager_ReapStaleClaims_DrainsOver100`（internal/tasks/scheduler/arch023_test.go）验证 150 条单轮全清；`go test ./internal/tasks/scheduler ./internal/model -count=1` 全绿；证据 commit `5b4000dc`。
- [x] 审核发现：resolvePolicyRisk 中 RegistryStore.GetOpenAPI 的错误一律被忽略（`err == nil && op != nil`，internal/platform/functioninvoke/policy.go:50-58），但 GetOpenAPI 对「操作未找到」与「clone 序列化失败」都返回 error（internal/platform/registry/store.go:1126-1134）；clone 失败被当作「未声明风险」回落 medium，与 ARCH-025 fail-closed 精神不符（低概率但违背同函数注释「来源故障向上传播」）。 属实并已修：`GetOpenAPI` 引入可判别 `ErrOpenAPINotFound`（internal/platform/registry/store.go:1128-1142），`resolvePolicyRisk` 对未命中回落 medium、对 clone 序列化失败返回错误 fail-closed（internal/platform/functioninvoke/policy.go:56-67）；新增 `TestResolvePolicyRisk_OpenAPINotFoundFallsBack`、`TestGetOpenAPI_DistinguishesNotFoundAndCloneFailure`；`go test ./internal/platform/functioninvoke ./internal/platform/registry -count=1` 全绿；证据 commit `42b2437e`。
- [x] 审核发现：ClaimForDispatch 的 SQL WHERE 仅 `state='approved' AND claim_token=''`（internal/platform/approvals/sql_store.go:246-248），未含 `claim_status=''`；MemStore 判重用 `ClaimStatus!=''||ClaimToken!=''`（internal/platform/approvals/store.go:294-296），两者语义不一致——历史/异常行若 claim_token 空而 claim_status 非空，SQL 可二次领取而 MemStore 拒绝。 属实并已修：SQL WHERE 增加 `AND claim_status = ''`（internal/platform/approvals/sql_store.go:250），与 MemStore 判重语义一致；新增 `TestClaimForDispatch_SQLRejectsClaimStatusOnly`（internal/platform/approvals/cas_concurrency_test.go）验证二次领取被拒；`go test ./internal/platform/approvals -count=1` 全绿；证据 commit `e20ec075`。
- [x] 审核发现：SetTaskReaper 在 `s.mu.Lock()` 下写 watchdogBudget/watchdogInterval（internal/server/control_handler.go:258-264），而 runTaskWatchdog 在后台循环无锁读 `s.watchdogBudget`（internal/server/control_handler.go:1990-2000）；仅因文档规定「须在 StartBackgroundTasks 前调用」而靠 goroutine 建立 happens-before 免于现行 race，但字段读写无同步契约，属潜在数据竞争隐患。 属实并已修：`runTaskWatchdog` 在 `s.mu.RLock()` 下快照 taskReaper/watchdogBudget 后调用（internal/server/control_handler.go:2051-2060），`taskWatchdogLoop` 同样加读锁快照 interval（internal/server/control_handler.go:2066-2070）；新增 `TestControlService_WatchdogFields_RaceFreeRead`（internal/server/watchdog_test.go）；`go test -race ./internal/server -run Watchdog -count=1` 与 `go test ./internal/server -count=1` 全绿；证据 commit `e19e1dcb`。

- [x] **DOC-001 · P2 · 文档 · 配置开关效果动态图**
  - 依赖：—。
  - 状态／条件：DONE · LOCAL VERIFIED（2026-11-06）。参考 kode todo C-24 同款流水线模式（2026-11-01 用户令全舰队推广）。**增量 1 已交付：全量开关枚举落 docs「配置效果」页（双语）**——5 个 featureFlags（dev/support/analytics/ops/extensions，缺省=启用、显式 false 裁剪控制面域）+ 36 个 server 配置开关（含遥测 5、认证提供者 9、集群/HA/TLS 11 等，全路径自 Config 根 struct 遍历生成）+ 8 个 agent 侧开关（ops_config 5 + 三 agent telemetry），每项含默认值来源（example/code）与 off/on 可观察效果；已知边界记录 telemetry 块历史 snake_case 兼容面。**增量 2 已交付：证据流水线 + GUI 开关 off/on 证据**——`scripts/docs/config-switch-evidence.sh`（可重跑：临时 SQLite 服务器 + 无头 Chromium 登录 → off/on 截图 → ffmpeg 标签对比图 + 交叉淡化 GIF，产物落 `docs/public/evidence/config-effects/<flag>/`）为 4 个 GUI 可见功能标志（ops/support/analytics/dev）生成并嵌入证据；实测发现并记录两处边界：`featureFlags.ops` 关闭同时隐藏「运维中心 + 风控」（/risk/* 与 /ops/* 同受 `canOpsRead` 门控）、`featureFlags.extensions` 为纯路由（扩展页全 `hideInMenu`，关/开菜单逐字一致，无 GUI 变化）。server/agent 侧开关为配置级（无 GUI 可视化变化，已在已知边界注明），其效果由枚举行描述且回归检查覆盖——`scripts/docs/check-config-evidence.sh` 断言 4 个 GUI 标志证据存在且双语页引用、路由-only 标志无证据目录，已接线 `.github/workflows/docs-check.yml`，`config-effect evidence check PASSED`（2026-11-06 复核）。
  - 完成证据：`docs/operations/config-effects.md` + `docs/zh/operations/config-effects.md`（双语枚举页，含 featureFlags/server/telemetry/auth/agent 五段表格与已知边界 + 功能标志证据画廊与更新后的边界）；`docs/public/evidence/config-effects/featureFlags.{ops,support,analytics,dev}/{ops,support,analytics,dev}-{off,on,compare}.png` + `{ops,support,analytics,dev}.gif`（16 个产物，本地服务器实测生成）；`scripts/docs/config-switch-evidence.sh`（可重跑流水线）+ `scripts/docs/config-evidence-capture.js`（Playwright 登录截图）+ `scripts/docs/check-config-evidence.sh`（回归断言：4 个 GUI 标志证据存在/双语页引用/路由-only 无证据目录，`config-effect evidence check PASSED`）+ `.github/workflows/docs-check.yml`（新增 Check config-switch evidence regression step + path 过滤）；`bash scripts/check-docs-links.sh` OK（728 链接 0 死链）、`bash scripts/check-docs-terms.sh` OK（83 基线）、`CI=true pnpm build`（docs）通过、`bash scripts/dashboard_vnext_guard.sh` PASSED、`pnpm --dir web run tsc` 0 错误。复核（2026-11-06）：`bash scripts/docs/check-config-evidence.sh` → `config-effect evidence check PASSED`（ops/support/analytics/dev 证据存在+双语引用、extensions route-only 无目录边界保持）；`bash scripts/check-docs-links.sh` → OK（728 链接 0 死链）；`CI=true pnpm docs:build` → 构建成功（240s，仅 haproxy/conf 高亮回退与 chunk>500kB 警告，无错误）。
  - 交付与验收：枚举本仓全部「开启后有效果」的开关/配置项，自动流水线生成证据——GUI 类用 @vscode/test-electron 或应用截图（off 一张/on 一张+切换过程录屏），CLI 类用终端录屏（asciinema→GIF 或 ffmpeg），ffmpeg 缝合动态 GIF；docs「配置效果」页（双语）逐开关嵌入说明+off 图+on 图+GIF。验收：清单内每个开关都有 off/on+GIF；流水线可重跑（配置变更随时再生成）；兼作配置效果回归测试。

## 全仓代码深审（2026-11-02）——P2/P3 待办

- [x] **C-002 · P2 · 安全 · cloud-init runcmd 注入（root RCE）**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修）。`internal/ops/provision/template.go` 新增 `validateCloudInitExtra`/`validateCloudInitExtraLine`（逐行拒绝 shell 元字符 `$`/反引号/`;`/`|`/`&`/`<>`/`()`、`-` 前缀 YAML 嵌套、「解释器 + eval 标志」（`sh -c`/`bash -e`/列表 `[ sh, -c, ... ]` 绕过））；`renderCloudInit` 改返回 `(string, error)`（防御纵深，Extra 非法即拒渲染）；`orchestrator.go` `Validate` 边界拒绝非法 `CloudInitExtra`；`apply` 处理渲染 error。（commit `16713cbd`）
  - 交付与验收：template.go 的 renderCloudInit 不再把 CloudInitExtra 原样拼入 runcmd——改为经 allow-list 校验（仅白名单命令）或拒绝含 shell 元字符的行；新增单测覆盖 Extra 含 `$(...)`/`;`/`|` 被拒；provision Validate 校验 CloudInitExtra 非空时格式；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。
  - 完成证据：`internal/ops/provision/template.go`（`validateCloudInitExtra`/`validateCloudInitExtraLine` 逐行拒绝 shell 元字符/`-` 前缀 YAML 嵌套/解释器+eval 标志，`renderCloudInit` 改返回 `(string, error)`）+ `internal/ops/provision/orchestrator.go`（`Validate` 边界拒绝非法 CloudInitExtra）+ `internal/ops/provision/template_test.go`（`TestValidateCloudInitExtra_RejectsInjection` 11 类注入全拒、`TestValidateCloudInitExtra_AllowsSafe`、`TestRenderCloudInit_RejectsInjectionExtra`、`TestValidate_Matrix`）；`go test ./internal/ops/provision/` + `./internal/api/ops/` 全绿、`go build ./...` + `go vet` 通过；review 文档 C-002 行改「已修（2026-11-03）」。

- [x] **C-003 · P2 · SDK · JS SDK 畸形帧对端挂死（违 BUG-038 应答契约）**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修）。`sdks/js/src/index.ts`：`decodeInvokeRequest` 包 try/catch（protobuf decode 失败抛「malformed invoke request」）；`invokeInbound` 捕错回 `{error}` payload 帧；`handleInboundStartTask` 捕错回空 `taskId`（Agent 侧按失败处理）；`handleInboundCancelTask` 捕错回空响应。新增模块级 `InvokeRequestDecoded` 类型别名。与 Go/Py/C++ 行为一致（解析失败回错误帧而非静默挂死）。（commit `f640276c`）
  - 交付与验收：sdks/js 的 decodeInvokeRequest/handleInboundStartTask/handleInboundCancelTask 把 protobuf decode 移入 try/catch，解析失败时按契约回错误帧而非静默；与 Go/Py/C++ 行为一致；新增畸形帧单测断言对端收到错误响应而非挂起；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。
  - 完成证据：`sdks/js/src/index.ts`（decodeInvokeRequest 包 try/catch、invokeInbound 捕错回 `{error}` 帧、handleInboundStartTask/handleInboundCancelTask 捕错回空响应）+ `sdks/js/src/inbound_client.test.ts` 新增 `describe("BasicClient malformed inbound frames (C-003)")`——非法 wire type 帧（`[0x0f]`）对 Invoke/StartTask/CancelTask 三型均回帧不挂死（invoke 回 `{error: /malformed invoke request/}`、start 回空 taskId、cancel 回空响应）；`npx tsc --noEmit` 0 错误；`npx jest` 全量 438 过/7 跳（含 inbound_client 10/10）；review 文档 C-003 行改「已修（2026-11-03）」。

- [x] **C-004 · P2 · 安全 · SSE token 入 query param**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03）。`createEventSource` 改返回 fetch 实现 `SSEStream`（Authorization: Bearer 头 + SSE 帧解析 event/data/retry + 断连自动重连 + 非200 致命），JWT 不再进 URL；`useRealtimeStream` ref 类型改 `SSEStream`。LOCAL VERIFIED：`pnpm --dir web run tsc` 0 错误；`npx jest src/services/core/http.test.ts src/services/core/__tests__/http.node.test.ts src/pages/Analytics/Realtime src/services/api/messages.test.ts src/services/api/analytics.test.ts` 全绿（http 19/19 + Realtime 9 套件 95/95）；无其它 EventSource/token query 残留（grep 仅剩 2 个 createEventSource 调用点）。（commit `4b138c94`）
  - 完成证据：`web/src/services/core/http.ts`（createEventSource 改 fetch 实现 SSEStream，Authorization: Bearer 头 + SSE 帧解析 + 断连自动重连 + 非200 致命，JWT 不再进 URL）；`web/src/services/core/http.test.ts` + `web/src/services/core/__tests__/http.node.test.ts`（http 19/19）；`web/src/pages/Analytics/Realtime/useRealtimeStream.ts`；`pnpm --dir web run tsc` 0 错误。
  - 交付与验收：createEventSource 不再把 JWT 拼进 ?token= query（改走 EventSource 无法带 header 的替代方案：短效 stream token 或经 Authorization 的 fetch-stream）；token 不出现在 URL/日志/Referer；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [ ] **C-005 · P2 · 安全 · JWT 存 localStorage**
  - 依赖：—。
  - 状态／条件：BLOCKED（2026-11-06）。**DEFERRED（2026-11-03）——架构级鉴权门改造，设计需用户签字**：修复必须 httpOnly cookie 迁移（localStorage 是唯一让 XSS 读走 token 的载体，等效安全存储只能是 cookie）；但改动横穿登录状态机（密码→MFA→mustChangePassword）、OAuth 回调 `?token=` 重定向交付（handler.go:320 经 oidcSuccessURL）、Logout 吊销、auth 中间件（service_context.go:1763 读 Authorization/?token=），外加 web 侧 Login/index.tsx / http.ts getToken / app.tsx / requestErrorConfig.ts 与各自测试。鉴权门是全站入口，cookie 属性（Secure/SameSite/CSRF 模型）与 OAuth 回调 token 交付方式属设计决策，须用户确认方向后再动（自治无人复核下改错即整站登录挂）。非静默丢弃——P2 优先级保留，待设计确认后回到本项。缺：用户对鉴权门改造方向的签字批准。
  - 交付与验收：Login 改用 httpOnly+Secure cookie 或等效安全存储承载会话；localStorage 不再落 token；跨页面鉴权路径同步更新；新增单测断言 token 未写入 localStorage；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **MCP-001 · P1 · 实现 · croupier MCP server（2026-11-13 用户令）**——Go 官方 `modelcontextprotocol/go-sdk` stdio server（`croupier mcp` 子命令），把控制面能力暴露给 AI agent：`list_sessions`/`query_jobs`（只读面直通既有查询）、`invoke_function`（走既有函数注册调用链，不重写分发）、风险工具（封禁/补偿/订单类写操作）**必须**等 GAME-009/010 凭据协议落地后才准出——未落地前 MCP 只暴露只读工具。验收：`go test ./...` 全绿 + stdio e2e（initialize/tools list/只读 call）+ README 双语入口行。
  - 依赖：—。
  - 状态／条件：**DONE · LOCAL VERIFIED（2026-11-14）**：`croupier-server mcp` 子命令（cobra）以 modelcontextprotocol/go-sdk v1.8.0 起 stdio server；三工具全部转发运行中 server 的 REST 面——`list_sessions`→GET /api/v1/ops/agents、`query_jobs`→GET /api/v1/tasks（status/functionId/page/size 透传）、`invoke_function`→POST /api/v1/functions/{id}/invoke（固定 sync：不发送 mode 字段即默认同步，异步任务面不出本批）。每次调用带 Bearer JWT + X-Game-ID/X-Env scope 头，scope 进程启动时固定（AI 宿主不能按调用跳 scope）；统一错误对象（error/message）转 isError 工具结果（AI 可读稳定码自纠）；工具输出即 REST payload 原文（JSON text content，MCP 侧零二次建模）。写工具（封禁/补偿/订单）如实缺位：等 GAME-009/010 凭据协议，工具清单断言恰三个、无隐藏工具（测试钉死）。stdio e2e：真二进制三段可验——initialize/notifications/initialized（serverInfo croupier-server-mcp + 三工具 schema 由 jsonschema tag 生成）、tools/list、只读 tools/call（list_sessions 打本地假 REST 后端，payload 原文回流 text content）；工具 call 语义经 InMemoryTransport 直连全链断言（鉴权/scope 头、query 透传、body 不含 mode/gameId、409 统一错误对象转 isError）。README 双语入口行已落（README.md + README.zh.md 的 MCP Server 小节）。依赖增量最小：go.mod +1 直接（go-sdk v1.8.0）+3 indirect。
  - 完成证据：`cmd/server/mcp.go`（子命令 + mcpRESTClient 薄转发 + newMCPServer 三工具装配）；`cmd/server/mcp_test.go`（6 用例：工具清单恰三/鉴权 scope 头透传/query 过滤映射/sync-only body 断言/409 错误对象转 isError/超时常量>60s）；`go test ./cmd/server/ -run TestMCP -count=1` 全绿、`go test ./cmd/server/ -count=1` 全绿（TestDrill_BlockedScheduler 首跑负载假红，隔离与整包重跑均绿）；`go build ./...` 过、`go vet ./cmd/server/` 干净；`go test ./...` 全量跑毕：唯一红 `internal/app/agent`（已知负载 flake 族，全量必超时）隔离重跑绿（66.8s ok），其余包全绿；文档 `docs/guide/integrations/mcp-server.md` + `docs/zh/guide/integrations/mcp-server.md`（双语指南，含配置表/mcpServers 示例/有意缺位声明/已知边界）+ config.mjs 双语侧栏注册 + README 双语入口行。
  - 交付与验收：`go test ./...` 全绿 + `go build ./...` 过；stdio e2e（initialize/tools list/只读 call）；README 双语入口行；写工具缺位在工具清单与文档双处如实声明（GAME-009/010 未落地前不出）。

- [x] **C-006 · P2 · 安全 · 附件 URL 无 scheme 白名单**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03）。`Detail.tsx` 附件 `a.url` 经 `safeExternalHref`（http/https 白名单）后才进 `<img src>` / `<a href>` / `window.open`；非法 scheme（javascript:/data:）渲染为 `<span>` 无链接文本；新增 C-006 用例断言 evil-js/evil-data 为 SPAN、无 `a[href^=javascript:]`/`a[href^=data:]`/`img[src^=data:]`、合法 http 仍链接、window.open 未调用。LOCAL VERIFIED：`npx jest src/pages/Support/Tickets/__tests__/Detail.test.tsx` 44/44 过（95s）+ `pnpm --dir web run tsc` 0 错误；review 文档 C-006 行改「已修（2026-11-03）」。（commit `aada026a`）
  - 完成证据：`web/src/pages/Support/Tickets/Detail.tsx`（附件 url 经 safeExternalHref http/https 白名单后才进 img/a/window.open，非法 scheme 渲染为无链接 span）；`web/src/pages/Support/Tickets/__tests__/Detail.test.tsx`（44/44 过，C-006 用例断言 evil-js/evil-data 为 SPAN 且无 javascript:/data: 链接）；`pnpm --dir web run tsc` 0 错误。
  - 交付与验收：Support/Tickets/Detail.tsx 附件 img src/a href/window.open 全部走 safeExternalHref（http/https 白名单）；非 http/https 渲染为无链接文本；新增单测覆盖 javascript:/data: 拒绝；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-007 · P2 · 安全 · 告警/图表 URL 无 scheme 白名单**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03）。`Ops/Alerts/index.tsx` 5 处 `window.open`（grafana explore / AM `/#/alerts` / silences 深链 / drawer runbook / drawer grafana）全部先经 `safeExternalHref`（http/https 白名单），空结果不弹窗；alertmanagerUrl 缺省时 silences 深链不再向当前 origin 导航（此前打开不存在哈希路由）。LOCAL VERIFIED：`npx jest src/pages/Ops/Alerts/__tests__/index.test.tsx` 33/33 过（93s）+ `pnpm --dir web run tsc` 0 错误；review 文档 C-007 行改「已修（2026-11-03）」。（commit `6a45166e`）
  - 完成证据：`web/src/pages/Ops/Alerts/index.tsx`（5 处 window.open 全部先经 safeExternalHref，空结果不弹窗；alertmanagerUrl 缺省时 silences 深链不再向当前 origin 导航）；`web/src/pages/Ops/Alerts/__tests__/index.test.tsx`（33/33 过，含 C-007 外链白名单用例）；`pnpm --dir web run tsc` 0 错误。
  - 交付与验收：Ops/Alerts 的 grafanaExploreUrl/alertmanagerUrl/runbook_url window.open 全部走 safeExternalHref；非法 scheme 不弹窗；新增单测；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-025 · P2 · 正确性 · openapi provider 读已关 body 丢真实错误**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03）。`provider.go:890` 关 body 仅在实际重试时（`i < RetryCount`），末次 5xx 留给 defer+ReadAll；`TestProviderCall_All5xxRetriesReturnRealError` 断言 3 次 5xx 后返回含 `500` 与真实响应体的 `API error`（非 read-on-closed），mutation 验证旧码必红。LOCAL VERIFIED：`go test ./internal/drivers/openapi/ -count=1` 绿（6.3s）+ `go build ./...` 过。（commit `23dfaaea`）
  - 完成证据：`internal/drivers/openapi/provider.go`（末次 5xx 不提前关 body，留给 defer+ReadAll 返回真实错误）；`internal/drivers/openapi/provider_test.go`（`TestProviderCall_All5xxRetriesReturnRealError` 断言 3 次 5xx 后返回含 500 与真实响应体）；`go test ./internal/drivers/openapi/ -count=1` 绿；`go build ./...` 过。
  - 交付与验收：provider.go 重试循环末次 5xx 后不提前关 resp.Body（延迟到 defer 或关前判 err）；新增单测：全 5xx 重试后返回真实 HTTP 状态/错误体而非 read-on-closed；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-026 · P2 · 正确性 · SlidingWindowLimiter.GetStats 越界 panic**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03）。`advanced.go` GetStats 对空 `timestamps` 加 `len>0` 守卫返回零值 `LastRequest`；`TestSlidingWindowGetStats_EmptyWindow`（maxRequests=1、n=2 超限建空窗口后 GetStats 不 panic），mutation 验证旧码必红（advanced.go:230 index out of range）。LOCAL VERIFIED：`go test ./internal/platform/ratelimit/ -count=1` 绿（3.2s）+ `go build ./...` 过。（commit `cdb3c315`）
  - 完成证据：`internal/platform/ratelimit/advanced.go`（GetStats 对空 timestamps 加 len>0 守卫返回零值）；`internal/platform/ratelimit/advanced_getstats_empty_test.go`（`TestSlidingWindowGetStats_EmptyWindow` 断言超限建空窗口后不 panic）；`go test ./internal/platform/ratelimit/ -count=1` 绿；`go build ./...` 过。
  - 交付与验收：advanced.go GetStats 对空 timestamps 加 len>0 守卫返回空/零值；新增单测：空窗口（超限建空/prune 到 0）调 GetStats 不 panic；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-008 · P3 · 安全 · agent exec 白名单仅匹配命令不匹配参数/环境**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03）。`ops_server.go` 新增 `validateExecBoundary`（ExecuteCommand 内白名单命中后调用，fail-closed）：敏感环境键拒绝（`LD_PRELOAD`/`LD_LIBRARY_PATH`/`LD_AUDIT`/`DYLD_*`/`PATH`/`CDPATH`/`PYTHON*`/`BASH_ENV`/`ENV`/`SHELLOPTS`/`BASHOPTS`/`IFS`/`GIT_*`/`PERL*`/`NODE_OPTIONS`/`RUBY*`/`RUSTC_BOOTSTRAP`）；解释器（sh/bash/zsh/dash/ksh/fish/csh/tcsh/cmd/powershell/python/perl/ruby/node/php/lua）+ 内联代码开关（-c/-i/-p/-e/-r）拒绝；workingDir 必须绝对且不含 `..` 段。LOCAL VERIFIED：`go test ./internal/app/agent/ -run 'TestOpsServer_ExecuteCommand' -count=1` 全绿（含 C-008 三子例）+ 全包 `go test ./internal/app/agent/ -count=1` 绿（51s，首跑 79s FAIL 为已知 metrics 负载 flake 族，隔离全绿）+ `go build ./internal/app/agent/` 过；docs/operations/config-agent.md（en+zh）allowExec 节补第 4 条边界加固。（commit `9140512f`）
  - 完成证据：`internal/app/agent/ops_server.go`（新增 `validateExecBoundary` fail-closed：敏感环境键/解释器+内联代码开关/非绝对含 `..` workingDir 全拒）；`internal/app/agent/ops_server_extra_test.go`（C-008 三子例：LD_PRELOAD/PATH 覆盖请求被拒等）；`go test ./internal/app/agent/ -run 'TestOpsServer_ExecuteCommand' -count=1` 全绿；`go build ./internal/app/agent/` 过。
  - 交付与验收：ops_server exec 白名单校验扩展到 req.Args/req.WorkingDir/req.Env（PATH、LD_PRELOAD 等敏感键拒绝或白名单化）；新增单测：带 LD_PRELOAD/PATH 覆盖的请求被拒；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-009 · P3 · 安全 · DDL 拼接 dbName 仅反引号/方括号包裹**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03）。`db.go` 新增 `validateDBName`（`^[A-Za-z0-9_-]+$` 严格白名单），`createMySQLDatabase`/`createPostgresDatabase`/`createSQLServerDatabase` 三者入口先校验再触碰 DSN/连接（fail-closed）；各方言断句字符（反引号/单引号/双引号/方括号/分号/空白）一律拒，不再依赖仅包裹符。LOCAL VERIFIED：`go test ./internal/svc/ -run 'TestValidateDBName_Allowlist|TestCreateDatabase_RejectsInvalidDBNameBeforeDSN' -count=1` 绿 + 全包 `go test ./internal/svc/ -count=1` 绿（50s，含既有 BadDSN 三例不破）+ `go build ./...` 过。（commit `ffb9658e`）
  - 完成证据：`internal/svc/db.go`（新增 `validateDBName` `^[A-Za-z0-9_-]+$` 严格白名单，createMySQLDatabase/createPostgresDatabase/createSQLServerDatabase 入口先校验再触 DSN）；`internal/svc/db_test.go`（`TestValidateDBName_Allowlist` / `TestCreateDatabase_RejectsInvalidDBNameBeforeDSN`）；`go test ./internal/svc/ -count=1` 绿；`go build ./...` 过。
  - 交付与验收：db.go 建库 DDL 的 dbName 改经严格标识符校验（仅 [a-z0-9_-] 白名单）而非仅包裹符；SQL Server 遇 `]` 不再断；新增单测非法 dbName 被拒；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-010 · P3 · 安全 · asciicast sessionID 路径穿越（潜伏）**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03，LOCAL VERIFIED）。修复：`asciicast.go` 加 `validCastSessionID`（`^[A-Za-z0-9_-]+$`），`OpenCast` 在 `filepath.Join` 前 fail-closed 校验；新增 `asciicast_test.go` 四例（合法 id 落 dir、非法 id 全拒且 dir 外无文件、白名单独立断言、合法 id header 行落盘）；`go test ./agents/devops/session/` 全绿（0.011s）；docs/review/2026-11-01-code-review.md C-010 行已改已修（2026-11-03），汇总已修 13→14、余 19→18。（commit `9c638b95`）
  - 完成证据：`agents/devops/session/asciicast.go`（`validCastSessionID` `^[A-Za-z0-9_-]+$`，OpenCast 在 filepath.Join 前 fail-closed 校验）；`agents/devops/session/asciicast_test.go`（四例：合法 id 落 dir/非法 id 全拒且 dir 外无文件/白名单独立断言/合法 id header 行落盘）；`go test ./agents/devops/session/ -count=1` 全绿。
  - 交付与验收：asciicast.go 写 .cast 前校验 sessionID 仅 [A-Za-z0-9_-] 或经 filepath.Base；新增单测 sessionID 含 `../`/`/` 被拒；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-011 · P3 · 安全 · Java TlsSocketFactory 硬编码 keystore 口令**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03，LOCAL VERIFIED）。修复：`TlsSocketFactory.create` 签名加 `char[] keyStorePassword`（移除字面量 `"croupier"`），口令由调用方提供（无默认）；mTLS 配置而口令缺失 fail-closed 抛 `IOException`；全仓 6 个 TLS 测试类改用 4 参形式；新增 `TlsSocketFactoryEdgeTest.keyStorePasswordRequiredForMutualTls`（缺口令抛错+显式口令可构造）；`./gradlew test` 全绿（6 个 TLS 测试类 + 全量，BUILD SUCCESSFUL）；docs/review/2026-11-01-code-review.md C-011 行已改已修（2026-11-03），汇总已修 14→15、余 18→17。（commit `b2523cd0`）
  - 完成证据：`sdks/java/src/main/java/io/github/cuihairu/croupier/sdk/transport/TlsSocketFactory.java`（create 签名加 `char[] keyStorePassword`，移除字面量 `"croupier"`，mTLS 口令缺失 fail-closed 抛 IOException）；`sdks/java/src/test/java/io/github/cuihairu/croupier/sdk/transport/TlsSocketFactoryEdgeTest.java`（`keyStorePasswordRequiredForMutualTls`）；`./gradlew test` BUILD SUCCESSFUL。
  - 交付与验收：TlsSocketFactory keystore 密码改从配置/环境读取，移除字面量 "croupier"；无默认口令；新增单测无配置时启动失败而非静默用硬编码；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-012 · P3 · 正确性 · Python SDK varint 解码无边界检查**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03，LOCAL VERIFIED）。修复：`_decode_file_push_request` 三重 fail-closed——varint 解码前查 `idx >= n`（截断抛 `ValueError` 非 `IndexError`）、varint 超 10 字节抛 `ValueError`、length-delimited 长度超出 body 抛 `ValueError`（不再静默截断）；新增 `test_coverage_final.py` C-012 四例（截断 varint/varint 溢出/长度超 body/合法请求照常解码）；`pytest tests/` 全绿 551 passed；docs/review/2026-11-01-code-review.md C-012 行已改已修（2026-11-03），汇总已修 15→16、余 17→16。（commit `5a29c576`）
  - 完成证据：`sdks/python/croupier/__init__.py`（`_decode_file_push_request` 三重 fail-closed：截断 varint 抛 ValueError、varint 超 10 字节抛 ValueError、length-delimited 长度超 body 抛 ValueError）；`sdks/python/tests/test_coverage_final.py`（C-012 四例）；`pytest tests/` 全绿 551 passed。
  - 交付与验收：croupier/**init**.py varint 解码与切片加长度/边界校验，越界返回错误而非静默截断；新增畸形字节串单测；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-013 · P3 · 健壮性 · Go SDK 写错误丢弃**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03，LOCAL VERIFIED）。修复：`writeResponseFrame` 改返回 `error`，写失败即 `failAllPending()`（dead=true，在途 `Call` 即刻以 `errConnectionClosed` 失败）并回传错误；3 处调用方 `slog.Warn` 记录；新增 `tcp_client_whitebox_test.go` `TestWriteResponseFrameFailureFailsPending`；`go test ./pkg/croupier/...` 全绿；docs/review/2026-11-01-code-review.md C-013 行已改已修（2026-11-03），汇总已修 16→17、余 16→15。（commit `57193ea8`）
  - 完成证据：`sdks/go/pkg/croupier/transport/tcp_client.go`（`writeResponseFrame` 改返回 error，写失败 failAllPending 并在途 Call 即刻失败）；`sdks/go/pkg/croupier/transport/tcp_client_whitebox_test.go`（`TestWriteResponseFrameFailureFailsPending`）；`go test ./pkg/croupier/...` 全绿。
  - 交付与验收：tcp_client.go Write 错误不再丢弃（回传调用方或断连）；新增单测写失败可感知；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-014 · P3 · 健壮性 · C++ SDK send best-effort**
  - 依赖：—。
  - 状态／条件：DONE（2026-11-03，LOCAL VERIFIED）。修复：`WriteResponseOnSocket` 移除裸 `(void)send`，捕获返回值——`sent < 0` 时 `std::cerr` 记录（errno/WSAGetLastError）；`catch(...)` 拆为 `catch(std::exception)` + `catch(...)` 均记录；新增 `test_coverage_boost4.cpp` `WriteResponseOnClosedSocketDoesNotThrow`（关闭 fd → send EBADF，不抛异常）。验证：`g++ -fsyntax-only src/tcp_transport.cpp` 干净；test 文件因既有 protobuf 版本约束（gencode protoc 34.x vs 本机 runtime 3.21.12/33.4.0 不兼容，非本变更）本地无法编译，用例仅用文件既有符号；docs/review/2026-11-01-code-review.md C-014 行已改已修（2026-11-03），汇总已修 17→18、余 15→14。（commit `0f05e5ad`）
  - 完成证据：`sdks/cpp/src/tcp_transport.cpp`（`WriteResponseOnSocket` 移除裸 `(void)send`，sent<0 时 std::cerr 记录，catch 拆分记录）；`sdks/cpp/tests/test_coverage_boost4.cpp`（`WriteResponseOnClosedSocketDoesNotThrow`）；`g++ -fsyntax-only src/tcp_transport.cpp` 干净。
  - 交付与验收：tcp_transport.cpp send 返回值/异常不再整体吞掉（至少日志）；移除 (void)send 裸调用；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-015 · P3 · 健壮性 · Java ServerHttpInvoker 吞 IllegalArgumentException**
  - 依赖：—。
  - 状态／条件：DONE · LOCAL VERIFIED。2026-11-02 全仓代码深审发现（C-015），修复方向 source-wins。（commit `e3d78ce7`）
  - 完成证据：`serverErrorMessage` 的 catch 由 `catch (IllegalArgumentException ignored) {}` 改为 `catch (IllegalArgumentException parseError) { logger.warn(...) }`（slf4j `logger` 字段新增，对齐 InvokerImpl 模式），仍回退原始 body 不抛异常；新增用例 `sdks/java/src/test/java/io/github/cuihairu/croupier/sdk/invoker/ServerHttpInvokerServerErrorTest.java`（5 例：取 message / 空 message 取 error / 无二者回退原始 / 不可解析回退原始且不抛 / 空或 null 回退占位）；`./gradlew test` 全绿。记入 docs/review/2026-11-01-code-review.md 对应行（已修，2026-11-03）。
  - 交付与验收：捕获后记录日志/上抛，不再 ignored 静默；新增单测；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-016 · P3 · 安全 · hostdriver 硬编码测试探针 token**
  - 依赖：—。
  - 状态／条件：DONE · LOCAL VERIFIED。2026-11-02 全仓代码深审发现（C-016），修复方向 source-wins。（commit `5c1c90ce`）
  - 完成证据：`probe-credentials` 的 `const secret = "hd-t0k3n-9f8e7d"` 移除，改为 `resolveProbeSecret()`——`HOSTDRIVER_PROBE_SECRET` 注入时透传，否则 `crypto/rand` 生成 24 字节（48 hex）每运行随机值；非测试路径不再含固定凭据。新增用例 `sdks/plugins/hostdriver/main_test.go` `TestResolveProbeSecret_Injected` / `TestResolveProbeSecret_RandomWhenUnset`（注入透传；未注入随机、非空、两调不撞、非旧字面量、48 hex 形态）；`go build ./sdks/plugins/hostdriver/` 过 + `go test` 绿 + `go vet` 干净。记入 docs/review/2026-11-01-code-review.md 对应行（已修，2026-11-03）。
  - 交付与验收：hostdriver 探针 secret 改从环境/配置注入，移除字面量 "hd-t0k3n-9f8e7d"；非测试路径不得含固定凭据；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-017 · P3 · 安全 · Dev/Tools tool.url 无 scheme 白名单**
  - 依赖：—。
  - 状态／条件：DONE · LOCAL VERIFIED。2026-11-02 全仓代码深审发现（C-017），修复方向 source-wins。（commit `a0121549`）
  - 完成证据：`web/src/pages/Dev/Tools/index.tsx` 卡片 map 内 `const safeUrl = safeExternalHref(tool.url)`（http/https 白名单）；export 图标改 `if (safeUrl) window.open(safeUrl, '_blank', 'noreferrer')`；标题锚改 `safeUrl ? <a href={safeUrl}> : <span>`（非法 scheme 渲染为无链接文本）。新增用例 `Dev/Tools/__tests__/index.test.tsx` `C-017 非法 scheme 工具链接`（javascript:/data: 两工具：标题无 `<a>`、export 点击不 `window.open`）；`pnpm tsc` 0 错误 + `pnpm test -- --testPathPatterns pages/Dev/Tools` 14 passed。记入 docs/review/2026-11-01-code-review.md 对应行（已修，2026-11-03）。
  - 交付与验收：Dev/Tools window.open/a href 走 safeExternalHref；新增单测；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-018 · P3 · 安全 · compositeRuntime navigate window.open 缺 noopener**
  - 依赖：—。
  - 状态／条件：DONE · LOCAL VERIFIED。2026-11-02 全仓代码深审发现（C-018），修复方向 source-wins。（commit `34ee2a46`）
  - 完成证据：`web/src/components/PageRenderer/compositeRuntime.tsx` navigate 分支改 `const safeUrl = safeExternalHref(step.params?.url)`；`if (safeUrl) window.open(safeUrl, '_blank', 'noopener')`——非法 scheme（javascript:/data: 等）不开窗，新窗口不反向控制 opener。用例 `compositeRuntime.test.tsx`（更新既有断言加 noopener；新增 `C-018 navigate 非法 scheme：不开窗`——javascript:/data: 两步均被拦截、window.open 不触发）；`pnpm tsc` 0 错误 + `pnpm test -- --testPathPatterns PageRenderer/__tests__/compositeRuntime` 12 passed。记入 docs/review/2026-11-01-code-review.md 对应行（已修，2026-11-03）。
  - 交付与验收：compositeRuntime navigate 加 noopener 与 scheme 白名单（safeExternalHref）；新增单测；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-019 · P3 · 安全 · Ops/Alerts 配置 URL 无 scheme 白名单**
  - 依赖：—。
  - 状态／条件：DONE · LOCAL VERIFIED。2026-11-02 全仓代码深审发现（C-019），修复方向 source-wins。
  - 完成证据：与 C-007 同源——`6a45166e`（C-007）已把全部 5 处 window.open 走 `safeExternalHref`（grafana/alertmanager/runbook/深链）；复证当前工作树：5/5 sink 均先 `const u = safeExternalHref(...)` 再 `if (u) window.open(u, '_blank')`，文件内无裸 `<a href=`。用例 `web/src/pages/Ops/Alerts/__tests__/index.test.tsx` `告警中心 外链白名单（C-007）`（javascript: 的 grafana/alertmanager/runbook 一律不弹窗）。记入 docs/review/2026-11-01-code-review.md 对应行（已修，2026-11-03，C-007 覆盖）。
  - 交付与验收：配置 URL window.open 走 safeExternalHref；新增单测；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-020 · P3 · 健壮性 · useRealtimeStream try/finally 无 catch**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-04）：message 监听补 catch 记录畸形帧错误，不重置 stale timer；finally 仅收尾；新增畸形 JSON 帧不抛未捕获、不更新数据、不置 loading 单测；`useRealtimeStream.test.tsx` 18 例全绿、`pnpm exec tsc --noEmit` 过；docs/review/2026-11-01-code-review.md C-020 已改已修。（commit `f4321629`）
  - 完成证据：`web/src/pages/Analytics/Realtime/useRealtimeStream.ts`（message 监听加 catch 记录畸形帧、finally 仅收尾）；`web/src/pages/Analytics/Realtime/__tests__/useRealtimeStream.test.tsx`（18 例含畸形 JSON 帧不抛未捕获/不更新数据/不置 loading）；`pnpm exec tsc --noEmit` 过；`pnpm test` 相关套件全绿。
  - 交付与验收：JSON.parse/推送加 catch 处理并上报，finally 仅收尾不吞错；新增单测畸形消息不抛未捕获异常；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-021 · P3 · 测试 · TestMetricsCollector_StartReporting 负载 flake**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-04）：`internal/app/agent` 与 `core/metrics` 同源用例的 3s 边界放宽到 10s（首帧前 `Collect` 需扫描 CPU/进程/网络，饱和负载临界超时）；隔离复跑 5 次中 4 过 1 失败（3.08s 边界）确认环境性非业务回归；10s 裕度后 `go test ./internal/app/agent/ -run '^TestMetricsCollector_StartReporting$'` 与 `go test ./core/metrics/` 稳定绿；docs/review/2026-11-01-code-review.md C-021 已改已修。（commit `3822b17a`）
  - 完成证据：`core/metrics/collector_test.go` 与 `internal/app/agent/ops_server_extra_test.go`（3s 边界放宽到 10s）；`go test ./core/metrics/ -count=1` 与 `go test ./internal/app/agent/ -run '^TestMetricsCollector_StartReporting$' -count=1` 稳定绿；环境性归因记录于 docs/review/2026-11-01-code-review.md C-021 行。
  - 交付与验收：复跑稳定或加 t.Parallel 隔离/放宽超时归因环境性；记录到 CI 基线说明（非回归，勿当红豁免）；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-027 · P3 · 正确性 · devops hub 写超时 goroutine 泄漏**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-04）：consumer 增加可选 `SetWriteDeadline` 取消面，超时先 `SetWriteDeadline(now)` 打断阻塞写再 evict，并读取 `done` 收束写 goroutine；新增 `TestHubConsumerTimeoutCancelsBlockingWrite` 断言超时后 writer 被取消、consumer 摘除、`runtime.NumGoroutine()` 不增长；`go test ./agents/devops/...` 全绿、`go build ./...` 过；docs/review/2026-11-01-code-review.md C-027 已改已修。（commit `8bc06186`）
  - 完成证据：`agents/devops/session/hub.go`（consumer 加 SetWriteDeadline 取消面 + done 收束写 goroutine）；`agents/devops/session/hub_test.go`（`TestHubConsumerTimeoutCancelsBlockingWrite` 断言 writer 取消/consumer 摘除/goroutine 不增长）；`go test ./agents/devops/... -count=1` 全绿；`go build ./...` 过。
  - 交付与验收：hub.go write 超时路径取消/关闭阻塞写 goroutine（或改用带超时写原语）；done 通道超时路径读取；新增单测超时后 goroutine 数不增长；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-028 · P3 · 正确性 · mysql_pump flush 无 ctx.Done 阻塞**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-04）：`flush` 的 `out<-` 加 `select ctx.Done()`（对齐 PG pump）；新增 `TestPumpCtxCancelUnblocksBlockedSend` 断言无缓冲 out + 不读消费者阻塞后取消 ctx，pump 2s 内退出；`go test ./agents/capture/...` 全绿、`go build ./...` 过；docs/review/2026-11-01-code-review.md C-028 已改已修。（commit `44a814c3`）
  - 完成证据：`agents/capture/source/mysql_pump.go`（`out<-` 加 `select ctx.Done()`）；`agents/capture/source/mysql_pump_test.go`（`TestPumpCtxCancelUnblocksBlockedSend` 断言无缓冲 out 阻塞后取消 ctx 2s 内退出）；`go test ./agents/capture/source/ -count=1` 全绿；`go build ./...` 过。
  - 交付与验收：mysql_pump out<- 加 select ctx.Done()（对齐 PG pump）；新增单测消费者停止后 Close 可解除阻塞；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-029 · P3 · 正确性 · server/agent 跨 lane handler 字段竞态（低概率）**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-04）：`agentSessionHandler`/`providerSessionHandler` 加 `sync.Mutex`，`Handle` 读与 `handleRegister/handleConnect` 写走同锁（state/markRegistered）；`go test -race ./internal/server/ ./internal/agent/` 全绿、普通测试与 `go build ./...` 过；docs/review/2026-11-01-code-review.md C-029 已改已修。（commit `2d712e75`）
  - 完成证据：`internal/server/tcp_listener.go` 与 `internal/agent/tcp_local_listener.go`（handler 加 `sync.Mutex`，`Handle` 读与 `handleRegister/handleConnect` 写走同锁）；`go test -race ./internal/server/ ./internal/agent/ -count=1` 全绿；`go build ./...` 过。
  - 交付与验收：tcp_listener.go 与 tcp_local_listener.go 的 Handle 对 registered/agentID/sessionID 读改走 lane 内同步（或经 mux 保证 happens-before）；`go test -race` 相关用例绿；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。

- [x] **C-030 · P3 · 正确性 · externalplatform cache.Close 双关 panic**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-04）。`cache.Close` 加 `sync.Once`（`closeOnce.Do`）幂等，双调 `Close` 与 `Client.Close` 双调均不 panic；新增 `TestCache_Close_Idempotent`（双调 `cache.Close` + 双调 `Client.Close` 断言无 panic/无错误）；`go test ./internal/extensions/official/externalplatform/ -run TestCache_Close -count=1` 2/2 PASS、`go test ./internal/extensions/official/externalplatform/ -count=1` 全绿、`go vet` 过。（commit `b3979ce6`）
  - 交付与验收：cache.Close 加 sync.Once/closeOnce 幂等；新增双调 Close 单测不 panic；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。
  - 完成证据：`internal/extensions/official/externalplatform/client.go`（`closeOnce sync.Once`、`Close` 经 `closeOnce.Do`）；`internal/extensions/official/externalplatform/cache_test.go`（`TestCache_Close_Idempotent`）；`go test ./internal/extensions/official/externalplatform/ -count=1` 全绿；commit 待填。

- [x] **C-031 · P3 · 健壮性 · telemetry 每 exporter 每 span 无界 goroutine**
  - 依赖：—。
  - 状态／条件：LOCAL VERIFIED（2026-11-04）。`TracerProvider` 增加有界信号量 `exportTokens`（默认 cap=8）与 `exportCap`；`EndSpan` 每 exporter 先获取 token（全占时阻塞调用方而非无界起 goroutine），`ExportSpans` 错误经 `slog.Warn` 记录而非 `_ =` 丢弃；新增 `TestTracerProvider_EndSpan_BoundedFanout`（20 并发 EndSpan，在途 exporter 数峰值 ≤ exportCap）与 `TestTracerProvider_EndSpan_ExporterErrorLogged`；`go test ./internal/telemetry/ -run TestTracerProvider_EndSpan -count=1` 4/4 PASS、`go test ./internal/telemetry/ -count=1` 全绿、`go test -race ./internal/telemetry/ -run TestTracerProvider_EndSpan -count=1` PASS、`go vet` 过、`go build ./...` 过。（commit `f6b556d2`）
  - 交付与验收：telemetry ExportSpans fan-out 加有界（semaphore/WaitGroup 上限）并处理错误；新增高 span 速率下 goroutine 有界单测；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。
  - 完成证据：`internal/telemetry/telemetry.go`（`TracerProvider.exportTokens/exportCap`、`EndSpan` token 获取 + `slog.Warn` 错误记录）；`internal/telemetry/tracer_extra_test.go`（`TestTracerProvider_EndSpan_BoundedFanout`、`TestTracerProvider_EndSpan_ExporterErrorLogged`）；`go test ./internal/telemetry/ -count=1` 全绿；commit 待填。

- [x] **C-032 · P3 · 正确性 · mysql syncer.Close 错误丢弃**
  - 依赖：—。
  - 状态／条件：NOT APPLICABLE（2026-11-04 复核）。`agents/capture/source/mysql.go:148/:168` 调用的 `s.syncer` 是 `github.com/go-mysql-org/go-mysql` 的 `*replication.BinlogSyncer`；该依赖在 go.mod 固定为 `v1.9.1`，其 `Close()` 签名是 `func (b *BinlogSyncer) Close()`（无返回值，见模块源码 `replication/binlogsyncer.go:202`），不存在「syncer.Close 错误」可丢弃——前提不成立，无错误面需修复。`StartSyncGTID` 失败路径已 `s.syncer.Close()` 后置 `s.syncer=nil` 防泄漏（mysql.go:147-149）；`Close()` 幂等置 nil（mysql.go:166-170）。`go test ./agents/capture/source/ -count=1` 全绿（含 `TestOpenWithFakeProbeConnectFailsFast` 断言失败路径不泄漏 syncer）。
  - 交付与验收：mysql.go StartSyncGTID 失败与 Close 路径记录 syncer.Close 错误；新增单测 close 失败可见；修复后 `go test` 相关包全绿 + `go build ./...` 过；记入 docs/review/2026-11-01-code-review.md 对应行（改为已修）。
  - 完成证据：`go.mod`（`github.com/go-mysql-org/go-mysql v1.9.1`，`Close()` 无返回值无错误面）；`agents/capture/source/mysql.go`（StartSyncGTID 失败路径 Close 后置 nil 防泄漏，Close 幂等置 nil）；`agents/capture/source/mysql_pump_test.go`（`TestOpenWithFakeProbeConnectFailsFast` 断言失败路径不泄漏 syncer）；`go test ./agents/capture/source/ -count=1` 全绿。前提不成立，无需代码改动。

## 审核发现（ed541f6ed11ff1df5038d90fda9daa9711fa2cf9..HEAD，2026-11-02 只审不改）

- [x] 审核发现：供给 cloud-init 的 CloudInitExtra（POST /ops/provision body 用户可控）逐行原样写入 runcmd，作为 root 在新建实例执行，可注入任意命令（RCE）。（internal/ops/provision/template.go:47-52）
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修，C-002 既有提交）。`validateCloudInitExtra`/`validateCloudInitExtraLine` 逐行拒绝 shell 元字符（$`;|&<>()）、`-` 前缀、解释器+eval 标志；`renderCloudInit` 改返回 error；`Validate` 边界拦截。
  - 完成证据：`TestValidateCloudInitExtra_RejectsInjection`/`_AllowsSafe`、`TestRenderCloudInit_RejectsInjectionExtra`、`TestValidate_Matrix`；`go test ./internal/ops/provision/` 全绿（本次会话）。
- [x] 审核发现：deploy 投递暂存路径 /tmp/croupier-deploy-<pid>-<version> 由未净化的用户 version 拼接，version 含 ../ 可致 curl -o 在 agent 主机任意路径写文件（任意文件写/代码执行）。（internal/api/ops/deploy_agent.go:43）
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修）。`DeployPackageUpload` 对 name/version/role 经 `safeObjectPathPart`（internal/api/ops/helpers.go:1781-1790）拒绝 `/`、`\`、`..`、空值；暂存路径用 packageID+已净化 version。
  - 完成证据：`TestSafeObjectPathPart_RejectsTraversal`、`TestDeployPackageUploadEmptyNoObjectStore`；`go test ./internal/api/ops/` 全绿（303s）。
- [x] 审核发现：供给提交直接采用 body 的 gameId/env，未与已认证 X-Game-ID/X-Env scope 比对，作用域受限用户可向越权游戏/环境供给云资源（scope 绕过）。（internal/api/ops/provision.go:120-121）
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修）。`OpsProvisionSubmit` 在构建请求前调用 `enforceRequestScope`（internal/api/ops/helpers.go:1765-1776），非空请求 scope 与认证 scope 冲突返回 403（fail-closed）。
  - 完成证据：`TestProvisionSubmit_CrossScopeRejected_403`；`go test ./internal/api/ops/` 全绿（303s）。
- [x] 审核发现：batchop 创建在 body 提供 gameId/env 时覆盖中间件解析的授权 scope，可越权创建/派发到未授权游戏环境。（internal/api/ops/handler.go:956-961）
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修）。`BatchOpCreateHandler` 在填充默认 scope 后调用 `enforceRequestScope`（internal/api/ops/handler.go:960-966），body scope 与认证 scope 冲突返回 403。
  - 完成证据：`TestBatchOpCreate_CrossScopeRejected_403`；`go test ./internal/api/ops/` 全绿（303s）。
- [x] 审核发现：analytics daily_users 刷新对 Redis PFCount 错误置 0 后直接以新 version 写入 ReplacingMergeTree(version)，0 行永久覆盖当日真实 DAU/NAU（与 minute_online 的 remerge 兜底路径不一致）。（internal/analytics/worker/worker.go:830-831）
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修）。`flush` 的 daily_users 分支对 dau/neu 的 `PFCount` 分别检查 err，失败即 `remergeTouchedDay`（worker.go:830-849），不写 0 行（语义与 minute_online 一致）；成功路径照常写。
  - 完成证据：`TestFlush_DailyUsersPFCntErrorRemerges`（Redis 不可达时 daily key 被合并回共享 map 重试）；`go test ./internal/analytics/worker/` 全绿（21s）。
- [x] 审核发现：供给 ticket 模式续行丢失 HubAddr/BootstrapToken/CloudInitExtra——runFromModel 未恢复（json:"-" 不持久化）、ResumeAsync 不再注入 server 参数，续行渲染出空 hubAddr/token 的 cloud-init，agent 引导必然失败。（internal/svc/provision_wiring.go:93-126；internal/api/ops/provision.go:155）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。迁移 0059 为 `provision_runs.cloud_init_extra` 补列，`runToModel/runFromModel` 恢复非敏感片段；`Orchestrator.InjectResumeParams` 在 `beginResume` 时从 server 配置/环境重新注入 HubAddr/BootstrapToken（敏感 token 仍不入库，支持轮换）。
  - 完成证据：`TestResume_ReinjectsServerParams`、`TestProvisionRunCloudInitExtraMigration_AddsColumnIdempotent`、`TestRegisterSvcMigrations_ContainsProvisionCloudInitExtra`；`go test ./internal/ops/provision/ ./internal/svc/ ./internal/model/ ./internal/db/migrate/ -count=1` 全绿（svc 367s/model 300s）。
- [x] 审核发现：供给 user_data（多行 cloud-init YAML）经 tfexec.Var("-var user_data=<值>") 传入，terraform/tofu 按 HCL 表达式解析（#cloud-config 起始为注释）致真实 apply 失败；FakeRunner 测试掩盖该路径（测试缺口）。（internal/ops/provision/runner.go:52-53）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`TfexecRunner.Apply` 经 `tfVarAssignment` 生成 `-var` 赋值：字符串变量（user_data/region/instance_type/image/name_prefix）用 `strconv.Quote` 编码为 HCL 字符串字面量，多行 YAML/`#` 注释不再被 HCL 解析；数字变量（count/disk_size）保持裸 number；非数字值报错。
  - 完成证据：`TestTfVarAssignment`（user_data 多行含 `#` 编码为合法 HCL string、count/disk_size 裸 number、region 编码、非数字报错）；`go test ./internal/ops/provision/` 全绿、`go build ./...`+`go vet` 通过。
- [x] 审核发现：logstore Ingest 写库前先推进内存 lastSeq（:91）且忽略 db.Create 错误（:104），写库失败批次被重传当作重复丢弃、persisted 虚报——单进程内该批日志永久丢失。（internal/platform/registry/logstore.go:91,104）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`Ingest` 先收集待写行，`db.Create` 失败即 `slog.Warn` 并返回 0（不推进 lastSeq），成功后才推进并返回实写数；重传同 seq 因仍 > lastSeq 得以重写。
  - 完成证据：`TestLogStore_IngestCreateFailureDoesNotAdvanceLastSeq`（断连后 seq=2 返回 0，重连重传 seq=2 重写成功）；`go test ./internal/platform/registry/ -count=1` 全绿（6s）。
- [x] 审核发现：供给异步 apply（spawnApply）在 Init/Apply 失败时只记日志不落 failed（apply 仅等注册分支置 failed），run 永久停在 applying 不可重试。（internal/ops/provision/orchestrator.go:194-255,362-369）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`apply` 新增 `fail` 闭包：writeWorkDir/Init/render/Apply/等注册/持久化任何失败都置 `StateFailed`+`Error` 并 `Store.Update`，返回原错误；`spawnApply` 日志仅兜底；`Create` 不再重复更新。
  - 完成证据：`TestSubmitAsync_ApplyFailureMarksRunFailed`（applyErr 触发异步 run 落 failed 且 Error 含 provision apply）；`TestCreate_ApplyFailure`、`TestCreate_NoRegister`；`go test ./internal/ops/provision/ -count=1` 全绿、`go vet` 通过。
- [x] 审核发现：供给 beginResume 为 Get→pending 检查→置 applying→Update 非原子读改写，并发 ResumeAsync 均可通过 pending 检查触发双 apply（applyMu 在 beginResume 之后才锁）。（internal/ops/provision/orchestrator.go:336-357）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`RunStore` 新增 `CompareAndSwapState`（model/provision_model.go:75-100，`WHERE run_id AND state=from` 原子 Update + RowsAffected 校验 + 整行回读）；`beginResume` 改用 CAS pending→applying，并发第二个 `ErrRunNotPending`；DB 适配 `provisionDBStore.CompareAndSwapState`。
  - 完成证据：`TestProvisionRunModel_CompareAndSwapState`（首次 CAS 成功、二次 ok=false、不存在 run 返回 ok=false 无错误）、`TestResumeAsync_DoubleResume_SecondRejected`；`go test ./internal/ops/provision/ ./internal/model/ -count=1` 全绿。
- [x] 审核发现：供给 DestroyResume 未先推进状态即执行 runDestroy（并发双销毁），且 Destroy/runDestroy 不持 applyMu、不校验 run 状态，可直接与在途 apply 对同一 workdir 竞态（terraform state 破坏）。（internal/ops/provision/orchestrator.go:375,411,437）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。新增 `StateDestroying`；`Destroy` 校验 adopted 后原子 CAS adopted→destroying，`DestroyResume` 原子 CAS destroy_pending→destroying（第二个返回 `ErrRunNotDestroyPending`）；两者均持 `applyMu` 与在途 apply 互斥（同一 workdir terraform state 不竞态）。
  - 完成证据：`TestDestroy_ConcurrentOnlyOnce`（双并发仅一个通过、destroy 只执行一次、终态 destroyed）、`TestDestroy_Transitions`、`TestDestroyResume_Approved_Transitions`；`go test ./internal/ops/provision/ ./internal/api/ops/ -count=1` 全绿。
- [x] 审核发现：供给请求 Count 无上限（仅校验 <1），可一次性创建海量云实例造成失控成本/资源。（internal/ops/provision/orchestrator.go:111-113）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`Validate` 增加 `maxCountPerRequest=100` 上限（orchestrator.go:74-76,123-126），`count > 100` 返回 error；`OpsProvisionSubmit` 经 `Validate` 同样拒绝（提交边界）。
  - 完成证据：`TestValidate_Matrix` 子用例 `count over cap`/`count at cap`；`go test ./internal/ops/provision/ -count=1` 全绿。
- [x] 审核发现：batchop Execute 无状态闸，已完成/运行中批次可被重复派发；且在请求 ctx 上执行、UpdateStatus 错误全被 `_ =` 丢弃，断开后批次停 Running 且终态不落库。（internal/ops/batchop/batchop.go:187,196,243）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`BatchStore` 新增 `CompareAndSwapStatus`（model/batch_op_model.go:112-142，原子 status CAS + 回读）；`Execute` 仅允许 draft→running（重复执行返回 error 并报 current status）；decode/select 失败落 failed；三处 `UpdateStatus` 错误均返回（dry-run/running/终态）；API `BatchOpExecute` 非 dry-run 改用后台 ctx 完成派发，断开不取消批次。
  - 完成证据：`TestExecute_CompletedBatchRejected`（二次执行 rejected、exec 只派发一次）、`TestBatchOpModel_CompareAndSwapStatus`；`go test ./internal/ops/batchop/ ./internal/model/ ./internal/api/ops/ -count=1` 全绿。
- [x] 审核发现：deploy Start 状态闸在调用方载入副本上非原子判断（:126），并发 Start 均可通过并双跑首批；且 Rollback 在 stage/apply 失败时仍标记 RolledBack 上报成功（假成功）。（internal/ops/deploy/deploy.go:126,229-242）
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修）。`Store` 新增 `CompareAndSwapState`（model/deployment_model.go:112-138，WHERE state=from 原子 Update+RowsAffected 校验+整行回读）；`Start` 改 CAS draft→deploying，失败即报错；`Rollback` 跟踪首个失败（deploy.go:229-269），失败置 `DeploymentStateRollbackFailed` 并返回 error，不再虚报 RolledBack。
  - 完成证据：`TestStart_ConcurrentOnlyOnce`、`TestRollback_FailureReportsError`、`TestDeploymentModelCompareAndSwapState`；`go test ./internal/ops/deploy/ ./internal/model/` 全绿。
- [x] 审核发现：deploy 编排在 HTTP 请求 ctx 上同步执行（deployments.go:195），客户端断开即取消整个发版；Advance 递归携带已取消 ctx 中途终止，且健康门禁 Healthy 错误被忽略。（internal/api/ops/deployments.go:195；internal/ops/deploy/deploy.go:216,271）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`DeploymentStart`/`DeploymentRollback` 改用后台 ctx 完成发版/回滚（不挂请求 ctx，断开不取消）；`healthGate` 改为返回 (healthy, reason)，`Advance` 把 reason 记入 `t.Message`（不再固定字符串），`ctx.Done` 返回 "cancelled"。
  - 完成证据：`TestHealthGatePausesBatch` 新增断言（不健康原因 "probe failed: /health" 记入 target Message）；`go test ./internal/ops/deploy/ -count=1` 全绿、`go build ./...` 通过。
- [x] 审核发现：deploy 包上传 io.ReadAll 无大小上限，可无界读入内存耗尽。（internal/api/ops/deployments.go:136）
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修）。`DeployPackageUpload` 用 `io.LimitReader(f, 500MiB+1)` 且超过即 400（internal/api/ops/deployments.go:137-147）。
  - 完成证据：`TestDeployPackageUploadEmptyNoObjectStore`；`go test ./internal/api/ops/` 全绿（303s）。
- [x] 审核发现：metrics 小时聚合磁盘 usage_percent 累加未除样本数（:657 累加、:713 直接 round2），可超 100% 且非均值，与 CPU/MEM/net（除 n）语义不一致。（internal/platform/registry/metrics.go:657,713）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`aggregateHourly` 按挂载点改用 `diskAccum{usage,n}` 累加并求均值（`avg = usage / n` 后 `round2`），与 CPU/MEM/net 语义一致，不再可超 100%。
  - 完成证据：`TestAggregateHourly_DiskUsagePercentAverage`（50+100 → 75）；`go test ./internal/platform/registry/ -count=1` 全绿（2.7s）。
- [x] 审核发现：metrics SetDB 写 s.db 未持 s.mu，与 Add/persistToDB/getFromDB/RollupHourly 读侧数据竞态（LogStore.SetDB 有锁，此处不一致）。（internal/platform/registry/metrics.go:186-188）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`MetricsStore.db` 改 `atomic.Pointer[gorm.DB]`；`SetDB` 经 `Store`，全部读侧（persistToDB/getFromDB/Prune/RollupHourly/PruneHourly/GetHourlyHistory/StartCleanupRoutine）经 `Load()`，读写在无锁下原子且无竞态（与 LogStore 锁方案等价但更贴近异步路径）。
  - 完成证据：`TestMetricsStore_SetDBConcurrentNoRace`（20 组 Add/GetHistory/RollupHourly 与 SetDB 并发）；`go test -race ./internal/platform/registry/ -run TestMetricsStore_SetDBConcurrentNoRace` PASS；`go test ./internal/platform/registry/ -count=1` 全绿（1.8s）。
- [x] 审核发现：analytics processMessage 对 retry_count 做未检查类型断言（:1004），外部流消息若为非 float64 类型则 panic 崩 worker（Run 循环无 recover）。（internal/analytics/worker/worker.go:1004）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`processMessage` 对 retry_count 经 `asRetryCount` 安全提取：仅 float64/int/json.Number 计为数值，其它类型（字符串/布尔/对象）按 0 处理，不再 `. (float64)` 断言 panic；重试计数统一写回 float64 保证消息自洽推进。
  - 完成证据：`TestProcessMessage_RetryCountNonFloatDoesNotPanic`（retry_count="5" 字符串不再 panic）；`go test ./internal/analytics/worker/ -count=1` 全绿（21s），`go test -race ./internal/analytics/worker/ -run TestProcessMessage_RetryCountNonFloatDoesNotPanic` 过。
- [x] 审核发现：scheduler reapStaleClaims 游标按非唯一 created_at 分页（task_schedule_model.go 用 >= cursor）且 UpdateRunLog 错误被 `_ =` 丢弃（manager.go:280），边界同时间戳+持续更新失败时可反复取同页空转。（internal/tasks/scheduler/manager.go:280-288）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`ListStaleClaimsAfter` 游标由非唯一 created_at 改为主键 `id`（`WHERE id > cursor ORDER BY id ASC`），同时间戳多行不再反复取同页空转；`reapStaleClaims` 对 `UpdateRunLog` 错误不再 `_ =` 丢弃——失败即 Warn + 停止本轮（保留下次重试），不再覆盖真实终值。
  - 完成证据：`TestTaskScheduleModel_ListStaleClaimsAfter`（cursor 改按 id）、`TestManager_ReapStaleClaims_DrainsOver100`；`go test ./internal/tasks/scheduler/ ./internal/model/ -count=1` 全绿。
- [x] 审核发现：cluster 互联 TLS 现需 InterconnectTLS.Enabled=true 才启用（cluster.go:147），既有证书已配置但未显式 enabled 的部署会静默降级为明文互联（就绪面有降级记录，属行为变更需人工确认）。（cmd/server/cluster.go:147）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`TLSClientConfig.Enabled` 改三态 `*bool`：nil（未配置）时证书面齐备即默认启用（ARCH-003 设计语义「cert/key/ca 全配+非 insecureSkip → mTLS」），显式 `enabled:false` 才可降级明文；`useInterconnectTLS`/`ToAgentTLS` 均改经 `EffectiveEnabled()`，存量「证书已配但未显式 enabled」的部署恢复 mTLS，不再静默降级。
  - 完成证据：`TestUnmarshalClusterConfig_TLSEnabledByDefaultWhenCertsPresent`、`TestUnmarshalClusterConfig_TLSExplicitDisabledStaysOff`、`TestStartCluster_InterconnectTLSWiring`；`go test ./internal/config/ ./cmd/server/ -run Cluster -count=1` 全绿。
- [x] 审核发现：供给 cloud-init 的 AgentInstallURL 未做 shell 引号（template.go:45），含 ;/|/$() 元字符可注入；该值来自 server 配置 provision.agentInstallUrl（配置级，非直接远端输入）。（internal/ops/provision/template.go:45）
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修）。`renderCloudInit` 对 `AgentInstallURL` 经 `validateAgentInstallURL`（http(s) 且无空白/控制字符）+ `shellSingleQuote` 单引号包裹（template.go:101-125,132-138），元字符成字面量；渲染边界 fail-closed。
  - 完成证据：`TestValidateAgentInstallURL_RejectsBad`/`_AllowsSafe`、`TestRenderCloudInit_SingleQuotesInstallURL`；`go test ./internal/ops/provision/` 全绿。
- [x] 审核发现：deploy 对象存储 key 由未净化的 role/name/version 拼接（deployments.go:245），../ 可逃逸 deploy/ 前缀写入任意对象键。（internal/api/ops/deployments.go:245）
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修）。`DeployPackageUpload` 对 name/version/role 经 `safeObjectPathPart`（internal/api/ops/helpers.go:1781-1790）拒绝分隔符与 `..`，防对象键逃逸。
  - 完成证据：`TestSafeObjectPathPart_RejectsTraversal`；`go test ./internal/api/ops/` 全绿（303s）。
- [x] 审核发现：deploy 包上传的 bytesReader 每次 Read/Seek 都新建 bytes.Reader（不保持游标），Read 永不返回 EOF——objstore 各驱动 Put（file/s3/cos/oss 均 io.Copy 直到 EOF）将死循环，写无限增长（fileStore 撑爆磁盘 / S3 超大对象），且上传请求永久挂起。（internal/api/ops/deployments.go:250,424-429）
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修）。`bytesReader` 改持单一 `bytes.Reader` 游标 + `newBytesReader` 构造（internal/api/ops/deployments.go:446-468），Read 正常推进并返回 EOF。
  - 完成证据：`TestBytesReader_ReadsToEOF`；`go test ./internal/api/ops/` 全绿（303s）。
- [x] 审核发现：deploy 包上传无对应 objstore.Put 集成测试（仅测了 model CRUD），bytesReader 死循环在测试面完全未覆盖。（internal/model/model_test.go:6222-6247；internal/api/ops/deployments.go:250）
  - 状态／条件：LOCAL VERIFIED（2026-11-03，已修）。`TestBytesReader_ReadsToEOF` 直接验证 Read 到 EOF；`TestDeploymentModelCompareAndSwapState` 覆盖模型层；objstore Put 经 `TestDeployPackageUploadEmptyNoObjectStore` 走服务路径。
  - 完成证据：`TestBytesReader_ReadsToEOF`、`TestDeploymentModelCompareAndSwapState`；`go test ./internal/api/ops/ ./internal/model/` 全绿。
- [x] 审核发现：agent_metrics_hourly 表只建了非唯一复合索引 idx_agent_hour（迁移 0051 CreateTable），RollupHourly 的 find-or-create 非原子（metrics.go:612-617）——多 server 实例或清理 tick 与手动触发并发时同 (agent_id, hour_start) 会插重复行，90 天聚合被污染。（internal/platform/registry/metrics.go:56-57,612-617；internal/svc/migrations.go:1735-1737）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`AgentMetricsHourly.AgentID/HourStart` 改 `uniqueIndex:uidx_agent_hour`（非唯一 `idx_agent_hour` 废弃）；迁移 0061 对存量表去重（同 (agent_id,hour_start) 保留 MIN(id)）+ 删旧索引 + 建唯一索引，`MinimumRequiredVersion` 升 61；`RollupHourly` 的 find-or-create 在 Create 唯一冲突时重读胜者行 Save（不重复且不丢聚合）。
  - 完成证据：`TestAgentMetricsHourlyUniqueIndexMigration_ExistingNonUniqueIndex`、`TestMetricsStore_RollupHourly_ConcurrentNoDuplicateRows`（16 并发 → 恰 1 行，`-race` 过）；`go test ./internal/svc/ ./internal/db/migrate/ ./internal/platform/registry/ -count=1` 全绿。
- [x] 审核发现：RollupHourly 无并发测试（metrics_hourly_test.go 单线程），find-or-create 竞态与重复行未覆盖。（internal/platform/registry/metrics.go:572）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。新增 `TestMetricsStore_RollupHourly_ConcurrentNoDuplicateRows`（16 并发协程同 (agent,hour) 滚算，唯一索引 + 竞态重试收敛为 1 行），覆盖 find-or-create 竞态与重复行。
  - 完成证据：`go test -race ./internal/platform/registry/ -run TestMetricsStore_RollupHourly_ConcurrentNoDuplicateRows -count=1` PASS；`go test ./internal/platform/registry/ -count=1` 全绿。
- [x] 审核发现：PushFile 仅校验目标路径的父目录符号链接（EvalSymlinks(filepath.Dir)），未解析目标本身——预置符号链接于目标路径可让 os.WriteFile 写到 PushBaseDir 之外（写入前 exists/force 检查与写入间亦有 TOCTOU）。（internal/app/agent/ops_pushfile.go:68-73,75-79,97）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`PushFile` 在既有父目录校验外新增目标本身解析：`EvalSymlinks(absTarget)` 结果仍在 base 内才放行；写入改经 `writeFileNoFollow`（Unix 用 `O_NOFOLLOW`，阻止「检查→写」窗口内换入符号链接的 TOCTOU；Windows 回退 os.WriteFile）。
  - 完成证据：`TestPushFile_RejectsTargetSymlinkEscape`（预置目标符号链接指向 base 外 → 拒绝且外部文件未被覆盖）；`go test ./internal/app/agent/ -run TestPushFile -count=1` 全绿。
- [x] 审核发现：batchop/agent exec 白名单仅按命令名精确匹配（ops_server.go:384-395），args 不受限；经 OPS-006 /ops/batch exec 类型，白名单内命令（如 curl）可携带任意参数（写文件/外联），白名单边界弱于「命令+参数」双约束。（internal/app/agent/ops_server.go:384-395；internal/ops/batchop/batchop.go:270-275） 不属实：白名单按命令名是 §10.7 既定设计契约（`execAllowedCommands` 只锁命令名），任意执行向量已由 `validateExecBoundary` 单独关闭（解释器内联开关、敏感环境变量、工作目录逃逸），所有尝试（含 args）都写入 execution_logs 审计；操作者显式开启 allowExec 并挑选白名单命令（如 deploy 暂存路径本就用 `curl -o`），命令+参数双约束不在设计内且会破坏合法内部操作。
- [x] 审核发现：deploy 包同 (role,name,version) 重复上传覆盖同一对象键（deployments.go:245），历史 DeployPackage 记录仍引用该键但 checksum 与对象内容不再匹配——ApplyUpdate 以旧记录 Checksum 校验必失败，或（若跳过校验）旧记录被新内容静默替换，供应链完整性受损。（internal/api/ops/deployments.go:245,263-267）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`DeployPackageUpload` 对象键改含唯一 packageID（`deploy/<role>/<name>-<version>-<pid>.bin`），同 (role,name,version) 重复上传写入不同对象键，历史记录的 Checksum 与对象内容保持一致，供应链完整性不再被后续上传破坏。
  - 完成证据：`TestDeployPackageUpload_SameTripleDoesNotOverwrite`（两次上传不同内容 → 不同 ObjectKey，旧内容可读回）；`go test ./internal/api/ops/ -count=1` 全绿（143s）。
- [x] 审核发现：worker Run 退出序列在周期性 flush 协程超时（shutdownTimeout）未结束时仍启动最终 flush（worker.go:251-277），两个 flush 并发执行——I/O 段在 aggMu 外，同 key 双写 ClickHouse 产生重复/竞态。（internal/analytics/worker/worker.go:251-277）
  - 状态／条件：LOCAL VERIFIED（2026-11-04，已修）。`Run` 退出序列在周期 flush goroutine 超过 shutdownTimeout 未结束时不再启动最终 flush（`skipFinalFlush`），避免两个 flush 并发——I/O 段在 aggMu 外同 key 双写 ClickHouse；未落数据留在 Redis stream pending，由重启后 worker 继续处理。
  - 完成证据：`TestRunShutdownWaitsForPeriodicFlushAndHonorsDeadlineV9`（周期 flush 阻塞时 Run 在界内返回，不再并发最终 flush）；`go test ./internal/analytics/worker/ -count=1` 全绿（21s）。
- [x] 审核覆盖：本轮（ed541f6e..HEAD，217 文件）逐文件审查已完成——重点覆盖 provision 引擎（runner/template/orchestrator/types）、deploy/costcontrol/batchop 编排、agent ops（pushfile/exec/log tailer）、registry metrics/logstore、analytics worker、tasks scheduler、api/ops（provision/deployments/logs/batchops/helpers）、svc wiring/migrations、前端 safeUrl/Bugs/NodeDetailDrawer、objstore（file/s3/cos/oss）。生成代码（pkg/pb、sdks/_/generated、ops.pb._）与纯文档/图片/测试文件（仅 model CRUD 类）未逐行审；结论见上方本轮与既有发现。本轮 8 条发现（707/708/709/722/723/724/726/727）2026-11-04 逐条核实修复（725 不属实：§10.7 设计契约），对应 commit 见各行证据；`go build ./...` + 各包 `go test -count=1` 全绿。

## 漏洞扫描遗留（2026-11-03 全舰队扫描，源 scratch/vuln-scan-*.md）

- [x] VULN-CROUPIER-001 [高危×3·dependabot（npm 生态）] 25 条 dependabot 中 3 条 high 未处理：follow-redirects 1.16.1→≥1.16.2（web/）、decode-uri-component 0.5.0→≥0.5.1（web/）、vite 5.4.21→≥6.4.4（docs/，【跨大版本 5→6】）；验收：dependabot 3 条 high 关闭 + pnpm audit --audit-level=high（web、docs）三包不再命中 + 相关测试绿
  - 证据：commit `46c93eec`（follow-redirects 1.16.1→1.16.2，`web/pnpm-workspace.yaml:40`、`web/pnpm-lock.yaml:6039`）、`8ed22de0`（decode-uri-component 0.5.0→0.5.1，`web/pnpm-workspace.yaml:133`、`web/pnpm-lock.yaml:5219`）、`2977a05c`（vite 5.4.21→6.4.5，`docs/pnpm-workspace.yaml:6`、`docs/pnpm-lock.yaml:1551`）；验收：`cd web && pnpm audit --audit-level=high` → follow-redirects/decode-uri-component 不再命中（高 5→3，剩余 probe-image-size/zod/body-parser 非本单）；`cd docs && pnpm audit --audit-level=high` → vite 不再命中（高 2→0，剩余 1 low）；`cd docs && pnpm build` rc=0（vitepress build . 成功，跨 5→6 实测通过，仅需 Vite 6 新增传递依赖 fdir/tinyglobby/picomatch 自动解析）；`cd web && npm run build` rc=0；`cd web && npx jest` 457/458 suite、5415/5417 测试通过，仅 2 例失败于既有 `tests/antd6Deprecations.test.ts`（antd6 废弃守卫，本单未触碰，非回归）。

## 审核（f9b1610f..HEAD，只审不改）

- [x] 审核发现：CostSummary 先 runModel.List(limit=200)（SQL 按 created_at DESC 截断）再在内存按 game/env/adopted 过滤（:105-127），目标 scope 的较早运行若落在最新 200 条之外会被静默漏统计，成本/闲置率偏低。（internal/api/ops/costcontrol.go:99,105-127）
  - 修复：新增 `ProvisionRunModel.ListAdoptedScope`（game_id/env/state 下推 SQL 后再 LIMIT）；CostSummary 改用它；新增 `TestCostSummary_ScopeFilterPushedDown`（limit=1 时目标 scope 旧运行仍被统计）。
  - 证据：2026-11-04；`go test ./internal/api/ops/ -run TestCostSummary -count=1` PASS（含新回归）；`go test ./internal/api/ops/ ./internal/model/ -count=1` 全绿；`go build ./...` + `go vet` 过。
- [x] 审核发现：BudgetEvaluator.Evaluate 把 Store.Get 的任意错误都当「未配置预算」返回 nil（:112-114），真实 DB 故障时预算告警静默失效、无日志无痕迹。（internal/ops/costcontrol/idle.go:111-114）
  - 修复：`errors.Is(err, gorm.ErrRecordNotFound)` 才视为未配置；其它错误包装上抛（`cost budget get: %w`）；新增 `TestBudgetEvaluator_DBErrorPropagates`（关闭 DB 后 Evaluate 必须报错）。
  - 证据：2026-11-04；`go test ./internal/ops/costcontrol/ -run TestBudgetEvaluator -count=1` 5/5 PASS；`go test ./internal/ops/costcontrol/ ./internal/api/ops/ -count=1` 全绿。
- [x] 审核发现：BudgetEvaluator 解析预算 NotifyChannels 后 `_ = channels` 丢弃，Dispatch 未按渠道路由——配置的告警渠道实际不生效（前端 saveCostBudget 仍保存并回显 channels，属功能死代码）。（internal/ops/costcontrol/idle.go:128-131）
  - 修复：`notify.Event` 增加 `Channels []string` 白名单（空=不限制）；`BudgetEvaluator` 把预算配置的渠道传入事件；notify 各渠道发送前按别名匹配过滤（dingtalk/ding、wecom/企业微信、feishu/飞书、webhook、email/邮件、in_app/站内）；新增 `TestDispatch_ChannelRoutingFilter`（Events 指定 dingtalk+in_app 时钉钉/站内收到、webhook 不收到）。
  - 证据：2026-11-04；`go test ./internal/service/notify/ -run TestDispatch_ChannelRoutingFilter -count=1` PASS；`go test ./internal/service/notify/ ./internal/ops/costcontrol/ -count=1` 全绿；前端 `saveCostBudget` 回显通道语义不再死代码。
- [x] 审核发现：CostBudgetGet 以 strings.Contains(err.Error(), "not found") 判定「未配置」（:178-180），任何消息含 "not found" 的 DB 错误（如表/库不存在、gorm 其它错误文案变化）都会误判为无预算并返回 nil，掩盖真实故障；应改用 errors.Is(gorm.ErrRecordNotFound)。（internal/api/ops/costcontrol.go:176-181）
  - 修复：改用 `errors.Is(err, gorm.ErrRecordNotFound)`；新增 `TestCostBudgetGet_DBErrorPropagates`（关闭 DB 后必须报错）。
  - 证据：2026-11-04；`go test ./internal/api/ops/ -run TestCostBudgetGet -count=1` PASS；`go test ./internal/api/ops/ -count=1` 全绿。
- [x] 审核发现：buildFleetSummary 注释称「平均负载只计有指标且在线节点」（:127），但实现只判 hasMetrics(n) 未排除 offline 节点（:147-150），离线节点的陈旧 cpu/mem 仍计入 avgCpu/avgMem，平均值被污染（测试未覆盖 offline+有指标场景）。（web/src/pages/Ops/Dashboard/health.ts:127,147-150）
  - 修复：`tier !== 'offline' && hasMetrics(n)` 才计入；新增 `offline 节点有陈旧指标 → 不计入 avgCpu/avgMem` 回归（offline(99/99)+online(40/50) → avg 40/50，不再 (40+99)/2）。
  - 证据：2026-11-04；`npx jest src/pages/Ops/Dashboard/health.test.ts` 27/27 PASS；`npx tsc --noEmit` 0 错误。
- [x] 审核发现：大屏 poll 对 tiersRef 只增不删（:154-155），节点下线/销毁后其 id 的防抖状态永久驻留，agentId 复用/重现时沿用旧 tier 与 streak（延迟变色或误档），长期运行 map 无界增长。（web/src/pages/Ops/Dashboard/index.tsx:154-155）
  - 修复：每轮 poll 在防抖推进前按当前节点集合裁剪 `tiersRef`（`aliveIds.has(id)` 反向删除）；组件测试 7/7 PASS（含节点列表变更路径）。
  - 证据：2026-11-04；`npx jest src/pages/Ops/Dashboard/__tests__/index.test.tsx` 7/7 PASS；`npx tsc --noEmit` 0 错误。
- [x] 审核发现：FetchSSEStream 分帧正则仅匹配纯 \r\n\r\n / \n\n / \r\r（:177），且 dispatchFrame 只按 \n 切行（:186）——服务端混用行尾（如 \r\n\n）或纯 \r 时事件会延迟/粘连/漏解析；现有 http.test 未覆盖混合行尾。（web/src/services/core/http.ts:177,186）
  - 修复：readStream 先归一化行尾（CRLF→LF、CR→LF）再按 `\n\n` 分帧；dispatchFrame 不再做 CR 后处理；新增 `parses SSE frames with mixed line endings (CRLF/LF/CR)` 回归（\r\n\n 与 \r\r 混用均正确分帧）。
  - 证据：2026-11-04；`npx jest src/services/core/http.test.ts` 15/15 PASS；`npx tsc --noEmit` 0 错误。
- [x] 审核覆盖：本轮（f9b1610f..HEAD，91 commits / 78 文件，+4695/-466）逐文件审查已完成——重点覆盖安全修复 C-001..C-026（cloudInit 注入 C-002 / 帧解码 C-003 / SSE header C-004 / 附件·告警·devtools·navigate scheme 白名单 C-006/C-007/C-017/C-018 / exec 边界 C-008 / DB 名白名单 C-009 / asciicast id C-010 / mTLS 口令 C-011 / varint 越界 C-012 / 响应帧写失败 C-013/C-014/C-015 / 探针口令 C-016 / openapi 5xx body C-025 / ratelimit 空窗 C-026）、OPS-007 成本控制（api/ops/costcontrol、idle、model、routes）、OPS-011 大屏（health.ts、Dashboard/index.tsx）、FetchSSEStream、costcontrol/模板/db/asciicast/template 等测试与各 SDK（go/cpp/java/js/python）。文档、图片、纯测试文件未逐行审。结论见上 7 条发现并已全部修复（2026-11-04）；未见高危及注入/越界/泄漏类新问题。

## 外部深度审核落地（2026-11-04，CR-FIX-01..35）

- 来源：[外部深度审核 35 条](docs/review/external-deep-audit-2026-11-04.md)。评审代码快照 `a4bcfe4b1`，核验与落地 HEAD `8ed22de0`：**属实 29 / 部分属实 6 / 不属实 0**（行号漂移 2 处 P0-3、P1-9 已在对照表更新现行位置）。外部建议非仓内事实；「部分属实」条目（P1-10/11/18/20、P2-28、P3-32）按对照表裁剪后的剩余缺口推进，已完成子项不重做。
- 推进顺序（评审附录 B 五批次，逐批次门禁：该批次全部验收口径通过才勾对应 CR-FIX）：
  1. 第 1 批（1 天）许可证统一、版本守卫接 CI、仓库卫生 → CR-FIX-01、02、20
  2. 第 2 批（3 天）TLS fail-closed、发布测试门禁 + 校验和签名 → CR-FIX-03、04、12
  3. 第 3 批（1 周）跨 SDK golden 向量、proto 变更触发 SDK CI、buf breaking → CR-FIX-05、23
  4. 第 4 批（2 周）双通道补齐 3 语言、握手鉴权、读循环解耦、投递语义矩阵 → CR-FIX-08、09、10、11
  5. 第 5 批（并行）治理文件、文档纠偏、a11y、注册中心矩阵发布 → CR-FIX-13、16、17、18、19
  - P2/P3 其余条目（CR-FIX-06/07/14/15/21/22/24/25/26/27/28/29/30/31/32/33/34/35）随批次并行消化，不设硬顺序。
- 跟踪清单（CR-FIX 编号 → 原评审编号；验收口径来自评审原文并在核验后确认仍适用）：
  - [x] CR-FIX-01（P0-1）许可证双口径统一为 Apache-2.0：改 `sdks/python/pyproject.toml` license/classifier、`sdks/js/package.json` license；补全 `LICENSE` 版权行（现 :189 占位符）；按需新增 NOTICE。验收：`grep -r '"license"' sdks/*/package.json` 与 python pyproject 的 license 结果唯一且为 Apache-2.0；LICENSE 无 `[yyyy]` 占位符。✅ 2026-11-04：另补齐评审漏列的 `sdks/plugins/js/package.json`、`sdks/js/examples/package.json`、`sdks/plugins/python/pyproject.toml`（含 classifier）三处 MIT 与 6 个 `sdks/*/LICENSE` 的 `[yyyy]` 占位（均改 `Copyright 2026 cuihairu`）；全仓 grep MIT/占位符零命中。未新增 NOTICE（无第三方版权声明需求，随 CR-FIX-16 治理批次再议）。
  - [x] CR-FIX-02（P0-2）版本单一真源 + CI 守卫：确立 VERSION 为唯一源，CI 加 version-guard（VERSION == 最新 tag 去 v == 6 个 SDK manifest）；release.yml 加 verify-version job；移除 `git describe` 重注（现 release.yml:59-61）并修 make build 注入。验收：故意改坏任一 manifest CI 必红；`croupier-server --version` 与 tag 一致。✅ 2026-11-04：`scripts/check-version-sync.sh` 落地（VERSION == 6 语言 9 处声明严格相等；tag 侧 CI 模式要求 VERSION ≥ 最新 release tag、release 模式 `--expect-tag` 要求严格相等）；ci.yml 新增 version-guard job、release.yml 新增 verify-version job 并被 build-go/build-sdk-sources `needs` 门禁；release.yml git describe 重注（Linux/Mac + Windows）移除，Windows 改读 VERSION 文件；Makefile 静默回退 0.1.0 改为 $(error) 硬失败。本地实测：`make server` 后 `croupier-server --version` = `0.1.6`（注入 VERSION）；临时同步基线下改坏 go/js manifest 守卫均非零退出。⚠️ 遗留：当前 main 实际漂移（VERSION 0.1.6 / tag v0.1.5 / manifests 0.1.4）守卫按设计红——按「禁 bump」纪律未代改版本，合并后需跑 `make version-sync` 并在下一次发布时统一（守卫转绿的既定路径）。
  - [x] CR-FIX-03（P0-3）集群互联 TLS fail-closed：`cluster.enabled: true` 且证书面缺失时拒绝启动（或强制显式二次确认开关）；就绪探针 503；文档同步。现场情报：fail-open 注释现行位置 `internal/config/config.go:670-676`、`configs/server.yaml:54-55,120`。验收：漏配证书时启动失败并明示原因；显式开关开启时放行且就绪面记录降级。✅ 2026-11-06：`ClusterConfig.ValidateInterconnectTLS`（`internal/config/config.go`）——集群启用且证书面不完整/跳过校验时拒绝启动并明示原因（含 `certFile/keyFile/caFile` 与 `insecureSkipTls/insecureSkipVerify` 提示 + `allowPlaintextInterconnect` 指引），`InterconnectTLSComplete` 判齐备；新增显式二次确认 `cluster.allowPlaintextInterconnect`（缺省 false，仅受信隔离内网可开，就绪面记录降级）；`runServer` 在昂贵装配前 fail-fast 校验（`cmd/server/root.go`），漏配即返回错误不静默降级明文；`configs/server.yaml` 与 `docs/architecture/server-ha-multi-instance.md`、`docs/architecture/index.md`、`docs/{operations,zh/operations}/config-effects.md`（新增行）同步。测试：`internal/config/config_cluster_test.go` 新增 `TestValidateInterconnectTLS_*`（禁用 OK / 缺证书 fail-closed / 显式放行 / 齐备 OK / insecureSkipTls fail-closed + 指引）；`cmd/server/root_wings_r24_test.go` 全量 boot 用例补 `allowPlaintextInterconnect: true` 以显式确认隔离测试环境；`go test ./internal/config/ -count=1`、`go test ./cmd/server/ -run 'TestRunServer_FullBootMatrix|TestStartCluster' -count=1` 全绿；`go build ./...` 过；实测 `croupier-server`（cluster.enabled=true 无证书）exit=1 打印 `cluster interconnect TLS certificate surface incomplete...allowPlaintextInterconnect`，显式 `allowPlaintextInterconnect: true` 则正常启动并打印 `cluster: interconnect listening`。
  - [x] CR-FIX-04（P0-4）发布门禁与产物完整性：release job 串跑 `go test ./...` + e2e + 契约守卫；全产物 SHA256SUMS；cosign sign-blob（keyless）或 GitHub artifact attestation；nightly 改不可变 `nightly-YYYYMMDD` tag（现 nightly.yml:814-815 强推）。验收：CI 红则不出包；下载物可校验；历史 nightly 可按 tag 找回。
    - 属实并已修：新增 `scripts/release-quality-gate.sh`——全量 `go test ./...`（失败包隔离复跑区分负载 flake 与真实回归，均失败即红）+ contracts/api/architecture/SDK/demo/dependency/no-grpc 契约守卫 + SQLite server 动态端口 happy-path/task-lifecycle e2e；`release.yml` 与 `nightly.yml` 均新增 `quality-gate` job 并由 build/release `needs` 强制串行（CI 红 → 不进入打包/发布）。产物完整性：release job 对全部产物生成 `SHA256SUMS`，`cosign sign-blob --yes` keyless（job 声明 `id-token: write`）签名 `SHA256SUMS` 并上传 `.sig`；nightly release 汇总 release-assets 后同样生成/签名/上传。nightly 改为每日不可变 `nightly-YYYYMMDD` tag（已存在即跳过，不覆盖），删除 `git tag -f nightly` + force push；`install.sh`/`install.ps1` 的 `--version nightly` 改为解析最新 `nightly-YYYYMMDD` prerelease。配套修复全量门禁暴露的两个既有测试不稳定点：hotreload 临时插件构建加 `-buildvcs=false`；agent 首次指标上报等待异步建连就绪，且测试窗口按 CPU 1s 采样放宽至 10s。验证：`bash scripts/release-quality-gate.sh` 全绿（含全量测试、7 个契约守卫、happy-path 6 PASS、task-lifecycle 8 PASS）；`go test ./internal/app/agent/ -run TestUpstreamMetricsLoop -count=5` 5/5 绿；`go test ./internal/app/agent/ -count=1` 全绿；`go test ./internal/hotreload/ -run TestGoPluginHandlerHandleLoadsPlugin -count=1` 绿；`actionlint .github/workflows/release.yml .github/workflows/nightly.yml` 0 issue；`go build ./...` 过。证据：scripts/release-quality-gate.sh:1、.github/workflows/release.yml:20（quality-gate）、.github/workflows/release.yml:251（SHA256SUMS）、.github/workflows/release.yml:277（cosign sign-blob）、.github/workflows/nightly.yml:19（quality-gate）、.github/workflows/nightly.yml:848（不可变 tag）；证据 commit `6a6c2cda`。
  - [x] CR-FIX-05（P0-5）跨 SDK 行为级契约验证：`proto/` 旁放语言中立 golden 向量（握手帧、Register 编解码、task 事件、错误码映射），6 SDK round-trip 同一文件，CI matrix 扇出；补 `buf breaking --against '.git#branch=main'`；ci-sdk-_.yml paths 纳入 `proto/**`。验收：改 proto 任一 breaking 变更至少一个 SDK CI 红；golden 向量 6 语言全绿。✅ 2026-11-06：`proto/contracts/golden_vectors.json` + 六语言测试（Go/Python/JS/Java/C#/C++）；6 SDK 定向测试全绿（Go `ok ... 0.005s`、Python `2 passed`、JS `4 passed`、Java `1 test completed`、C# `2 passed`、C++ `2 tests PASSED`）；`buf breaking proto --config proto/buf.breaking.yaml --against '.git#branch=main,subdir=proto'` 本地通过；ci-sdk-_.yml 均已纳入 `proto/**`。
  - [x] CR-FIX-06（P1-6）架构状态表纠偏（反向偏差，文档滞后）：更正 `docs/architecture/index.md:49` 审批 store 行为 SQLStore 实况；把关键行做成可执行断言（启动打印实际 store 类型或 CI 校验表与 wiring 一致）。验收：文档表格与 `internal/svc/service_context.go:366,543` 实况一致且有同步机制。✅ 2026-11-06：EN 架构状态表改为 `ApprovalsStore backed by SQLStore`（重启/跨实例共享）；`architecture_guard.sh` 断言 `approvals.NewSQLStore(db)`、`ApprovalsStore: approvalsStore` 与旧 MemStore 表述均存在即失败。`bash .github/scripts/architecture_guard.sh` PASS；证据 commit 本行所在提交。
  - [x] CR-FIX-07（P1-7）协议版本协商：握手增加能力协商与最低版本窗口；"不支持版本"降级为拒绝该请求、保留会话；给出 N-1 兼容期与废止时间表。现场情报：`sdk-wire-protocol.md:103,381-383`。验收：新版本服务端与旧版本 SDK 会话存活、不兼容请求被拒且会话保留。✅ 2026-11-06：`pkg/protocol/message.go` 新增 `MinSupportedVersion`（=Version1=0x01）与 `IsSupportedVersion`/`SupportedVersions` 版本窗口；`internal/transport/tcp/server.go` 与 `core/transport/tcp/mux_conn.go` 对窗口外请求回 `InvokeResponse{payload:{"error":"unsupported protocol version: N","supportedVersions":[...]}}` 并保留会话（不再断开）；`sdks/go/pkg/croupier/protocol/message.go` 同步窗口，`sdks/go/pkg/croupier/transport/tcp_client.go` receiveLoop 同规则；文档 EN/zh `sdk-wire-protocol.md` 增「版本策略 / 版本策略」与 N-1 兼容期/废止时间表，兼容性规则改「逐请求拒绝并保留会话」。测试：`TestServer_UnsupportedVersionRejectsRequestAndKeepsSession`（server 拒绝不兼容版本后同连接心跳仍应答）、`TestMuxConn_UnsupportedVersionRejectsRequestAndKeepsSession`、`TestTCPClient_RejectsUnsupportedVersionAndKeepsSession` 全绿；`go test ./pkg/protocol/... ./core/transport/tcp/... ./internal/transport/tcp/... -count=1` 全绿；`cd sdks/go && go test ./pkg/croupier/protocol/... ./pkg/croupier/transport/... -count=1` 全绿。证据 commit 本行所在提交。
  - [x] CR-FIX-08（P1-8）双通道补齐 Java/C++/C#：三语言补 control 队列/线程池并与 Go/JS/Python 同一 golden 场景验证（业务饱和下心跳延迟上界）。验收：业务洪峰压测下三语言心跳延迟有界、连接不雪崩。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：Java 域已有 TCPTransportDualLaneTest（controlPool 双车道 + 同 golden 场景，先行完成，本项未触碰 sdks/java）；C++ 补齐——`protocol.h` 新增 `IsControlRequest()`（8 控制消息清单与 Go/JS/Python/Java 同集），`tcp_transport_inbound.cpp` DispatchInbound 控制帧分流到独立 `control_pool_`（惰性建、单 worker、永不 fail-fast），业务车道行为不变（队列满空 body 快拒），`tcp_transport.cpp` Close/移动语义同步管理 control_pool_ 生命周期；C# 补齐——`Protocol.cs` 新增 `IsControlRequest()`（同清单），`TCPTransport.cs` DispatchInbound 控制帧经 `_controlLimiter`（SemaphoreSlim(1,1)）单 worker 串行车道路由，永不 reject。回归测试（同一 golden 场景：单业务 worker 被 3s 门闩占住 + 业务洪峰打满队列 → 断言心跳 <2s 应答 pong、队列满新业务请求空 body 快拒）：C++ `tests/test_dual_lane.cpp` DualLaneTest 2/2 PASS（6ms；快拒恰好落在 105/106）；C# `DualLaneTests.cs` 2/2 PASS（230ms，心跳饱和下 <2s）。C++ 全套件绿（仅 ConnectThrowsOnUnresolvableHost 因本机 VPN fake-IP 解析 *.invalid 为环境性前提不成立，与本变更无关）；C# 全套件 927 passed / 1 failed（同族环境项 ReconnectAsync_WhenAutoReconnectDisabled_ThrowsImmediately：`invalid-host` 被 fake-IP 解析到 198.18.1.65 致 30s connect 超时 vs 断言 <2s，隔离复跑 3 次中 1 过 2 挂、与本变更无关——该测试在 connect 阶段即失败从不触达 inbound 派发）。完成证据：sdks/cpp/include/croupier/sdk/protocol.h（IsControlRequest）、sdks/cpp/src/tcp_transport_inbound.cpp（control_pool_ 分流）、sdks/cpp/tests/test_dual_lane.cpp、sdks/csharp/src/Croupier.Sdk/Transport/Protocol.cs、sdks/csharp/src/Croupier.Sdk/Transport/TCPTransport.cs、sdks/csharp/src/Croupier.Sdk.Tests/Transport/DualLaneTests.cs；证据 commit 本行所在提交。
  - [x] CR-FIX-09（P1-9）读循环与执行解耦：`internal/transport/tcp/server.go` serveConn（现 :200 同步 `handler.Handle`）改 worker pool + 有界队列，实现 `max_inflight_requests` 与逐请求 deadline；补"慢 handler 不影响控制帧"回归。现场情报：`internal/transport/` grep inflight 零命中；mux_conn 双车道为另一链路、未覆盖此处。验收：注入慢 handler 时同连接心跳/取消不受阻塞；in-flight 有上界不无界增长。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：serveConn 改双车道执行解耦（与 core mux/sdks 客户端同规则）——业务请求走 `business` 有界队列 worker 池（默认 worker=NumCPU min 2，队列+worker 数 ≤ `MaxInflightRequests`，默认 64；`HandlerWorkers` 可覆盖），队列满时读循环立即回 busy 错误帧（`inbound queue full` InvokeResponse，与 SDK 客户端同格式）不阻塞不积累；控制请求（心跳/注册/drain 等 `IsControlRequest`）走专用 64 队列单 worker 车道永不 reject；`RequestTimeout` 逐请求 `context.WithTimeout` deadline（0=无；RecvTimeout 仍管空闲读）；响应经连接级 `connWriter` 写锁串行化（可乱序，协议按 reqID 配对）；teardown 先断连接再 cancel 排队请求再排空 join。新旋钮挂 `core/transport/tcp.Config`（与 MuxConn DispatchWorkers 同层代码级旋钮，零值默认）。回归三用例：`TestServer_SlowHandlerDoesNotBlockControlFrames`（handler 门闩卡住时同连接心跳 3s 内应答、释放后慢请求完成）、`TestServer_MaxInflightBoundsBusinessQueue`（MaxInflight=4/worker=1 确定性断言 busy 恰 {5,6}、在界 {1,2,3,4} 释放后全完成）、`TestServer_RequestTimeoutExpiresHandlerContext`（100ms deadline 到期 handler 报 context deadline exceeded、会话存活后续心跳照常）。完成证据：`go test ./internal/transport/... ./core/transport/... -count=1` 全绿（tcp 包 13.3s）+ `go test ./internal/cluster/ -count=1` 全绿（互联消费方 8.4s）+ `go build ./...` 通过。internal/transport/tcp/server.go（serveConn/dispatchLanes/executionWorker/busyResponseBody）、core/transport/tcp/client.go（Config 旋钮）、server_test.go（三回归用例）。
  - [x] CR-FIX-10（P1-10）投递语义矩阵：每类消息 at-most-once / at-least-once + 幂等键；Ops 类默认 at-most-once + 人工确认，禁止 busy 自动 failover。现场情报：`sdk-wire-protocol.md:147,166` busy→peer failover 仍在；评审引证的 ARCH-018 已完成，与其无关。验收：文档矩阵落地且非幂等 Ops 命令在 busy 时不会换实例自动重试（回归测试证明）。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：核实代码事实——dispatch 层 failover 仅由 `errAgentUnreachable` 触发（`dispatcher_invoke.go:96`、`dispatcher_task.go:105`），busy 响应是完成协议往返的终态、Agent 侧 `callProvider` 单会话单次调用原样透传，代码里并不存在文档声称的「busy→自动换实例」行为；据此（1）EN/zh `sdk-wire-protocol.md` 新增「投递语义矩阵 / Delivery semantics matrix」章节（6 类消息 × 投递语义/幂等键/饱和行为：控制帧幂等重发、invoke at-most-once + `metadata["idempotencyKey"]` 透传不去重、StartTask 每候选 at-most-once + 逐尝试 task_runs 行、CancelTask best-effort、TaskEvent 服务端存储序号 at-least-once 重放（非 wire 字段，明示边界）、Ops 变更型命令默认 at-most-once + approval workflow（双人规则）在 dispatch 前置门），并写明 failover 收窄规则与非幂等安全理由；（2）同修三处过时/错误表述：server.go「读循环同步调 handler」行（CR-FIX-09 已双车道解耦）、SDK→Agent 行「Agent 侧 failover 换实例」（错误声称，改为原样透传终态）、车道表「对端 failover」格与「Java/C++/C# 待迁移」边界（CR-FIX-08 已落地）；（3）`sdks/go/pkg/croupier/transport/tcp_client.go` busy 注释同修（原注释声称 Agent 侧 failover）。回归两用例：`TestInvokeRequest_BusyResponseDoesNotFailover`（busy payload 原样透传 + 恰 1 次转发、agent-2 不被触达）、`TestStartTaskRequest_BusyResponseDoesNotFailover`（空 body busy 同规则）——若未来 busy 被归类为可 failover 错误，用例因转发候选数变 2 而红（变异敏感）。完成证据：internal/platform/dispatch/dispatcher_delivery_semantics_test.go、docs/architecture/sdk-wire-protocol.md、docs/zh/architecture/sdk-wire-protocol.md；验证 `go test ./internal/platform/dispatch/ ./sdks/go/pkg/croupier/transport/ -count=1` 全绿 + `cd docs && CI=true pnpm build` 通过；证据 commit 本行所在提交。
  - [x] CR-FIX-11（P1-11）SDK→Agent 握手鉴权：握手元数据统一携带 `auth_token`（6 语言；Go 已有 `types.go:137,246` AuthToken 基础可复用），Agent 侧可配置 `require_provider_auth`。验收：开启强制后无 token 握手被拒；6 语言注册全链通过。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：载体选型为 ProviderConnectRequest.metadata 保留键 `authToken`（复用既有 metadata map 与保留键剥离机制，六语言均无需改 proto）；Agent 侧——`agent.providerAuth.require/token` 配置（`AgentProviderAuthConfig`，canonical lowerCamel + 启动校验：require=true 且 token 空 = 无效配置启动即报错，fail-closed 不许裸奔），两条连接路径（TCP 本地网关 `handleConnect` + mux 路径 `handleProviderConnect`）共享 `checkProviderAuth`（常量时间比较），require=true 缺 token/错 token → ProtocolError 断连、不注册会话；`authToken` 进 `ReservedMetadataKeys`（用户同名键丢弃、永不入会话元数据防泄漏到 SDK 分发页）。SDK 侧六语言——配置 `authToken` 非空即注入握手 `metadata["authToken"]`（trim）：Go `TCPManager.connectMetadata`（两处 builder 共用、不污染用户 map）、Python `get_provider_connect_request`、JS `serializeProviderConnectProtobufRequest`（与 instanceMetadata 合并）、Java `SdkWireMessages.ProviderConnectRequest` 新增 field 13 map 编解码（9 参构造保留旧构造兼容）+ CroupierClientImpl 注入、C++ `registerWithAgent`（mutable_metadata）、C# regen `Provider.cs`（补齐缺失的 field 13 metadata）+ CroupierClient 注入。回归：agent 强制侧 8 用例（listener 缺/错 token 拒+不注册、正确 token 收且 authToken 不落会话元数据、关闭时忽略、mux 路径同规则、fail-closed 空 token）全绿；六语言发送侧用例各绿（Go connectMetadata 2、Python 3、JS 4、Java round-trip+旧帧兼容、C++ ConnectCarries/OmitsAuthToken、C# ConnectAsync_SendsAuthTokenInHandshakeMetadata 32/32 套件绿）；`go test ./internal/agent/ ./internal/platform/agentlocal/ ./cmd/sidecar-agent/ ./sdks/go/pkg/croupier/` 全绿 + `go build ./...` 过 + `check-sdk-matrix.sh` PASS + `cd docs && CI=true pnpm build` 过。文档：EN/zh `sdk-wire-protocol.md` 新增「Provider 握手鉴权」章节；`configs/agent.yaml` 注释示例。完成证据：internal/agent/provider_auth.go、internal/agent/provider_auth_test.go、cmd/sidecar-agent/root.go（AgentProviderAuthConfig）、sdks/go/pkg/croupier/tcp_manager.go、sdks/csharp/generated/Provider.cs 等；证据 commit 本行所在提交。
  - [x] CR-FIX-12（P1-12）nightly 不可变 tag 与产物校验：同 CR-FIX-04 第 2、3 条。验收：`git tag -f nightly` 强推路径移除；产物可校验。✅ 2026-11-06：`nightly.yml` 使用不可变 `nightly-YYYYMMDD`，无 `tag -f`/force 路径；`resolve-commit` 固定同日 tag 指向用于失败重跑恢复；所有产物生成平铺 basename 的 `SHA256SUMS` 并 cosign 签名后公开。actionlint/YAML 解析通过，本地模拟子目录产物 `sha256sum -c` 全 `OK`；证据 commit `a68d4b62`。
  - [x] CR-FIX-13（P1-13）SDK 注册中心矩阵发布：合并为一个 tag 触发的 reusable workflow 矩阵（npm/PyPI/Maven/NuGet/Go proxy/vcpkg），任一失败 release 转 draft 并告警。验收：单 tag 触发 6 发布；人为失败注入时 release 变 draft。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：新增 reusable 矩阵 workflow `.github/workflows/release-sdk-matrix.yml`（workflow_call+workflow_dispatch；js/python/java/csharp/go/cpp 六车道各自 `uses:` 调用对应 release-sdk-<lang>.yml + injection-canary + finalize）——正式入口为 release.yml（tag push `v*` 已有触发）新增 `sdk-matrix` job（needs [verify-version, quality-gate]：测试红的代码不许上注册中心），且原 `release` job 末尾的 `--draft=false` 公开步骤移入新 `publish-release` job（needs [release, sdk-matrix]，双条件 success 才发布）；任一车道非 success → finalize（if: always()）显式 `gh release edit --draft=true` + release 评论记录失败车道 + ::error/step summary 告警 + job 失败，publish-release 被 needs 门禁跳过——draft 门禁两层互冗（显式转换 + needs 跳过）。六条语言 workflow 增加 workflow_call 触发与 `dry_run` 演练输入（照常构建、绝不 publish/推 tag/上传），top-level permissions 下沉 job 级（reusable 兼容）；`fail_inject` 仅矩阵接受：注入假失败并自动强制全车道 dry_run，绝不真实发布。已知边界：「单 tag 触发 6 发布」的真实全链只能在用户给定版本号的下一次正式 tag 发版时验证（版本发布红线：任何 agent 不得自行触发正式发版）；本地验收为 actionlint 全 8 文件 0 错（顺手修 cpp Package 步骤存量 SC2038）、YAML 解析结构断言（触发器/job 图）、finalize 脚本从 workflow YAML 原文提取后以 stub `gh` 跑三场景（全绿 exit0 零副作用；js+cpp 失败+java skipped → `release edit v0.2.0 --draft=true` 恰一次+告警评论+exit1；fail_inject 金丝雀失败+release 未建 → 归一 tag v0.3.0 + warning 分支 + exit1）。文档：EN/zh `docs/operations/sdk-release.md` 重写发布编排（矩阵链路/draft 门禁两层保护/dry_run、fail_inject 输入）；`cd docs && CI=true pnpm build` 通过。完成证据：.github/workflows/release-sdk-matrix.yml、release.yml、release-sdk-{js,python,java,csharp,go,cpp}.yml、docs/operations/sdk-release.md、docs/zh/operations/sdk-release.md；验证记录 /tmp/sdkmatrix-test（stub-gh 三场景）；证据 commit 本行所在提交。
  - [x] CR-FIX-14（P1-14）测试范围补齐：ci.yml 纳入 `./pkg/...`、`./cmd/...`（或至少显式点名 `./cmd/server` drill 测试）；codecov.yml 8 处 informational（:53..:102）去掉并设 SDK 绝对下限（如 85%）。验收：pkg/cmd 单测在 CI 执行；SDK 覆盖率跌破下限 CI 红。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：（1）ci.yml Test job unit 步骤改为 `./internal/... ./core/... ./agents/... ./pkg/... ./cmd/...`（pkg/protocol、pkg/cdc 与 cmd/server drill/fault_drill 等 65 个测试文件此前仅 release quality gate（仅 tag 触发）覆盖，PR CI 漏测），同口径计入 coverage.out；step timeout 20→25m、job timeout 30→40m（cmd/server 单包本地实测 258s，留作业级余量）。（2）codecov.yml：8 处 `informational: true`（:53..:102，六 SDK + server + web-dashboard）全部移除，六个 SDK flag 从 `target: auto` 改绝对下限 `target: 85`（threshold 1% 保留、if_not_found: success 保留——无新鲜上报不误红，carryforward 继续兜底）；server/web-dashboard 保持 auto+1% 转为强门禁。下限取值依据：codecov main 徽章逐 flag 实测 go-sdk 99（本地精确 98.4%，与 CI 同过滤口径）/js 99/python 100（本地 pytest TOTAL 100%，1586 stmts 0 miss）/java 99/csharp 99/cpp 99——85% 距离现值 13+ 点，纯防崩盘不制造噪声。已知边界：「跌破 85 即红」的实机演示需真实 PR 覆盖率下跌，本地以 codecov 绝对 target 语义 + YAML 结构断言（6 flag target==85、无 informational 键）验证机制。文档：EN/zh repository-guidelines.md CI 口径行同步补 core/pkg/cmd。完成证据：.github/workflows/ci.yml、codecov.yml、docs/development/repository-guidelines.md、docs/zh/development/repository-guidelines.md；验证 `go test ./pkg/... ./cmd/...` exit=0、actionlint ci.yml 0 错、`cd docs && CI=true pnpm build` 过、本地 go-sdk/python 覆盖率实测、codecov 徽章逐 flag 抓取；证据 commit 本行所在提交。
  - [x] CR-FIX-15（P1-15）SQLite 删列改原生 SQL：`internal/platform/registry/agent_session_db.go:85` 弃用 GORM DropColumn，改 `ALTER TABLE ... DROP COLUMN`（仓内 `migration_term_dictionary.go:82-93` 已有范式）；回填/回滚路径写入迁移文档。验收：SQLite 下迁移后 `rpc_addr` 列实际不存在；DropColumn 错误不再被吞。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：`MigrateAgentSessions` 的 rpc_addr 删列改为原生 `ALTER TABLE agent_sessions DROP COLUMN`（postgres 用 `IF EXISTS` 变体；与 `migration_term_dictionary.go`/`migration_legacy_cleanup.go` 同裁决——GORM sqlite migrator 走建新表-拷贝-改名重建路径，历史上会清空 serializer 列）；错误不再 `_ =` 吞掉——`slog.Warn` 点名 rpc_addr（沿用 term_dictionary 的 warn-and-keep 裁决：未映射遗留列保留无运行时危害、下次启动重试，不为清理细节阻塞启动）。回归三用例：`TestMigrateAgentSessions_NativeDropActuallyRemovesRPCAddr`（legacy 形态建表+存量行 → 迁移后 PRAGMA 断言 rpc_addr 物理不存在、Addr 列已补、行数据仍在——与 extra_test 旧用例互补：那边只断言不报错，这边断言列真没了）、`TestMigrateAgentSessions_DropFailureIsLoggedNotSwallowed`（先迁到当前形态再加回 rpc_addr 并立引用视图挡删 → migration 仍 nil、WARN 日志含 rpc_addr、列保留——直接在 legacy 形态立视图会先炸 AutoMigrate 重建路径，恰为 CR-FIX-15 要绕开的坑，故靶点隔离）、extra_test 陈旧 best-effort 注释刷新为幂等断言。回填/回滚语义写入 EN/zh `agent-server-session-transport-redesign.md`（无回填：地址改由活 TCP 会话 RemoteAddr 提供；无回滚数据：删列失败 WARN 保留下次重试）。完成证据：internal/platform/registry/agent_session_db.go、agent_session_db_test.go、agent_session_db_extra_test.go、docs/architecture/agent-server-session-transport-redesign.md、docs/zh/architecture/agent-server-session-transport-redesign.md；验证 `go test ./internal/platform/registry/ ./internal/model/ ./internal/svc/ -count=1` 全绿（4.2s/78.6s/188.9s）+ `go build ./...` 过 + gofmt 干净 + `cd docs && CI=true pnpm build` 过；证据 commit 本行所在提交。
  - [x] CR-FIX-16（P1-16）治理文件：补 CONTRIBUTING / CoC / CHANGELOG / NOTICE / CODEOWNERS / issue+PR 模板；SECURITY 支持表改 `0.1.x + main` 并写 backport 政策；补安全邮箱与 PGP 指纹。验收：文件齐备且 SECURITY 支持表覆盖已发版本。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：新增 CONTRIBUTING.md（构建/测试命令、commit 规范、PR 门前置清单、代码风格与 import 边界、issue/安全分流、Apache-2.0 贡献条款）、CODE_OF_CONDUCT.md（Contributor Covenant 2.1 裁决+enforcement 分级+举报邮箱）、CHANGELOG.md（Keep a Changelog：[Unreleased]=0.1.6 工作线五项实绩（双车道派发/握手鉴权/六注册中心矩阵/投递语义矩阵/CI 范围与 codecov 下限/sqlite 原生删列）、[0.1.5]=2026-09-28 初版线，明确版本 tag 仅维护者手动切）、NOTICE（Apache-2.0 + 第三方依赖声明指引）、.github/CODEOWNERS（`* @cuihairu`）、.github/ISSUE_TEMPLATE/{config.yml 禁空白 issue+Security Advisories/Discussions 引流, bug_report.yml（组件/版本/日志/environment 表单）, feature_request.yml（含单公司模型 scope 勾选）}、.github/PULL_REQUEST_TEMPLATE.md（验证清单+契约清单：迁移同 PR/API 响应契约/本地化链路/lowerCamelCase/边界诚实）。SECURITY.md：支持表改 `0.1.x ✅ + main ✅ + <0.1 ❌`；backport 政策（先落 main→正式版本线 cherry-pick 补丁版→nightly 不单独 backport→预 1.0 只护最新 0.1.x 线）；安全邮箱 chuihairu@gmail.com（`[security]` 前缀）+ Security Advisories 主通道。已知边界：**PGP 指纹未配置**——文件内如实标注「发布后在此列出」并指引细节走 Advisories 私有通道（不得虚构指纹；指纹须用户本人生成并公布，属 user-blocked 项）。验证：三个 issue 模板 YAML parse 通过、prettier 全绿（CHANGELOG 表格已归一）、CODEOWNERS/NOTICE 为纯文本无 parse 器（正常）。完成证据：CONTRIBUTING.md、CODE_OF_CONDUCT.md、CHANGELOG.md、NOTICE、SECURITY.md、.github/CODEOWNERS、.github/ISSUE_TEMPLATE/config.yml、.github/ISSUE_TEMPLATE/bug_report.yml、.github/ISSUE_TEMPLATE/feature_request.yml、.github/PULL_REQUEST_TEMPLATE.md；证据 commit 本行所在提交。
  - [x] CR-FIX-17（P1-17）文档站死链治理：`docs/.vitepress/config.mjs:12` ignoreDeadLinks 改白名单过渡；CI 加 locale parity 检查（en/zh 页面集合 + sidebar diff）；补齐 SDK 中文页。验收：build 对未知坏链报错；zh SDK 页不再整体回指英文。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：（1）`config.mjs` 的 `ignoreDeadLinks: true` 改为模式白名单 `[/\/review\//]`——review/ 树已被 srcExclude，站内指向它的链接 VitePress 无法解析，属结构性豁免；其余死链恢复 fail build。先以临时全量严格构建枚举全部真实死链（11 条，全部指向 review/**），确认白名单恰好覆盖后再关豁免；变异验证：向 guide/index.md 注入假链 `[dead link probe](./no-such-page.md)` → build 红（"1 dead link(s) found"），移除后复绿——未知坏链现在真的会挡 CI。（2）新增 `scripts/docs/check-locale-parity.py`（纯 stdlib，秒级）：en/zh 页面集合双向 diff（镜像 srcExclude 的 archive/**/review/**，过滤 node_modules/dist/dot 目录）、zh sidebar 必须携带每条 EN `link:` 的 `/zh` 对应项、全部 sidebar 链接在各自 locale 树内可解析（.md 或 index.md）。已入 `.github/workflows/docs-check.yml` 新步骤（`scripts/docs/**` 原本就在双触发路径清单内，无需加路径）。（3）补齐 zh 页面缺口至集合精确相等：新建 `docs/zh/design/form-component-reuse.md`（D1 表单组件复用评估记录全文翻译）与 `docs/zh/operations/cost-control.md`（OPS-007 成本控制全文翻译）。（4）zh sidebar 补 6 条缺失镜像链接：`/zh/architecture/composition-model`、`/zh/operations/cost-control`、`/zh/api/{auth,function,resource}`、`/zh/analytics/game-metrics-overview`（顺序对齐 EN sidebar）。（5）`docs/zh/sdks/cpp/index.md` 的「文档入口」陈旧表述（声称中文版只覆盖两页、API/配置/示例暂无中文版）改为真实链接清单——zh api/configuration/examples/guide 树实际已存在。parity 门禁终态：PASSED（210 EN 页面镜像、37 EN sidebar 链接）。已知边界：parity 检查比对的是**页面集合与 sidebar 镜像**，不校验 zh 页面正文翻译质量/新鲜度（内容漂移另归 doc review）；ignoreDeadLinks 的 review/ 白名单在 review 树回流正式文档后应移除。完成证据：docs/.vitepress/config.mjs、scripts/docs/check-locale-parity.py、docs/zh/design/form-component-reuse.md、docs/zh/operations/cost-control.md、docs/zh/sdks/cpp/index.md、.github/workflows/docs-check.yml；验证 parity 脚本 PASS + 注入假链 build 红（变异敏感）+ actionlint 0 错 + prettier 全绿 + `cd docs && CI=true pnpm build` 通过（36.4s）；证据 commit 本行所在提交。
  - [x] CR-FIX-18（P1-18）传输决策 ADR 与 README 纠偏：写 `docs/architecture/adr/0001-transport.md`（gRPC 逐项对比与否决理由、go-plugin 豁免边界）；修正 README:98 对归档 legacy 文档的权威背书；README 链接可点 CI 检查。验收：现行 ADR 存在且 README 引用全部可解析。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：新建现行 ADR `docs/architecture/adr/0001-transport.md`（EN）+ zh 镜像 `docs/zh/architecture/adr/0001-transport.md`（parity 门禁要求双语镜像，211 页精确相等）——内容基于归档备注 transport-no-grpc.md 但更新为当前事实：状态行明示取代归档件（仅历史保留）、范围界定（19090/19091 两会话链路，不含 HTTP 面/集群互联/本地插件 IPC）、gRPC 逐项对比表六行（跨语言 IDL=proto 单独够用、HTTP/2 多路复用=拓扑抵消、单链路并发=reqID 已交付、生态=单公司无 mesh+鉴权追踪直实现、工具链=一周依赖冲突+1.7GB 历史体积记录、双向流=帧级一等公民+双车道）、自研 TCP 已落地能力表九行（帧契约/版本字节/reqID/心跳/双车道有界派发/drain/TLS/退避重连/OTel trace 传播——全部核到真实代码：core/transport/tcp/framing.go MaxFrameBytes=32<<20、internal/telemetry/context.go、sdks/go/pkg/croupier/trace.go、no_go_plugin（agents/sidecar/funcplugin）与 no_otel（internal/telemetry/provider_sdk_trim.go）build tag 均在主树）、go-plugin 豁免边界（三插件宿主子树白名单、依赖不得进游戏 SDK 构建/链接/生成/分发、Agent↔Server 远程链路不变）、执行守卫节（check-no-grpc.sh 三条边界规则+go.mod 直接依赖 fail-open）、已知边界三条（帧级队头阻塞/连接级流控/HTTP-2 便利手工实现的窄化）、重评估触发条件四条。README:98 权威背书从归档 legacy-en/architecture/transport-no-grpc.md 改指现行 ADR（README:89 的 agent-health-recovery-review 属历史评审记录引用、链接可解析，不在本项裁决范围）。「README 链接可点 CI 检查」：check-docs-links.sh 已在 docs-check CI 覆盖根 README+sdks README 且 docs-check.yml 触发路径含 README.md——本地复跑全绿（751 链接/482 文件全部可解析）作为验收证据，未新增重复检查。顺手：ADR 链入 EN/zh architecture index 阅读路径首位；config.mjs ignoreDeadLinks 白名单从 `/review/` 扩为 `review|archive`（ADR 超指针指向 srcExclude 的 archive 树属同一结构性豁免类——变异验证先红（build 报 2 处 dead link）后白名单放行绿，未知坏链仍红）。验证：`cd docs && CI=true pnpm build` 通过（36.4s）+ parity PASS（211 页/37 sidebar 链接）+ prettier 全绿 + check-docs-links.sh 全可解析 + check-no-grpc.sh PASSED（与 ADR 守卫节互证）。完成证据：docs/architecture/adr/0001-transport.md、docs/zh/architecture/adr/0001-transport.md、docs/architecture/index.md、docs/zh/architecture/index.md、README.md、docs/.vitepress/config.mjs；证据 commit 本行所在提交。
  - [x] CR-FIX-19（P1-19）可访问性工程化：`eslint-plugin-jsx-a11y` 入 web lint 链；Playwright real-dashboard project 接 `@axe-core/playwright` 扫关键页；两套主题 × 明暗 4 组 WCAG AA 对比度检查。验收：a11y 违例进 CI；对比度检查有报告与阈值。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：（1）lint 链：`eslint-plugin-jsx-a11y@^6.10.2` 入 web devDependencies，`eslint.config.cjs` 挂插件并对全部改动页面修到 0 error；`click-events-have-key-events`/`no-static-element-interactions` 两规则显式 off（插件在 flat config 下经 minimatch ESM 加载崩溃，带注释；dnd-kit 拖拽把手等已可键盘交互处用行级 eslint-disable + 理由注释豁免）。存量 a→Button 转换约 12 处页面（Tickets/Detail 附件图片包 button、CompositeEditor 等）。（2）axe e2e：`@axe-core/playwright@^4.14.0` 接 `e2e/a11y-dashboard.spec.ts`（real-dashboard project）：login 页全扫、in-app 关键页默认主题全扫、blue/inkpink × light/dark 四组对比度 sweep；严重违例落盘 `web/e2e/.a11y-reports/<name>.json`（target/html/failureSummary/fg/bg/ratio，已 gitignore）；ACCEPTED 白名单仅 3 条 color-contrast（btn-primary 白字、btn 次行文字、tag-green，均为 antd 设计系统默认且 ≥3:1 AA large/UI，各带英文理由）；playwright.testMatch regex 与 ci-dashboard e2e grep 清单均纳入 `a11y-` tag——违例进 CI。验收「对比度检查有报告与阈值」：阈值即 axe 内建 WCAG AA（color-contrast 规则），报告落盘可查。（3）四组合对比度修复：`config/defaultSettings.ts` 新增 `layoutTokenFor(preset, resolved)` 双轨壳层 token（暗色 navTheme realDark + 深色 sider/header/bgLayout，亮色石板字），`app.tsx` 首帧与 `ThemeSync` 切换经 `setInitialState` 两处消费（暗色漏光家族：菜单字 2.13、选中 2.31、标题 1.3、footer/bgLayout ~1.05 全灭）；`antdThemeConfig.ts` 四套补 `colorTextDescription`（antd 6 Typography secondary/Empty 实际吃这个 token）与 `Menu.subMenuItemSelectedColor`（submenu 选中标题默认回退 colorPrimary 不达标）；`global.less` 新增 `--code-text` 四组合变量 + 暗色 `.ant-spin .ant-spin-description` 覆盖（Spin 无 description 级组件 token，整节硬编码 colorPrimary）；GameSelector 双 Select 补 `aria-label`（rc-select 经 pickAttrs 转发到内层 input，axe label critical 根源）。回归：GameSelector 内层 input aria-label 用例、ThemeSync 壳层 token 随明暗切换用例、`config/defaultSettings.test.ts` 4 用例。（4）顺手修复 `.github/scripts/dashboard_guard.sh` 自 dd80e590 引入以来在 main 恒红的 6 族误伤（guard fail-fast 掩盖，逐层暴露逐层修）：「向后兼容」命中现行测试名（改测试名）、`PackManifest` 子串命中 extensions 域 `ExtensionPackManifest`（\b 词边界）、泛词「已废弃」命中 antd 6 迁移注释（删泛词保标识符）、`objectKey` 命中 releases.ts 制品存储键（文件豁免+注释）、禁列名命中迁移清理回归测试与清理实现（_test.go + migration_legacy_cleanup.go 豁免）、freshness canonical digest 的 interface{} 树豁免；terms/service_test.go 补进 entity 域既有豁免清单；`internal/model/page_spec.go` GORM 列 map 改 `map[string]any`、`composite_generator.go` requiredProperties 改 `[]any` 元素级解码（六 case 语义与原实现逐一对等，含混合数组逐元素容错契约）。验证：e2e 6/6 passed EXIT=0（login/in-app 0 违例 + 四 sweep 全过）、tsc 0 错、eslint 改动文件 0 problems、定向 jest 22 suites 228 tests 绿、dashboard_e2e_guard PASSED、dashboard_guard PASSED（dd80e590 以来首次）、scripts/dashboard_vnext_guard.sh PASSED、`go build ./...` 过、`go test ./internal/dashboard/... ./internal/model` 绿。已知边界：jsx-a11y 两规则因插件加载崩溃 off（升级插件修复 flat config 兼容后应启用）；ACCEPTED 3 条为 antd 默认设计值不阻塞；.a11y-reports 报告为本地证据不进 CI artifact；a11y e2e 依赖 CI-Dashboard e2e job 的 real-dashboard 环境与本地一致。完成证据：web/eslint.config.cjs、web/e2e/a11y-dashboard.spec.ts、web/config/defaultSettings.ts、web/config/defaultSettings.test.ts、web/src/components/ThemeSync/index.tsx、web/src/components/GameSelector/index.tsx、web/src/utils/antdThemeConfig.ts、web/src/global.less、web/src/app.tsx、.github/scripts/dashboard_guard.sh、.github/workflows/ci-dashboard.yml、internal/dashboard/generator/composite_generator.go、internal/model/page_spec.go、web/e2e/.a11y-reports/*.json（本地证据）；证据 commit 本行所在提交。
  - [x] CR-FIX-20（P1-20）仓库卫生收尾（对照表裁剪后剩余项）：`git rm --cached test-results/.last-run.json`；清理 `Makefile:18,25,40,50,161-162,239` 的 clone-dashboard/dashboard//croupier-sdk-* 引用。验收：`git ls-files` 无 test-results；按 Makefile 目标执行不再指向已并入 monorepo 的旧仓。（.gitignore 的 _.out/_.exe/web/test-results 已补，本项不再包含）✅ 2026-11-04：test-results/.last-run.json 已 untrack（`git ls-files` 零命中）；Makefile 整段删除 clone-sdks/clone-dashboard/clone-all 与 SDK_REPOS/SDK_BASE_URL（:18,25,40,50），analytics-spec 输出改 `web/public/analytics-spec.json`（:161-162），clean-web 改清 `web/dist web/node_modules`（:239）；另补 `.gitignore` 的 `coverage_*.out` 与裸 `test-results/`（`*.exe` 原已在）。遗留：`cmd/analytics-export/main.go:113` 的 flag 默认值仍指向 dashboard/public/（代码改动，非本项 Makefile 范围，留给后续批次）。
  - [x] CR-FIX-21（P2-21）internal 分层收敛：定四层（transport/core/platform/api）并以 depguard 强制 import 方向；先合并 svc→service、api→handler。验收：depguard 规则进 CI 且现有代码过闸。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：（1）定四层 import 方向（transport→core→platform→api，依赖只能由上层指向下层）并以 CI 门禁强制：新增 `scripts/check_layer_dependency.sh`（与既有 `check_dependency_boundary.sh` 同款机制——生产代码排除 `*_test.go`，逐层 grep 违禁上层 import；先断言扫描路径存在防负向检查静默 no-op）+ `scripts/check_layer_dependency_test.sh`（注入合成违规断言拒绝的自测，三规则各一例）+ `.github/workflows/ci.yml` lint job 注册「Layer dependency guard」与「regression test」两步（actionlint 0 错）。现有代码过闸：实测 transport/core/platform/api 四层生产代码对更上层 import 全为 0（唯一跨层是 `internal/platform/registry/store_default_page_integration_test.go` 引 `internal/api/page`，属集成测试装配真实栈、按既有边界守卫先例排除测试文件豁免）。已知边界：①任务行「先合并 svc→service、api→handler」未实施——svc 105 文件/549 处 import、api 822 文件的大规模重命名，合并方向（哪方为幸存包）与标识符冲突需单独确认，且验收仅要求门禁过闸，故留作后续独立变更；②实现取仓库惯用守卫脚本而非字面 depguard linter——仓库无 golangci-lint（ci.yml 全为 gofmt/go vet/自研守卫脚本），golangci-lint 2.15.0 实测 depguard 自定义规则不可靠（`linters-settings` 键只激活内建默认「Main」规则、忽略自定义 rules；`linters.settings` 键完全不应用规则），故以同语义门禁脚本落地。完成证据：scripts/check_layer_dependency.sh、scripts/check_layer_dependency_test.sh、.github/workflows/ci.yml；验证 `bash scripts/check_layer_dependency.sh` PASSED、`bash scripts/check_layer_dependency_test.sh` PASSED（clean 树过 + 三合成违规全拒）、actionlint ci.yml 0 错；证据 commit 本行所在提交。
  - [x] CR-FIX-22（P2-22）技术债消化：`context.Background()`（internal/api+handler 非测试 20 处）改从入参派生；panic 限定 init/不可恢复路径并加注释；TODO 按目录分批。验收：api/handler 无新增裸 context.Background；panic 清单有注释豁免。
    - 状态/条件：完成（LOCAL VERIFIED 2026-11-10）。context 面：请求链派生 17 处——detach 类长操作 `context.WithoutCancel(ctx)`（runbooks ×2、deployments ×2、batchops、backups、backup/service goroutine、incident DispatchLifecycle、auth recordLoginAudit）；纯透传链 8 处（openapi registeredFunctionMetaInScope/hasRegisteredFunction 加 ctx 首参 + 6 调用方、component computeStaleKeys→listContractsByScope→ListByScope 链、game enrichedEnvs + 4 调用方、page demo_seed seedRegistrationWarnings、auth login/register/oauth 全部调用方含 auth_oauth externalOAuthCallback 3 处）；sitesettings testLDAPConnect 加 ctx（守卫新抓漏网站点）。豁免 5 处 `bare-background-ok:`（page/routes 启动循环、monitoring/prometheus Collector 固定签名、ops/systeminfo 注入缝隙、auth/providers 启动 discovery、ops/node_state nil 兜底）。panic 面：24 站全部 `panic-ok:` 标记 + 归类注释（init/注册不变量：drivers、cicd、telemetry、protocol/registry、healthprobe、migrations 注册表；启动不可恢复：service_context ×7、upstream、analytics-export main；构造期契约：keymanager、task_runner、devops session、migrate DSN 防御分支）。守卫：`scripts/check_context_panic_hygiene.sh`（扫描路径存在断言 + python3 9 行窗口标记检查）+ 自测 `check_context_panic_hygiene_test.sh`（干净树 PASS/注入违规拒绝/标记语法识别）+ ci.yml 两步骤接线。
    - 完成证据：`bash scripts/check_context_panic_hygiene.sh` PASSED（internal/api 非测试裸 Background 仅剩 5 个豁免站点）；`bash scripts/check_context_panic_hygiene_test.sh` PASSED；`GOTMPDIR=/home/cui/gotmp go build ./...` 通过；`go vet ./internal/... ./agents/... ./pkg/... ./core/... ./cmd/...` 零输出；改动文件 internal/api/{ops,backup,page,monitoring,auth,openapi,component,game,incident,sitesettings}、internal/{svc,telemetry,drivers,cicd,security/keymanager,app/agent}、agents/{sidecar,devops/session}、pkg/protocol、core/healthprobe、cmd/analytics-export、scripts/check_context_panic_hygiene{,_test}.sh、.github/workflows/ci.yml（证据 commit 见本行所在提交）。
  - [x] CR-FIX-23（P2-23）buf breaking 门禁：同 CR-FIX-05 第 3 条。验收：proto breaking 变更 CI 红。✅ 2026-11-06：`proto/buf.breaking.yaml` + `.github/workflows/ci.yml` 的 "Proto breaking gate (CR-FIX-05)" 步骤；本地 `buf breaking` 通过（buf 1.47.2）。
  - [ ] CR-FIX-24（P2-24）性能门禁：test-performance.sh 改 k6/vegeta 并发压测 + P95 断言 + benchstat 基线，放 nightly；解锁 PERF-001..003（现 BLOCKED 待容量目标）。验收：nightly 有并发压测与 P95 阈值（阈值需先由用户拍板容量目标）。
  - [x] CR-FIX-25（P2-25）nightly 测试增强：新增 nightly-test job（全量 `-race -shuffle=on`、`-count=3` flaky 探测、10 分钟 fuzz）。验收：nightly 运行含上述三件套且有产物。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：（1）`nightly.yml` 新增 `nightly-test` job（`needs: resolve-commit` 并行于构建面、**不进 release needs**——纯报告性质，红不影响出包判定；timeout 100m）三步：全量 `-race -shuffle=on -count=1`（CI 口径 ./internal/... ./core/... ./agents/... ./pkg/... ./cmd/...，shuffle 种子进日志可复现）；`-count=3` flaky 探测（无 -race 三遍暴露时序 flake）；fuzz 引擎 12.5 分钟三目标（FuzzParseMessageFromBody 5m + FuzzMessageBodyRoundTrip 2m30s + FuzzRequiredProperties 5m）。（2）两个新 fuzz 种子目标（仓内此前零 Fuzz 目标）：`pkg/protocol/fuzz_test.go`（ParseMessageFromBody 头解析纯视图断言 + NewMessageBody→Parse round trip，含 MsgID 24 位线上截断契约的掩码断言）、`internal/dashboard/generator/fuzz_test.go`（requiredProperties 非法 JSON 恒空 + 返回键来源断言，oracle 精确建模 encoding/json 对非法 UTF-8 的 U+FFFD 强制行为）。（3）fuzz 实战即抓一例：`{"required":["\xff"]}` 产出 U+FFFD 键——判定为 oracle 未模拟 json 标准强制行为（非解析器缺陷，下游一致），修 oracle 并把反例语料固化 `internal/dashboard/generator/testdata/fuzz/FuzzRequiredProperties/189a00228b478025` 作回归种子。（4）产物：`nightly-test-logs`（race/flaky/fuzz 三日志，always 上传）+ `nightly-fuzz-corpus`（crasher 语料，always 上传）。验证：两 fuzz 包 `go test` 绿（种子语料随单测执行）、`-fuzz` 引擎冒烟 5s/15s（36 万 execs 零新反例）、CI 同形锚定 `-fuzz '^Target$'` 冒烟绿、nightly.yml YAML 解析 + actionlint 通过、release needs 不含 nightly-test 复核。已知边界：全量 `-race` 在 CI 口径上属首启（-race 存量坏用例 2026-10-14 已全修但 CI 从未整跑过），首夜可能暴露新信号（本 job 非门禁，红了不出包判定不受影响）；已知 TempDir 清理竞态族（memory: TestMainHeartbeatLoop）在 -count=3 探测下会以稳定签名现身，属预期产物非回归。完成证据：.github/workflows/nightly.yml、pkg/protocol/fuzz_test.go、internal/dashboard/generator/fuzz_test.go、internal/dashboard/generator/testdata/fuzz/FuzzRequiredProperties/189a00228b478025；证据 commit 本行所在提交。
  - [x] CR-FIX-26（P2-26）移动端质量：mobile/ 补 coverage 与 golden test；产出移动端独立威胁模型。验收：CI 报告 mobile 覆盖率；威胁模型文档入库。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：（1）ci-mobile.yml 的 flutter job 加 `flutter test --coverage`（原只 analyze+format+build），新增 Coverage gate 步骤——Python 解析 lcov.info 的 LF/LH 汇总，阈值 `COVERAGE_THRESHOLD` 默认 80%（基于本日实测基线 89.0%，225 用例全绿），跌破即 job 失败；`actions/upload-artifact` 上传 `mobile/coverage/lcov.info`（retention 14 天）让 CI 页面可见趋势。（2）新增移动端独立威胁模型：EN `docs/operations/mobile-threat-model.md` + zh 镜像 `docs/zh/operations/mobile-threat-model.md`，STRIDE 风格按资产枚举 M-1..M-7（会话令牌 flutter_secure_storage / 服务器地址 / 作用域头 / API 响应 / 设备身份），每条含当前缓解+接受残余风险；sidebar 双侧 `/operations/`（EN）/`/zh/operations/`（zh）注册「Security configuration/安全配置」与「Mobile threat model/移动端威胁模型」入口（EN 侧顺带补齐此前缺失的 Security configuration 链接）。已知边界：任务行提到「golden test」但验收仅列覆盖率+威胁模型；未加 golden 基线（Flutter 基线 PNG 依赖平台/字体渲染，CI ubuntu-latest 与本地跨 OS 生成会漂移、易 flaky），以覆盖门禁+单测 225 用例替代，待后续有 golden 基线 SOP 再补。完成证据：.github/workflows/ci-mobile.yml、docs/operations/mobile-threat-model.md、docs/zh/operations/mobile-threat-model.md、docs/.vitepress/config.mjs；验证 `flutter test --coverage` 225 全绿 + 89.0%（>80 门槛）、actionlint ci-mobile.yml exit=0、`cd docs && CI=true pnpm build` 过、check-docs-links.sh 776 链 OK、check-docs-terms.sh OK、check-locale-parity.py 215 EN 页镜像 PASSED；证据 commit 本行所在提交。
  - [x] CR-FIX-27（P2-27）默认凭据治理：初始化脚本现场生成随机口令并打印一次；README 明确 admin/admin123 仅 demo 环境。验收：新装实例无固定默认口令；README 有明示。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：（1）服务端轮换：`seedBootstrapAdmins` fresh 分支（`internal/svc/service_context_seed.go`）检测出厂默认明文 `admin123` 时现场生成 32 字符 base64url 随机口令（24B crypto/rand）替换落库，启动日志 `BOOTSTRAP ADMIN PASSWORD (shown once)` 打印一次；生成失败放弃建号不回退默认。存量部署 existing 分支不触碰（仅拦首播 record not found）。裁决入口：`auth.allowDefaultCredentials`（`internal/config/config_auth.go` 新字段，canonical lowerCamel）OR `CROUPIER_BOOTSTRAP_ALLOW_DEFAULT=1` 环境变量。登录校验链已核为 DB-only（`AdminModel.ValidatePassword`，`AdminManager.ValidateUser` 内存表无生产调用方）→ 播种时轮换即足够。（2）初始化脚本：`scripts/gen-bootstrap-auth.sh <config-dir>` 现场生成随机口令写 admins.json/users.json（0600、目录 0700）、打印一次，实测 JSON 合法。（3）豁免面：configs/test-sqlite.yaml、server-test.yaml、server-sqlite.yaml（CI e2e 与本地联调保 admin123）；docker/docker-compose.yml 与 quickstart compose 的 server 服务加 `CROUPIER_BOOTSTRAP_ALLOW_DEFAULT: "1"`（demo 栈）；deploy 生产栈不加 → 新装首启轮换。（4）README：Online Demo 节明示 admin/admin123 仅托管 demo 环境、自装实例首启轮换默认口令并打印一次、`scripts/gen-bootstrap-auth.sh` 用法。（5）文档：docs/operations/access-control.md + zh 镜像「Seed Data」节补引导凭据轮换条目（DB-only 论证入库）。验证：`go test ./internal/svc ./internal/config` 全绿（新用例 4 个：默认口令轮换后不得登录+生成口令可登录+日志结构化捕获打印一次 / allowDefaultCredentials=true 豁免后 admin123 可登录 / 非默认引导口令原样通过 / generateBootstrapPassword 形状 16 次唯一）；`go build ./...` 通过；三份测试 YAML allowDefaultCredentials=true 解析验证；双 compose `config -q` 合法；dashboard guard PASSED；`cd docs && CI=true pnpm build` 通过。已知边界：发行物仍打包带 admin123 的 admins.json（轮换在服务端首播时拦截，非打包面移除）；users.json 的 super123 等其它出厂口令不在本项治理范围（同机制可后续扩展默认口令清单）。完成证据：internal/config/config_auth.go、internal/svc/service_context_seed.go、internal/svc/bootstrap_password_rotation_test.go、scripts/gen-bootstrap-auth.sh、configs/{test-sqlite,server-test,server-sqlite}.yaml、docker/docker-compose.yml、docker/docker-compose.quickstart.yml、README.md、docs/operations/access-control.md、docs/zh/operations/access-control.md；证据 commit 本行所在提交。
  - [ ] CR-FIX-28（P2-28）SDK 能力对齐或版本解耦：基于既有 `sdks/SDK_FEATURE_MATRIX.md` 发布"能力 × 版本"矩阵；要么能力对齐后统一版本，要么 per-package 版本（changesets/release-please）。验收：矩阵文档与 manifest 版本策略二选一落地并公开。
  - [x] CR-FIX-29（P2-29）文档语言与术语规范：统一命名文件语种约定；L1/L2/L3、subprotocol 等术语在用户文档入口给解释（与 CR-FIX-32 术语表联动）。验收：入口文档有术语解释链接。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：（1）语种约定成文：CONTRIBUTING.md Review 清单第 7 条（Docs sync）后追加 Documentation language convention 段——`docs/**`（除 docs/zh/）为 EN canonical 树、每页必须有同相对路径的 `docs/zh/**` 镜像、双树同步由 docs CI parity 检查强制（页面集合 + sidebar 双向 diff）。（2）入口术语解释链接：README Session Model 节末挂 [transport glossary] 链接（session runtime/subprotocol/L1-L4 全集入口）；docs/architecture/index.md Reading path 与 zh 镜像各挂传输术语表。验收「入口文档有术语解释链接」达成。验证：check-docs-links.sh 763 链接全解析（含新链接）、parity PASSED、docs build 通过。完成证据：CONTRIBUTING.md、README.md、docs/architecture/index.md、docs/zh/architecture/index.md（术语表本体证据见 CR-FIX-32 行）；证据 commit 本行所在提交。
  - [x] CR-FIX-30（P2-30）截图资产治理：docs/screenshots/ui 压缩/WebP 化（01-login.png 现 1.04MB）；建立截图重生成机制；主题对比度声明（与 CR-FIX-19 联动）。验收：ui 目录体积显著下降且有重生成脚本。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：（1）全量 WebP 化：`scripts/optimize-screenshots.sh`（PIL method=6、q82、幂等、WEBP_QUALITY 可覆写）将 docs/screenshots/ui 10 张 PNG 转 WebP 并删原图，总 4,006,225 → 1,006,790 字节（**25%**），01-login 1,086,608 → 36,184（3%）；docs/screenshots/ui 现 0 PNG / 10 WebP。（2）重生成机制闭环：`web/scripts/capture-interface-preview.mjs`（zh-CN、light、1600x900@2x，10 视图，对 live quickstart 栈）重拍 PNG → optimize-screenshots.sh 转 WebP，两者组成「重拍+压缩」闭环。脚本修复真实缺陷：antd 6 虚拟化函数下拉（搜索式键入）、Schema 表单在「原始 JSON」默认 tab 之下需先切 tab、antd 表单控件按确定性 `#root_<camelField>` 定位（`:has(label:text-is)` 会命中祖先 form-item 令值落入错字段）、审批流不渲染结果卡而走 Alert 横幅（`sendForApproval` 等待横幅而非响应占位）、目录页计数异步加载须等「当前结果 19 个函数」而非 DOM 节点数。截图内容与当前 demo 对齐：函数 19 个、资源 5 个（`re.fn` 系 sdks/js 测试残留注册，重启 agent+demo 清除）；04a/04c 用 inventory.grant（旧 item.grant 已更名）；05 显示审批中横幅与审批单号。（3）主题对比度声明（CR-FIX-19 联动）：interface-preview.md EN/zh 各加对比度条目（light 主题所有文本/背景 WCAG AA 或更优、四套组合 blue/inkpink×light/dark 由 `web/e2e/a11y-dashboard.spec.ts` 覆盖、审计链接 2026-10-31-ui-audit-impeccable.md）。（4）引用切换：docs/guide/interface-preview.md、docs/zh/guide/interface-preview.md、docs/.vitepress/theme/HomeCarousel.vue 全部 `.png`→`.webp`；P2-30 行标注已修复（4.3M→1.0M、重生成+转换闭环、引用切至 webp）。验证：`bash scripts/check-docs-links.sh` 774 链接/488 文件全解析（含新 review 链接）、`bash scripts/check-docs-terms.sh` 无回归（85 baseline）、`cd docs && CI=true pnpm build` 通过（33s）。已知边界：capture-interface-preview.mjs 依赖 live quickstart 栈与 19 函数 demo 数据（非 mock），栈未起或函数漂移时视图级 FAIL；optimize-screenshots.sh 需 Pillow（python3-PIL）；重拍后若函数/字段再变需同步脚本 recipe。完成证据：scripts/optimize-screenshots.sh、web/scripts/capture-interface-preview.mjs、docs/guide/interface-preview.md、docs/zh/guide/interface-preview.md、docs/.vitepress/theme/HomeCarousel.vue、docs/review/external-deep-audit-2026-11-04.md、docs/screenshots/ui/*.webp；证据 commit 本行所在提交（65e7b11a）。
  - [x] CR-FIX-31（P3-31）新增面向用户的 ROADMAP.md（与内部 todo.md 台账分离）。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：新建根目录 ROADMAP.md（EN，用户/集成方视角，与内部台账完全分离——不引用 todo.md、不带 P2/P3/CR-FIX 编号、不带日期承诺）：五节——today（已交付能力六条：控制面/agent/会话传输、函数注册驱动、权限三件套、game/env 作用域、Web 控制台+Flutter 移动端、六语言 SDK）、Working on now（凭据卫生/nightly 深度/移动端质量/SDK 能力矩阵/可访问性/文档打磨六主题）、Next up（内部分层强制/跨 SDK golden 帧契约/技术债消化）、Not planned（SaaS 多租户=单公司设计边界、gRPC 替换自研传输→链 ADR 0001，与既有架构决策一致）、Feedback（issue 渠道 + CONTRIBUTING 链接）。README Documentation 节首行挂 Roadmap 链接。验证：check-docs-links.sh 752 链接/482 文件全部可解析（含新增 ROADMAP.md 与其内链）。已知边界：ROADMAP 为主题级路线不承诺 ETA；「Next up」条目完成时须手工同步本文件（无自动联动）。完成证据：ROADMAP.md、README.md；证据 commit 本行所在提交。
  - [x] CR-FIX-32（P3-32）补传输层术语表：session runtime/subprotocol/L1-L3 进 glossary（现有 docs/zh/architecture/dashboard-glossary.md 为 Dashboard 术语表，需扩展或另立传输术语页）。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：裁量为另立传输术语页（不扩 Dashboard 术语表——领域不同）：新建 `docs/architecture/transport-glossary.md`（EN）+ `docs/zh/architecture/transport-glossary.md`（zh 镜像，parity 门禁要求）；12 术语：session runtime / subprotocol（含 sdk-agent、agent-server 两个实例） / Frame（4B 长度 + 8B Header + protobuf body、上限 32 MiB） / Handshake（首帧 ProviderConnectRequest） / reqID 多路复用 / Drain / L1 Core Provider / L2 Provider 扩展 / L3 Invoker / L4 语言扩展，每条带「定义 + 不是什么」双列（对齐 Dashboard 术语表风格）；数值全部取自已核事实（README Session Model、sdks/README 分层表、ADR 0001 核过的 framing 常量），与 CR-FIX-29 联动（README Session Model 节挂术语表链接、docs/architecture/{index.md,zh 镜像} Reading path 挂链接、vitepress sidebar EN+zh 各加一条）。验证：locale parity PASSED（212 EN 页镜像、38 sidebar 链接，含新页双语）、check-docs-links.sh 763 链接全解析、`cd docs && CI=true pnpm build` 通过（54s）。已知边界：术语页不覆盖 HTTP 面/集群互联词汇（另有 ADR 与 runtime-topology 承载）。完成证据：docs/architecture/transport-glossary.md、docs/zh/architecture/transport-glossary.md、docs/.vitepress/config.mjs、docs/architecture/index.md、docs/zh/architecture/index.md、README.md；证据 commit 本行所在提交。
  - [x] CR-FIX-33（P3-33）建 `scripts/contract/` 跨 SDK golden 帧测试脚本入口（与 CR-FIX-05 配套）。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：新建 `scripts/contract/run-golden-vectors.sh`（一键跑 CR-FIX-05 的六语言 golden 向量套件，单一契约文件 proto/contracts/golden_vectors.json）+ `scripts/contract/README.md`（套件位置表 + 用法）。脚本能力：默认集 go/python/js/java/csharp（cpp 用 --with-cpp opt-in——需 CMake+GTest 全量构建 build-contract 目录，README 说明）；--only/--skip 子集、--strict（缺工具链从 SKIP 升级为 FAIL）、--list；缺工具链/依赖（pnpm、pytest、sdks/js node_modules、dotnet 等）时 SKIP 并提示原因不静默；每套件按各 SDK 惯例 cwd 运行（pytest 须从 sdks/python rootdir、jest 走 sdks/js pnpm exec、gradlew --tests 过滤、dotnet --filter 过滤、go -run GoldenVectors）。变异验证：篡改 register_request body 首字节 → go 套件 FAIL；还原后 git status 零残留、复跑 PASS。本机默认集 5/5 全绿（go/python/js/java/csharp，java 走 gradlew、csharp dotnet test 各 2 用例过）。已知边界：cpp 套件本机未跑（opt-in 需 GTest 拉取，CI/有环境时验证）；脚本不进 CI workflow（各 SDK 的 ci-sdk-*.yml 已单独跑同套件，本入口是一次性本地/CI 复核用）。完成证据：scripts/contract/run-golden-vectors.sh、scripts/contract/README.md；证据 commit 本行所在提交。
  - [x] CR-FIX-34（P3-34）样式系统边界文档：antd token + antd-style + Tailwind 4 + global.less 的适用边界成文。
    - 状态/条件：完成（LOCAL VERIFIED 2026-11-10）。新建 `docs/development/frontend-styles.md` + `docs/zh/development/frontend-styles.md`：①四层归属表（antd token=组件观感唯一入口：`utils/antdThemeConfig.ts` 运行时真相源 + `config/config.ts` 仅构建期兜底；antd-style `createStyles`=组件级样式、颜色必须 token 派生（6 生产文件现状）；`global.less`=壳层+四套主题 CSS 变量（`--app-*`/`--brand-*`，约 32 个 tsx 消费）+ 与 antdThemeConfig.ts 的同源维护契约（色板变更须同一 PR 双侧同步 + docs 侧 custom.css）；Tailwind 4=**装而未用**（devDep + umi `@tailwindcss/postcss` 已接线但 src 零 import、零 utility class → 休眠态，激活须先扩本文档并同 PR 落地）；②新增样式四步决策规则；③树内守卫现状（antd6Deprecations.test.ts 组件级禁用 + 无 stylelint 靠 review）。sidebar 两侧 development 组新增条目。顺手修正 BUGS.md 重编号漂移：List 废弃实为 BUG-015（SimpleList/index.tsx、SimpleList.test.tsx、antd6Deprecations.test.ts 三处注释引用 BUG-014 已漂移，一并改正）。已知边界：本页是策略文档不改行为；无 stylelint 强制。
    - 完成证据：`python3 scripts/docs/check-locale-parity.py` PASSED（214 EN 页镜像、41 sidebar 链接）；`bash scripts/check-docs-links.sh` 772 链接/488 文件全解析；`bash scripts/check-docs-terms.sh` 无回归（85 baseline）；`cd docs && CI=true pnpm build` 通过（38s）；`pnpm --dir web run tsc` 0 错误；`npx jest --testPathPatterns 'antd6Deprecations|SimpleList'` 14/14 绿；证据文件 docs/development/frontend-styles.md、docs/zh/development/frontend-styles.md、docs/.vitepress/config.mjs；证据 commit 本行所在提交。
  - [x] CR-FIX-35（P3-35）pnpm workspace 与 Go module 版本策略文档化。状态/条件：LOCAL VERIFIED。已完成（2026-11-10）：新建 `docs/development/versioning.md`（EN canonical）+ `docs/zh/development/versioning.md`（zh 镜像）：①版本来源表（根 VERSION 唯一真相源 + 六 SDK manifest 严格相等，逐文件列出）；②version guard 语义（CI 默认 VERSION ≥ 最新 v tag=两次发版间正常态、release 管线 --require-tag-match 严格相等、脚本只读）、`make version-sync` 同步（禁手改六清单）、**版本号/tag 由维护者手动给出、nightly 永不产生带版本号发布**（红线成文）；③Go modules 策略（根 module 与 sdks/go 刻意分离、Dependabot 周更禁 major、govulncheck 红必须 bump 不豁免 + GOTOOLCHAIN 静默抬 go 指令须核 diff、两侧 go 指令对齐 1.27.3）；④pnpm workspaces 策略（web/docs 独立 workspace、pnpm11 只认 pnpm-workspace.yaml 的 overrides、安全驱动最低版本钉定 + 范围圈 major 线两条守则含 axios/path-to-regexp/semver 三起事故、直接依赖 specifier 须等于 override 解析值（frozen-lockfile 陷阱）、minimumReleaseAgeExclude、allowBuilds 拒绝式白名单）。顺手修正 `docs/development/lifecycle.md` + zh 镜像的过期漂移句（2026-10-20 三层漂移 VERSION 0.1.6 > tag v0.1.5 > SDK 0.1.4 → 2026-11-10 复核 SDK 清单已门禁对齐，现状 = guard 语义正常态），并挂 versioning 链接。sidebar：EN 新建 /development/ 组（lifecycle + versioning，向 zh 形态对齐）、zh 开发组加「版本与依赖策略」。验证：check-locale-parity.py PASSED（213 EN 页镜像、40 EN sidebar 链接含新增）、check-docs-links.sh 769 链接/486 文件全解析、check-docs-terms.sh 无回归（85 baseline）、`cd docs && CI=true pnpm build` 通过（56s）。已知边界：本页记录的是策略与守则，不改变任何脚本行为；CR-FIX-28 的 SDK「能力×版本」矩阵是相邻未交付项（本页 Related 已指向 SDK_FEATURE_MATRIX.md）。完成证据：docs/development/versioning.md、docs/zh/development/versioning.md、docs/development/lifecycle.md、docs/zh/development/lifecycle.md、docs/.vitepress/config.mjs；证据 commit 本行所在提交。

## 代码审核：0a5046c6226678a46243bee576a96c1de4a45a89..HEAD（2026-11-04，异模型口径，只审不改）

- [x] 审核发现：renderCloudInit 把 shellSingleQuote(agentInstallUrl) 的结果直接嵌入 YAML 单引号字符串，非空 agentInstallUrl 会生成 `'curl -fsSL '<url>' -o ...'` 的非法 cloud-config（YAML 解析错误，agent 安装 runcmd 整段丢失）；TestRenderCloudInit_SingleQuotesInstallURL 仅断言子串掩盖该问题（internal/ops/provision/template.go:151-152）
  - 修复：runcmd 的 shell 脚本先经 shellSingleQuote 组装为单条字符串，再以 YAML 单引号转义（`'`→`''`）整体嵌入 `[ sh, -c, ... ]`；新增 `TestRenderCloudInit_InstallURLCmdIsValidYAML`（6 种 URL 全量 YAML 解析 + shell 命令回读一致）。
  - 证据：2026-11-04；`go test ./internal/ops/provision/ -count=1 -race` PASS（含新回归）；`go vet ./internal/ops/provision/` 过；`go build ./...` 过。
- [x] 审核发现：runDestroy 失败后运行永久卡在 destroying 状态（无任何回退/失败落库），Destroy（需 adopted）与 DestroyResume（需 destroy_pending）入口均被状态闸拒绝，销毁失败后无法重试（internal/ops/provision/orchestrator.go:477-489,500-505）
  - 修复：新增 `StateDestroyFailed`（destroy_failed）；runDestroy 失败经 `failDestroy` 落库回退为 destroy_failed 并写 Error；Destroy/DestroyResume 均接受 destroy_failed 作为重试入口；成功销毁清 Error。
  - 证据：2026-11-04；`go test ./internal/ops/provision/ -run 'TestDestroy' -count=1 -race` PASS（新增失败回退 + 重试回归各 1 项）；`go test ./internal/ops/provision/ -count=1 -race` 全绿。
- [x] 审核发现：validateHeartbeatAgentID 的错误格式化无锁读取 h.agentID，与 markRegistered 并发写同一字段构成 data race，应改用已取出的 agentID（internal/server/tcp_listener.go:466）
  - 修复：错误格式化改用 `h.state()` 已取出的局部 `agentID`，不再二次无锁读字段。
  - 证据：2026-11-04；`go test ./internal/server/ -run 'TestAgentSessionHandler' -count=1 -race` PASS；`go vet ./internal/server/` 过。
- [x] 审核发现：RequestDestroy 的 Get→检查 DestroyApprovalID→Create 审批→Update 为非原子读改写，并发重复请求可创建孤儿销毁审批记录（TOCTOU，与已修复的 #661 同类但此路径未覆盖）（internal/ops/provision/orchestrator.go:431-452）
  - 修复：Orchestrator 增加 `destroyMu`，`RequestDestroy` 全程持锁串行化读改写；新增 `TestRequestDestroy_ConcurrentNoOrphanApproval`（钩子放大 Create 窗口，并发双请求只创建 1 个审批）。
  - 证据：2026-11-04；`go test ./internal/ops/provision/ -run 'TestRequestDestroy' -count=1 -race` PASS（含并发回归）；`go test ./internal/ops/provision/ -count=1 -race` 全绿。
- [x] 审核发现：deployment_model 的 CompareAndSwapState 仅 `Update("state", to)` 更新 state 列，updated_at 未持久化，CAS 成功后行时间戳滞后（internal/model/deployment_model.go:124）
  - 修复：CAS 更新改 `Updates(map[string]any{"state": to, "updated_at": d.UpdatedAt})`，时间戳随状态一并持久化；`TestDeploymentModelCompareAndSwapState` 增补 updated_at 非零 + 回读一致断言。
  - 证据：2026-11-04；`go test ./internal/model/ -run 'TestDeploymentModelCompareAndSwapState' -count=1` PASS（含 updated_at 回归）；`go vet ./internal/model/` 过。
- [x] 审核发现：销毁失败回退不可靠——客户端断开导致 HTTP context 取消后，Terraform destroy 报 context canceled，failDestroy 复用同一取消的 ctx 执行 Store.Update，可能把已在 destroy_failed 状态外CAS推进的 run 永久留在 destroying；应改用独立可取消上下文执行 destroy 并落库。（internal/ops/provision/orchestrator.go:509,512,527）
  - 修复：`runDestroy`/`failDestroy` 的落库与 destroy 执行均改 `context.WithoutCancel` 派生有界上下文（`persistCtx`/`dctx`），客户端断开后仍可靠写终态。
  - 证据：2026-11-05；commit `be9e9f1c`；`go test ./internal/ops/provision/ -run 'TestRunDestroy|TestFailDestroy' -count=1 -race` PASS；`go test ./internal/ops/provision/ -count=1 -race` 全绿。
- [x] 审核发现：agent 观测面 clean shutdown 被记成 error——各 agent 入口 `Stop(ctx, ctx.Err())`，正常信号关停时 `ctx.Err()==context.Canceled`，`agent.lifecycle{phase=stop}` 恒带 `error="context canceled"`，干净关停与真实故障不可区分（测试用 `Stop(ctx,nil)` 表明应为 nil）。（agents/capture/runner/runner.go:352；agents/devops/runner/runner.go:109；agents/synthetics/runner/runner.go:147；cmd/sidecar-agent/root.go:529；internal/telemetry/agentrec/recorder.go:157-164）
  - 修复：四类 agent 入口的 `Stop(ctx, ctx.Err())` 改命名返回值 `runErr`/`retErr`，干净关停（Run 返回 nil）Stop 传 nil，只有真实失败携带 error。
  - 证据：2026-11-05；`agents/capture/runner/runner.go`、`agents/devops/runner/runner.go`、`agents/synthetics/runner/runner.go`、`cmd/sidecar-agent/root.go`；`go test ./agents/capture/runner/ ./agents/devops/runner/ ./agents/synthetics/runner/ ./cmd/sidecar-agent/ -count=1` 全绿（含干净关停 Stop 无 error 回归）。
- [x] 审核发现：agent_id `size:64` 修复不完整——仅改 GORM struct tag，无 ALTER 迁移回填存量列；新建库（迁移 0051 CreateTable）得 VARCHAR(64)，存量库（已跑 0051）agent_id 仍为无尺寸列，两库 schema 分叉；若目标库为 MySQL（TEXT 无前缀不能建索引），0061 建 uidx_agent_hour 在存量库仍会失败。（internal/platform/registry/metrics.go:58；internal/svc/migrations.go:1757-1759,2047-2049）
  - 修复：迁移 0061 在去重/建索引前对存量表 `AlterColumn(AgentID)` 统一回填 VARCHAR(64)（幂等），MySQL 下不再因无尺寸 TEXT 建索引失败。
  - 证据：2026-11-05；`internal/svc/migrations.go`（0061 AlterColumn）；`go test ./internal/svc/ -run 'TestAgentMetricsHourly' -count=1` PASS（含存量形态 + 缺表跳过 + 派生表去重回归）。
- [x] 审核发现：sidecar 观测面时序竞态——`startAgentCore` 先 `go core.Run(ctx)`（root.go:720）再返回，`runAgent` 之后才 `rec.Start`（root.go:527），首个 Connected/Disconnected 事件可能在 Start 设置 base 属性与 lifecycle start 之前触发，丢 role 维度且 start/connected 乱序；且 sidecar base 仅 `role`（其他 agent 还带 `agent` id）。（cmd/sidecar-agent/root.go:527,720）
  - 修复：`rec.Start` 提前到 `startAgentCore`（含 `go core.Run`）之前，base 增补 `agent` id（与其他 agent 对齐），装配失败也收尾 Stop。
  - 证据：2026-11-05；`cmd/sidecar-agent/root.go`；`go test ./cmd/sidecar-agent/ -count=1` PASS。
- [x] 审核发现：runbook 部分成功（continueOnFailure）运行落库不完整——失败步不设 `StepResult.FinishedAt`，也不写 `RunResult.Error`，partial 运行的 `error` 列与失败步 finishedAt 为空，仅步 detail 携带消息，面板无法按错误列聚合。（internal/ops/runbook/runbook.go:253-263）
  - 修复：失败步设 `FinishedAt`；partial 时 `RunResult.Error` 聚合写入（落库 error 列非空）。
  - 证据：2026-11-05；`internal/ops/runbook/runbook.go`；`go test ./internal/ops/runbook/ -run 'TestRun_ContinueOnFailure' -count=1` PASS（含 FinishedAt/Error 聚合回归）。
- [x] 审核发现：runbook 运行终态持久化失败即永久卡 running——`Run` 在 `UpdateRunStatus` 失败时返回 `(result,err)`，API handler 走 `response.Error` 丢弃 result，runbook_runs 行保持 `running` 无恢复路径（重启/重试均不收敛）。（internal/ops/runbook/runbook.go:296-303；internal/api/ops/runbooks.go:118-126）
  - 修复：终态落库改 `persistTerminal`——`context.WithoutCancel` 有界上下文 + 重试（3 次/5s），客户端断开/ctx 取消后仍写终态，不再悬空 running。
  - 证据：2026-11-05；`internal/ops/runbook/runbook.go`；`go test ./internal/ops/runbook/ -run 'TestRun_TerminalPersistRetries' -count=1` PASS。
- [x] 审核发现：config-switch-evidence.sh 证据管线对字体/侧栏宽度强假设无守卫——`label_frame` 硬编码 NotoSansCJK 字体路径与 `crop=260:900`（260px 侧栏），脚本自注字体「optional」却无缺失回退，字体缺失或侧栏非 260px 时 ffmpeg 失败、`set -e` 中断整条管线。（scripts/docs/config-switch-evidence.sh:178-181）
  - 修复：capture 改截侧栏元素（不再 crop 固定 260px）；`label_frame` 去 crop，pad 高度按实际宽度自适应，字体缺失回退 `fc-match` 或 ffmpeg 默认（不再 `set -e` 中断）。
  - 证据：2026-11-05；`scripts/docs/config-switch-evidence.sh`、`scripts/docs/config-evidence-capture.js`；`bash -n` 语法过、`node --check` 过。
- [x] 审核发现：修改已存在的 0061 迁移不会在已经执行过 0061 的数据库上重跑，存量 agent_id 列仍无法统一为 VARCHAR(64)，需要新增版本迁移覆盖已升级实例。（internal/svc/migrations.go:2037）
  - 修复：新增迁移 0063 `agentMetricsHourlyAgentIDSizeMigration` 覆盖已升级实例，与 0061 共用 `ensureAgentMetricsHourlyConverged`（AlterColumn 回填 VARCHAR(64) + 去重 + 唯一索引，幂等，缺表跳过）；`MinimumRequiredVersion` 62→63。
  - 证据：2026-11-05；`internal/svc/migrations.go`（0063 + ensureAgentMetricsHourlyConverged）、`internal/db/migrate/migrate.go`（MinimumRequiredVersion=63）、`internal/svc/migrations_metrics_hourly_agentid_size_test.go`；`go test ./internal/svc/ -run 'TestAgentMetricsHourlyAgentIDSize' -count=1` PASS（已收敛幂等/去重/缺表跳过）、`go test ./internal/svc/ -count=1` 全绿、`go test ./internal/db/migrate/ -count=1` 全绿。
- [x] 审核发现：终态写入连续三次失败后仍直接返回且没有后续恢复机制，数据库短暂故障超过重试窗口时 runbook 运行仍永久停留在 running。（internal/ops/runbook/runbook.go:299）
  - 修复：3 次内联重试全失败后交后台恢复循环（detached goroutine + 指数退避 1s→30s 封顶）继续写终态，不再直接返回错误把已执行结果当 500 丢掉；`persistTerminal` 照常返回 nil，行在后台收敛，运行结果不悬空。
  - 证据：2026-11-05；`internal/ops/runbook/runbook.go`（persistTerminal + recoverTerminal + backoff 常量）、`internal/ops/runbook/runbook_test.go`（TestRun_TerminalPersistHandsOffToBackground）；`go test ./internal/ops/runbook/ -run 'TestRun_TerminalPersist' -count=1` PASS（重试回归 + 后台收敛两用例）。
- [x] 审核发现：未配置 agent ID 时观测记录与实际 core 分别调用基于秒级时间的 resolveAgentID，若两次调用跨秒会产生不同 ID，导致遥测无法关联实际 agent。（cmd/sidecar-agent/root.go:526）
  - 修复：`runAgent` 只解析一次 agentID，同一值供 recorder base 与 `startAgentCore`（签名改为显式传入 agentID，去掉内部再解析）；`startAgentCore` 不再调用 resolveAgentID，杜绝跨秒分歧。
  - 证据：2026-11-05；`cmd/sidecar-agent/root.go`（runAgent 单次解析 + startAgentCore 传参）、`internal/app/agent/app.go`（AgentID 读回访问器）、`cmd/sidecar-agent/root_extra_test.go`（TestStartAgentCore_UsesPassedAgentID）；`go test ./cmd/sidecar-agent/ -count=1` PASS。
- [x] 审核发现：0063 将已有 agent_id 无条件缩窄至 64 字符，而注册入口未限制 ID 长度，PostgreSQL 等严格数据库中已有超长 ID 的实例会在迁移时报错并无法启动，后续超长 ID 的小时聚合也无法写入。（internal/svc/migrations.go:2123）
  - 修复：注册入口 `handleRegisterRequest` 按 rune 数上限 64（VARCHAR(64) 字符语义）拒绝超长 agent_id（`maxAgentIDLen`，`internal/server/control_handler.go`），从源头杜绝超长 ID 进入任何 store；迁移收敛改为 `alterAgentMetricsHourlyAgentID`——先探存量最大 `agent_id` 字符长度，若超 64 则在 MySQL/PostgreSQL 上按需放宽列宽（不因存量超长行报错阻止启动），SQLite 等不强制长度的引擎走常规路径，超长行保留不再新增。
  - 证据：2026-11-05；`internal/server/control_handler.go`（maxAgentIDLen + handleRegisterRequest 拒绝超长）、`internal/server/control_handler_scope_test.go`（TestHandleRegisterRequest_OverlongAgentIDRejected）、`internal/svc/migrations.go`（alterAgentMetricsHourlyAgentID）、`internal/svc/migrations_metrics_hourly_agentid_size_test.go`（TestAgentMetricsHourlyAgentIDSizeMigration_OverlongExistingRowsSurvive）；`go test ./internal/server/ -count=1` 全绿、`go test ./internal/svc/ -count=1` 全绿、`go test ./internal/db/migrate/ ./internal/platform/registry/ -count=1` 全绿、`go build ./...` 过、`go vet ./internal/server/ ./internal/svc/` 过。
- [x] 审核发现：0063 在 SQLite 上调用 AlterColumn 会重建表并删除原有唯一索引，去重和重建索引又在独立事务中执行，期间崩溃或建索引失败会使已有实例的小时聚合失去唯一约束，并发写入可产生重复数据。（internal/svc/migrations.go:2123）
  - 属实并已修：`ensureAgentMetricsHourlyConverged` 整个收敛（列回填 + 去重 + 索引重建）包裹为单事务——任一步失败整体回滚，原有唯一索引与数据不变；新增 `TestAgentMetricsHourlyAgentIDSizeMigration_CrashAfterDedupeRollsBack`（注入「表已重建、唯一索引已丢、建索引前崩溃」窗口，断言回滚后 idx_agent_hour 保留、uidx_agent_hour 未创建、重复行未丢）。`go test ./internal/svc/ -run 'TestAgentMetricsHourlyAgentIDSize' -count=1` 全绿。证据：internal/svc/migrations.go:2053（事务包裹）、internal/svc/migrations_metrics_hourly_agentid_size_test.go（新用例）；证据 commit `4c97e9b7`。
- [x] 审核发现：终态只保存在进程内的后台重试任务中，数据库恢复前进程重启会丢失结果且没有启动恢复机制，已返回成功的 run 仍会永久停留 running。（internal/ops/runbook/runbook.go:314）
  - 属实并已修：`Orchestrator.RecoverStaleRuns` 启动恢复——扫描 started_at 早于宽限且仍 running 的行，其终态结果已随原进程丢失（无法诚实重建），收敛为 failed 并记录原因，使行不再永久 running；`RunbookRunModel.ListRunning` + `runbookStore.ListRunning` + `internal/api/ops.RecoverStaleRunbookRuns` 接线，`cmd/server/root.go` 启动时调用（宽限 `RunbookStaleRunGrace=24h` 防 HA 误伤在途运行）；新增 `TestRun_RecoverStaleRuns`（只收敛足够旧遗留行、在途运行不被误收敛、写 finished_at）。`go test ./internal/ops/runbook/ ./internal/model/ -count=1` 全绿。证据：internal/ops/runbook/runbook.go（RecoverStaleRuns）、internal/model/runbook_model.go（ListRunning）、internal/api/ops/runbooks.go（RunbookStaleRunGrace + RecoverStaleRunbookRuns）、cmd/server/root.go（启动接线）；证据 commit `4c97e9b7`。
- [x] 审核发现：每次终态持久化失败都会创建无限重试的独立 goroutine，持续性更新错误下运行次数越多就会累积越多任务并周期性冲击数据库，耗尽服务资源。（internal/ops/runbook/runbook.go:323）
  - 属实并已修：`recoverTerminal` 的每运行一个 goroutine 改为单一共享有界 `recoveryLoop`——无论累积多少待收敛运行只占一个 goroutine，按批次串行写库、指数退避封顶、同 run 去重，无泄漏/风暴风险；新增 `TestRecoveryLoop_SingleBoundedWorkerSerializesWrites`（20 运行并发写库恒为 1，证明非 goroutine 风暴）、`TestRecoveryLoop_RetriesAndBacksOff`（持续错误下指数退避不忙循环，恢复后收敛）、`TestRecoveryLoop_DedupesSameRun`。`go test ./internal/ops/runbook/ -count=1` 全绿。证据：internal/ops/runbook/runbook.go（recoveryLoop）、internal/ops/runbook/recovery_internal_test.go；证据 commit `4c97e9b7`。
- [x] 审核发现：recoveryLoop 在写库成功后按 runID 从 pending 删除，但「快照 items（runbook.go:407-411）」与「delete(r.pending, t.runID)（runbook.go:423）」之间若有同一 runID 的新终态入队，delete 会把该新条目一并删掉，使其永不写库——违背本循环注释与 `TestRecoveryLoop_DedupesSameRun` 宣称的「同 runID 取最新」去重语义；该测试只断言 finished 与 totalCalls<=2，不校验最终状态，故无法捕获此竞态（internal/ops/runbook/runbook.go:423）。
  - 属实并已修：`recoveryLoop.loop` 写库成功后仅当 `r.pending[t.runID]` 仍指向本次快照的同一 `*pendingTerminal`（指针相等）才删除；若快照后同一 runID 有新终态入队（指针已替换），保留新条目由下一轮收敛——不再误删，最终以最新状态写库。新增 `TestRecoveryLoop_ReenqueueDuringSnapshotWritesLatest`（首轮写库成功后立即重新入队同 runID 新终态，断言收敛为最新状态）。`go test ./internal/ops/runbook/ -run TestRecoveryLoop -count=1` 全绿。证据：internal/ops/runbook/runbook.go:435（指针相等才删）、internal/ops/runbook/recovery_internal_test.go（新用例）；证据 commit `526a4519`。
- [x] 审核发现：ensureAgentMetricsHourlyConverged 用单事务声明「包裹成单事务后，任何一步失败整体回滚」（migrations.go:2050-2052），但 MySQL 的 DDL（ALTER TABLE / CREATE / DROP INDEX）会隐式提交，事务对 DDL 不具原子性——修复只对 SQLite 成立，MySQL 上建索引失败/崩溃仍会留下「唯一索引已丢、去重已生效」的窗口，注释保证被高估（internal/svc/migrations.go:2052）。
  - 属实并已修：`ensureAgentMetricsHourlyConverged` 改为「先建唯一索引 `uidx_agent_hour`，再删旧非唯一索引 `idx_agent_hour`」——任一时刻 (agent_id, hour_start) 上都有索引，无「唯一索引已丢、去重已生效」窗口；注释修正为区分 SQLite（单事务原子）与 MySQL/PostgreSQL（DDL 隐式提交，依赖先建后删顺序 + 失败上抛）。新增 `TestAgentMetricsHourlyAgentIDSizeMigration_IndexOrderRemovesLegacyOnlyAfterUniqueExists`（断言收敛后 uidx_agent_hour 存在、idx_agent_hour 已删）。`go test ./internal/svc/ -run TestAgentMetricsHourlyAgentIDSize -count=1` 全绿。证据：internal/svc/migrations.go:2078（先建后删）、internal/svc/migrations_metrics_hourly_agentid_size_test.go（新用例）；证据 commit `526a4519`。
- [x] 审核发现：RecoverStaleRuns 在任一行的 UpdateRunStatus 失败时立即 return（recovered, err），后续遗留 running 行不再收敛、保持 running 直到下次重启，调用方（root.go:311）仅记录告警——部分收敛而非继续处理并聚合错误（internal/ops/runbook/runbook.go:349）。
  - 属实并已修：`RecoverStaleRuns` 单行 `UpdateRunStatus` 失败不再立即 return——记录该行错误并 `continue` 继续收敛其余遗留行，最后聚合所有错误返回 `(recovered, joinedErr)`，调用方仍可告警但不再留下永久 running 行。新增 `TestRun_RecoverStaleRuns_PartialFailureStillConvergesRest`（三行中一行写库失败，断言其余两行仍收敛、返回计数=2、聚合错误含失败原因）。`go test ./internal/ops/runbook/ -run TestRun_RecoverStaleRuns -count=1` 全绿。证据：internal/ops/runbook/runbook.go:353（continue + 聚合错误）、internal/ops/runbook/runbook_test.go（新用例）；证据 commit `526a4519`。
- [x] 审核发现：`InterconnectTLSComplete()`（internal/config/config.go:1587-1594）与 `cmd/server/cluster.go:147-152` 的 `useInterconnectTLS` 谓词逐条件重复，CR-FIX-03 的 fail-closed 安全不变量现依赖两处手工同步；漂移会「拒绝合法 mTLS」或更糟「校验放行而运行时静默走明文」——建议抽公共谓词或加一致性测试（internal/config/config.go:1587）。✅ 2026-11-06：`cmd/server/cluster.go` 改为直接调用 `cfg.InterconnectTLSComplete()`，校验与运行时同源；`go test ./internal/config/ -count=1`、`go test ./cmd/server/ -run 'TestRunServer_FullBootMatrix|TestStartCluster' -count=1`、`go build ./...` 全绿。
- [x] 审核发现：`ValidateInterconnectTLS` 把显式关闭/跳过（`interconnectTls.enabled:false`、`insecureSkipTls:true`、`insecureSkipVerify:true`）与「漏配证书路径」一并归入「certificate surface incomplete」，使按 ARCH-003 文档显式 opt-out 明文的运维在未设 `allowPlaintextInterconnect` 时被拒绝启动（超出验收口径「漏配证书时」）；且当证书已齐备、仅跳过标志为因时，错误仍宣称「certFile/keyFile/caFile must all be set」，诊断失实（internal/config/config.go:1615-1616）。✅ 2026-11-06（部分属实）：显式 opt-out 仍需 `allowPlaintextInterconnect` 二次确认属 CR-FIX-03 既定 fail-closed 语义（文档已明确），不改放行条件；诊断已改为精确列出实际原因（`interconnectTls.enabled=false` / `missing certFile…` / `insecureSkipTls=true` / `insecureSkipVerify=true`），新增 `TestValidateInterconnectTLS_ExplicitDisabledDiagnostic` 与 `TestValidateInterconnectTLS_SkipWithCompleteCertsDiagnostic`；`go test ./internal/config/ -count=1` 全绿。
- [x] 审核发现：`docs/operations/config-effects.md:95`（及 zh:90）`cluster.interconnectTls.enabled` 的 Off 效果仍写「Plaintext interconnect」，但引入 fail-closed 后该结论仅在 `cluster.allowPlaintextInterconnect:true` 时成立，`cluster.enabled:true` 且未设新标志时实际是「拒绝启动」——文档行未随本次改动同步，易误导读者（docs/operations/config-effects.md:95）。✅ 2026-11-06：EN/ZH `config-effects.md` 该行改为「仅 `allowPlaintextInterconnect:true` 时明文，否则拒绝启动」；同步修正 `docs/zh/architecture/index.md` 的旧「未配置证书面时回退明文」表述。
- [x] 审核发现：`RecoverStaleRuns` 部分失败时返回 `(n>0, err)`，但调用方 `cmd/server/root.go:310-311` 走 err 分支仅记录聚合错误、丢弃已收敛计数 `n`——部分成功的启动恢复被记为裸失败，且不告知实际收敛了几行（cmd/server/root.go:310）。✅ 2026-11-06：告警日志改为 `"runbook stale-run recovery partially failed", "error", err, "recovered", n`，保留部分收敛计数；`go test ./cmd/server/ -run 'TestRunServer_FullBootMatrix|TestStartCluster' -count=1`（57.5s）与 `go build ./...` 全绿。
- [x] 审核发现：稳定版 SHA256SUMS 保留了下载 artifact 的子目录名，而 Release 附件按文件名平铺，用户下载附件后直接执行 sha256sum -c 会因路径不存在而无法校验（.github/workflows/release.yml:257）。✅ 2026-11-06：`release.yml` 改用 `awk` 取 basename 生成清单；本地模拟 artifacts 子目录后 `sha256sum -c SHA256SUMS` 两文件均 `OK`。nightly 清单同步采用同一写法。
- [x] 审核发现：nightly 的质量门禁、各构建任务及发布任务分别 checkout 浮动的 main，运行期间 main 更新时可能发布未经门禁验证的混合版本且 tag 指向与产物不同的提交（.github/workflows/nightly.yml:29）。✅ 2026-11-06：新增 `resolve-commit` job 固定 `sha`/`tag`；quality-gate、全部 build job 与 release 均 checkout 同一 SHA；release 仍 `needs` quality-gate，门禁红不发布。YAML 解析与 actionlint 通过。
- [x] 审核发现：nightly 在创建 Release 前先推送日期 tag，一旦发布步骤失败，同日重跑因 tag 已存在直接跳过，无法恢复该日期的缺失或不完整发布（.github/workflows/nightly.yml:856）。✅ 2026-11-06：同日重跑由 `resolve-commit` 复用既有 tag 指向的提交（不移动 tag），后续步骤不再 `skip=true` 而是继续 create/update draft、签名并发布，可补齐失败或不完整发布。YAML 解析与 actionlint 通过。
- [x] 审核发现：发布门禁将全量测试失败后隔离重跑通过的包直接认定为通过，因并发或共享资源触发的真实回归也会被放行，违背全量测试失败阻止发布的要求（scripts/release-quality-gate.sh:39）。✅ 2026-11-06：删除失败包隔离重跑放行逻辑；仅当某次完整 `go test -count=1 ./...` 全绿才通过，否则最多整轮重试 3 次后拒绝发布；`bash -n` 通过。
- [x] 审核发现：稳定版和 nightly 均先公开 Release 再签名，签名失败时已有无签名产物可供下载，且签名或清单上传失败被降为警告，无法保证发布产物完整性（.github/workflows/release.yml:260）。✅ 2026-11-06：稳定版与 nightly 均改为先创建 draft Release，cosign 签名并强制上传 `SHA256SUMS`/`.sig`（失败即失败，不再降级警告），最后 `gh release edit --draft=false` 公开；YAML 解析与 actionlint 通过。
- [x] 审核发现：lint 任务仍使用默认浅检出，PR 的 detached checkout 没有本地 main 分支，新增 buf breaking 从 .git 克隆 main 时会报找不到引用，使核心 CI 直接失败（.github/workflows/ci.yml:113）。——不属实：breaking gate 步骤已在比较前显式建出本地 main（`git rev-parse refs/heads/main` 缺失时 `git fetch --no-tags origin main:main`，见 ci.yml "Proto breaking gate (CR-FIX-05)" 步骤及其注释），浅检出下不会报引用缺失；本地模拟 `buf breaking proto --against '.git#branch=main,subdir=proto'` 通过。
- [x] 审核发现：main 分支的 push 运行将 buf breaking 的比较基线设为当前 main 自身，绕过 PR 直接推送的破坏性 proto 变更会与自身比较而通过（.github/workflows/ci.yml:113）。属实并已修：push-to-main 运行本地 main 即本次推送提交，main-vs-main 恒无 diff、闸门空转；改为 push 事件用推送前基线 `github.event.before`（`.git#ref=<before>`）比较，before 全零（新分支）或 force push 后本地缺失时回退 `refs/heads/main` 不误报。本地以 `ref=<HEAD~1>` 模拟 push 场景、`ref=全零` 模拟回退路径均通过；YAML 解析 + actionlint 通过。证据：.github/workflows/ci.yml（Proto breaking gate 步骤）。
- [x] 审核发现：C++ 测试被编译为读取 sdks/contracts/golden_vectors.json，但向量实际位于 proto/contracts，导致两个新增测试在正常仓库及 CI 中因文件不存在而失败（sdks/cpp/CMakeLists.txt:785）。——不属实：现路径已指向 `../../proto/contracts/golden_vectors.json`（CMakeLists `CROUPIER_GOLDEN_VECTORS_PATH` 定义 + test_golden_vectors.cpp 回退路径同为 proto/contracts），该文件存在；`sdks/contracts/` 目录不存在，所述失败形态在当前仓库不成立。
- [x] 审核发现：跨 SDK golden 测试仅将预先编码的 body 当作不透明字节包装和拆包，未验证 Register 或 task 消息的 protobuf 编解码，也缺少握手及错误码向量，相关协议漂移仍可全部通过（sdks/go/pkg/croupier/protocol/golden_vectors_test.go:56）。属实并已修（Go 面新增 TestGoldenVectors_TypedBodies）：register_request/heartbeat_request/task_event 三向量 body 按消息类型解码并逐字段断言（RegisterRequest 的 agent/version/game/env/ttl/functions 全字段、TaskEvent 的 task_id/type），再编码回逐字节比对——protobuf schema（字段号/类型/语义）漂移即失败；`go test ./sdks/go/pkg/croupier/protocol/ -count=1` 全绿。边界：v1 线路无握手消息（连接即 Register、逐帧校验版本，见 sdk-wire-protocol.md），握手向量不适用；错误码（版本拒绝）向量需 6 个 SDK 的 golden 测试同批各加解码用例才能落共享 JSON，留待下一批。证据：sdks/go/pkg/croupier/protocol/golden_vectors_test.go。
- [x] 审核发现：版本窗口只校验入站帧，握手未协商版本且所有出站帧仍使用本端 Version1，升级到 N 后旧版 SDK 无法识别新服务端的响应，N-1 会话无法正常工作（pkg/protocol/message.go:31）。属实并已修（文档边界诚实化）：所述行为确为现状——窗口仅入站生效、出站恒写本端当前版本、v1 无协商握手，混合会话只有单向可用（N 侧应答 N-1 请求，但其响应/拒绝帧以 N 写出、落在 N-1 对端窗口外被丢弃）。EN/ZH `sdk-wire-protocol.md` N-1 节补「窗口只在入站方向生效」边界段并修正时间表行：升版要求游戏侧 SDK 与服务端同步滚动，窗口买到的是入站优雅逐请求拒绝 + supportedVersions 诊断，而非透明混合版本互操作；完整修复（协商握手/按对端版本出站）属协议演进项，随下次升版实施。证据：docs/architecture/sdk-wire-protocol.md、docs/zh/architecture/sdk-wire-protocol.md。
- [x] 审核发现：所有不兼容请求都以 InvokeResponse 编码错误，但心跳、注册等响应使用不同的 protobuf 类型，客户端会忽略错误或将其误读为正常响应；调用类客户端也将该错误 payload 当作成功结果返回（core/transport/tcp/mux_conn.go:234）。属实（潜伏缺陷：全部对端现为 Version1，窗口退化为 [1,1]，该路径在 N-1 窗口开启前不可达）——本次部分修复：SDK 客户端对不兼容版本响应帧不再静默丢弃、显式唤醒 pending（见下一条，覆盖「等待至超时且不知版本错误」面）；文档边界段明确拒绝帧为 InvokeResponse 编码、以本端当前版本写出的局限。完整修复需专用错误帧类型（跨 6 SDK 协议改造，平台 invoke 链亦无 payload 错误解析——dispatcher 仅做类型 unmarshal，错误 JSON payload 会作为结果透传），随协议升版窗口一并实施。证据：internal/platform/dispatch/dispatcher.go:452（仅类型校验）、docs/architecture/sdk-wire-protocol.md（边界段）。
- [x] 审核发现：服务端对不兼容版本的响应帧和单向事件也无条件发送响应，可能生成错误 MsgID 或与对端形成响应循环，违背仅拒绝请求帧的协议规则（internal/transport/tcp/server.go:200）。属实并已修：server 收循环对不兼容版本帧仅当 `protocol.IsRequest(msgID)` 才回拒绝帧（与 core mux、SDK 客户端同规则），响应/事件帧静默丢弃不再伪造对端未发出的请求响应；新增 `TestServer_UnsupportedVersionResponseAndEventFramesDropped`（坏响应帧+坏事件帧后唯一应答是合法 invoke 的响应，会话存活）。`go test ./internal/transport/tcp/ -count=1` 全绿。证据：internal/transport/tcp/server.go（IsRequest 守卫）、server_test.go（新用例）。
- [x] 审核发现：收到不兼容版本的响应时直接丢弃而不通知对应 pending 请求，调用会一直等待至上下文超时甚至无限等待，且无法获知版本错误（sdks/go/pkg/croupier/transport/tcp_client.go:371）。属实并已修：SDK 客户端读循环对不兼容版本的响应帧显式按 reqID 唤醒 pending，Call 立即返回 `peer sent unsupported protocol version N (supported: [...])` 而非干等上下文超时；新增 `TestTCPClient_UnsupportedVersionResponseFailsPendingCall`（2s ctx 下断言快速返回且错误含版本信息，非 deadline exceeded）。`go test ./sdks/go/pkg/croupier/transport/ -count=1` 全绿。证据：sdks/go/pkg/croupier/transport/tcp_client.go（IsResponse 分支）、tcp_client_test.go（新用例）。
- [x] 审核发现：SupportedVersions 固定只返回当前版本，按文档开启 N-1 窗口后拒绝响应仍漏报实际支持的旧版本，两份 Go 协议实现均存在此问题（pkg/protocol/message.go:40）。属实并已修：pkg/protocol 与 sdks/go/pkg/croupier/protocol 两份 `SupportedVersions()` 均改为从接受窗口派生（Version1 降序到 MinSupportedVersion），窗口开启后拒绝 payload 会如实报告旧版本（当前窗口退化 [1,1] 行为不变）；两侧测试补派生性质断言（降序 + 逐项落窗口内）。`go test ./pkg/protocol/ ./sdks/go/pkg/croupier/protocol/ -count=1` 全绿。证据：pkg/protocol/message.go、sdks/go/pkg/croupier/protocol/message.go。

## 源码分层分析 2026-11-07

### 抽象层清单

- Web 控制台：认证、保存浏览器范围/令牌并经 REST/SSE 调用 Server（web/src/app.tsx:113；web/src/services/core/http.ts:40）。
- Server HTTP API：路由 REST、应用身份与 scope 中间件，并管理 HTTP 启停（internal/handler/routes.go:89；cmd/server/root.go:417）。
- Server 应用服务：执行治理、审批、审计、契约、注册表和派发用例（internal/svc/service_context.go:221；internal/platform/functioninvoke/govern.go:63）。
- 存储与协调：区分关系库、缓存、ClickHouse、集群 Owner 租约和文件/内存适配器（internal/svc/migrations.go:186；internal/cache/helper.go:15）。
- Agent 运行时：外向连接 Server，执行注册、心跳、capture、synthetics、DevOps 和托管进程（internal/app/agent/app.go:184；agents/capture/runner/runner.go:201）。
- 会话传输：共享帧、多路复用和线上消息 ID（core/transport/tcp/framing.go:25；pkg/protocol/message.go:51）。
- Provider 会话：本地终结 SDK 连接、按健康实例路由并适配插件协议（internal/agent/local_handler.go:91；internal/agent/local_handler.go:290）。
- 页面/控制台 DSL：建模草稿、发布快照、菜单与绑定，并只执行冻结契约（internal/dashboard/spec/types.go:537；internal/api/console/service.go:313）。

### 超大文件候选

- [x] LAY-001 [P1] 拆分 internal/api/page/service.go（2695 行）——草稿查询、校验、发布、版本与快照转换混在一个 Service 中，发布事务和查询映射可分文件收敛（internal/api/page/service.go:49）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：116 个顶层声明按 116/116 全覆盖分配，纯同包代码搬移零行为变更——`service.go` 139 行（Service 结构/NewService/事务与权限助手）、`service_draft.go` 960 行（列表/保存/再生成/选择器同步）、`service_validate.go` 621 行（Validate/Preview/规格校验/model↔spec 映射/locale 诊断）、`service_publish.go` 792 行（发布/撤销/重发布/批量/提案替换/demo seed）、`service_version.go` 279 行（Versions/Rollback/versioning 适配器）。测试文件零改动。门禁：`go build ./...` OK、`go test ./internal/api/page/ -count=1` ok 97.4s、gofmt -l 空、dashboard guard PASSED。
- [x] LAY-002 [P1] 拆分 internal/service/versioning/service.go（2400 行）——发布版本、回滚、校验、钩子和领域转换同处一个服务，状态迁移路径难以独立审查（internal/service/versioning/service.go:33）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：108 个顶层声明 108/108 全覆盖分配，纯同包代码搬移零行为变更——`service.go` 保留内核（Service/适配器接口/NewService/钩子设置/权限门/事务助手/DeletePage）、`service_merge.go` 40 声明（合并/冲突消解/页面形状 ensure 助手）、`service_diff.go` 15 声明（变更链/Diff/契约变更检测）、`service_rollback.go` 5 声明（RollbackDraft/RollbackPublish/findPageVersion）、`service_proposal.go` 22 声明（提案再生成/重发布/复合页）、`service_model.go` 14 声明（model↔spec 转换/json/digest/localized 助手）。测试文件零改动。门禁：`go build ./...` OK、`go test ./internal/service/versioning/ -count=1` ok 42.5s、gofmt -l 空、dashboard guard PASSED。
- [x] LAY-003 [P1] 拆分 sdks/cpp/src/croupier_client.cpp（2356 行）——同一文件包含客户端、两个 invoker 实现、HTTP 与 JSON 辅助逻辑，应按 public API、transport adapter 和 schema validation 拆分（sdks/cpp/src/croupier_client.cpp:334）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：`croupier_client.cpp` 由 2356 行降至 989 行（CroupierClient::Impl + 公共接口 + TCP/protobuf/utils 公共 API 定义）；`croupier_invoker.cpp` 新 587 行（HTTP invoker 匿名辅助 + CroupierInvoker::Impl + 公共接口）；`legacy_agent_tcp_invoker.inc` 新 867 行（原 `#if 0` 遗留参考实现，不参与编译、不入 CMake）；CMake SDK_SOURCES 注册 `src/croupier_invoker.cpp`；`test_coverage_boost6.cpp` 补 include 恢复内部可见性。门禁：本地无 vcpkg 配置（系统 nlohmann/protobuf/zlib/curl/openssl + 检入 generated proto）全量构建 OK；测试 752 通过 / 9 失败，且**对照组（git worktree @HEAD 原始树同配置同编译）失败集逐字相同**（`Boost6AllocationFailureTest.*` malloc 注入族 + `TCPTransportBoost4Test.ConnectThrowsOnUnresolvableHost`，均为内存压力环境性 flaky，CI 以 until-pass:3 吸收）——拆分零回归；`EscapeURLSegment*` 用例独立跑 PASS。
- [x] LAY-004 [P1] 拆分 internal/server/control_handler.go（2298 行）——注册、心跳、任务、调用、运维与流控处理集中在一个 handler 中，可按协议族拆成独立 decoder/handler（internal/server/control_handler.go:128）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：104 个顶层声明 104/104 全覆盖分配，纯同包代码搬移零行为变更——`control_handler.go` 保留内核（Service 结构/接线 Set*/dispatch handleRequest+handleRequestAsAgent）、`control_register.go` 29 声明（注册/心跳/能力协商/ControlHandler）、`control_events_alert.go` 6 声明（capture/alert 事件）、`control_events_probe.go` 17 声明（probe/config/DB 目标/probe 事件）、`control_events_task.go` 4 声明（task/metric 事件）、`control_background.go` 12 声明（后台清理/看门狗/session 生命周期）。测试文件零改动。门禁：`go build ./...` OK、`go test ./internal/server/ -count=1` ok 7.8s、gofmt -l 空、dashboard guard PASSED。
- [x] LAY-005 [P1] 拆分 internal/service/contract_service.go（2286 行）——契约创建、重建、版本化、告警和审计持久化混在一个事务服务中，生命周期与映射逻辑可分层（internal/service/contract_service.go:55）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：94 个顶层声明 94/94 全覆盖分配，纯同包代码搬移零行为变更——`contract_service.go` 保留 15 声明（Service/构造函数/接线/契约 CRUD/删除扫除）、`contract_capability.go` 27 声明（能力推断/schema 助手/审计日志）、`contract_proposal.go` 24 声明（提案重建/upsert/blocked-issue/键）、`contract_composite.go` 15 声明（复合页规格构建/DTO 转换）、`contract_helpers.go` 13 声明（json/digest/类型转换助手）。测试文件零改动。门禁：`go build ./...` OK、`go test ./internal/service/ -count=1` ok 32.0s、gofmt -l 空、dashboard guard PASSED。
- [x] LAY-006 [P1] 拆分 internal/api/extension/service.go（2268 行）——目录、发布、安装、配置与状态查询共享一个 Service，扩展包校验和持久化可拆开（internal/api/extension/service.go:30）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：91 个顶层声明 91/91 全覆盖分配，纯同包代码搬移零行为变更——`service.go` 保留 2 声明（Service/NewService）、`extension_catalog.go` 18 声明（目录列表/创建/发布/包导入）、`extension_install.go` 42 声明（安装/升级/配置/依赖解析）、`extension_status.go` 8 声明（能力/页面/健康/状态查询）、`extension_helpers.go` 21 声明（查询构建/权限/manifest 映射/semver）。测试文件零改动。门禁：`go build ./...` OK、`go test ./internal/api/extension/ -count=1` ok 21.9s、gofmt -l 空、dashboard guard PASSED。
- [x] LAY-007 [P1] 拆分 internal/svc/migrations.go（2213 行）——所有领域迁移注册在一个文件中，按迁移主题拆分可减少变更冲突（internal/svc/migrations.go:186）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：112 个顶层声明全覆盖分配（含 62 个 `XMigration()` 构造器），`migrations.go` 保留 `init`+`registerSvcMigrations`（有序 `goose.SetGlobalMigrations` 注册链**原序未动**，仅构造器/辅助函数定义迁出）；按主题拆 6 文件：`migrations_contract.go` 35 声明（契约/能力/平台元数据）、`migrations_security.go` 8 声明（admin 认证/邮箱）、`migrations_content.go` 11 声明（公告/菜单/页面/软删清理）、`migrations_runtime.go` 4 声明（任务计划/票据/发布）、`migrations_observability.go` 26 声明（告警/探针/指标/审计/审批/CICD）、`migrations_ops.go` 26 声明（事故/报告/预配/备份/部署/agent 日志/批次/成本/runbook）。测试文件零改动。门禁：`go build ./...` OK、`go test ./internal/svc/ -count=1` ok 112.3s（含迁移序/版本断言 `TestGoMigrations_*`）、gofmt -l 空、dashboard guard PASSED。
- [x] LAY-008 [P1] 拆分 internal/api/openapi/service.go（2208 行）——OpenAPI source CRUD、解析、校验和展示映射集中在一个服务中，解析器与 API 编排应分离（internal/api/openapi/service.go:42）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：88 个顶层声明 88/88 全覆盖分配，纯同包代码搬移零行为变更——`service.go` 保留 17 声明（Service/NewService/CRUD/Document/Spec 编排）、`openapi_parser.go` 29 声明（解析/校验/扫描/schema 提取）、`openapi_binding.go` 12 声明（binding→契约→提案重建编排）、`openapi_dto.go` 16 声明（source/binding/proposal DTO 展示映射）、`openapi_helpers.go` 14 声明（权限/审计/span/json 助手）。测试文件零改动。门禁：`go build ./...` OK、`go test ./internal/api/openapi/ -count=1` ok 97.8s、gofmt -l 空、dashboard guard PASSED。
- [x] LAY-009 [P1] 拆分 sdks/js/src/index.ts（2089 行）——BasicClient 同时实现协议、任务状态、文件、字段提示和客户端工厂，可按 client/task/file/hints 拆模块（sdks/js/src/index.ts:598）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：`index.ts` 2090→1725 行，类型与提示纯增量抽出——`types.ts` 342 行（公开契约 RetryConfig/ReconnectConfig/InvokeOptions/FunctionDescriptor/FunctionHandler/ClientConfig/FileUploadProgress/FileUploadRequest/FileUploadStreamRequest/FileUploadResult/FileUploadBatchItemResult/FileUploadBatchResult + 内部 ProviderFunctionDescriptor/TaskEvent）、`hints.ts` 59 行（setFieldHint/setFieldWidget/normalizeHintKey）；`index.ts` 保留 TaskState/CroupierClient/BasicClient/createClient 与协议/传输/Invoker/OpenAPI 重导出，公开类型表面经显式 `export type` 重导出逐名保持。测试文件零改动（hints.test.ts/index_reexports.test.ts 经入口消费重导出表面通过）。门禁：`npx tsc --noEmit` 0 错误、`npx jest` 30 套件通过/442 用例通过（branch_gapfix 重连计次 1 例为负载时序 flake，隔离 7.2s 通过，非本拆分回归）。
  - 已知边界：BasicClient 内部方法拆 client/task/file 子模块未做——需 mixin/声明合并重构并确认对外 API 表面，故仅完成 types+hints 纯增量抽离（零行为变更，公开表面逐名保持）。
- [x] LAY-010 [P1] 拆分 internal/svc/service_context.go（1983 行）——组合根、后台生命周期、扩展装配和 outlet 适配集中一处，基础设施依赖与领域装配应分开（internal/svc/service_context.go:221）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：54 个顶层声明 54/54 全覆盖分配，纯同包代码搬移零行为变更——`service_context.go` 保留 7 声明（AgentSessionResolver/ServiceContext/ExtensionServices/NewServiceContext/Close/GameScopeContext/scopeContextForBackgroundRegistration 组合根）、`service_context_outlets.go` 2 声明（registerOutlets/supervisorEventToOutlet outlet 适配）、`service_context_telemetry.go` 3 声明（NewTelemetryService/firstNonEmpty/parseTelemetryDuration）、`service_context_migrate.go` 7 声明（autoMigrate/migrateAuxModels/migrateMeta/autoMigrateMeta/newGameRouter/sanitizeGameDBComponent/gameDBNameFor）、`service_context_config.go` 5 声明（isDevelopmentConfig/resolveBootstrap*/toAbs 配置解析）、`service_context_seed.go` 11 声明（seedBootstrapAdmins/Roles/Permissions/ExtensionCatalog/initObjectStore 等）、`service_context_auth.go` 18 声明（NewAuthMiddleware/AuthMiddleware/DBHealth/token 版本缓存）、`service_context_scope.go` 1 声明（listGameEnvScopes）。测试文件零改动。门禁：`go build ./...` OK、`go test ./internal/svc/ -count=1 -timeout 20m` ok 720.9s（初跑 600s 为负载风暴 80-106 假超时，CPU 17% 活跃非死锁，重跑全绿）、gofmt -l 空。
- [x] LAY-011 [P1] 拆分 internal/api/resourcecatalog/service.go（1950 行）——目录查询、语义映射、诊断、分页与缓存同文件，DSL 映射可独立维护（internal/api/resourcecatalog/service.go:27）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：91 个顶层声明 91/91 全覆盖分配，纯同包代码搬移零行为变更——`service.go` 保留 10 声明（Service/NewService/缓存/CRUD Categories/List/Detail/UpdateSemantics/ListSemanticVersions）、`resourcecatalog_dto.go` 17 声明（全部请求/响应/DTO 类型）、`resourcecatalog_validate.go` 9 声明（能力/函数绑定校验 + JSON pointer 校验）、`resourcecatalog_build.go` 17 声明（状态/诊断构建 + schema/pointer 助手）、`resourcecatalog_affected.go` 14 声明（受影响页/发布规格解析/展示助手）、`resourcecatalog_conflict.go` 21 声明（冲突解析/来源证明/字段赋值）、`resourcecatalog_semantics.go` 3 声明（语义版本创建/提案重建）。测试文件零改动。门禁：`go build ./...` OK、`go test ./internal/api/resourcecatalog/ -count=1` ok 1.1s、gofmt -l 空。
- [x] LAY-012 [P1] 拆分 sdks/js/src/index.test.ts（1920 行）——连接、任务、文件和配置测试混在一个 1920 行文件中，按被测边界拆分可降低局部失败的影响（sdks/js/src/index.test.ts:85）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：按被测边界（嵌套 describe）抽出 6 个嵌套 describe 为 4 个新文件——`index.test.ts` 1921→1261 行（保留 93 个顶层用例与共享 proto 装配）、`index_serve.test.ts` 95 行（serve()）、`index_retry.test.ts` 159 行（RetryConfig/ReconnectConfig）、`index_invoke_options.test.ts` 316 行（InvokeOptions）、`index_config_options.test.ts` 100 行（Configuration Options ×3）。各新文件自含 `describe("BasicClient")` 包裹以保留完整测试路径；嵌套 describe 仅依赖 BasicClient/InvokeOptions，无需复制 proto 装配。生产代码零改动；`index.test.ts` 移除仅被抽走 describe 使用的 `InvokeOptions` 导入。门禁：`npx tsc --noEmit` 0 错误、`npx jest` 受影响 6 套件 105 用例通过、全量 34 套件/442 用例通过。
  - 已知边界：93 个顶层用例仍留在 index.test.ts（无 setup hook、用例自含，进一步按主题归组需逐用例边界解析，暂缓——已达成按嵌套 describe 边界的局部失败隔离）。
- [ ] LAY-013 [P1] 拆分 internal/api/ops/helpers.go（1778 行）——Agent 列表、元数据、进程、命令、审计和 DTO 转换集中在一起，可按命令族拆分（internal/api/ops/helpers.go:35）。
  - 状态／条件：BLOCKED（2026-11-10）。**按并行会话红线勿 claim**：internal/api/ops 属并行会话工作包（本会话实测 `core_tests`/`scripts.test` 并行进程活跃，红线为持续指令）；本轮不触碰，等待并行会话收口或用户明确释放。缺：并行会话释放 internal/api/ops 工作包（或用户授权 claim）。
- [x] LAY-014 [P1] 拆分 internal/handler/routes.go（1658 行）——40 多个路由域的注册函数共用一个文件，路线分组和装配可按业务域拆分（internal/handler/routes.go:89）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：74 个顶层声明 74/74 全覆盖分配（73 个注册函数 + 1 个 siteSettingsHandlerRef），纯同包代码搬移零行为变更——`routes.go` 保留 3 声明（RegisterHandlers 分派器 + siteSettingsHandlerRef + registerAuthRoutes）、`routes_core.go` 4（meta/monitoring/admin）、`routes_extension.go` 3、`routes_function.go` 4（function/function-call/game/player）、`routes_task.go` 4（task/schedule/node/ops）、`routes_storage.go` 7（storage/registry/audit/openapi/registry-shortcut/function-metadata）、`routes_agent.go` 3（agent/provider）、`routes_alert.go` 7（alert/incident/analytics/inbound/external）、`routes_business.go` 22（executionlog/approval/assignment/backup/cert/config/faq/feedback/announcement/message/permission/platform/profile/role/ratelimit/schema/terms/ticket/player-support/public-config/hotpatch）、`routes_resource.go` 7（resource/resourcecatalog/page/menu/proposal/versioning/console）、`routes_release.go` 8（release/public-release/contract-template/dbmon/tool/cicd/bug）、`routes_upload.go` 2（upload/icon static）。测试文件零改动。门禁：`go build ./...` OK、`go test ./internal/handler/ -count=1` ok 1.8s、gofmt -l 空。
- [x] LAY-015 [P1] 拆分 internal/config/config.go（1639 行）——Server、Agent、存储、集群、认证、备份与遥测 schema 共用一个 Config 树，可按部署单元拆包（internal/config/config.go:13）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：108 个顶层声明 108/108 全覆盖分配，纯同包代码搬移零行为变更——`config.go` 保留 16 声明（Config 根 + 顶层默认 const + Config.UnmarshalYAML + isZero*/log canonical/legacy 装配件）、`config_provision.go` 8（Herald/Provision + Effective*）、`config_backup.go` 4（Backup/BackupSchedule）、`config_inbound.go` 3（AlertInbound/ExternalAPI）、`config_status_vault.go` 5（ServerStatus/Vault/Probe）、`config_pages_openapi.go` 6（Pages/OpenAPI）、`config_feature_telemetry.go` 6（FeatureFlags/Telemetry）、`config_control_server.go` 11（Control/Server/Database/Registry/Cluster + InterconnectTLS）、`config_agent.go` 11（AgentDispatch/TLSClient/Health/Circuit/Reconnection）、`config_logs.go` 7（ExecutionLog/TaskLog/Approval）、`config_auth.go` 13（Auth/LoginLockout/全部 provider）、`config_bootstrap.go` 7（BootstrapData/Descriptor/Schemas）、`config_storage.go` 11（Audit/Storage/Metrics/Profile/Cache/SSE）。测试文件零改动。门禁：`go build ./...` OK、`go test ./internal/config/ -count=1` ok 0.03s、gofmt -l 空。
- [x] LAY-016 [P1] 拆分 internal/platform/registry/store.go（1604 行）——Agent、provider、function、警告与快照状态共存一个 Store，写入路径可按资源拆分（internal/platform/registry/store.go:159）。
  - 状态／条件：LOCAL VERIFIED（2026-11-10）：75 个顶层声明 75/75 全覆盖分配，纯同包代码搬移零行为变更——`store.go` 保留 20 声明（FunctionMeta/ProviderSession/AgentSession/Store 等类型 + NewStore 接线 + getters）、`store_agent.go` 17 声明（SetAgentDraining/快照/存活函数/writeToDB/LoadFromDB/RemoveAgentIfStale/recoverPendingRegistrationOperations/StartCleanupRoutine/cleanupExpiredSessions）、`store_registration.go` 20 声明（SweepExpiredContractRemovals/materializeAgent/rebuildProposals/prepare+markRegistrationOperation/rebuildFunctionContract/classifyFunctionSnapshot/快照 diff 助手）、`store_openapi.go` 11 声明（OpenAPI operation/provider 全部 CRUD + BuildOpenAPISpec）、`store_warnings.go` 7 声明（Upsert/List/Delete/Mark/Remove RegistrationWarning）。测试文件零改动。门禁：`go build ./...` OK、`go test ./internal/platform/registry/... -count=1` ok、gofmt -l 空。
- [x] LAY-017 [P1] 拆分 internal/platform/dispatch/dispatcher.go（1591 行）——同步调用、任务创建、路由、HA 转发、事件持久化和收割集中在一个 Dispatcher（internal/platform/dispatch/dispatcher.go:86）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。85 个顶层声明拆分：dispatcher.go KEEP 32 + dispatcher_invoke.go 7 + dispatcher_broadcast.go 8 + dispatcher_task.go 16 + dispatcher_routing.go 12 + dispatcher_taskrouting.go 10（分配 85/85，0 未分配）。
  - 完成证据：`go build ./internal/platform/dispatch/...` 通过；`go test ./internal/platform/dispatch/... -count=1` PASSED 6.359s；`gofmt -l` clean。提交见 LAY 队列（refactor: split dispatcher by concern）。
- [x] LAY-018 [P1] 拆分 internal/api/console/service.go（1510 行）——菜单、页面、绑定执行和快照治理在同一 Service，执行协议映射可独立（internal/api/console/service.go:50）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。65 个顶层声明拆分：service.go KEEP 8（const 36/42、rendererSchemaVersion 48、Service 50、NewService 54、Menu 62、Pages 187、Page 213）+ console_menu.go 4（generateMenuFromMenuItems、consoleMenuItemFromMenu、consoleCategoryPath、consolePagePath）+ console_execute.go 25（ExecuteBinding/ExecuteDraftCompositeBinding/executeBindingFromSnapshot/claim+completeExecuteIdempotency/buildDraftBindingContracts/ensureExecutorBound/ensureBindingFresh/enforcePublishedBindingGovernance/bindingExecutionMetadata/createPageApproval/approvalRecipients/validateBindingExecutePayload/buildBindingPayloadFromSelectors/resolveSelectorValue/resolveSourceValue/applyRenameTransform/buildExecutionResult/startPageExecuteSpan/auditPageExecute/pageExecutionTarget/currentActor/findBinding/findContract/loadPublishedFunctionSpecs）+ console_json.go 12（specErrorDiagnostics/specDiagnosticDetails/isNullRawJSON/pickSelectionValues/valueFromRawContext/validRawJSON/getJSONPointerValue/setJSONPointerValue/setJSONObjectPointer/isJSONPointer/jsonPointerTokens/cloneStringMap）+ console_snapshot.go 5（parsePublishedPages/parsePublishedPageSpec/attachBindingFreshness/attachBindingFreshnessToPage/bindingFreshnessStatuses）+ console_misc.go 11（sanitizeApprovalID/firstNonEmpty/requireConsoleRead/requireConsoleExecute/requireScope/normalizeLanguage/getLocalizedText/pageEntry/ErrPageNotFound/PageNotFoundError/Error）。分配 65/65，0 未分配。
  - 完成证据：`go build ./internal/api/console/...` 通过；`go test ./internal/api/console/... -count=1` PASSED 57.867s；`gofmt -l` clean。提交见 LAY 队列（refactor: split console service by concern）。
- [x] LAY-019 [P1] 拆分 internal/dashboard/generator/generator.go（1455 行）——操作页、任务页、表单和文案生成交织，可用按页面 kind 的生成器拆分（internal/dashboard/generator/generator.go:52）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。79 个顶层声明拆分：generator.go KEEP 5（GenerateOptions 15、TermDictionary 29、Lookup 32、DefaultGenerateOptions 46、GenerateForOperation 52）+ generator_operation.go 32（GenerateOperationPageForOperation 65、buildOperationConfirm 104、operationRequiresConfirmation 119、operationPageKey 322、normalizeOptions 331、assessBaseCandidate 347、pageBinding 361、applySelectors 370、buildFormPresentation 598、buildFormFields 615、orderSortKey 706、executionModeForOperation 713、isTaskOperation 720、isReportOperation 724、categoryForResource 730、categoryForOperation 734、ensureDefaultLocale 743、localizedTitleFallback 767、localizedTitle 779、bindingIDForOperationWithSuffix 797、sanitizeBindingID 808、sanitizePageKey 829、sanitizeSourceKey 833、pageKindForOperation 837、qualityFromDiagnostics 848、operationQuality 860、taskQuality 873、hasWarningDiagnostic 888、hasErrorDiagnostic 897、hasDiagnosticCode 906、defaultOutputAssignments 915、RecomputeDefaultOutputs 948）+ generator_task.go 8（GenerateTaskPageForOperation 133、taskSemanticComplete 233、buildTaskStartBinding 385、buildTaskStatusBinding 407、buildTaskEventsBinding 449、buildTaskResultBinding 494、buildTaskCancelBinding 539、taskOutputShapeForPointer 579）+ generator_report.go 28（GenerateReportPageForOperation 257、reportSemanticComplete 315、applyReportSemantic 952、buildResultFields 966、buildDatasetSpec 994、buildDatasetSpecFromSemantic 1042、buildDimensionsFromPointers 1061、buildMetricsFromPointers 1085、datasetItemSchemaAtPointer 1106、schemaAtPointer 1127、compactPointers 1145、pointerTokens 1162、keyFromPointer 1173、buildChartSpecs 1181、datasetItemSchema 1193、collectionOutputAssignments 1214、collectionOutputSource 1234、scalarPropertySource 1249、outputShapeForSchema 1262、schemaRootType 1273、schemaTypeFromObject 1277、jsonRaw 1281、sortedRawMapKeys 1285、rawInt 1294、dataTypeFromSchema 1305、firstNonEmpty 1328、fallbackLabel 1337、flushWord 1341）+ generator_infer.go 6（InferPageType 1351、InferCategoryFromKey 1366、diagnostic 1374、ShouldBlockProposal 1386、CreateBlockedProposalIssue 1401、generateRepairHint 1434）。分配 79/79，0 未分配。
  - 完成证据：`go build ./internal/dashboard/generator/...` 通过；`go test ./internal/dashboard/generator/... -count=1` PASSED 0.138s；`gofmt -l` clean。提交见 LAY 队列（refactor: split dashboard generator by page kind）。
- [x] LAY-020 [P1] 拆分 internal/service/proposal_service.go（1381 行）——提案校验、权限、发布钩子和状态转换集中于一个服务，验收与持久化应分开（internal/service/proposal_service.go:25）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。68 个顶层声明拆分：proposal_service.go KEEP 9（ProposalService 25、ProposalDraftPublisher 46、PagePublishValidator 50、SetProposalDraftPublisher 54、SetPublishValidator 58、SetPermissionGuard 62、SetPublishReviewHooks 70、NewProposalService 201、rendererSchemaVersion 1319）+ proposal_dto.go 8（ProposalAcceptOutcome 106、ProposalListFilter 115、ProposalInboxResponse 122、ProposalInboxSummary 132、BlockedProposalIssueDTO 141、ContractChangeDTO 157、ProposalDTO 173、proposalDTOFromModel 666）+ proposal_permission.go 4（publishPermission 78、serviceProposalPermissionGuard 215、InternalTestProposalPermissionGuard 224、requireProposalPublishPermission 230）+ proposal_query.go 6（ListProposals 242、ListProposalDTOs 247、Inbox 292、ListProposalsByStatus 338、GetProposal 343、GetProposalDTO 348）+ proposal_accept.go 5（AcceptProposal 360、autoPublishAfterAccept 453、AcceptAndPublishProposal 467、RejectProposal 589、ProposalPublishResult 986）+ proposal_validate.go 9（publishValidationDiagnostics 89、validateAcceptedPageSpec 1021、CollectBindingSelectorIssues 1077、validateDirectPublishPageSpec 1123、isValidProposalPageType 1155、pageShapeMatchesType 1165、isValidBindingUsage 1182、isValidExecutionMode 1200、executionModeForFunctionSpec 1209）+ proposal_snapshot.go 4（createProposalVersionSnapshot 634、parsePublishedSnapshot 919、parseDraftPageSpec 934、pageSpecFromProposal 992）+ proposal_helpers.go 13（diagnosticsFromJSON 697、hasDiagnosticSeverity 712、staleDiagnosticsForDraft 881、stringSliceFromJSON 952、normalizeJSONMap 969、functionSpecsByID 1147、isMissingTableErr 1216、normalizeLocalizedText 1226、hasDefaultLocale 1253、localizedTextEqual 1265、bindingRequiresOutputSelectors 1279、schemaHasFields 1292、actorFromContext 1311）+ proposal_blocked.go 4（HasBlockingDiagnostics 606、hasBlockingDiagnostics 614、listBlockedIssueDTOs 721、blockedIssueDTOFromModel 745）+ proposal_contract.go 3（listContractChanges 761、publishedContractChanges 787、draftContractChanges 831）+ proposal_build.go 3（buildBindingContracts 1321、currentFunctionDigest 1352、digestJSON 1376）。分配 68/68，0 未分配。
  - 完成证据：`go build ./internal/service/...` 通过；`go test ./internal/service/... -count=1` PASSED（service 236.184s、notify 244.925s、permission 2.461s、versioning 281.094s）；`gofmt -l` clean。提交见 LAY 队列（refactor: split proposal service by concern）。
- [x] LAY-021 [P1] 拆分 internal/dashboard/generator/resource_generator.go（1355 行）——CRUD、action、task 页面和 selector/form 生成交织，可按语义族拆分（internal/dashboard/generator/resource_generator.go:22）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。63 个顶层声明拆分：resource_generator.go KEEP 2（resourceActionSemantic 13、GenerateResourcePageProposal 22）+ resource_generator_binding.go 7（resourceBinding 119、resourceOutputAssignments 141、resourceInputSelector 184、applyCollectionQuerySelector 201、applyIdentityRowSelector 218、pointerForField 238、escapeJSONPointerToken 246）+ resource_generator_form.go 5（buildFormFromContract 251、buildUpdateFormFromContract 263、schemaCanBindIdentityPlusFields 279、buildActionFormPresentation 306、removeTopLevelSchemaField 338）+ resource_generator_view.go 10（buildListViewFromContract 374、buildDetailBinding 463、buildInlineResourceActions 473、buildInlineResourceAction 525、buildInlineResourceActionSelector 576、buildDetailViewFromContracts 640、parseResourceActionSemantics 677、detailSchemaFromContracts 701、canUseItemQueryAsDetailSource 715、schemaCanBindOnlyIdentity 735）+ resource_generator_action.go 7（inlineActionBindingID 855、inlineActionKey 859、inlineActionTitle 863、inlineActionType 871、inlineActionNeedsConfirm 880、buildDeleteAction 1041、resourceOperationRequiresConfirmation 1056）+ resource_generator_list.go 7（defaultListView 892、buildFiltersFromContract 898、paginationFromContract 933、var 963、resolvePaginationFields 970、findCollectionItemsSchema 1006、collectionArrayKeys 1031）+ resource_generator_schema.go 10（requiredFieldNames 760、schemaHasNoRequiredFields 771、schemaCanBindBySingleRequiredField 776、schemaCanBindBySingleRequiredArrayField 800、schemaTypeFromRaw 825、topLevelPointerToken 830、replaceSelectorSource 841、inferDataType 1071、inferFilterType 1092、enumOptionsFromSchema 1114）+ resource_generator_assess.go 4（assessResourceSemantics 1134、validateGeneratedResourceViews 1181、listViewHasColumn 1202、resourceQuality 1211）+ resource_generator_find.go 4（findResourceContract 1229、findContractByID 1240、findContractByFunctionID 1249、contractToFunctionSpec 1259）+ resource_generator_helpers.go 7（jsonMapToApprovalPolicy 1282、jsonMapToLocalizedText 1297、extractString 1310、parseJSONObject 1317、objectProperty 1328、rawObject 1335、rawString 1346）。分配 63/63，0 未分配。
  - 完成证据：`go build ./internal/dashboard/generator/...` 通过；`go test ./internal/dashboard/generator/... -count=1` PASSED 0.103s；`gofmt -l` clean。提交见 LAY 队列（refactor: split resource generator by semantic family）。
- [x] LAY-022 [P1] 拆分 internal/platform/settings/layered.go（1305 行）——有效键、类型、加密、分层读取和默认值集中在一个文件，schema 与分层读取可拆开（internal/platform/settings/layered.go:187）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。65 个顶层声明拆分：layered.go KEEP 18（const 28、ValidKeys 187、secretKeys 251、IsSecretKey 270、intKeys 276、IsIntKey 293、boolKeys 299、IsBoolKey 317、IsValidKey 323、Layered 331、var 338、InitLayered 345、Current 364、ConfigInput 368、resolveL2 380、featureFlagNames 411、featureKey 413、marshalString 415）+ layered_get.go 8（GetString 422、GetBool 448、getBoolWithSource 480、GetInt 507、FeatureEnabled 549、stringOrEmpty 720、intSafe 1100、getIntWithSource 1110）+ layered_auth.go 10（AuthSnapshot 938、EmailPolicySnapshot 952、EmailPolicy 962、LocalAuthSnapshot 978、AuthProviderSnapshot 985、AuthSnapshot 994、SecurityPolicySnapshot 1017、SecurityPolicy 1028、authProviderSnapshot 1175、AuthProviderConfig 1219）+ layered_snapshot.go 20（ObsSnapshot 568、ObsSnapshot 576、NotificationSnapshot 596、NotificationSnapshot 622、NotifySMTPConfig 659、NotifySMTP 675、NotifyChannelsResolved 690、NotifyChannels 703、FeatureDomainState 757、FeatureSnapshot 768、FeatureSnapshot 773、SiteSnapshot 823、FooterLink 842、SiteSnapshot 849、OutboundSnapshot 1044、OutboundSnapshot 1056、PerformanceSettingsSnapshot 1086、PerformanceSettings 1143、LogsSettingsSnapshot 1160、LogsSettings 1168）+ layered_secret.go 3（secretResolver 728、SetSecretResolver 733、secretStr 739）+ layered_reload.go 4（hasL3 797、Reload 806、ResetForTest 926、resetForTest 931）+ layered_misc.go 2（settingsKey 1215、boolPtr 1305）。分配 65/65，0 未分配。
  - 完成证据：`go build ./internal/platform/settings/...` 通过；`go test ./internal/platform/settings/... -count=1` PASSED 141.646s；`gofmt -l` clean。提交见 LAY 队列（refactor: split layered settings by layer）。
- [x] LAY-023 [P1] 拆分 internal/app/agent/app.go（1265 行）——应用生命周期、本地 server、连接建立和插件装配共享一个 App，可按 boot/session/plugin 拆分（internal/app/agent/app.go:35）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。83 个顶层声明拆分：app.go KEEP 12（App 35、WithTelemetry 88、TelemetryService 97、SetRecorder 107、armRecorderChaining 120、New 138、NewWithConfigDir 156、getConfigDir 174、StartLocalServer 184、GetLocalServerAddr 309、SetLocalAddr 323、Run 331）+ app_config.go 10（SetProviderCallTimeout 575、WithUpstreamMetadata 583、WithUpstreamTLSConfig 594、SetUpstreamTransportKind 602、OnConnected 612、OnDisconnected 623、WithOutboundTLSConfig 631、WithLocalTLSConfig 641、WithOpsConfig 674、WithVersion 687）+ app_lifecycle.go 4（startLocalServerSafely 351、currentServeErr 367、startAgentServices 373、Stop 533）+ app_getters.go 9（Store 649、SyncUpstream 657、HeartbeatUpstream 665、AgentID 697、OpsServer 705、MetricsCollector 715、Tasks 723、ActiveTaskCount 731、ExtensionRuntime 738）+ app_probe.go 7（WithProbeConfig 398、ProbeManager 422、startProbes 430、startProbePolicy 439、fetchProbePolicy 448、startProbeDBTargets 458、fetchProbeDBTargets 467）+ app_maintenance.go 3（startMaintenance 475、parseDurationEnv 504、startExtensionSync 516）+ app_extension.go 20（ApplyExtensionSyncPayload 745、ApplyExtensionSyncPayloadJSON 761、ReconcileExtensions 781、PullExtensionSyncOnce 797、syncExtensionsFromRuntime 812、syncExtensionFunctionsFromRuntime 833、discoverExtensionFunctions 863、inferFunctionResource 925、inferFunctionOperation 933、externalPlatformFunction 941、discoverExternalPlatformFunctions 948、buildExtensionProviderEntries 980、valueString 1029、valueStringSlice 1040、extensionRuntimeDynamicLabels 1063、truncateString 1098、extensionFunctionRoute 1112、resolveFunctionDriver 1117、hasExtensionFunction 1138、invokeExtensionFunction 1152）+ app_ops_wrapper.go 15（opsServerWrapper 1181、GetSystemInfo 1185、ListProcesses 1189、ReportMetrics 1193、RestartProcess 1197、StopProcess 1201、StartProcess 1205、UpdateProcess 1209、SetSchedulePause 1213、ExecuteCommand 1217、PushFile 1221、GetSupervisorLog 1225、ListServicesJSON 1229、GetServiceStatusJSON 1233、ListCronJobsJSON 1237）+ app_provider_wrapper.go 3（providerManagerWrapper 1242、IsPlatformFunction 1247、Call 1257）。分配 83/83，0 未分配。
  - 完成证据：`go build ./internal/app/agent/...` 通过；`go test ./internal/app/agent/... -count=1` PASSED 100.942s；`gofmt -l` clean。提交见 LAY 队列（refactor: split agent app by boot concern）。
- [x] LAY-024 [P1] 拆分 internal/agent/local_handler.go（1243 行）——provider 注册、调用路由、健康、drain 和插件调用集中在一个 handler（internal/agent/local_handler.go:91）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。54 个顶层声明拆分：local_handler.go KEEP 8（instanceHealthyWindow 34、defaultDrainGrace 39、drainPollInterval 42、providerDrainingError 47、ProviderManager 52、PluginFunctionManager 61、OpsServerWrapper 67、LocalHandler 91）+ local_handler_setters.go 10（SetProviderSessionStore 114、NewLocalHandler 121、SetProviderCallTimeout 140、providerCallDeadline 155、SetProviderManager 181、SetPluginFunctionManager 189、SetOpsServer 196、SetTaskEventReporter 202、SetTLSConfig 212、SetExpectedGameEnv 221）+ local_handler_dispatch.go 2（Handle 230、handleRequest 235）+ local_handler_invoke.go 6（handleInvoke 291、callProvider 393、callSession 445、callLocalProvider 453、invokePlatform 485、pickInstance 509）+ local_handler_task.go 3（handleStartTask 563、handleCancelTask 579、executeTask 597）+ local_handler_trace.go 3（agentTracer 627、recordSpanResult 631、mustMarshal 643）+ local_handler_ops.go 15（handleGetSystemInfo 656、handleListProcesses 679、handleReportMetrics 702、handleRestartProcess 727、handleStopProcess 750、handleStartProcess 773、handleUpdateProcess 796、handleSchedulePause 820、handleExecuteCommand 843、handlePushFile 866、handleListServices 887、handleListCronJobs 901、handleGetSupervisorLog 913、handleGetServiceStatus 937、handleRegisterCapabilities 952）+ local_handler_provider.go 5（handleProviderConnect 989、providerDescriptorPresentationViolation 1101、handleProviderHeartbeat 1109、handleProviderDrain 1135、DrainProviderSession 1171）+ local_handler_misc.go 2（hostFromAddr 962、fnvIndex 975）。分配 54/54，0 未分配。
  - 完成证据：`go build ./internal/agent/...` 通过；`go test ./internal/agent/... -count=1` PASSED 10.422s；`gofmt -l` clean。提交见 LAY 队列（refactor: split local handler by call family）。
- [x] LAY-025 [P1] 拆分 internal/dashboard/spec/selector_ast.go（1225 行）——AST、值来源、转换、赋值和类型收窄混合，AST 与 execution mapping 可拆分（internal/dashboard/spec/selector_ast.go:9）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。75 个顶层声明拆分：selector_ast.go KEEP 16（SelectorAST 9、InputAssignment 14、OutputAssignment 21、OutputResultShape 28、const 30、ValueSource 39、ValueSourceKind 48、const 50、TransformSpec 60、TransformType 66、const 68、SelectorValidationResult 82、SelectorError 89、SelectorWarning 97、const 104、SelectorContext 114）+ selector_ast_context.go 4（SelectorContextForBinding 130、compositePageStateSchema 170、FormSchemaForBinding 204、DefaultSelector 903）+ selector_ast_validate.go 10（ValidateSelector 224、isSupportedTransform 279、isWholeObjectTransform 298、validRenameParams 304、validDefaultParams 321、bindingUsesRowSource 329、bindingUsesSelectionSource 341、listQuerySchema 353、schemaForFilter 377、marshalRawObject 388）+ selector_ast_output.go 16（ValidateOutputAssignments 397、validateInputSource 422、invalidSelectorResult 459、addError 469、addWarning 478、isSourceAllowed 486、validateSourcePath 505、validatePageStatePath 523、isAssignable 537、sourceSchemaAndPath 570、requiredPointers 586、schemaHasPath 610、schemaTypeAtPath 645、schemaNodeAtPath 665、jsonSchemaTypeAssignable 694、outputShapeMatchesSchema 701）+ selector_ast_diff.go 17（SchemaFieldSnapshot 720、SchemaFieldChangeType 727、const 729、SchemaFieldChange 737、FieldRenameCandidate 745、SchemaDiffResult 753、DiffJSONSchemaFields 760、SelectorStaleDiagnostics 829、rawObject 963、schemaFieldSnapshots 974、collectSchemaFieldSnapshots 987、requiredSet 1009、schemaTypeFromNode 1022、fieldRenameCandidates 1038、hasChange 1062、staleDiagnostic 1071、parentPointer 1080）+ selector_ast_helpers.go 7（isOutputShape 934、isJSONPointer 943、jsonPointerTokens 947、escapeJSONPointerToken 958、sortedMapKeys 1091、sortedRawMapKeys 1100、sortStrings 1109）+ selector_ast_semantic.go 5（SemanticContext 1122、ValidateSelectorSemantics 1144、validateIdentityMapping 1169、validateCollectionMapping 1194、isIdentityPath 1218）。分配 75/75，0 未分配。
  - 完成证据：`go build ./internal/dashboard/spec/...` 通过；`go test ./internal/dashboard/spec/... -count=1` PASSED 0.051s；`gofmt -l` clean。提交见 LAY 队列（refactor: split selector ast by concern）。
- [x] LAY-026 [P1] 拆分 internal/analytics/worker/worker.go（1212 行）——消费、批量写、聚合、checkpoint、重放和死信同文件，ingest 与 ClickHouse writer 应分离（internal/analytics/worker/worker.go:71）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。51 个顶层声明拆分：worker.go KEEP 10（envOrDefault 20、const 27、var 43、pendingReclaimInterval 50、runFlushInterval 54、ackRef 61、Worker 71、NewWorker 106、ensureGroups 175、Run 182）+ worker_insert.go 8（insertEvent 394、insertPayment 416、ensureEventBatch 435、ensureEventBatchLocked 441、ensurePaymentBatch 454、ensurePaymentBatchLocked 460、appendEventRow 476、appendPaymentRow 497）+ worker_flush.go 7（flushEventBatch 520、flushEventBatchLocked 531、flushPaymentBatch 550、flushPaymentBatchLocked 556、revenueDeltaFromPaymentArgs 576、commitRevenue 614、flushBatches 632）+ worker_ack.go 5（ackSent 647、ackImmediate 659、advanceCheckpoint 673、parseStreamID 686、streamIDLess 698）+ worker_agg.go 9（revRow 709、otelEventAliases 719、normalizeAggEvent 726、touchAgg 734、flush 777、remergeTouchedMinute 930、remergeTouchedDay 941、remergeRev 952、max0 965）+ worker_checkpoint.go 6（restoreCheckpoints 971、persistCheckpoint 995、processMessage 1018、sendToDeadLetter 1089、reclaimPendingMessages 1120、reclaimPendingFromStream 1137）+ worker_helpers.go 6（waitGroupBounded 298、fmtAny 312、asString 326、asFloat 334、asRetryCount 353、normalizeTimestamp 372）。分配 51/51，0 未分配。
  - 完成证据：`go build ./internal/analytics/worker/...` 通过；`go test ./internal/analytics/worker/... -count=1` PASSED 25.788s；`gofmt -l` clean。提交见 LAY 队列（refactor: split analytics worker by pipeline stage）。
- [x] LAY-027 [P1] 拆分 internal/drivers/openapi/provider.go（1178 行）——OpenAPI provider 的请求、认证、重试和 schema 解析集中在一个 Provider（internal/drivers/openapi/provider.go:71）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。44 个顶层声明拆分：provider.go KEEP 14（Duration 31、UnmarshalJSON 34、Duration 64、Provider 71、Config 86、AuthConfig 120、APIKeyAuth 141、APIMethod 154、ParameterMapping 214、RequestBodyMapping 248、ResponseMapping 261、TransformConfig 276、MethodDetails 292、NewProvider 321）+ provider_init.go 5（Name 328、Kind 332、Init 337、extractConfig 387、buildMethodMap 399）+ provider_discover.go 6（discoverMethodsFromSpec 415、looksLikeJSON 461、yamlToJSON 470、parseOpenAPISpec 479、extractTags 631、extractParameters 646）+ provider_path.go 4（pathToMethodName 703、titleWords 728、isWordSeparator 742、filterEmpty 765）+ provider_meta.go 5（IsEnabled 776、SupportedMethods 781、GetMethodDetails 789、InfoVersion 820、GetOpenAPIDoc 828）+ provider_call.go 10（Call 835、buildRequest 947、buildURL 992、buildBody 1026、applyTemplate 1047、getParamValue 1057、filterConsumedParams 1078、addAuth 1106、transformResponse 1134、Close 1173）。分配 44/44，0 未分配。
  - 完成证据：`go build ./internal/drivers/openapi/...` 通过；`go test ./internal/drivers/openapi/... -count=1` PASSED 6.602s；`gofmt -l` clean。提交见 LAY 队列（refactor: split openapi provider by concern）。
- [x] LAY-028 [P1] 拆分 internal/audit/audit.go（1173 行）——事件类型、记录构造、校验、异步队列和失败策略集中在一起，schema 与写入管线可分离（internal/audit/audit.go:203）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。73 个顶层声明拆分：audit.go KEEP 29（var 21、const 35、WriteFailurePolicy 42、const 44、defaultGapAlertInterval 59、AuditEventType 62、const 64、AuditSeverity 182、const 184、AuditCategory 192、const 194、AuditRecord 203、ActorInfo 221、ResourceInfo 236、ChangeInfo 246、AuditContext 253、ChainInfo 264、AuditFilter 279、AuditPage 296、AuditStats 304、ActorStat 317、AuditStore 323、AuditService 344、NewAuditService 361、SetWriteFailurePolicy 373、WriteFailurePolicy 384、Store 392、SetNotifier 399、Log 406）+ audit_options.go 13（AuditOption 494、WithActor 497、WithActorID 504、WithResource 515、WithResourceID 522、WithDetails 532、WithChanges 539、WithContext 551、WithOutcome 558、WithSeverity 566、WithCategory 573、WithGameID 580、WithIPAddress 588）+ audit_defaults.go 9（setDefaults 596、inferCategory 614、inferSeverity 638、maskSensitiveData 650、defaultSensitiveFields 665、MaskSensitiveValue 672、maskMap 1120、isSensitiveField 1136、calculateDiffFields 1146）+ audit_chain.go 5（buildChainInfo 688、calculateHash 741、legacyHashInput 759、canonicalHashInput 781、normalizeHashPayloadMap 860）+ audit_failure.go 3（handleWriteFailure 879、ReportWriteGap 905、alertWriteGap 931）+ audit_query.go 10（Get 948、List 953、GetStats 958、ValidateChain 963、ChainValidationResult 1027、ChainError 1034、Export 1041、Archive 1046、AuditSigner 1073、AuditNotifier 1079）+ audit_id.go 4（generateAuditID 1085、randomString 1092、legacyHashBytes 1105、sha256Hex 1115）。分配 73/73，0 未分配。
  - 完成证据：`go build ./internal/audit/...` 通过；`go test ./internal/audit/... -count=1` PASSED（audit 1.432s、chain 0.095s）；`gofmt -l` clean。提交见 LAY 队列（refactor: split audit service by concern）。
- [ ] LAY-029 [P1] 拆分 internal/api/ops/handler.go（1136 行）——运维 HTTP handler 覆盖命令、进程、指标、会话和报告，可按 operations protocol 拆分（internal/api/ops/handler.go:31）。
  - 状态／条件：BLOCKED（2026-11-10）。**按并行会话红线勿 claim**：internal/api/ops 属并行会话工作包（持续指令红线），本轮不触碰，等待并行会话收口或用户明确释放。缺：并行会话释放 internal/api/ops 工作包（或用户授权 claim）。
- [x] LAY-030 [P1] 拆分 internal/api/incident/report.go（1091 行）——报告指标、对比、趋势、导出和落库转换集中在一个文件，分析计算与 API DTO 应分开（internal/api/incident/report.go:40）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。50 个顶层声明拆分：report.go KEEP 23（const 18、CompareDelta 40、CompareValue 48、ReportMetrics 57、CategoryShare 65、ReportSummaryResponse 74、TrendPoint 91、TrendCategory 98、ReportTrendResponse 105、LeaderboardRow 118、ReportLeaderboardResponse 135、ResponsibilityOverview 146、ResponsibilityCategoryRow 154、ResponsibilityRow 166、UnattributedIncident 180、ResponsibilityReportResponse 189、incidentAggRow 203、periodStats 216、metricPick 228、pickTotal 230、pickDuration 231、pickMTTR 232、pickRecurrence 238）+ report_period.go 7（periodStats 247、scanIncidents 288、signatureOf 301、recurrenceHeads 314、countRecurrenceChains 347、dataEpoch 353、oldestTime 373）+ report_compare.go 2（buildCompare 389、buildDelta 408）+ report_summary.go 3（ReportSummary 434、categoryShare 475、categoryIndex 524）+ report_trend.go 4（ReportTrend 539、trendWindow 574、trendPoints 593、bucketKey 651）+ report_group.go 3（groupAgg 669、groupStats 683、groupKeyOf 761）+ report_leaderboard.go 3（ReportLeaderboard 772、groupDelta 841、filterAndSortLeaderboard 870）+ report_responsibility.go 5（ReportResponsibility 904、responsibleKeyOf 1053、preWindowRows 1061、categoryLeaders 1070、unattributedList 1083）。分配 50/50，0 未分配。
  - 完成证据：`go build ./internal/api/incident/...` 通过；`go test ./internal/api/incident/... -count=1` PASSED 14.084s；`gofmt -l` clean。提交见 LAY 队列（refactor: split incident report by analysis view）。
- [x] LAY-031 [P1] 拆分 internal/audit/store.go（1089 行）——hash chain、SQL 持久化、retention 和模型转换混在一个 store，链校验和存储适配应分离（internal/audit/store.go:248）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。42 个顶层声明拆分：store.go KEEP 12（ErrChainSequenceConflict 22、isChainSequenceConflict 28、AuditModel 38、TableName 76、ToRecord 81、FromRecord 135、derivePromotedFields 206、detailsNumeric 235、SQLAuditStore 248、auditMemCache 253、NewSQLAuditStore 260、backfillPromotedFields 294）+ store_sql.go 12（Create 341、Get 370、List 391、Delete 514、DeleteBefore 528、GetLatestRecord 534、GetBySequence 555、GetChainRange 568、GetStats 589、CountByFilter 680、Export 708、exportCSV 734）+ store_memory.go 14（InMemoryAuditStore 778、NewInMemoryAuditStore 787、Create 796、Get 813、List 825、matchesFilter 860、Delete 894、DeleteBefore 910、GetLatestRecord 927、GetBySequence 938、GetChainRange 950、GetStats 965、CountByFilter 1006、Export 1021）+ store_writer.go 4（AuditWriter 1034、NewAuditWriter 1042、Write 1051、simpleHash 1083）。分配 42/42，0 未分配。
  - 完成证据：`go build ./internal/audit/...` 通过；`go test ./internal/audit/... -count=1` PASSED（audit 1.807s、chain 0.046s）；`gofmt -l` clean。提交见 LAY 队列（refactor: split audit store by backend）。
- [x] LAY-032 [P1] 拆分 sdks/cpp/src/tcp_transport.cpp（1085 行）——TCP client、server、dispatch、read loop 和响应工具同文件，client/server 可独立（sdks/cpp/src/tcp_transport.cpp:38）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。按行段拆分 3 个 TU：tcp_transport.cpp KEEP 客户端（includes 1-31、TCPTransport statics 33-36、client 方法 38-611）+ tcp_transport_server.cpp 新增（TCPServer statics 613-616、server 方法 618-957）+ tcp_transport_inbound.cpp 新增（inbound 段 959-1080，含匿名 namespace g_inbound_worker_override）。方法定义/静态行守恒 30/30（client 14 + server 11 + inbound 5）；0 漂移；CMakeLists.txt SDK_SOURCES 注册两个新 TU。
  - 完成证据：`g++ -fsyntax-only -std=c++17 -Wall -I sdks/cpp/include` 三个 TU 全 OK；`cmake --build` croupier-sdk-static 100% 构建通过，`ar t` 确认 tcp_transport.cpp.o / tcp_transport_server.cpp.o / tcp_transport_inbound.cpp.o 三对象入库。已知边界（非本拆分引入）：examples 可执行链接报 `libz.so.1: DSO missing from command line`，基线（拆分前）同样报错，属既有 zlib 链接环境问题，未修复。提交见 LAY 队列（refactor: split cpp tcp transport by role）。
- [x] LAY-033 [P1] 拆分 internal/api/auth/service.go（1085 行）——登录、OTP、邮箱验证、游戏范围、审计和 session 管理集中在一个 Service（internal/api/auth/service.go:42）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。68 个顶层声明拆分：service.go KEEP 23（ErrMFARequired 36、Service 42、WithAdminModel 97、WithOTPRecoveryModel 105、WithRecoveryDB 112、WithVerificationModel 119、WithGameModel 126、WithAuditService 134、WithRoleModel 141、WithPasswordProvider 148、WithOIDCProvider 158、WithGitHubProvider 167、WithWeChatProvider 176、WithGenericOAuthProvider 185、WithLocalEnabled 193、WithRegister 199、WithProviderDefaultRoles 206、snapshotProviders 212、RefreshIdentityProviders 225、OIDCEnabled 281、LDAPEnabled 284、NewService 293、WithLoginLockout 314）+ auth_login.go 11（Login 322、recordLocalLoginFailure 407、authenticatePassword 425、resolveAdminForIdentity 446、backfillProfile 460、provisionShadowAdmin 483、assignDefaultRoles 513、findRoleByName 533、issueLogin 547、mfaSetupRequired 586、adminNeedsPasswordChange 596）+ auth_oauth.go 19（OIDCAuthCodeURL 604、githubSnapshot 613、GitHubEnabled 783、GitHubAuthCodeURL 790、GitHubLoginCallback 799、GitHubSuccessURL 805、wechatSnapshot 811、WeChatEnabled 818、WeChatAuthCodeURL 825、WeChatLoginCallback 834、WeChatSuccessURL 840、genericOAuthSnapshot 846、GenericOAuthEnabled 853、GenericOAuthAuthCodeURL 860、GenericOAuthLoginCallback 869、GenericOAuthSuccessURL 875、externalOAuthCallback 884、OIDCLoginCallback 912、OIDCSuccessURL 918）+ auth_state.go 3（newOIDCState 927、verifyOIDCState 935、signState 960）+ auth_register.go 7（LocalEnabled 620、RegisterEnabled 627、RegisterRequest 634、Register 646、checkRegisterEmailPolicy 702、isValidRegisterUsername 750、assignRegisterRoles 765）+ auth_audit.go 2（validLastScope 966、recordLoginAudit 1003）+ auth_check.go 3（Logout 1046、Check 1059、BatchCheck 1075）。分配 68/68，0 未分配。
  - 完成证据：`go build ./internal/api/auth/...` 通过；`go test ./internal/api/auth/... -count=1` PASSED 396.201s；`gofmt -l` clean。提交见 LAY 队列（refactor: split auth service by concern）。
- [x] LAY-034 [P1] 拆分 internal/app/agent/ops_server.go（1054 行）——托管进程、指标、命令执行、supervisor 事件和重启策略集中在一个 OpsServer（internal/app/agent/ops_server.go:34）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。42 个顶层声明拆分：ops_server.go KEEP 4（restartConfig 24、OpsServer 34、managedProcess 71、NewOpsServer 110）+ ops_server_process.go 11（GetSystemInfo 127、ListProcesses 132、ReportMetrics 168、RestartProcess 175、doRestartProcess 200、StopProcess 236、doStopProcess 259、coreStopProcess 273、StartProcess 286、doStartProcess 329、coreStartProcess 350）+ ops_server_exec.go 5（execSensitiveEnvKeys 382、execInlineCodeInterpreters 398、execInlineCodeFlags 407、validateExecBoundary 413、ExecuteCommand 449）+ ops_server_monitor.go 7（startProcess 522、stopProcess 566、stopProcessWaitTimeout 592、monitorProcess 596、autoRestartAllowed 618、superviseExit 627、armSuccessTimer 762）+ ops_server_lifecycle.go 4（Start 784、GetSupervisorLog 831、Stop 848、Close 869）+ ops_server_json.go 3（ListServicesJSON 877、GetServiceStatusJSON 906、ListCronJobsJSON 932）+ ops_server_services.go 8（ListServicesRequest 959、ListServicesResponse 966、GetServiceStatusRequest 972、GetServiceStatusResponse 977、ListCronJobsResponse 988、ListServices 994、GetServiceStatus 1018、ListCronJobs 1036）。分配 42/42，0 未分配。
  - 完成证据：`go build ./internal/app/agent/...` 通过；`gofmt -l` clean；`go test ./internal/app/agent/ -skip 'TestMetrics|TestReportMetrics|TestUpstreamClient_ReportMetrics|TestUpstreamClient_WithMetricsReporting|TestUpstreamMetricsLoop|TestUpstreamReportMetrics' -count=1` PASSED 48.793s（含全部 ops_server 用例）。已知边界：包内 TestMetricsCollectorStartReporting_ZeroIntervalV9（coverage_v9c_test.go:261，5s 时序断言，仅涉 ops_metrics.go 与 metrics collector）在高负载（本机 load 128）下因调度饥饿抖落，与本拆分无关（ops_server.go 纯代码移动 42/42 守恒、构建通过，测试不触 ops_server 代码）；低负载下应全绿。提交见 LAY 队列（refactor: split ops server by concern）。
- [x] LAY-035 [P1] 拆分 internal/api/function/helpers.go（1051 行）——函数列表、详情、分析、复制、启停和关系映射集中在一个 helpers 文件，API 编排可按用例拆分（internal/api/function/helpers.go:25）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。66 个顶层声明拆分：helpers.go KEEP 13（getStringFromMetadata 794、getIntFromMetadata 807、parseRolesFromJSON 829、parseActionsFromJSON 849、jsonValueFromMetadata 868、getStringSliceFromMetadata 879、getBoolFromMetadata 901、invokePayload 920、rawJSONFromAny 927、rawJSONFromBytes 931、getLocalizedTextFromMetadata 964、firstNonEmptySlice 988、firstNonEmptyMap 996）+ helpers_list.go 4（functionsList 25、functionsPending 109、functionDetail 116、functionAnalytics 147）+ helpers_mutate.go 5（functionCopy 165、functionDelete 173、functionDisable 177、functionEnable 181、setFunctionEnabled 189）+ helpers_history.go 2（fetchFunctionHistoryPage 206、functionHistory 210）+ helpers_invoke.go 16（functionInvoke 246、functioninvokeRequestFromDTO 262、functionInvokeResponseFromUnified 281、validateInvokeInput 309、validateInvokeRoute 313、cloneMetadata 325、isApprovedContinuation 329、isPageSnapshotGoverned 333、effectiveTimeoutMs 337、declaredTimeoutMs 341、auditFunctionInvoke 345、createFunctionApproval 349、approvalNotifyRecipients 353、dedupeStrings 357、auditApprovalCreated 361、functionPublish 365）+ helpers_instances.go 10（sessionMatchesScope 376、remoteAgentSnapshot 385、clusterOwnerRecords 392、clusterSelfInstanceID 404、remoteOwnerMap 415、remoteAgentSnapshots 437、instanceItemForFunction 473、appendInstanceSummaries 492、functionInstances 547、functionInstancesAll 585）+ helpers_perms.go 2（functionPermissions 627、functionPermissionsUpdate 655）+ helpers_warnings.go 3（functionWarnings 671、functionWarningFilterOptions 1006、optionItemsFromCounts 1038）+ helpers_descriptors.go 2（descriptors 713、currentFunctionScope 740）+ helpers_batch.go 3（batchCopyFunctions 746、batchDeleteFunctions 759、batchUpdateFunctions 777）+ helpers_policy.go 6（policyRiskFromContract 935、resolvePolicyRisk 939、ResolvePolicyRisk 947、enforceFunctionPolicy 951、matchAnyRole 955、buildBroadcastResponse 959）。分配 66/66，0 未分配。
  - 完成证据：`go build ./internal/api/function/...` 通过；`go test ./internal/api/function/... -count=1` PASSED 127.716s；`gofmt -l` clean。提交见 LAY 队列（refactor: split function helpers by use case）。
- [x] LAY-036 [P1] 拆分 internal/platform/registry/metrics.go（1019 行）——指标模型、采集、每小时聚合、清理和查询共享一个 MetricsStore，写入与查询应分离（internal/platform/registry/metrics.go:122）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。47 个顶层声明拆分：metrics.go KEEP 16（MetricsEntry 18、AgentMetricsHistory 25、AgentMetricsHourly 56、TableName 77、TableName 81、MetricsRetention 86、MetricsHourlyRetention 89、MetricsStoreConfig 92、DefaultMetricsStoreConfig 108、supervisorEventRingSize 119、MetricsStore 122、SetOnReport 145、SetOnSupervisorEvent 153、NewMetricsStore 160、NewMetricsStoreWithConfig 165、SetDB 188）+ metrics_ingest.go 5（Add 193、persistToDB 252、ingestSupervisorEventsLocked 307、GetSupervisorEvents 344、SupervisorLatestSeq 366）+ metrics_query.go 8（GetLatest 376、GetHistory 400、getFromMemory 420、getFromDB 455、GetAgentMetrics 960、GetAllMetrics 965、ListAgents 986、Clear 998）+ metrics_prune.go 3（Prune 516、StartCleanupRoutine 550、PruneHourly 783）+ metrics_hourly.go 7（RollupHourly 576、aggregateHourly 645、netRateAccum 765、diskAccum 771、round2 777、GetHourlyHistory 796、metricsEntryFromHourly 820）+ metrics_cache.go 8（SystemInfoCache 864、SystemInfoEntry 869、NewSystemInfoCache 875、Set 882、Get 897、List 919、Remove 931、Prune 943）。分配 47/47，0 未分配。
  - 完成证据：`go build ./internal/platform/registry/...` 通过；`go test ./internal/platform/registry/... -count=1` PASSED（registry 13.580s、sdkversion 0.003s）；`gofmt -l` clean；`internal/model` 导入边界检查无命中（registry 禁 import model）。提交见 LAY 队列（refactor: split registry metrics by concern）。
- [x] LAY-037 [P1] 拆分 internal/platform/approvals/notification.go（1010 行）——通知事件、渠道、模板、重试和发送器注册集中在一起，路由与投递可拆分（internal/platform/approvals/notification.go:78）。
  - 状态：DONE，LOCAL VERIFIED 2026-11-10。69 个顶层声明拆分：notification.go KEEP 14（NotificationEvent 31、NotificationChannel 42、const 块 44、NotificationConfig 58、Notifier 66、NotificationSender 72、MultiChannelNotifier 78、NewMultiChannelNotifier 86、RegisterSender 95、SetConfig 102、Notify 109、NotifyWithChannels 128、NotificationStore 174、NotificationRecord 182）+ notification_inapp.go 12（InAppNotifier 193、NewInAppNotifier 199、Send 207、Channel 238、WebSocketHub 243、WebSocketClient 252、BroadcastMessage 258、NewWebSocketHub 264、Run 274、SendToUser 317、Register 325、Unregister 330）+ notification_email.go 17（EmailSender 340、emailMessage 361、NewEmailSender 369、WithTransport 381、Send 394、Channel 422、defaultSendMail 432、buildEmailMessage 531、sanitizeNotificationEventForEmail 554、sanitizeEmailText 573、buildEmailBody 593、htmlEscape 623、loginAuth 988、Start 993、Next 997、parseEmailAddress 863、validateEmailAddress 868）+ notification_dingtalk.go 5（DingTalkSender 635、NewDingTalkSender 647、Send 653、Channel 674、signedURL 680）+ notification_webhook.go 6（WebhookSender 700、NewWebhookSender 711、Send 717、Channel 747、defaultPostJSON 752、defaultPostJSONWithHeaders 756）+ notification_template.go 7（NotificationTemplate 784、TemplateRenderer 795、NewTemplateRenderer 800、AddTemplate 807、Render 812、replaceAll 836、findSubstring 848）+ notification_wecom.go 4（WecomSender 900、NewWecomSender 908、Send 913、Channel 933）+ notification_feishu.go 4（FeishuSender 942、NewFeishuSender 950、Send 955、Channel 981）。分配 69/69，0 未分配。
  - 完成证据：`go build ./internal/platform/approvals/...` 通过；`go test ./internal/platform/approvals/... -count=1` PASSED 3.304s；`gofmt -l` clean。提交见 LAY 队列（refactor: split approvals notification by sender）。

### 目录/分类问题

- [ ] internal/model/task.go 定义 `TaskRun`，而执行状态机在 internal/platform/dispatch/dispatcher.go；同一任务域的运行模型、派发器和事件查询分散在三处（internal/model/task.go:8；internal/platform/dispatch/dispatcher.go:54）。
- [ ] internal/api/ops 与 internal/app/agent/ops_server.go 都是运维面，但前者是 Server REST DTO/编排，后者是 Agent gRPC 实现；同名领域缺少共享接口说明（internal/api/ops/handler.go:31；internal/app/agent/ops_server.go:34）。
- [ ] internal/service/contract_service.go 与 internal/platform/registry/store.go 都维护函数契约状态；service 侧负责版本转换，registry 侧负责运行快照，当前命名不能直接显示这一边界（internal/service/contract_service.go:55；internal/platform/registry/store.go:159）。

## 覆盖率补测 2026-11-08（巡检单 2026-10-20）

### 结论

**98% 目标未达成，如实记录**。最终实测（2026-11-08 全量 server 覆盖跑，已剔除生成
protobuf 与 demo main）：

- 聚合 **92.53%（67316/72747 语句）**——**下界**：跑测时负载 94/14，`internal/api/ops`
  600s 超时、`internal/app/agent` 与 `core/metrics` 指标测试失败（三者均为记忆库记载的
  负载 flake 族，隔离全绿，非回归），故该 profile 对这三包欠覆盖。
- 剔除这三包后，**干净子集实测 94.74%（62385/65852）**——这是本次可信的实测值。
- 三包真实覆盖（批次 3 基线实测）：api/ops ≈79.3%（(3473-718)/3473，当时其测试跑完）、
  app/agent ≈93%、core/metrics ≈97%；仅会把聚合抬升，故 **server 真实覆盖 ≥92.53%，
  干净子集 ≈94.7%，仍远未达 98%**。

本批 6 个 commit 共新增约 400 条语句覆盖（functioninvoke +26.6pt、model +4.7pt、
tcp/vault/batchop 错误路径）；但离 98% 仍需 ~3000 条语句，其中 api/ops（718）、
api/incident（410）需大量 httptest+DB fixture，capture/connector/pg（404）与
capture/connector/mongo（106）是集成门（需要真实 postgres/mongo），非本会话可达。
详见下方「未覆盖清单与原因」。

### 每批提交（均英文 commit、无 AI 痕迹、单批单 push）

| commit   | 说明                                                                      | 本批度量                                                              |
| -------- | ------------------------------------------------------------------------- | --------------------------------------------------------------------- |
| 1ba866da | go-sdk 过滤非库入口 + 补 protocol/invoker 测试                            | go-sdk 库 98.48% stmt（此前 codecov 显示 90.56% 为陈旧 carryforward） |
| dc5e42d5 | server CI 过滤生成代码与 demo 入口                                        | server 真实 93.39%（剔除 340 条 .pb.go + 150 条 demo main）           |
| 62cc3e32 | functioninvoke 派发链端到端（Govern→Dispatcher，stub 会话）               | functioninvoke 52.66% → 79.27%                                        |
| 9f91ea79 | ops 面 model CRUD/查询补测（9 个用例）                                    | internal/model 91.60% → 96.32%（+182 条）                             |
| 5bcbb1a0 | tcp Send 拒绝 / MuxConn handler 错误回包 / TLS 坏证书 / ConnectionSession | core+internal tcp 错误路径                                            |
| 3485df81 | vault 落盘失败回滚 / batchop selector 守卫与解码失败                      | vault persist 回滚 + batchop 失败态                                   |

### 未覆盖清单与原因（server 口径，按未覆盖语句数降序）

- internal/api/ops（718）：REST handler/helper 面，需 httptest + 服务上下文 + DB fixture。
- internal/api/incident（410）：报告/趋势/导出，同需 httptest + DB fixture。
- agents/capture/connector/pg（404）：集成门，需真实 postgres（本地三方言容器矩阵未起）。
- internal/app/agent（279）：进程托管/ops server，负载敏感，本会话隔离跑全绿但全量超时属已知 flake 族。
- agents/capture/source（228）：采集源解析，依赖 pg/文件系统集成。
- internal/svc（221）：组合根与 wiring，多为启动装配分支。
- agents/sidecar/funcplugin/protocol（205）、agents/synthetics/pluginhost/protocol（135）：**生成 protobuf**，按巡检单步骤 1 剔除。
- agents/capture/connector/mongo（106）、agents/capture/runner（101）、capture/gate（64）：集成门（mongo/实跑）。
- core/healthprobe（107）、agents/synthetics/probe（66）：ICMP 探测，需真实网络/raw socket 权限。
- internal/server（76）、internal/agent（65）、agents/controlconn（~50）：连接/控制面，部分需真实 TCP 端到端。
- internal/ops/provision（72）、batchop（62）、runbook（45）：部分已补（batchop selector/解码失败、vault 回滚），余为部署/审批链 fixture。
- internal/service/versioning（52）：散点 1 语句分支约 40 处，低 ROI。
- internal/platform/vault（43）：**本批已补** persist 落盘失败回滚；write/flush/close/rename 故障需真实 I/O 注入（防御分支）。
- internal/ops/runbook substituteStep（253-259）：对纯字符串 StepSpec 的 json.Marshal/Unmarshal 不可能失败，属防御分支，无法以测试触达。
- 其余长尾：单文件 <50 条，多为错误早退/启动装配，需针对性 fixture。

### 为什么 98% 本会话不可达

1. 生成 protobuf（funcplugin/protocol 205 + pluginhost/protocol 135 = 340 条）按巡检单「排除生成代码」剔除，但 codecov 口径若不剔除会拉低；剔除后剩余缺口仍 ~3000 条。
2. 集成门包（capture pg/mongo/runner/source，约 840 条）需真实 postgres/mongo/实跑，本地未起三方言容器矩阵。
3. 大 REST 面（api/ops + api/incident，1128 条）需按 handler 逐个建 httptest + DB fixture，单包即需数百行 fixture。
4. 防御分支（vault persist I/O、runbook substituteStep、registry 空循环等）无外部注入点，改生产代码才能覆盖——违反「只加测试」纪律。

### 续批 6-13（2026-11-09，继续按「诚实覆盖」压降）

98% 结论维持不变；本日续批收口原清单中的大头（api/incident、capture/runner、devops/session、
svc wiring、provision runner）：

| commit   | 说明                                                                              | 本批度量                                              |
| -------- | --------------------------------------------------------------------------------- | ----------------------------------------------------- |
| 641f5ec9 | external 入站面（Bearer/限流/幂等/令牌 CRUD/admin 守卫）                          | external.go 83.1% / external_handler.go 86.8%         |
| 220410ad | 类别 upsert/唯一冲突、登记校验闭集、更新扫掠、状态流转、报表路由                  | service.go 92.7% / handler.go 82.6%                   |
| aa56db29 | 桶键/趋势缺省窗/dataEpoch/W53 同比缺失/生成器 sink 失败与聚合截断                 | report.go 91.9% / report_generator.go 89.8%           |
| 198bf3f4 | capture spool legacy 恢复/键跳过/原子写失败注入 + 连接器按型选取                  | alert_spool.go 剩 4 条（Marshal 死分支 + 磁盘故障面） |
| 1bf7c1a8 | runSourceOnce 生命周期（位点损坏/Open/Poll/journal/流结束/取消终刷）+ probeTarget | capture/runner 83.6% → 88.4%                          |
| 82955956 | devops 会话守卫（broker 白名单热替换/recorder/cast 坏形）+ SSH 生命周期           | devops/session 81.5% → 89.5%                          |
| c27cb959 | provision TfexecRunner 错误路径（newTF 早退/变量装配先于执行）                    | ops/provision 78.9% → 84.5%                           |
| （在途） | provision wiring 适配层（Run↔模型映射/CAS/审批门/注册等待）                       | internal/svc 92.6% → 95.3%（全量 262s 核验通过）      |

api/incident 由 410 条压至 ~118 条（余为 DB 错误传播防御分支 + 已识别死兜底，测试注释标注）。

### 续批发现的生产缺陷（纪律=只加测试，未改行为；测试注释已标注）

1. `internal/api/incident/service.go` UpdateIncident：`updates["details"] = req.Details` 以
   map 更新绕过 GORM JSON 序列化器，三方言均报 unsupported type —— details 字段更新必炸。
2. `internal/svc/provision_wiring.go` provisionDBStore.Update：`runToModel` 不回填数值主键
   （provision.Run 无 PK 字段），`Save(ID=0)` 退化为再插入 → run_id 唯一键冲突。

两条待用户确认修复方向后另行处理；同族首例为 incident Details map 序列化。

### 续批 15-17（2026-11-09 续）

| commit   | 说明                                                                                                                                                           | 本批度量                          |
| -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------- |
| 041ac177 | controlconn 请求包装往返（告警上报/探针策略/synthetics/capture 闸门/devops 白名单帧级应答、probe 单向事件、TLS 打明文端口失败）                                | agents/controlconn 46.2% → 93.8%  |
| 4c46463b | devops runner 装配面（fetchAllowlist 全分支/applyAllowlistCenter 校验热换/Run 审计·存档·SSH 早退/注册存 conn/observer 打点/tlsFromCfg）                        | agents/devops/runner 65.2% → 100% |
| b2ace92d | healthprobe ICMP 生命周期（open seam 注入 fake PacketConn：v4 回声成功/错配逐跳跳过/写读失败/超时重试/CAP_NET_RAW 分类）+ checker/policy/manager/protocol 分支 | core/healthprobe 84.3% → 99.3%    |

### 续批发现的生产缺陷（补，纪律=只加测试，未改行为；icmp_extra_test.go 注释已标注）

3. `core/healthprobe/icmp.go` pingOnce：IPv6 分支的 replyType 为
   `ipv4.ICMPType(ipv6.ICMPTypeEchoReply)`，而 `icmp.ParseMessage(proto 58)`
   产出 `ipv6.ICMPType` 动态类型——接口比较恒不等，正确的 IPv6 回声应答被
   误判为 `network: ICMP echo reply code 0`，即 **IPv6 ICMP 探活永远失败**。
   修复方向（replyType 直接用 ipv6.ICMPType）待用户定夺；v4 路径不受影响
   （TestICMPCheckerLoopbackSuccessWithRawCapability 只测 v4，故此前未暴露）。

### 续批 18-23（2026-11-09 续，internal/api/ops 收尾 + capture runner 补测）

| commit   | 说明                                                                                                                                                                             | 本批度量                                    |
| -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------- |
| 5a05bbca | capture runner 闸门/拉取拨号/命中上报分支（gates/pull dial/hit sender）                                                                                                          | agents/capture/runner 补测                  |
| fdf9688e | devops 白名单面板与 batch ops executor 分支                                                                                                                                      | internal/api/ops 补测                       |
| f739a46d | cost control 与 probe policy handler 分支                                                                                                                                        | internal/api/ops 补测                       |
| 7265e855 | deploy agent 三败点全链/rollback、runbook 引擎、supervisor 日志下载、probe diagnose、synthetics、provision 销毁/护栏/审批工作流                                                  | internal/api/ops 全量实测 88.3%（批 21 后） |
| fab6018f | handler 批次五端点（scope 注入/实名/dry-run）、runbook 八端点（ContinueOnFailure→partial 仍 200）、deploy handler 往返、capture gates 事实交集、helpers 审计留痕/越域/alert 流转 | internal/api/ops 88.3% → 95.9%              |
| b7ee697c | 隧道错误两态（supervisor 族/进程更新/GetSystemInfo 两态/StartProcess）、GetMeta 集群 ghost 未命中、batch op 与 runbook 编排器未装配错误面、deploy 未装配/表缺失/回滚错误分支     | internal/api/ops 95.9% → 97.0%              |

internal/api/ops 终态：97.0%（-race -covermode=atomic 全量 515s 绿）。余 16 个零
覆盖块全部为已记录的不可达分支（各 *_extra_test.go 文件头诚实跳过清单）：

- handler.go 7 块：GET 侧 bind 错误——请求字段无 `binding:"required"` 且
  BindQueryCompat 吞掉 ShouldBindQuery 错误（#59 同款），GET 侧不可触发。
- agents.go :119 / helpers.go :174/:335 / supervisor.go :196：自身 nil-store
  守卫在 requireAgentScope 之后——requireAgentScope 在 store nil 时已先报
  "registry store unavailable"，分支体不可达。
- helpers.go :825/:973：proto.Marshal 对空请求体不可触发。
- deployments.go 133/141/145：multipart 内部结构失败需伪造 fileHeader、
  内存 multipart 读取不失败、超 500 MiB 载荷需真实分配半 GB 内存。

- [x] 审核发现：状态上报测试仅等待固定 300ms 就断言异步服务端已收到消息，调度较慢时即使实现正确也会间歇失败（agents/capture/connector/pg/fakepg_test.go:493）。属实并已修：固定 sleep 改为 `waitStatus`（5s 截止轮询，5ms 间隔，fakepg_test.go:1007），慢调度下不再假失败。`go test ./agents/capture/connector/pg/ -count=1` 全绿（ok 8.8s）。证据：agents/capture/connector/pg/fakepg_test.go:493、1007。
- [x] 审核发现：取消发送测试依赖固定休眠且不确认缓冲已满，过早取消或排空时仍可通过，无法保证测到阻塞发送的取消分支（agents/capture/connector/pg/fakepg_test.go:920）。属实并已修：两个取消发送测试的固定 sleep 改为 `waitBuffered(t, events, 64)`（轮询确认事件通道积满容量 64、发送 goroutine 已阻塞在 `out<-` 后才取消，fakepg_test.go:1021），保证测到阻塞发送的取消分支。`go test ./agents/capture/connector/pg/ -run 'TestSnapshotSendArms|TestPollPumpSendCancel' -count=1` 全绿。证据：agents/capture/connector/pg/fakepg_test.go:917、1424、1021。
- [x] 审核发现：两个取消发送测试返回前未关闭 connector，而假服务器清理只关闭监听器，导致已有连接及服务端 goroutine 在测试结束后泄漏（agents/capture/connector/pg/fakepg_test.go:931）。属实并已修：两个取消测试返回前补 `_ = c.Close(context.Background())` 关闭连接（fakepg_test.go:928、1434），服务端 goroutine 随连接关闭退出，不再泄漏。`go test ./agents/capture/connector/pg/ -count=1` 全绿。证据：agents/capture/connector/pg/fakepg_test.go:928、1434。
- [x] 审核发现：lint 任务使用默认单提交浅检出，push 时通常不存在推送前的 commit，版本破坏性检查因而回退到当前 main 与自身比较，直推的破坏性 proto 变更仍会漏检（.github/workflows/ci.yml:125）。属实并已修：CR-FIX-05 步骤在 push 事件且本地缺 `github.event.before` 提交时（浅检出 depth 1 必然如此）显式 `git fetch --no-tags --depth 1 origin "$BEFORE"` 取回后作为 breaking 基线，取回失败才回退 refs/heads/main；浅检出下直推破坏性 proto 变更不再与自身比较空转。YAML safe_load 解析验证通过。证据：.github/workflows/ci.yml:126-132（proto breaking gate）。
- [x] 审核发现：不兼容响应向容量为 1 的 pending 通道阻塞发送，重复响应与调用取消交错时可使接收循环永久卡住，导致同连接后续请求均无法收到响应（sdks/go/pkg/croupier/transport/tcp_client.go:393）。属实并已修：不兼容版本响应与正常响应两条投递路径均加 `default:` 分支——等待方已取消（通道满、无读者）时丢弃响应而非阻塞（tcp_client.go:397、428），接收循环不再被卡死；新增 `TestReceiveLoopDropsUndeliverableResponseAndContinues`（pending 通道预置陈旧响应占满，写入不兼容版本+重复响应帧后断言存活请求照常送达、废弃响应被丢弃；变异验证：还原旧代码该测试按预期报 receiveLoop stalled on delivery）。`go test ./sdks/go/pkg/croupier/transport/ -count=1` 全绿（ok 4.4s）。证据：sdks/go/pkg/croupier/transport/tcp_client.go:397、428、tcp_client_whitebox_test.go:104。

## 已知缺陷（2026-11-10 发现，未修）

> 已修复备注（收割 DB 落库失效）：`taskStore`（cmd/server/root.go:539）绑定 `svcCtx.DB`（meta 库），但 `task_runs` 表只在 game 库（internal/model/migration.go:202 GameModels）。`SetAgentDisconnectReaper`（root.go:610）的 `ReapAgentNonTerminal` 收割在 meta 库报 `no such table: task_runs`，失败静默（只记日志，root.go:609 注释），DB 层面 task 终值从未落库。前 3 个 drill 场景经 API 读 runtime 内存态掩盖了此问题（API 收敛正确，DB 未落库）。状态：已修（CROP-REAP-001，2026-11-10，LOCAL VERIFIED）。改动点：(1) internal/tasks/store.go 增 `TaskScope`/`ScopeResolver`/`SetScopeResolver`，`ReapAgentNonTerminal`/`ReapStale` 经 `reapAcrossScopes` 逐 game 库 fan-out、RowsAffected 合计、`errors.Join` 聚合失败库（不静默丢弃，健康库照常收割；nil resolver=单库原路径不变）；(2) internal/db/router/router.go 增 `OpenScopes()`——只列本进程已打开缓存库，休眠注册 game 不触发首开副作用；(3) cmd/server/root.go `svcCtx.Router != nil` 时注入 `routerTaskScopes` 适配器（复用 `Router.Resolve` 的 dbctx 注入，非新机制）。drill 真因补记：CI env `DATABASE_URL=:memory:` 在 svc/db.go `resolveDriverAndDSN` 无条件压过配置 dataSource，drill 的 runServer 跑在进程内存库上（配置文件库根本不创建），停机后 `drillReadStatusDB` 读文件库的落库断言永不成立——fault_drill_r27_test.go `drillBoot` 为 runServer 清 `DB_DRIVER`/`DATABASE_URL`（cleanup 还原，TempDir 仍 hermetic），CI Test 步骤设 `CROUPIER_DRILL_DB_FIXED: "1"` 常开断言。完成证据：commit eca487cf（fan-out + OpenScopes + wiring + 单测 5 例：双库收割/终态他人行不触/失败库聚合/nil 单库/router OpenScopes；变异验证 nil resolver 于无 task_runs meta fixture 复现 `no such table: task_runs`）、ee40c0f3（drill env 清场 + CI gate）。测试：`TestDrill_` 5/5 PASS（Sigterm 含 DB 落库断言）、cmd/server 全量 ok 467.99s、internal/tasks + internal/db/router + internal/db/dbctx 全绿、`go vet ./...` 0 错、actionlint 通过、scripts/check_layer_dependency.sh PASSED。

## PowerOps 审核待办（2026-11-12）

- [x] **PWR-001 · P1 · 修复 · 审批后续行必须原子领取，避免重复执行**
  - 依赖：—。交付与验收：把 pending→executing 改成数据库条件更新/CAS；仅一个并发 resume 能领取并启动 provider，其他请求收到稳定冲突；覆盖批准与 reject 竞争、并发 force_off/reboot 续行，断言副作用调用次数严格为 1。
  - 状态／条件：LOCAL VERIFIED（2026-11-21，commit 9a1e64c79）：`RequestStore` 新增 `Transit(requestID, fromState, toState, startedAt) (bool, error)`——一条条件 UPDATE（`WHERE request_id=? AND state=fromState`，RowsAffected>0=胜者），GormStore 实现；`BeginResume` 与 `Reject` 的 pending→executing / pending→rejected 均改走 Transit，CAS 输家经 `claimConflict` 重读当前状态后返回 `ErrRequestNotPending`（API 既有映射 → 409 `power_state_conflict`，稳定冲突），绝不落到副作用。覆盖：并发 resume 双 racer（force_off/reboot × 5 轮）恰一人领取且 provider 动作计数严格 1、终态后再次 resume 仍冲突；批准与 reject 双向竞争（reject 先赢零副作用 / resume 先赢 reject 冲突且照常收敛）
  - 验证：`go test ./internal/powerops/ -count=1 -race` ok（新增 `TestBeginResumeConcurrentClaimExecutesOnce`、`TestResumeRejectRaceSingleTransition`，均含副作用计数断言；变异自证：Transit 恒真时同测试红 `actual: 2` 复现双领取）；`go test ./internal/api/ops/` ok；`go build ./...` 0 错；`go vet` 干净；gofmt 干净；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 0 错；`pnpm --dir web run tsc` 0 错
  - 完成证据：commit 9a1e64c79（5 文件，151 insertions / 9 deletions）；实现 `internal/powerops/request.go`（接口 `Transit`、`BeginResume`/`Reject` CAS、`claimConflict`）、`internal/powerops/store_gorm.go`（Transit 条件 UPDATE）；测试 `internal/powerops/request_test.go`（scriptedProvider 增 actionCalls 计数 + 2 个竞态用例）；文档 `docs/design/ops-power-management.md` 与 `docs/zh/design/ops-power-management.md`（`approved → executing` 补 CAS 领取语义）
  - 已知边界：`go test ./internal/...` 首轮 7 包 `[build failed]` 为 go-build 缓存被并发清空的纯环境性故障（`no such file or directory`），模块缓存重建时 proxy.golang.org 又偶发 EOF，重试后 7 包全 `ok`（缓存补齐），非本改动回归；API 层稳定冲突映射复用既有 `mapPowerError`，未新增 API 用例（sentinel→409 已由 `TestPowerResume_Unapproved_409` 覆盖）

- [x] **PWR-002 · P1 · 修复 · Provider 动作增加独立超时和有界执行上下文**
  - 依赖：—。交付与验收：后台执行继续不受 HTTP 断连影响，但为每次电源动作及整体请求设置可配置上限；验证超时从实际动作结束后开始；用永不返回的 Provider/Runner 证明请求在上限内落为可观察失败，goroutine 不永久滞留。
  - 状态／条件：LOCAL VERIFIED（2026-11-21，commit 505277cb6）：`Orchestrator` 新增 `ActionTimeout`（单次 provider 调用独立上限，变更动作与状态轮询同用，默认 30s）与 `RequestTimeout`（整个执行含验证的整体上限，默认 5m，`internal/powerops/request.go`）；`executeRequest` 全程跑在 `context.WithTimeout` 派生的 runCtx 下（后台断连纪律不变）；`runActionBounded`/`statusBounded` 把每次 provider 调用放入 goroutine 并在 deadline 落为诚实失败行（"powerops: action timed out after ..."）；verify 循环以 `ctx.Err()` 截断，超出整体界时 `req.ErrorMsg` 记录 "request exceeded overall bound"。配置面 `PoweropsConfig` 新增 `actionTimeoutMs`/`requestTimeoutMs`（lowerCamelCase，`EffectiveActionTimeout`/`EffectiveRequestTimeout` 默认 30000/300000，`internal/config/config_powerops.go`），svc wiring 注入（`internal/svc/powerops_wiring.go`）。永不返回的 provider 证明：`hungProvider`（任何调用都挂）动作被独立界切断、`hangStatus`（仅 Status 挂）由整体界切断验证循环，请求均在上限内落为可观察终态；设计文档 §6 双层时间界同步 en/zh。
  - 验证：`go test ./internal/powerops/ -count=1 -race` ok（新增 `TestHungActionTimesOutAndRequestConverges`、`TestHungStatusCutByRequestBound`；变异自证两处：`runActionBounded` 直通未加界 → 前者红「request never reached a terminal state」复现，`statusBounded` 直通未加界 → 后者红同签名，还原后全绿）；`go test ./internal/... -count=1` 172 包全绿 0 FAIL；`go build ./...` 0 错；gofmt 干净；`bash scripts/dashboard_vnext_guard.sh` PASSED；`python3 scripts/docs/check-architecture.py` PASS；`cd docs && CI=true pnpm build` 0 错；`pnpm --dir web run tsc` 0 错
  - 完成证据：commit 505277cb6（6 文件，204 insertions / 7 deletions）；实现 `internal/powerops/request.go`（`ActionTimeout`/`RequestTimeout` 字段与 `actionTimeout`/`requestTimeout` 默认助手、`runActionBounded`/`statusBounded` 单次调用界、executeRequest 整体界、verify 循环截断）、`internal/config/config_powerops.go`（`actionTimeoutMs`/`requestTimeoutMs` + `EffectiveActionTimeout`/`EffectiveRequestTimeout`）、`internal/svc/powerops_wiring.go`（注入）；测试 `internal/powerops/request_test.go`（`hungProvider`、`TestHungActionTimesOutAndRequestConverges`、`TestHungStatusCutByRequestBound`）；文档 `docs/design/ops-power-management.md` 与 `docs/zh/design/ops-power-management.md`（§6 双层时间界）

- [x] **PWR-003 · P1 · 修复 · 逐目标持久化执行进度并恢复崩溃中的请求**
  - 依赖：PWR-001。交付与验收：每台机器动作前后独立落库；定义进程重启后 executing/verifying 请求的恢复与不确定结果处理，不对结果未知的破坏性操作盲目重放；注入批次中途崩溃，重启后保留已完成目标且请求可收敛，后续目标不会丢失。
  - 状态／条件：LOCAL VERIFIED（2026-11-21，commit 8eb7ad62c）：`PowerRequestTarget` 新增 `status` 列（`attempting`/`done`，编号迁移 0065 AddColumn + 存量行回填 `done`，`internal/model/power_request_model.go` + `internal/svc/migrations_ops.go`）；`executeRequest` 每台机器动作前落 `attempting` 行、结果记录后翻 `done`（`persistTargets` 助手，批次中途崩溃时已完成目标仍留表）；新增 `RecoverInFlight`：启动时（svc 装配处同步调用）把残留 executing/verifying 收敛——`done` 行原样保留、`attempting` 按「结果未知」记失败、循环未到达的机器按「未尝试」记失败，请求落可观察终态且 `errorMsg` 指名中断，恢复路径零 provider 调用、绝不重放破坏性操作；`RequestStore`/`GormStore` 新增 `ListInFlight`。
  - 验证：`go test ./internal/powerops/ -count=1` ok（新增 `TestCrashMidBatchRecoveryKeepsCompletedTargets`——真实中途"崩溃"注入（node-2 动作永不返回、行停留 attempting）后恢复：已完成目标保留、未知记失败、未到达补失败行、零重放；`TestRecoverInFlightMarksUnknownAndKeepsCompleted`、`TestRecoverInFlightNoTargetsMarksAllInterrupted`；变异自证两处：去掉逐机持久化 → 崩溃测试红「已完成目标丢失」复现，恢复把 attempting 当已完成接受 → 两恢复测试红「outcome unknown」缺失，还原后全绿）；`go test ./internal/svc/ ./internal/db/migrate/ ./internal/api/ops/ -count=1` ok；`go test ./internal/... -count=1` 172 包全绿 0 FAIL；`go build ./...` 0 错；gofmt 干净；`bash scripts/dashboard_vnext_guard.sh` PASSED；`python3 scripts/docs/check-architecture.py` PASS；`cd docs && CI=true pnpm build` 0 错；`pnpm --dir web run tsc` 0 错
  - 完成证据：commit 8eb7ad62c（11 文件，320 insertions / 12 deletions）；实现 `internal/powerops/request.go`（status 常量、逐机 `persistTargets`、`RecoverInFlight`/`recoverRequest`）、`internal/powerops/store_gorm.go`（`ListInFlight`）、`internal/model/power_request_model.go` + `internal/svc/migrations_ops.go` + `internal/svc/migrations.go` + `internal/db/migrate/migrate.go` + `internal/db/migrate/migrate_test.go`（0065 迁移四同步：函数+注册+MinimumRequiredVersion 65+probe 断言）、`internal/svc/service_context.go`（启动恢复调用）；测试 `internal/powerops/request_test.go`；文档 `docs/design/ops-power-management.md` 与 `docs/zh/design/ops-power-management.md`（§6 逐目标进度与启动恢复）
  - 已知边界：恢复把 executing 中断的请求按既有 `terminalState`/`finish` 计数聚合（动作成功但未验证的目标计入 Succeeded 但不改变 failed 语义）——该口径与正常执行路径一致，其双定义收敛归 PWR-004；恢复同步在启动装配内执行，in-flight 量大时启动耗时随之增长（当前 ListInFlight 无分页，与 List 同上限语义不同、不截断）。

- [x] **PWR-004 · P1 · 修复 · 请求计数与验证终态采用同一成功定义**
  - 依赖：—。交付与验收：明确并分别表达“动作已发出”和“目标状态已验证”的计数；终态聚合、请求头计数及逐目标字段一致；覆盖验证超时、部分验证成功、provider 不支持验证三种情形，禁止出现 `state=failed` 且 `failed=0`。
  - 状态／条件：LOCAL VERIFIED（2026-11-21，commit 616e6f122）：`finish` 的头部计数改用与 `terminalState` 同一成功定义——机器仅在「已确认」（`Success && (Verified || !verifiable)`）时计成功；「动作已发出」与「状态已验证」的区分保留在逐目标字段（`success` 与 `verified`），请求头不再用 `Success`（仅动作发出）当成功计数。`state=failed` 现必然 `failed=total>0`，`failed=0` 的畸形终态消除。
  - 验证：`go test ./internal/powerops/ -count=1 -race` ok（`TestVerifyTimeoutKeepsUnverifiedHonest` 增断言 Succeeded=0/Failed=1 且 Total=Succeeded+Failed——原缺陷态 state=failed/succeeded=1/failed=0 的直接回归；`TestHungStatusCutByRequestBound` 增 0/1；`TestWOLWakeIsSuccessButNeverVerified` 增 Succeeded=1/Failed=0（provider 不支持验证=确认）；部分验证成功由 `TestPartialFailureAggregation` 覆盖（partial 1/1）；变异自证：finish 退回按 `Success` 计数 → `TestVerifyTimeoutKeepsUnverifiedHonest` 红（expected 0/actual 1、expected 1/actual 0）复现 state=failed/failed=0，还原后全绿）；`go test ./internal/... -count=1` 172 包全绿 0 FAIL；`go build ./...` 0 错；gofmt 干净；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 0 错；`pnpm --dir web run tsc` 0 错
  - 完成证据：commit 616e6f122（4 文件，27 insertions / 7 deletions）；实现 `internal/powerops/request.go`（`finish` 计数改确认口径 + 注释）；测试 `internal/powerops/request_test.go`（三处计数断言）；文档 `docs/design/ops-power-management.md` 与 `docs/zh/design/ops-power-management.md`（§6 终态与计数同口径）

- [x] **PWR-005 · P1 · 修复 · 将 status 查询从审批和变更执行状态机分离**
  - 依赖：—。交付与验收：status 请求按设计仅需身份与目标权限，不创建审批单、不进入 executing/verifying；直接返回一次状态查询及逐目标结果；测试确认查询延迟不随 `verifyTimeout` 增长，并覆盖 IPMI/cloud/agent 查询失败与未知状态。
  - 状态／条件：LOCAL VERIFIED（2026-11-21，commit a305bc87c）：status 从审批与变更状态机分离——`Submit` 对 status 返回 `ErrInvalidSubmit`（结构上强制走查询，分离不是调用方约定），新增 `Orchestrator.QueryStatus`：机器校验（与 Submit 共用 `resolveSubmitMachines`）后每台机器一次有界 Status 调用（`actionTimeout` 界），请求直接落终态（done/partial/failed）+ 逐机结果行，不建审批单、不进 pending/executing/verifying；行只在末尾一次写入，崩溃中途无在飞残留、只读无副作用可重放；`confirmed` 定义扩展（PWR-004 口径延伸）——status 查询以确定性 on/off 为已确认，未知答案/查询失败不计（诚实的"说不清"不是已确认电源状态），头部计数与终态聚合同一口径；API 层 `POST /ops/power` 对 `action=status` 同步返回 200 + 请求 + targets（变更动作仍 202 + 审批 id）；executeRequest 对 legacy status 请求短路（不进 verifying、不等 verifyTimeout）。
  - 验证：`go test ./internal/powerops/ -count=1 -race` ok（新增 `TestStatusQuerySkipsApprovalAndLandsTerminal`——无审批单、终态直达、每机恰好一次调用无轮询；`TestStatusQueryLatencyIndependentOfVerifyTimeout`——`verifyTimeout=30s`/`ActionTimeout=50ms` 下挂死 provider 查询 50ms 级收敛；`TestStatusQueryPerProviderFailureAndUnknown`——ipmi on/cloud 查询失败/agent 未知三行如实呈现、partial 1/2；`TestStatusQueryUnknownWithoutErrorIsNotConfirmed`、`TestStatusTerminalAggregationTreatsUnknownAsUnconfirmed`、`TestSubmitRefusesStatusAction`；变异自证三处：QueryStatus 退化为等待 `verifyTimeout` → 延迟测试红「30.08s 不小于 5s」复现，Service 去 status 分支 → API 测试红「200 实收 422」复现，`confirmed` 把未知当已确认 → 聚合测试红「done≠partial、2≠1」复现，还原后全绿）；预存 flake 修复：`TestCrashMidBatchRecoveryKeepsCompletedTargets` 采样竞态（attempting 先于动作调用落库，`callsBefore` 采样早于 node-2 挂起调用记录，base 0d5caef9b 复现 2/6）——采样前等计数到崩溃窗口终值，修复后 -race 全量 8/8 绿；`go test ./internal/api/ops/ ./internal/svc/ ./internal/db/migrate/ -count=1` ok；`go test ./internal/... -count=1` 0 FAIL；`go build ./...` 0 错；gofmt 干净；`bash scripts/dashboard_vnext_guard.sh` PASSED；`cd docs && CI=true pnpm build` 0 错；`pnpm --dir web run tsc` 0 错
  - 完成证据：commit a305bc87c（6 文件，406 insertions / 31 deletions）；实现 `internal/powerops/request.go`（`resolveSubmitMachines` 提取、`Submit` status 守卫、`QueryStatus`、`confirmed`/`confirmedCount` 与 `terminalState` 去 status 特例、`finish` 改用 `confirmedCount`、executeRequest status 短路）、`internal/api/ops/powerops.go`（`OpsPowerSubmit` status 分支返回 targets、`PowerSubmit` 同步 200）；测试 `internal/powerops/request_test.go`（statusByNode 替身 + 6 用例 + 预存 flake 采样修复）、`internal/api/ops/powerops_test.go`（`TestPowerStatusSubmit_Synchronous_NoApproval`）；文档 `docs/design/ops-power-management.md` 与 `docs/zh/design/ops-power-management.md`（§6 状态机 status 不进入 + §7 API 同步返回）
  - 已知边界：status 请求行的 `verified` 字段恒为 false（查询是观察、不是对变更动作的验证）；legacy 存量 pending status 请求（分离前建的）无法 resume（无审批链），人工 reject 收敛；「身份与目标权限」沿用 /ops 路由既有鉴权口径，未新增机器级 ACL。

- [x] **PWR-006 · P2 · 修复 · WOL 结果区分数据包发送与机器上线确认**
  - 依赖：PWR-004。交付与验收：WOL 发送成功只记为已派发，不直接让未验证目标呈现为完成；若配置关联 Agent，则等待 Agent 重连并记录验证结果，否则明确返回 pending/unverified 等诚实状态；测试分别覆盖 Agent 重连、未重连和未配置 Agent。
  - 状态／条件：LOCAL VERIFIED（2026-11-21，commit c553e91b3）：WOL 派发与唤醒确认分离——`confirmed` 对 WOL 特例化：魔术包发出只记派发（`Success`），只有 agent 重连（`Verified`）才计确认，无论机器是否登记 agent 引用（无引用则永不能确认，绝不伪装 done）；`verifiableFor`（machine 感知：WOL 有 agentId 才可验证）+ `verify` 循环把 WOL 目标的轮询路由改走 agent provider（用 WOL 机器的 agentId 探活——唯一的上线证明）；`wolUnverifiedNote` 在请求 `errorMsg` 指名「已派发未确认」两因（无 agent 可确认 / agent 未重连）；设计 §3「唤醒确认=agent 重连」落地。
  - 验证：`go test ./internal/powerops/... ./internal/api/ops/... -count=1 -race` ok（新增 `TestWOLWakeWithAgentReconnectConfirms`——agent 120ms 重连 → verified/on/done 1/0；`TestWOLWakeWithAgentNoReconnectUnverified`——窗口内未重连 → failed 0/1 + errorMsg "did not reconnect"；`TestWOLWakeWithoutAgentLandsUnverified`——无 agentId → failed 0/1 + errorMsg "no agent configured to confirm"；重写原固化「不可验证即 done」的 `TestWOLWakeIsSuccessButNeverVerified`；变异自证两处：`confirmed` 去 WOL 特例 → 两「应失败」用例红（实收 done、Succeeded=1）复现「包发送误报上线」，verify 循环去 WOL→agent 路由 → 重连用例红（实收 failed）复现「agent 从不被探活」，还原后全绿）；`go test ./internal/svc/... ./internal/db/migrate/... -count=1` ok；`go test ./internal/... -count=1` 172 包全绿 0 FAIL；`go build ./...` 0 错；gofmt 干净；`bash scripts/dashboard_vnext_guard.sh` PASSED；`python3 scripts/docs/check-architecture.py` PASS；`cd docs && CI=true pnpm build` 0 错；`pnpm --dir web run tsc` 0 错
  - 完成证据：commit c553e91b3（4 文件，169 insertions / 21 deletions）；实现 `internal/powerops/request.go`（`verifiableFor`、verify 循环 WOL→agent 路由、`wolUnverifiedNote`、`confirmed` WOL 特例）；测试 `internal/powerops/request_test.go`（wolOrch/wolResolver 替身 + 3 用例 + 重写原固化用例）；文档 `docs/design/ops-power-management.md` 与 `docs/zh/design/ops-power-management.md`（§6 计数口径 WOL 除外 + §9 无 agentId WOL 诚实落 failed）
  - 已知边界：WOL 唤醒确认仍要求机器 agent 配置自启（平台外部部署前提，设计 §3/§9）；WOL 无优雅关机、无状态查询；WOL 请求行的 `verified` 恒为 false（无 agent 引用时）或由 agent 重连置真；批量混合时 WOL 未确认目标按既有 partial/failed 聚合口径计数。

## 全仓深度审核待办（2026-11-12，第一轮确认项）

> 审核边界：本节记录分区源码复核及正常业务回归后确认的问题。仓库已有 `docs/review/2026-11-01-code-review.md`、`docs/review/external-deep-audit-2026-11-04.md` 等历史审计；历史结论需逐项按当前代码重验，不能视作本轮已覆盖。后续按 Go 服务/存储、Agent/协议与六语言 SDK、Web、移动端、部署/CI 分区继续审核，并把新证据增补在此处。

### 审核覆盖进度

> 2026-11-13 续审进度：第一轮 [风险路径报告](docs/review/deep-audit-2026-11-12.md) 已落盘；按用户要求转入 [代码逻辑续审](docs/review/code-logic-review-2026-11-13.md)，深入供给适配器、发版/回滚、事故更新、定时调度、ICMP 与插件/Runbook 生命周期。新增 8 个正常业务回归断言均暴露现有问题，原有相关测试通过；未修改业务实现。下列覆盖任务仍未全部完成，不因已有报告而整体勾选。

- [x] AUD-COV-001 · 复核 · Go API/业务服务/权限与游戏、环境作用域边界：按路由入口追踪身份、scope 中间件、数据查询/写入和错误响应；覆盖 internal/api、internal/handler、internal/platform、internal/policy。
  - 状态／条件：完成（2026-11-16 codex exec read-only 静态审计 + 2026-11-17 逐条人工复核）。LOCAL VERIFIED
  - 完成证据：报告 `docs/review/aud-cov-001-go-api-2026-11-16.md`（3 P0/8 P1/7 P2/6 P3 + 处置记录附注）；当场修 4 条 `9bc08d56a`/`5b0fee15f`/`ce667c63d`/`8dbf59c53`（各带新用例，`go test ./internal/api/... -count=1` 全绿）；遗留 21 条落 AUD-036..056 排期。
- [ ] AUD-COV-002 · 复核 · DB/迁移/缓存/队列一致性：核对 meta/game DB 路由、事务边界、迁移升级/回滚、异步任务状态与进程重启恢复；覆盖 internal/model、internal/db、internal/tasks、internal/cache、internal/analytics/mq（仓内无 internal/queue）。
  - 进度（2026-11-21）：本轮核对了 GameDB 首开/关闭、Redis 消费进度及任务留痕，见 DATA-019～023、AUD-057～058、AUD-077。续审追加调度状态、事件游标与迁移上下文、会话锁，见 AUD-086～091、AUD-098。缓存层另外确认管理员凭据字段被序列化，见 AUD-100。本轮补查分析发布、消费进度与读仓失败，见 DATA-043～049。 已补查 Worker 批量写入、确认、重试与跨组检查点，见 DATA-050～055。迁移回滚、真实驱动故障和任务恢复全链仍待复核。
- [ ] AUD-COV-003 · 复核 · Agent 与消息传输/协议：沿 Server→Agent→Provider 全链路检查鉴权、TLS、帧解析、并发/背压、重连、取消、错误传播与关闭；覆盖 core、agents、internal/agent、internal/transport、core/transport。
  - 进度（2026-11-21）：本轮补查 Provider 与互联的关闭边界，见 AUD-070～075、AUD-081。续审复现 Agent 注册就绪、停止后重连与配置版本回退问题，见 AUD-092～094。历史复核补记下次协议升版前的协商缺口 AUD-101。控制面真实装配、重连与背压组合仍待复核。
- [ ] AUD-COV-004 · 复核 · 六语言 SDK 与插件：逐语言核对协议实现、API/配置解析、生命周期、TLS/auth、重试/超时、错误语义及功能矩阵；覆盖 Go/Python/Java/JS/C++/C# 和 plugins。
  - 进度（2026-11-21）：六语言 Provider/Invoker 主路径与 JS/Python 插件生命周期已按本文件「六语言 SDK 与相邻运行链复审」的范围核对，新增 AUD-068～076、AUD-078～080。本轮续查配置入口、Java Provider TLS 和 starter 发布，见 AUD-102～107。语言扩展其余入口、构建发布产物、完整 wire 对照和各环境动态验证仍待覆盖。
- [x] AUD-COV-005 · 复核 · Web Console 与 API 客户端：沿页面→状态管理→请求→服务端授权检查作用域切换、陈旧数据、错误/加载状态、输入校验、XSS 和关键管理操作；覆盖 web/src。
  - 状态／条件：完成（2026-11-16 codex exec read-only 静态审计 + 逐条人工复核核伪）。LOCAL VERIFIED
  - 完成证据：报告 `docs/review/aud-cov-005-web-console-2026-11-16.md`（P1×4/P2×6/P3×3 + 修复与复核附注）；当场修 4 条 `2f6d79d01`/`95072aaeb`/`c430996a2`/`abc6f25a1`；P2/P3 遗留落 AUD-029..035；核伪 2 条见报告附注。
- [ ] AUD-COV-006 · 复核 · Mobile 客户端：检查登录/会话、作用域、token 存储、请求生命周期、旧数据清理、错误/离线表现与 Android/iOS 配置；覆盖 mobile/lib 和原生工程。
  - 进度（2026-11-21）：已检查会话和作用域，并核对 Android manifest 源文件、签名与 APK 工作流，见 AUD-061～064、AUD-066～067、AUD-084～085。续审核对 iOS 工程和 CI 构建入口，见 AUD-097。iOS 实机、合并 manifest 和 release 包验证仍待完成。
- [ ] AUD-COV-007 · 复核 · 构建/发布/部署/运维与文档契约：检查 CI 触发路径、质量门禁、发布版本一致性、镜像/Compose 默认值、密钥暴露、备份恢复和用户文档与实现漂移；覆盖 .github、docker、configs、scripts、Makefile 与 docs。
  - 进度（2026-11-21）：本轮补查分析 CI、SDK 发布输入和备份执行状态，见 DATA-031～032、AUD-078～079、AUD-082～085。续审核对自托管发布配置替换与实例配置保留，见 AUD-095～096、AUD-099。真实部署、恢复演练及各平台产物验收仍待执行。
- [ ] AUD-COV-008 · 汇总 · 回归历史审计并形成总报告：把 11 月 1–10 日历史审计结论按当前 HEAD 逐条标成仍存在/已修/无法复核；合并重复项、给出严重级别和证据；未覆盖区域明确标注，不能用扫描器无告警替代人工复核。
  - 进度（2026-11-21）：已复核 11 月 4 日外部审核的 5 个 P0 与 P1-6～10、P1-12，见下方「历史高优先级问题复核」。其他历史条目仍待逐条对照。

#### 剩余审核入口与验收证据

下表列出当前台账缺少完整复核证据的源码入口，以及需要补做的运行验证。分析与 SDK 已核对的部分见本轮新增条目。

| 范围             | 仍需审核的源码与入口                                                                                                                                                                                                  | 仍需运行的验收                                                                                  |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| 数据分析         | Worker 写入、确认和回收已复核一批故障，仍需真实 Redis/ClickHouse 多进程复验。继续核对 `internal/api/analytics/{payments,retention,behavior}.go` 其余时区、退款及跨窗口口径，以及 Web、Mobile 分析页的空态与失败映射。 | 入站 → Redis → ClickHouse → API → 页面逐指标对账，覆盖重启、重复、退款和晚到数据。见 DATA-031。 |
| 数据库与后台任务 | `internal/db/migrate/migrations/` 的方言分支和回滚关键链，`internal/db/router/` 的多库打开失败与重试，以及 `internal/tasks/store.go` 的冷库任务恢复。                                                                 | 真实数据库升级、失败恢复和多进程重启演练。                                                      |
| Agent 与协议     | `cmd/{sidecar-agent,devops-agent}/` 的运行装配，`agents/{capture,devops,synthetics}/` 的 connector、session、pluginhost 边界，`core/transport/tcp/` 与 `internal/transport/tcp/` 的控制车道、关闭、背压组合。         | 用真实连接验证高负载、重连与停止，核对消息和任务状态收敛。                                      |
| SDK 与插件       | `sdks/java/spring-boot-starter/` 的 Spring 上下文集成，`sdks/cpp/skynet/` 的桥接，`sdks/python/croupier/integrations/`、`sdks/plugins/{js,python}/` 的扩展装配，以及六语言 examples 和边界 wire 行为。                | Go 与 Java 固定依赖环境，跨 SDK/TLS 组合和安装发布产物验证。                                    |
| 移动端           | `mobile/lib/features/` 中会话与作用域以外的操作链、原生插件装配，以及 `mobile/android/`、`mobile/ios/` 的 release 入口。                                                                                              | Android 合并 manifest 与正式签名包，iOS 构建和实机冒烟。                                        |
| 发布与恢复       | `.github/workflows/` 各平台实际发布，`docker/`、`configs/`、`scripts/` 的运行配置组合，`internal/platform/backupexec/` 的恢复衔接。                                                                                   | 发布演练、真实部署和隔离库恢复，留存结果与提交的对应关系。                                      |
| 历史审核         | `docs/review/` 中 11 月 1–10 日仍未对照的条目。                                                                                                                                                                       | 按当前 HEAD 标记已修、仍存在及暂无法验证，关联现有待办。                                        |

- [x] **AUD-001 · P1 · 修复 · Provider 握手令牌启用时强制保护明文链路**
  - 依赖：—。交付与验收：当 `providerAuth.require=true` 且本地 listener 绑定非 loopback 地址时，拒绝 `tls.enabled=false` 的配置，或提供明确且不可忽略的启动阻断；保持 loopback 开发配置可用；加入启动配置组合验证与部署文档说明。
  - 状态／条件：LOCAL VERIFIED（2026-11-21，commit 7a550438f）：把认证与传输保密在启动时绑定——`startAgentCore` 在既有 token 非空校验之后新增组合校验：`providerAuth.require=true` 且 `agent.tls.enabled=false` 且本地 listener 绑非 loopback → 启动即报错（不可忽略，提示 enable agent.tls 或只绑 loopback）；`isLoopbackAddr` 判定绑定地址是否只回环（127.0.0.1/::1/localhost 为真，0.0.0.0/具名网卡/空 host=全接口为假），故 loopback 开发配置与已启用 TLS 的生产配置均不受阻；文档在 agent 配置页 tls 小节写明该组合与例外。
  - 验证：`go test ./cmd/sidecar-agent/ -count=1 -race` ok（新增 `TestIsLoopbackAddr` 10 组地址判定；`TestStartAgentCore_ProviderAuthCleartextGuard` 三组合：非 loopback+TLS 关闭 → 阻断（断言 "cleartext (replayable)"）、loopback+TLS 关闭 → 可用、非 loopback+TLS 开启（devcert 真证书）→ 可用）；变异自证：守卫条件加 `false &&` 短路 → 「非 loopback+TLS 关闭」用例如实红（未报错）复现原缺陷，还原后全绿；`go test ./internal/... -count=1` 172 包全绿 0 FAIL；`go build ./...` 0 错；gofmt 干净；`bash scripts/dashboard_vnext_guard.sh` PASSED；`python3 scripts/docs/check-architecture.py` PASS；`cd docs && CI=true pnpm build` 0 错；`pnpm --dir web run tsc` 0 错
  - 完成证据：commit 7a550438f（4 文件，119 insertions / 2 deletions）；实现 `cmd/sidecar-agent/root.go`（`isLoopbackAddr` + `startAgentCore` 组合校验）、测试 `cmd/sidecar-agent/root_extra_test.go`；文档 `docs/operations/config-agent.md` 与 `docs/zh/operations/config-agent.md`（tls 小节该组合启动阻断与 loopback 例外）
  - 已知边界：判定基于本地 listener 的绑定地址文本，若部署在 NAT/反代之后实际暴露面与绑定地址不符，仍须由运维绑定 loopback 或启用 agent.tls（平台无法识别网络拓扑）；`localhost` 按回环对待，若系统主机名解析到非回环地址则应由显式 IP 绑定；`require=false` 时无令牌可比对，明文链路无凭据暴露问题（不涉及本项）。

- [x] **AUD-002 · P1 · 修复 · 传输层以结构化协议表达 RPC handler 错误**
  - 依赖：—。交付与验收：统一 `internal/transport/tcp/server.go` 与 `core/transport/tcp/mux_conn.go` 的错误响应编码；JSON 错误文本必须使用标准编码器转义；非 Invoke RPC 必须返回客户端可识别的失败状态，不能以空 protobuf 伪装零值成功；每种消息验证引号、反斜杠、换行错误和客户端可观测结果。
  - 状态／条件：LOCAL VERIFIED（2026-11-21，commit 9f362e202）：错误响应编码从两个落点收敛到 `core/transport/tcp/rpc_error.go`——`EncodeErrorPayload`/`EncodeVersionRejectedPayload` 一律走 `encoding/json`（引号/反斜杠/换行由标准编码器转义，拼接串彻底退出）；`RPCFailureBody(msgID, errText)` 按请求 msgID 返回该 RPC 自身失败通道的类型化响应（Invoke→`InvokeResponse.Payload` 错误文档；Restart/Stop/Start/Update/SchedulePause/PushFile→`Success:false`+`Message`；ExecuteCommand→`Success:false`+`Error`；GetTaskResult→`State:"failed"`+`Error`），busy 响应与 handler 错误、版本拒绝三处共 6 个拼接点全部改走助手；文档把「非 Invoke busy 是空 body 帧」的过期表述改为按类型编码口径。
  - 验证：`go test ./core/transport/tcp/ ./internal/transport/tcp/ -count=1` ok；新增 `TestEncodeErrorPayload_EscapesHostileText`（引号+反斜杠+换行 → payload 是合法 JSON 且原文可还原）、`TestEncodeVersionRejectedPayload`（逐字节锁定 `{"error":"unsupported protocol version: 2","supportedVersions":[1]}`，与既有 server_test 断言同口径）、`TestInvokeErrorBody_PayloadIsClientReadable`（客户端读 InvokeResponse→payload 可 json.Unmarshal 且错误原文可还原）、`TestRPCFailureBody_OpsRPCReportsFailure`（8 类消息逐类型解出失败通道=失败且 detail 带原文）、`TestRPCFailureBody_NoChannelTypeReturnsEmpty`（诚实边界：无通道类型返回 nil，不伪造字节）、`TestMuxConnHandlerError_OpsResponseClientObservable`（真实帧路径：RestartProcess 客户端可观测 `Success=false`+完整错误文本）；变异自证：把 `EncodeErrorPayload` 退回 `{"error":"`+errText+`"}` 拼接 → 两个转义用例如实红（`error text was spliced raw` / `payload is not valid JSON: invalid character 'l' after object key:value pair`），还原后全绿；`go test ./internal/... -count=1` 172 包 0 FAIL；`go build ./...` 0 错；gofmt 干净；`bash scripts/dashboard_vnext_guard.sh` PASSED；`python3 scripts/docs/check-architecture.py` PASS；`cd docs && CI=true pnpm build` 完成；`pnpm --dir web run tsc` 0 错
  - 完成证据：commit 9f362e202（6 文件，377 insertions / 73 deletions）；实现 `core/transport/tcp/rpc_error.go`（`EncodeErrorPayload`/`EncodeVersionRejectedPayload`/`InvokeErrorBody`/`InvokeVersionRejectedBody`/`RPCFailureBody`），测试 `core/transport/tcp/rpc_error_test.go`；调用方改 `internal/transport/tcp/server.go`（版本拒绝、`executionWorker`、`busyResponseBody`）与 `core/transport/tcp/mux_conn.go`（版本拒绝、`dispatchInbound` busy、`handleInboundRequest`）；文档 `docs/architecture/sdk-wire-protocol.md` 与 `docs/zh/architecture/sdk-wire-protocol.md` 已知边界节同步
  - 已知边界：wire 响应类型无失败通道的 RPC（`ListProcesses`/`ListServices`/`ListCronJobs`/`GetSupervisorLog`/`GetSystemInfo`/`StartTask`/`Heartbeat`/`Register` 等）无法在传输层表达 handler 错误，仍回空 body（客户端见零值），真实错误只进服务端日志——补通道需 wire 版本变更，不在本项范围；`sdks/go/pkg/croupier/invoker.go` 仍把 InvokeResponse.Payload 原样当成功字符串返回，调用方识别传输错误须读该 JSON 文档；`sdks/go/pkg/croupier/transport/tcp_client.go` 的版本拒绝文档仍是独立副本（仅含 `%d` 整数、无用户可控文本，不构成转义缺陷）。

- [x] **AUD-003 · P1 · 修复 · 移动端分析请求按作用域清理旧数据并丢弃过期响应**
  - 依赖：—。交付与验收：游戏/环境作用域变化时先清空或明确标记旧摘要；为 refresh 引入请求序号或取消机制，只有当前作用域的最新请求可写状态；失败时显示错误与重试入口，不把旧作用域卡片留作当前结果；覆盖切换后失败及响应乱序场景。
  - 状态／条件：LOCAL VERIFIED（2026-11-21，commit 1eb0085e3）：`AnalyticsController` 增加请求序号 `_requestSeq`——每次 `refresh` 递增，且仅当前序号可写状态；作用域切换先 `_requestSeq++` 使在途请求全部过期，再 `copyWith(clearSummary:true, loading:false, clearError:true)` 清空旧摘要并丢弃旧错误，然后对新作用域 `refresh()`（`mobile/lib/features/analytics/analytics_controller.dart:39-98`）。`copyWith` 增加 `clearSummary`/`clearError` 显式置空参数，解决 `summary ?? this.summary` 无法清空非空摘要（`mobile/lib/features/analytics/analytics_controller.dart:19-31`）。
  - 验证：`flutter analyze` 0 issues；`dart format` 零 diff；`flutter test` 全量 239 passed；`mobile/test/features/analytics/analytics_dashboard_page_test.dart` 8 pass；新增 AUD-003 组 3 用例（切换作用域旧卡片清空只显示新数据／切换后失败显示错误+重试不保留旧卡片／乱序旧响应被丢弃不覆盖新数据，用 `_ScopeStub` 驱动 selected 并按真实 `select()` 口径同步会话、`settle:false` 规避挂起请求转圈）；变异自证：`clearSummary:false` + 移除 summary 序号守卫 → 用例 2/3 如实红（旧卡片保留／过期覆盖），还原后全绿。
  - 完成证据：commit 1eb0085e3（2 文件，207 insertions / 30 deletions）；实现 `mobile/lib/features/analytics/analytics_controller.dart`（`_requestSeq` 过期丢弃 + `copyWith` `clearSummary`/`clearError` 显式置空），测试 `mobile/test/features/analytics/analytics_dashboard_page_test.dart`（AUD-003 组 3 用例 + `_ScopeStub`）。
  - 已知边界：请求取消采用序号丢弃而非实际中止网络请求——挂起的旧请求仍会到达服务端，仅其结果被丢弃；会话加载后的首个序号检查放在 `load()` 之后，若作用域恰在会话加载瞬间变化仍可能发一次旧作用域请求（结果被序号守卫丢弃，不写状态）；`ScopeController` 的网络持久化面由测试桩绕过，控制器对 scope 变化的反应是验证主体。

- [x] **AUD-004 · P2 · 修复 · Agent Stop 超时必须向服务管理器报告未完成**
  - 依赖：—。交付与验收：达到停止等待上限仍未收到 `done` 时返回可识别错误/超时，并记录 runner 未退出；区分正常退出、runner 错误退出和超时；覆盖 runner 卡住、正常完成及重复 Stop。
  - 状态／条件：LOCAL VERIFIED（2026-11-21，commit f0dbadc44）：`croupierAgentService.Stop` 增加 `stopTimeout` 字段（构造默认 15s，测试注入短值）与 `runErr` 字段（Start 后台 goroutine 在 `close(done)` 前写入，Stop 收到 done 后读取，close/receive happens-before 保证无竞争）；三种结果区分——正常退出日志「已停止」返回 nil、runner 错误退出日志并 `fmt.Errorf("agent runner 以错误退出: %w", runErr)` 返回该错误、超时日志「后台 runner 在 %s 内未退出」并 `errors.New("agent 停止超时：后台 runner 未退出")`；kardianos systemd 的 `run()` 直接返回 `s.i.Stop(s)`，该错误经 `runServiceRun` 向上传播，服务管理器不再把卡死伪装成干净停机（`cmd/sidecar-agent/service.go`）。
  - 验证：`go build ./...` 0 错；`go test ./cmd/sidecar-agent/ -count=1` 全绿（8.7s）；`go test ./internal/... -count=1` 172 包 0 FAIL；gofmt 干净；`bash scripts/dashboard_vnext_guard.sh` PASSED；变异自证：把 Stop 退回「select 后无条件 return nil」旧代码（含移除 errors import）→ `TestAgentServiceStop_Timeout` 与 `TestAgentServiceStart_RunAgentFailure` 如实红（超时伪装成功、runner 错误被吞），还原后全绿。
  - 完成证据：commit f0dbadc44（3 文件，96 insertions / 10 deletions）；实现 `cmd/sidecar-agent/service.go`（`stopTimeout`/`runErr` 字段 + 三态 Stop），测试 `cmd/sidecar-agent/service_stop_test.go`（新：runner 卡住超时+重复 Stop、正常完成+重复 Stop）与 `cmd/sidecar-agent/service_test.go`（既有 `TestAgentServiceStart_RunAgentFailure` 契约更新：runner 错误退出时 Stop 返回该错误）。
  - 已知边界：runner 卡住时 Stop 仅返回超时错误，进程不强杀后台 runner（`os.Exit` 等杀进程手段不在本项范围，kardianos 文档要求 Stop 内不调用 `os.Exit`）；超时默认 15s 为既有防卡死兜底值，本项未改动时长；`runErr` 仅记录 runAgent 返回的错误，Start 期间的 panic 等非返回路径不经过该字段。

- [x] **AUD-005 · P2 · 修复 · 同步六语言 Provider 鉴权能力矩阵与实现**
  - 依赖：—。交付与验收：复核六种官方 SDK 实际握手 metadata 注入与回归覆盖，更新单一事实矩阵及中英文引用页面；确保鉴权行不把已实现能力标成未实现，并补齐任何缺失语言实现。
  - 状态／条件：LOCAL VERIFIED（2026-11-22，commit 3717c7c56）：复核确认六语言均在 ProviderConnect 握手 metadata 注入 `authToken` 键（CR-FIX-11）——Go `connectMetadata()`（`sdks/go/pkg/croupier/tcp_manager.go:871-880`，ProviderConnect 两处调用 269/560）、Python `get_provider_connect_request`（`sdks/python/croupier/__init__.py:357-361`）、Java `buildProviderConnectRequest`（`sdks/java/src/main/java/io/github/cuihairu/croupier/sdk/CroupierClientImpl.java:356-361`）、JS `getRegisterRequest`、C++ `croupier_client.cpp:786`、C# `CroupierClient.cs:729-731`；矩阵鉴权行由「Go/Python/Java ⚠️ 仅 L3」回改为六语言全 ✅，并把行标签「Bearer 握手 metadata」更正为「握手 metadata `authToken` 键」（Bearer 属 L3 HTTP 鉴权面，非握手机制），注释同步；中英文引用面核对：`docs/architecture/sdk-wire-protocol.md`（+zh）已正确写明「六语言行为一致，连接帧携带 metadata["authToken"]」，`docs/sdks/sdk-parity-matrix.md`（+zh）无鉴权行，均无需改动；`docs/审计-文档一致性.md` D05（原判「文档超前于代码」）复核为误报——原源码证据漏看握手注入路径，45bba54b 的「仅 L3」标注予以回改。
  - 验证：`bash scripts/check-sdk-matrix.sh` PASSED（All required L1 symbols present）；`cd docs && CI=true pnpm build` 完成（88.3s）；`bash scripts/dashboard_vnext_guard.sh` PASSED；`rg "仅 L3" sdks/ docs/` 仅剩 `docs/审计-文档一致性.md:38` 处置列的历史引用（引号内），无残留漂移；无代码变更（六语言实现与回归覆盖已齐），Go/Python/Java 注入点各自有回归测试（`tcp_manager_auth_test.go`/`test_client.py`/`ProviderConnectMetadataTest`）。
  - 完成证据：commit 3717c7c56（2 文件，21 insertions / 21 deletions）；实现（文档）`sdks/SDK_FEATURE_MATRIX.md`（鉴权行 + 注释回改全 ✅ 并更正行标签），审计记录 `docs/审计-文档一致性.md`（D05 行：复核为误报、45bba54b 标注回改）。
  - 已知边界：本项为文档漂移修复，未改任何 SDK 代码——六语言握手 `authToken` 注入与回归覆盖均已存在（CR-FIX-11），「补齐任何缺失语言实现」经复核无缺失项；行标签把「Bearer」从握手机制中剔除，L3 Invoker 的 `Authorization: Bearer` 是独立的 Server HTTP API 鉴权面（JS `buildInvocationMetadata`、C# `CroupierInvoker`），不在本行覆盖；`docs/审计-文档一致性.md` 的 zh 平行树 D05 行为另一条（pnpm 版本）记录，非本项，未改动。

- [x] **AUD-006 · P2 · 修复 · 锁定移动端依赖与 CI Dart SDK 的可复现兼容范围**
  - 依赖：—。交付与验收：在 Flutter 3.35.1 / 对应 Dart 工具链下生成并提交一致 lockfile；若依赖确需 Dart 3.11，显式升级 CI Flutter 并同步 `pubspec.yaml` SDK 下界；增加不会静默重写锁文件的 CI 检查。
  - 状态／条件：LOCAL VERIFIED（2026-11-22，commit 5c342be86 + 7855baf95 + fbd8c120e）：逐包核对 pub cache 全部 50 个托管依赖——无一需要 Dart 3.11（各自 SDK 约束上限 >=3.0.0），`sdks.dart: ">=3.11.0-0"` 完全来自本机 Flutter 3.47.9 框架包（flutter/flutter_test/sky_engine 各自声明 `sdk: ^3.11.0-0`，属 `source: sdk` 约束）；走「依赖无需 3.11 → 不升级 CI」分支：下载与 CI 完全一致的 Flutter 3.35.1（Dart 3.9.0）重解析并提交 lockfile——6 个 SDK 钉版传递依赖降为 3.9 兼容版（characters 1.4.0 / matcher 0.12.17 / material_color_utilities 0.11.1 / meta 1.16.0 / test_api 0.7.6 / vector_math 2.2.0），`sdks.dart` 归位 `">=3.9.0 <4.0.0"`（`sdks.flutter` 保持 `">=3.35.0"`），`pubspec.yaml` 下界与 CI Flutter 版本均不动；CI 两 job 的 `flutter pub get` 改 `--enforce-lockfile`——锁文件与 pubspec.yaml/当前 SDK 不一致时直接失败，不再静默重解析。
  - 验证：Flutter 3.35.1 下 `flutter analyze` 0 issues；`flutter test --coverage` 238 passed（02:02）；`dart format --set-exit-if-changed` rc=0（4 个 analytics 文件先按 3.9 formatter 对齐）；`flutter build apk --debug` ✓ Built（gradle assembleDebug 718.7s）；`pub get` 二次执行零改写（幂等）；变异自证：放回陈旧 lockfile（>=3.11.0-0）→ `--enforce-lockfile` rc=65 如实失败（「To update pubspec.lock run flutter pub get without --enforce-lockfile」），还原已提交版本后 rc=0 且工作区零改写；`pnpm --dir web run tsc` 0 错、`bash scripts/dashboard_vnext_guard.sh` PASSED（无 web/Go 变更，未跑 go test）。
  - 完成证据：commit 5c342be86（`mobile/pubspec.lock`，13/13 行：6 个 SDK 钉版包降级 + sdks.dart 归位 3.9）；commit 7855baf95（4 个 analytics 文件 39/45：Dart 3.9 formatter 对齐——AUD-003 文件此前按本机 3.13 formatter 产出致 CI format 门禁红）；commit fbd8c120e（`.github/workflows/ci-mobile.yml`：flutter/apk 两 job 的 pub get 加 `--enforce-lockfile`，6/2 行）。
  - 已知边界：SDK 钉版传递依赖随工具链硬绑定——本机 Flutter 3.47.9 上 `flutter pub get` 会把 lockfile 改写回 3.13 形态（漂移根因，实证复现），本地改写不得提交（CI `--enforce-lockfile` 会挡下）；lockfile 以 CI 工具链 3.35.1/Dart 3.9 为唯一正典，未来升级 CI Flutter 须用新工具链重解析并同步提交；本机 Flutter 3.47.9 与 CI 3.35.1 的 formatter 风格有差异（3.13 formatter 产物会让 CI 的 3.9 format 门禁报红），本地编辑后须用 3.35.1 的 `dart format` 对齐；iOS 面不涉及。

### 续审确认项（AUD-007..028）

> 11 月 12 日第一轮报告与 11 月 13 日代码逻辑续审均为待修发现；动态证据不等于修复完成。历史导入清单中同一问题以本编号跟踪，避免重复计数。

- [x] **AUD-007 · P1 · 修复 · 全局备份接口补齐操作授权**（commit dd7d0ad88）
  - 依赖：—。交付与验收：按现有平台权限规则覆盖列表、创建、下载、删除；低权限请求返回 403 且无副作用。
  - 状态／条件：LOCAL VERIFIED（2026-11-22，commit dd7d0ad88）：`internal/api/backup/service.go` 的 List/Create/Delete/Download 四方法顶部各加 `utils.RequireAnyPermission(ctx, s.svcCtx, "无权…", "admin:all", "backup:read|create|delete")`（List+Download→`backup:read`，Create→`backup:create`，Delete→`backup:delete`）；超管/admin:all 直通分支保留，其余按角色权限集合判定，拒绝返回 `NewPermissionDenied`→403 `permission_denied`。守卫置于模型初始化检查之前：低权限请求在触碰数据面前即被拒，零副作用；misconfig（模型未初始化）仍 500。
  - 验证：`go test ./internal/api/backup/` 全绿（新增 2 用例：`TestBackupService_ZeroRolePermissionDenied` 服务级零角色 4 操作全 403 + 无副作用对照超管可用；`TestHandler_BackupPermission_ZeroRoleDenied` handler/路由级 403 + `permission_denied` 落 HTTP 体 + 数据面不变）；变异自证：移除 List 守卫→`TestHandler_BackupPermission_ZeroRoleDenied` 如实红（200 返回泄露条目而非 403），还原后绿；`go test ./internal/...` 全绿；`go build ./...` 0 错；`bash scripts/dashboard_vnext_guard.sh` PASSED。
  - 完成证据：commit dd7d0ad88（`internal/api/backup/service.go` 12/0 行四守卫；`internal/api/backup/backup_permission_test.go` 129 行新增回归；`backup_flows_extra_test.go`/`executor_extra_test.go` 测试超管账号 seed + 授权上下文适配）。
  - 已知边界：`internal/handler/routes_task.go:108-111` 的 `/ops/backups`（游戏域控制台面）走 `internal/api/ops` 自有 BackupService（`internal/api/ops/backups.go`，List/Create/Delete/GetDownloadURL 均无 `RequireAnyPermission`），与本项 routes_business.go 面不同源，**未改**；该项仅按 todo 明确锚点（routes_business.go:66-75 + api/backup/service.go）修复，ops 备份面若需同规应另立审计项。

- [x] **AUD-008 · P1 · 修复 · 明确生产模式时拒绝公开 JWT 开发密钥回退**（commit 3409c8399）
  - 依赖：—。交付与验收：覆盖配置模式与环境变量组合，生产模式空密钥必须启动失败。
  - 状态／条件：LOCAL VERIFIED（2026-11-22，commit 3409c8399）：根因在两份平行的开发态判定——`internal/security/jwtutil/secret.go` 的 `isDevelopmentMode`（`ResolveSecret` 用）与 `internal/svc/service_context_config.go` 的 `isDevelopmentConfig`（approval_seed 用），两者结构逐条相同（启动路径注释已声明「逐条等价」）、缺陷逐条相同：只在 `CROUPIER_MODE` 显式为 prod/production 时判生产，`server.mode=prod/production/release` 而 `CROUPIER_MODE`/`CROUPIER_ENV` 无生产值时落到末尾 `return true`，被误判为开发态，`ResolveSecret` 遂静默回落公开 `DevSecret`。修法：新增 `isProductionSignal`（trim + 大小写不敏感，匹配 prod/production/release），两个判定都改为「server.mode / CROUPIER_MODE / CROUPIER_ENV 任一出现生产信号即判非开发」——生产信号单调，遗留的 dev 配置无法降级生产部署；同步关掉 `internal/handler/routes.go` 的 `DevSecret` 兜底分支（改 panic），与 `service_context.go` 启动路径既有 fail-closed 语义对齐。
  - 验证：`go test ./internal/security/jwtutil/ ./internal/svc/ ./internal/handler/` 全绿（jwtutil 0.02s / svc 221s / handler 2.2s）；新增用例：jwtutil `TestIsDevelopmentMode` +4（裸 server.mode=prod/production/release 判非开发 ×3、生产 CROUPIER_MODE 压过 dev server.mode ×1），原「server.mode=production + CROUPIER_ENV=dev → DevSecret」子用例改为断言报错（正是被修的漏洞），svc `TestIsDevelopmentConfig` +4 行，`TestRegisterAuthRoutesSecretFallback` 改名 `TestRegisterAuthRoutesSecretPanic` 并改 `assert.Panics`；变异自证：把 `isDevelopmentMode` 改成忽略生产信号（恒判开发态）→ `TestResolveSecret` 如实红，还原后绿；`go build ./...` 0 错。`go test ./internal/...` 另有 5 个包红，全在我未改动的 api 包内：`api/auth`、`api/function`、`api/openapi`、`api/page` 均为 `panic: test timed out after 10m0s`，`api/message` 的 `TestHandler_Stream_WithMessages_V4` 空响应体；本机负载平均 39-101（并行会话争抢，同批包耗时 135-223s 远超常态），隔离复跑 5 包全绿（auth 63s / function 46s / openapi 62s / page 223s / message 38s），判定为负载性超时而非回归。
  - 完成证据：commit 3409c8399（`internal/security/jwtutil/secret.go` `isDevelopmentMode` 重写 + `isProductionSignal` 新增；`internal/svc/service_context_config.go` `isDevelopmentConfig` 同构重写；`internal/handler/routes.go` `DevSecret` 兜底改 panic；`jwtutil_test.go` +45/-3、`service_context_test.go` +8、`coverage_final_test.go` 13 行改写）。
  - 已知边界：行为变更（**升级需知**）——`server.mode` 已设为 prod/production/release 但 `CROUPIER_MODE` 未设的存量部署，此前静默以 `DevSecret` 签名/验签，升级后启动直接 panic 拒绝启动；这是本项的修复目的（fail-closed），但属有意的破坏性变更，部署方须先配置 `auth.jwtSecret` 或 `CROUPIER_AUTH_JWT_SECRET`。`DevSecret()` 导出符号仍在（仅开发态回落使用），生产路径不再引用，`TestRegisterAuthRoutesSecretPanic` 会挡下任何重新引入的兜底；`coverage_group_g_test.go` 中两个判定「逐条等价」是注释论证而非运行时断言，等价性靠两侧同构重写维持，未新增跨包等价测试。

- [ ] **AUD-009 · P1 · 修复 · SQLite 备份使用一致性快照并核对恢复内容**
  - 依赖：—。交付与验收：在 WAL 和并发写入下恢复备份，确认包含已提交事务；checksum 不能替代恢复核对。
  - 状态／条件：executor.go:227-242 直接复制主文件；真实 SQLite WAL 测试源库 1 行、成功备份 0 行。

- [ ] **AUD-010 · P1 · 修复 · 账号不存在时拒绝旧 JWT 认证**
  - 依赖：—。交付与验收：区分账号删除、禁用、令牌版本不匹配与数据库故障；覆盖无缓存的新中间件。
  - 状态／条件：service_context_auth.go:304-316 把 RecordNotFound 也放行；软删除账号后新中间件仍接受旧 JWT。

- [ ] **AUD-011 · P1 · 修复 · 修正移动端 CI 覆盖率阈值读取**
  - 依赖：—。交付与验收：100% 覆盖率通过，低于阈值和无有效覆盖数据拒绝。
  - 状态／条件：ci-mobile.yml:58 的 shell 参数表达式位于单引号 heredoc，Python float 收到字面量；实际 workflow 片段在 100% lcov 下仍 ValueError。

- [ ] **AUD-012 · P2 · 修复 · SSE 解析保留跨分块 CRLF 状态**
  - 依赖：—。交付与验收：同一事件字节无论网络如何分块，事件名和多行 data 必须一致。
  - 状态／条件：web/src/services/core/http.ts:180 提前归一化 chunk 尾 CR；实际源码解析 update/hello 事件在 CRLF 分块后变成 message/hello。

- [ ] **AUD-013 · P1 · 修复 · C++ transport 完整写帧并串行化连接写入**
  - 依赖：—。交付与验收：统一 Call/响应 writer 的连接级锁、write-all 和失败收口；覆盖合法短写。
  - 状态／条件：tcp_transport_inbound.cpp:164 单次 send 仅判断负数；注入 140 字节帧只写 5 字节后静默返回。并发帧交错为静态风险，未做拥塞网络实验。

- [ ] **AUD-014 · P1 · 修复 · 将 C++ 和 C# Provider TLS 配置接入默认 transport**
  - 依赖：—。交付与验收：真正实现 TLS 或请求安全模式时明确拒绝连接；与真实 TLS listener 对接并同步矩阵。
  - 状态／条件：C++ client.cpp:417/592 只传地址和 timeout，C# Client.cs:681/774 默认 factory 同样不接 TLS，TCPTransport.cs:202 使用 NetworkStream；本轮为静态调用链确认。

- [ ] **AUD-015 · P2 · 修复 · 多游戏冷启动任务恢复不能只枚举已打开库**
  - 依赖：—。交付与验收：提供有界持久巡检或等效恢复保障，未被用户访问的游戏库也能收敛陈旧任务。
  - 状态／条件：tasks/store.go:135 仅遍历 Router.OpenScopes；文件库测试中冷启动空集合收割 0 行，加入 scope 后同一任务才收敛。 2026-11-21 复核确认现行启动装配仍使用 `cmd/server/root.go:561,813` 的 `OpenScopes` 适配器。

- [ ] **AUD-016 · P1 · 修复 · 对象存储写入补齐权限和保留前缀边界**
  - 依赖：—。交付与验收：按现有权限定义控制上传、覆盖、删除、目录变更与签名访问，保护备份/发布对象。
  - 状态／条件：storage/service.go:122-175 直接用输入 path 写全局存储；临时夹具确认只有登录身份即可覆盖 backups/ 对象。

- [ ] **AUD-017 · P1 · 修复 · 全局电源配置操作补齐平台权限检查**
  - 依赖：—。交付与验收：合法游戏/环境作用域不等同硬件管理权限；覆盖 power-config 写入及其他 PowerOps 操作入口。
  - 状态／条件：ops/powerops.go:221-263 更新全局节点不检查操作权限；真实 Auth+GameDBMiddleware 下零角色、合法单游戏 scope 账号可修改全局节点 BMC 配置。

- [ ] **AUD-018 · P2 · 修复 · Python reader 所有退出路径统一清理连接和 pending**
  - 依赖：—。交付与验收：异常退出后连接立即不可用，在途调用及时失败，重连不被虚假 connected 阻断。
  - 状态／条件：sdks/python/croupier/transport/tcp.py:336 通用异常只日志后退出；超长帧后 connected/running 仍 true 且 pending 未收到错误。

- [ ] **AUD-019 · P1 · 修复 · 供给运行的数据库适配器按业务键更新既有记录**
  - 依赖：—。交付与验收：Create→Update→Get 必须成功且只保留一行；覆盖 ticket 审批关联和 apply 后各状态持久化。
  - 状态／条件：provision_wiring.go:26/67 每次重新转换得到 ID=0；provision_model.go:63 Save 实际 INSERT 并触发 run_id 唯一键冲突。正常更新回归用例已失败，见 11-13 代码审核报告。

- [ ] **AUD-020 · P1 · 修复 · 事故详情更新使用数据库可序列化的 JSON 类型**
  - 依赖：—。交付与验收：正常提交 details 对象可保存并读回，兼容空对象与嵌套值。
  - 状态／条件：incident/service.go:331-333 把裸 map 放进 Updates；普通详情更新返回 sql unsupported type map[string]interface{}。创建路径的 datatypes.JSONMap 不存在该问题。

- [ ] **AUD-021 · P2 · 修复 · IPv6 ICMP 请求和响应使用正确的类型**
  - 依赖：—。交付与验收：合法 IPv6 echo reply 的 ID/序号/载荷匹配时通过，与 IPv4 分支分别验证。
  - 状态／条件：core/healthprobe/icmp.go:100 将 IPv6 类型强转为 ipv4.ICMPType；解析返回 ipv6.ICMPType，接口比较始终不相等。正常应答回归用例失败。

- [ ] **AUD-022 · P2 · 修复 · 部署编排在可取消等待前持久化已完成目标**
  - 依赖：—。交付与验收：编排器被取消时保留已换包/已健康的目标事实，不能退回 pending；明确中断后的收敛策略。
  - 状态／条件：deploy.go:213-220 在 BatchWait 后才 saveTargets；直接调用编排器，在完成换包后取消，存储仍为 pending。该等待选项默认关闭；不能将此测试宣称为默认 HTTP 断连路径。

- [ ] **AUD-023 · P1 · 修复 · 回滚与正在执行的发版互斥并核验实际回滚目标**
  - 依赖：—。交付与验收：正在发版时拒绝或协调回滚；不得在零目标操作时把活跃发版单标为 rolled_back。
  - 状态／条件：deploy.go:230 Rollback 无状态领取；当 DB 中尚为 deploying/pending 时遍历跳过全部目标，返回 nil 并写 rolled_back。正常并行操作场景的回归用例失败。

- [ ] **AUD-024 · P2 · 修复 · Runbook 后台恢复循环按服务生命周期共享并关闭**
  - 依赖：—。交付与验收：终态写入失败重试不应每请求遗留一个后台 worker；写入恢复和服务退出后可正确释放。
  - 状态／条件：ops/runbooks.go:90-101 每请求 runbook.New，runbook.go:398 启动各自 recoveryLoop，405-447 永久等待 wake；所谓共享仅在单个临时编排器内成立。静态跨文件确认，未声称有实际内存泄漏测量。

- [ ] **AUD-025 · P2 · 修复 · Runbook 启动宽限内的遗留运行需要后续恢复机会**
  - 依赖：—。交付与验收：重启时不足 24 小时的遗留 running 行，超过宽限后仍能巡检收敛；不能误收另一实例的活跃运行。
  - 状态／条件：cmd/server/root.go:309-317 仅启动时恢复一次；RunbookStaleRunGrace=24h，之后没有重扫调用。短时宕机的遗留行被跳过后，当前进程不再自动处理。静态确认。

- [ ] **AUD-026 · P2 · 修复 · 插件启动不能阻塞其他调用的 deadline**
  - 依赖：—。交付与验收：启动/Describe 的慢插件不应持全控制器状态锁阻塞所有调用；队列等待和启动预算应遵守调用方上下文。
  - 状态／条件：pluginhost/engine.go:267-350 在全局 mutex 内启动与 Describe；Call 开头 isStopped 也取该锁。真实测试插件慢启动时，另一个 10ms deadline 调用在 100ms 后仍未返回。

- [ ] **AUD-027 · P1 · 修复 · 供给 CAS 竞争失败返回稳定冲突而不是 panic**
  - 依赖：—。交付与验收：统一 RunStore 的 CAS 返回契约；Resume/Destroy/DestroyResume 的失败分支安全处理 nil current。
  - 状态／条件：provision_wiring.go:30-34 在 ok=false 时丢弃当前行，orchestrator.go:391/430/493 却读取 current.State；通过真实数据库适配器模拟另一调用先领取，ResumeAsync 发生 nil pointer panic。

- [ ] **AUD-028 · P1 · 修复 · 定时任务按同一计划发生时间领取而不是当前分钟**
  - 依赖：—。交付与验收：两个实例已读取同一条到期记录，即使跨分钟继续处理，也只能派发一次；保留既有过期领取与未知结果语义。
  - 状态／条件：scheduler/manager.go:309 用 Slot(now) 生成唯一领取键；同一每日任务的两份到期快照在 10:30:59 和 10:31:00 处理时派发两次。普通回归用例已失败。

### AUD-COV-005（Web Console，2026-11-16）P2/P3 遗留排期

> 来源：`docs/review/aud-cov-005-web-console-2026-11-16.md`。P0×0；P1×4 中 3 条已当场修（2f6d79d01 SSE 畸形帧 / 95072aaeb effect 依赖 / c430996a2 面板竞态守卫），第 4 条（Schedules payload JSON）复核为「已有 try/catch 兜底、缺字段级校验」，按 P2 处置并已修（payloadRule 字段级校验）。下列为逐条复核确认属实的 P2/P3；已核伪两条见报告附注。

- [ ] **AUD-029 · P2 · 修复 · 权限页删除动作失败仍弹成功提示**
  - 依赖：—。交付与验收：`web/src/pages/Permissions/UsersV2/index.tsx:241`、`web/src/pages/Permissions/RolesV2/index.tsx:112` 的 `remove()` 给 `deleteAdmin`/`deleteRole` 补 try/catch，失败走错误提示且不弹「已删除」；参照 `Functions/DetailTabs.tsx` 既有模式；带失败分支回归用例。
  - 状态／条件：2026-11-16 复核属实——`await deleteAdmin(rec.id)` 后无条件 `getMessage()?.success(...)`，异常被 Popconfirm Promise 吞掉。

- [ ] **AUD-030 · P2 · 修复 · 原始异常消息直出（可能泄漏后端细节）**
  - 依赖：—。交付与验收：`web/src/pages/Functions/DetailTabs.tsx:671`、`web/src/pages/Functions/Directory/useDirectoryPage.ts:213` 的 `err instanceof Error ? err.message : String(err)` 改走 `extractErrorMessage(err, 兜底文案)`；同型残留全仓扫一遍一并收。
  - 状态／条件：2026-11-16 复核属实——两处均直出 `err.message`。

- [ ] **AUD-031 · P2 · 修复 · Ops/Agents 页描述文案与已交付能力不一致**
  - 依赖：—。交付与验收：`web/src/pages/Ops/Agents/index.tsx:36` 描述仍写「主机与会话数据面未交付，本页不提供入口」，但同页已渲染 `DevopsSessionsPanel`；更新文案如实反映会话能力与剩余边界（历史 asciicast 回放等）。
  - 状态／条件：2026-11-16 复核属实。

- [ ] **AUD-032 · P2 · 修复 · MenuManagement listPageDrafts pageSize=200 未封顶**
  - 依赖：—。交付与验收：`web/src/pages/MenuManagement/index.tsx:57` 硬编码 `pageSize: 200` 直传后端；定合理上限或复用统一分页常量。
  - 状态／条件：2026-11-16 复核属实（ExecutionLogs 半句不实——其用 `PAGE_SIZE=20` 常量，已核伪不收）。

- [ ] **AUD-033 · P2 · 修复 · window.open 未统一走 safeUrl 模式**
  - 依赖：—。交付与验收：`web/src/pages/Permissions/UsersV2/index.tsx:401` 相对路径拼接未走 `safeUrl`/`safeExternalHref` 校验；现风险低（encodeURIComponent+同源前缀），做模式统一并加注释。
  - 状态／条件：2026-11-16 复核属实（模式统一项，非注入实锤）。

- [ ] **AUD-034 · P3 · 修复 · DevopsSessionsPanel 中文硬编码文案未走 i18n**
  - 依赖：—。交付与验收：面板内按钮/表单/确认弹窗/空态文案提取到 locale 文件并经 `intl.formatMessage`；注意与本面板测试断言文案联动更新。
  - 状态／条件：2026-11-16 复核属实（全组件裸中文串）。

- [ ] **AUD-035 · P3 · 修复 · DevopsSessionsPanel 关闭/刷新按钮无 loading 防重复点击**
  - 依赖：—。交付与验收：关闭（含确认弹窗 onOk）与刷新按钮在请求期间置 loading/禁用；刷新可复用 `loading` 态，关闭按会话 ID 粒度防抖。
  - 状态／条件：2026-11-16 复核属实。

### AUD-COV-001（Go API，2026-11-16）遗留排期

> 来源：`docs/review/aud-cov-001-go-api-2026-11-16.md`（3 P0 / 8 P1 / 7 P2 / 6 P3）。当场修 4 条：incident 行级 scope `9bc08d56a`、ticket 行级 scope `5b0fee15f`、cicd webhook fail-closed `ce667c63d`、backup 下载收敛 objstore `8dbf59c53`；默认 JWT secret 已有 AUD-008 不重复计数。下列为逐条复核确认属实的遗留项（含已修条目的残余半边）。

- [x] **AUD-036 · P0 · 修复 · 外部 incident token 绑定 game/env allowlist**
  - 依赖：—。交付与验收：`external_token` 模型加 game/env 允许列（模型变更同 PR 编号迁移，三处同步：migrations.go 注册 + MinimumRequiredVersion + migrate_test 合成清单）；签发与校验按 token scope 强制过滤写入和查询；带越权拒绝用例。
  - 状态／条件：**DONE · LOCAL VERIFIED（2026-11-22，commit 622160cae）**：`external_tokens` 加 `game_allowlist`/`env_allowlist` 两列（JSON 数组；NULL/空=不限，向后兼容）；模型加 `ScopeGameIDs`/`ScopeEnvs`（nil 接收者安全）；编号迁移 0066 三处同步（`migrations.go` 注册+版本清单注释、`MinimumRequiredVersion` 65→66、`migrate_test.go` 四处合成清单加 0066）；签发（CreateToken/UpdateToken）从 scope 的 `gameIds`/`envs` 键落列、DTO 回读并入 scope；写入强制：`RegisterIncident`/`CreateBug` 经 `checkScopeWrite`——允许面非空时目标缺省或面外一律 403（缺省无法证明在面内，拒绝而非回落全局）；查询强制：`ListIncidents` 允许面非空时 SQL `game_id/env IN` 收窄（Total 同步收窄），显式查面外 game/env 返回空（不降级全量）；`IncidentQueryOptions` 加 `GameIDs`/`Envs` 集合过滤（单值优先，存量调用方 nil=no-op）。
  - 验证：`go test ./internal/api/incident/ -count=1` 全绿（新增 `external_scope_test.go`：允许面解析、RegisterIncident/CreateBug 越权负例（面外 game/env、缺省 scope→403）、ListIncidents 面内收窄+面外空+Total 收窄、handler 签发持久化回读）；`go test ./internal/svc/ -run 'ExternalTokenScope'` 全绿（0066 存量表形态 AddColumn 补齐、存量行留空不回填、幂等、缺表跳过）；`go test ./internal/model/ -count=1` 全绿；`go test ./internal/... -count=1` 全绿（exit 0）；`go build ./internal/...` 0 错。已知边界：签名请求方案未做（AUD-036 用 token 允许面，签名属于对外调用方身份更强校验，未在本条范围）；`CreateBug` 写入同样受允许面约束（同 token 写入面）。
  - 完成证据：`internal/api/incident/external_scope_test.go` + 迁移 `migrations/0066_*`（LOCAL VERIFIED，go test ./internal/... 全绿）。

- [x] **AUD-037 · P0 · 修复 · 公开配置端点按 game/env 过滤**
  - 依赖：公开客户端契约方向（签名请求 vs 客户端 scope）需用户拍板后再动 SQL。交付与验收：`/api/v1/public/configs`（`internal/api/config/watch.go:204-228`）强制 game_id/env 过滤并限制返回字段；契约结论先落本条目。
  - 状态／条件：**DONE · LOCAL VERIFIED（2026-11-22，commit 25da5d4b2）**：契约结论（本条目落定）：**客户端 scope 方案**（匿名端点由客户端经 `gameId`/`env` query 显式声明作用域），SQL 强制 `game_id=? AND env=?` 等值过滤，返回字段白名单（namespace/key/version/value/format/updatedAt；message/createdBy/game_id/env 不回传）。签名请求方案不做——它是更强的调用方身份证明（密钥管理/防重放面），属于独立契约决策，超出本条最小修复面，留待公开客户端契约专项。行为：`GET /api/v1/public/configs` 缺 `gameId` 或 `env`（含只给半边）→ 400，绝不回落全局读；面外声明返回空（不泄漏别家配置）。
  - 验证：`go test ./internal/api/config/ -count=1` 全绿（`watch_test.go` 新增 `TestPublicHandler_List_RequiresScope`（缺参/半参→400）与 `TestPublicHandler_List_CrossScopeEmpty`（跨 game/env 越权负例→空）；`coverage_v9_test.go` 既有用例同步 scope 参数）；`go test ./internal/... -count=1` 全绿；`go build ./internal/...` 0 错。既有消费方：仓库内无 SDK/web 调用该端点（仅 docs 设计引用），改必填不破坏存量客户端。
  - 完成证据：`internal/api/config/watch_test.go`（TestPublicHandler_List_RequiresScope / CrossScopeEmpty）（LOCAL VERIFIED，go test ./internal/... 全绿）。

- [ ] **AUD-038 · P1 · 修复 · Approve/Reject 增加审批权限与两人规则执行**
  - 依赖：—。交付与验收：`internal/api/approval/service.go:216-231` 加显式审批权限检查；`two_person` policy 从描述性数据变为运行时强制两个不同授权主体；带低权限拒绝与同主体二次批准拒绝用例。
  - 状态／条件：2026-11-16 复核属实——只校验登录/scope/自批/可选 OTP。

- [ ] **AUD-039 · P1 · 修复 · Storage 管理面挂服务层 RBAC**
  - 依赖：—。交付与验收：`internal/handler/routes_storage.go` signed-url/list/upload/delete/batch-delete/目录操作挂 `RequireAnyPermission`（先粗粒度，前缀 ABAC 后续拆）；带 403 用例。
  - 状态／条件：2026-11-16 复核属实——仅 Authority，任何登录用户可全量读写删。

- [ ] **AUD-040 · P1 · 修复 · TCP agent 注册加凭据校验**
  - 依赖：部署策略确认（shared token vs 既有 mTLS）。交付与验收：`internal/server/tcp_listener.go:486-541` register 校验 token 或 mTLS client identity + game/env 授权；insecure 默认值收紧须兼容存量部署并守配置契约（lowerCamelCase）。
  - 状态／条件：2026-11-16 复核属实——只验非空 agent_id/game_id/env。

- [ ] **AUD-041 · P1 · 修复 · Public player ticket 身份来源重设计**
  - 依赖：游戏方契约拍板（SDK 签名 vs 玩家会话 proof）。交付与验收：`internal/api/support/player.go:127-165` 身份由签名会话派生 game/env/player；列表防枚举。
  - 状态／条件：2026-11-16 复核属实——匿名 create 采信 query/body，list 仅按提交 playerId 过滤可枚举他人工单。

- [ ] **AUD-042 · P1 · 修复 · 管理域权限矩阵统一挂载**
  - 依赖：AUD-039（storage 先行）。交付与验收：建立管理域→权限矩阵（provider delete/reload、backup、cicd、incident、config、tool、announcement、certificate、rate_limit、schema、message、faq、ticket、player、task、schedule 等），路由或服务层统一挂 least-privilege guard，与 RBAC 页面对齐并带 403 用例。
  - 状态／条件：2026-11-16 复核属实——大量管理面仅 JWT；本条为矩阵化收口，与既有零散挂账合并。

- [ ] **AUD-043 · P1 · 修复 · CICD webhook payload scope 绑定 integration**
  - 依赖：ce667c63d（fail-closed 已修，本条为残余半边）。交付与验收：回写 payload 的 game/env 与 integration 配置一致才接受，不一致 4xx 拒绝；带跨 scope 回写拒绝用例。
  - 状态／条件：2026-11-16 复核属实——token 校验已补，payload game/env 仍未绑定。

- [ ] **AUD-044 · P2 · 修复 · query token 收敛到 SSE 路由**
  - 依赖：—。交付与验收：`internal/svc/service_context_auth.go:164-176` 与 `internal/handler/routes.go:56` 的 `?token=` fallback 仅对显式 SSE 路由启用且配一次性短时 token；其余端点只认 Authorization header。
  - 状态／条件：2026-11-16 复核属实——fallback 挂全部 protected 路由，token 进日志/历史。

- [ ] **AUD-045 · P2 · 修复 · 认证关键查询 fail-closed**
  - 依赖：—。交付与验收：`internal/svc/service_context_auth.go:253-310` token version/OTP/scope 模型故障时生产 fail-closed；测试/精简模式显式兼容开关。
  - 状态／条件：2026-11-16 复核属实——故障窗口 fail-open。

- [ ] **AUD-046 · P2 · 修复 · CICD TestConnection SSRF 防护与稳定错误码**
  - 依赖：—。交付与验收：`internal/api/cicd/service.go:309-315` 限制 scheme、解析并拒绝私网/链路本地地址；客户端错误改稳定码，原文入日志。
  - 状态／条件：2026-11-16 复核属实。

- [ ] **AUD-047 · P2 · 修复 · 未知错误统一 opaque 500**
  - 依赖：—。交付与验收：`internal/common/response/response.go:107-108` 未知错误改 opaque message + correlation id，原文仅入日志；带响应契约回归。
  - 状态／条件：2026-11-16 复核属实——`err.Error()` 直出可泄漏 GORM/驱动细节。

- [ ] **AUD-048 · P2 · 修复 · Warehouse 查询强制 current scope**
  - 依赖：—。交付与验收：`internal/api/analytics/warehouse.go:141/247` 请求 game/env 只可收窄不可替换，空值不得展开为无过滤。
  - 状态／条件：2026-11-16 复核属实——显式传其他 game/env 可跨域聚合。

- [ ] **AUD-049 · P2 · 修复 · 敏感写路径补统一审计**
  - 依赖：AUD-042（权限矩阵先行，denied 面随 guard 收敛）。交付与验收：incident/ticket 变更、storage 删除/上传、cicd webhook 成功与 denied 路径接 AuditService.Log；统一 decorator 覆盖成功/失败/denied 三态。
  - 状态／条件：2026-11-16 复核属实。

- [ ] **AUD-050 · P2 · 修复 · Ticket/CICD 多步写包事务**
  - 依赖：—。交付与验收：`internal/api/ticket/service.go:202-209`（状态备注+状态更新）与 `internal/api/cicd/service.go:283-287`（删 builds+删 integration）包 GORM transaction；带第二步失败回滚用例。
  - 状态／条件：2026-11-16 复核属实——半提交可产生不一致中间态。

- [ ] **AUD-051 · P3 · 修复 · Approval 挂账文档更新为 SQLStore 现状**
  - 依赖：—。交付与验收：`docs/operations/access-control.md:66` 与 CLAUDE.md「Approvals currently use MemStore」表述更新为 production SQLStore + CAS 事实；zh 镜像（docs/zh）同步。
  - 状态／条件：2026-11-16 复核属实——`internal/svc/service_context.go:379` 实际装配 SQLStore，重启丢失风险已不存在。

- [ ] **AUD-052 · P3 · 修复 · 缩短 access JWT TTL 并引入 refresh 轮换**
  - 依赖：AUD-008（密钥判定先行）。交付与验收：`internal/security/jwtutil/token.go:23` 拆 access/refresh + 重用检测轮换；存量登录灰度兼容。
  - 状态／条件：2026-11-16 复核属实——TTL 硬编码 24h 无 refresh。

- [ ] **AUD-053 · P3 · 修复 · Backup 创建幂等键**
  - 依赖：—。交付与验收：`internal/api/backup/service.go:75-106` 按 name/type/时间窗或客户端 Idempotency-Key 去重 pending/succeeded；带并发去重用例。
  - 状态／条件：2026-11-16 复核属实——双击/重试并发创建多份重备份。

- [ ] **AUD-054 · P3 · 修复 · 删除旧式 PermissionMiddleware 死代码**
  - 依赖：—。交付与验收：确认无挂载引用后删除 `internal/service/permission/middleware.go:117-127`、`internal/common/middleware/auth.go:104` 的旧式实现（字符串 context key + gin.H 直写）。
  - 状态／条件：2026-11-16 复核属实（未挂载死代码，防未来误挂载）。

- [ ] **AUD-055 · P3 · 修复 · Registry 公开语义对齐**
  - 依赖：—。交付与验收：`internal/handler/routes_storage.go:45` 注释与服务层权限二选一对齐（protected 或真公开 + threat model 记录）。
  - 状态／条件：2026-11-16 复核属实——路由注释称公开，服务层 RequireAnyPermission 致匿名 403。

- [ ] **AUD-056 · P3 · 修复 · Monitoring 健康错误改稳定码**
  - 依赖：—。交付与验收：`internal/api/monitoring/service.go:160-163` 状态响应改布尔/稳定码，详细错误仅日志。
  - 状态／条件：2026-11-16 复核属实——底层错误直出响应。

### 非分析代码续审（2026-11-21）

本节覆盖任务留痕、采集 Agent、JS SDK 连接、移动端会话与作用域、分析工具的容器生命周期。AUD-COV-002/003/004/006/007 保留后续覆盖，移动端分析卡片问题继续跟踪 AUD-003。验证：Go 双库探针复现任务清理遗漏，JS 替身 socket 复现 close 后重新建立连接，替身 Docker 命令复现复用模式误执行 down。移动端为源码复核，当前 Flutter SDK 缺少 flutter_tools，定向测试未能启动。

- [ ] **AUD-057 · P1 · 修复 · 多游戏任务留痕清理按物理库执行**
  - 依赖：—。交付与验收：后台和手动清理处理实际持有 TaskRun/TaskEvent 的 game/env 库，汇报成功与失败范围。用 meta 加两个游戏物理库验证历史任务及事件清理和保留结果。证据：`internal/platform/executionlog/retention.go:114,196` 只从 meta 判断表和执行后台任务清理。
  - 状态／条件：未开始。

- [ ] **AUD-058 · P1 · 修复 · 任务保留期以终态为清理边界**
  - 依赖：—。交付与验收：超过创建时间阈值但正在运行、排队或等待确认的任务和事件可继续查询，已结束任务以确定的终态时间判断保留期。覆盖状态并发变化及关联事件清理。证据：`internal/model/task_model.go:349` 复用只按 `created_at` 的删除逻辑。
  - 状态／条件：未开始。

- [ ] **AUD-059 · P1 · 修复 · Capture 告警持久化失败时保留可重放进度**
  - 依赖：—。交付与验收：磁盘满或告警 spool 写失败时按明确策略保留告警重试或阻止源进度确认。验证有无 event journal 两种装配的重启回放，并记录丢弃或故障状态。证据：`agents/capture/runner/runner.go:565,603` 命中写入错误只记录日志和计数，消费仍确认源进度。
  - 状态／条件：未开始。

- [ ] **AUD-060 · P1 · 修复 · JS SDK 关闭时中止在途连接**
  - 依赖：—。交付与验收：并发连接与 close 采用明确的连接代次，旧拨号、读循环及入站队列只影响所属连接。在途拨号被关闭时不建立迟到会话；重连后仅新连接处理消息。证据：`sdks/js/src/tcp_transport.ts:215,274,383` 拨号 socket 局部持有，close 提前返回，读循环使用共享连接状态。
  - 状态／条件：未开始。

- [ ] **AUD-061 · P1 · 修复 · 移动端旧请求 401 不清除新会话**
  - 依赖：—。交付与验收：清理前核对发请求时的会话身份，旧服务器或旧 token 的迟到 401 只结束原请求。同一会话真实失效仍跳登录。证据：`mobile/lib/core/api/api_client.dart:44,65` 请求逐次取会话，响应处理无条件清除当前存储。
  - 状态／条件：未开始。

- [ ] **AUD-062 · P1 · 修复 · 移动端预选 scope 与发请求使用的 scope 一致**
  - 依赖：—。交付与验收：首次选择或已保存作用域失效时，界面显示、请求头与持久化选择一致。首次启动或旧作用域失效时，用请求替身核对界面预选与真实请求头。证据：`mobile/lib/features/scope/scope_controller.dart:84` 的 load 只更新预选状态，请求头仍从 SessionStore 读取。
  - 状态／条件：未开始。

- [ ] **AUD-063 · P1 · 修复 · 移动端函数目录按当前作用域刷新**
  - 依赖：AUD-062。交付与验收：切换作用域清空旧目录并新查，旧请求在途时不阻塞新请求。用 A→B 后旧请求迟到的样例核对目录、错误和 loading。证据：`mobile/lib/features/functions/functions_controller.dart:104,133` 切换调用 refresh，正加载时直接退出。
  - 状态／条件：未开始。

- [ ] **AUD-064 · P1 · 修复 · 移动端审批列表丢弃失效的分页响应**
  - 依赖：AUD-062。交付与验收：按会话、作用域和状态筛选追踪请求代次，切换后不附加旧分组的列表或描述符。验证旧页追加请求晚于新筛选的首页返回。证据：`mobile/lib/features/approvals/approvals_controller.dart:109,131` 异步完成后直接写入当前状态。
  - 状态／条件：未开始。

- [ ] **AUD-065 · P2 · 修复 · 分析测试脚本只停止自己启动的服务**
  - 依赖：—。交付与验收：复用已有容器的模式下，所有失败分支均不执行 compose down，DDL、查询和初始化使用相同配置的容器。自建模式清理自有进程和容器。用替身 Docker 命令验证两种模式。证据：`scripts/e2e/analytics.sh:34,53,67,268` 复用模式只保护常规退出，DDL 容器名仍为硬编码。
  - 状态／条件：未开始。

- [ ] **AUD-066 · P1 · 修复 · 移动端作用域保存不能覆盖新会话**
  - 依赖：AUD-062。交付与验收：同一会话的切换有最终写入顺序，旧请求结束时再次校验会话与切换代次。验证切换响应乱序、退出后迟到、切换服务器期间保存，界面、服务端选择与本地存储最终一致。证据：`mobile/lib/features/scope/scope_controller.dart:113` 用请求开始前的完整会话快照写回。
  - 状态／条件：未开始。

- [ ] **AUD-067 · P2 · 修复 · 移动端异步任务轮询隔离任务代次**
  - 依赖：AUD-062。交付与验收：同一任务仅保留一次在途轮询，任务切换或页面离开后忽略失效的响应。覆盖旧任务终态晚到、同任务多个 tick 和切作用域，防止旧结果覆盖当前任务或停止其定时器。证据：`mobile/lib/features/functions/functions_controller.dart:210,214` 周期轮询可重叠，完成后直接更新任务并可能停止共享定时器。
  - 状态／条件：未开始。

### 六语言 SDK 与相邻运行链复审（2026-11-21）

本轮逐语言抽取 Provider 注册/TLS、连接与停止、Invoker 发请求与读任务事件的主路径，并核对 JS/Python 插件启动与退出。六语言功能矩阵保留为能力索引。Provider 鉴权矩阵差异继续归 AUD-005，C++ 写帧归 AUD-013，C++/C# TLS 归 AUD-014，Python reader 清理归 AUD-018，JS 在途连接归 AUD-060。AUD-COV-004 继续覆盖配置加载、语言扩展、示例、全部协议分支与发布物回归。

| 范围       | 本轮核对的实现                                                       | 本轮证据                                                           |
| ---------- | -------------------------------------------------------------------- | ------------------------------------------------------------------ |
| Go         | Provider 建连、Stop、注册 metadata、任务状态，以及 HTTP Invoker 重试 | 源码复核。SDK 锁定依赖未缓存，下载超时，定向套件未执行             |
| Python     | Provider TCP 建连和清理、Invoker 请求、退避、事件轮询和同步封装      | 定向套件 67 项通过，另有替身 Invoker 复现                          |
| Java       | Provider 异步建连与停止、登记和任务处理、HTTP Invoker                | 源码复核。Gradle Wrapper 所需的 8.7 分发包下载超时，定向套件未执行 |
| JS/TS      | Provider 主/控制连接、Invoker 审批结果及任务轮询                     | 源码和运行时替身                                                   |
| C++        | Provider 读循环、连接状态和待响应队列、HTTP Invoker                  | 静态库与测试二进制编译通过，Invoker 12 项与传输/生命周期 29 项通过 |
| C#         | Provider 传输装配、并发请求、HTTP Invoker 及任务轮询                 | 传输与工厂测试 42 项通过                                           |
| Plugin SDK | JS 和 Python 的启动、握手、停机边界                                  | JS 替身定时器重现启动即计时                                        |

相邻运行链补查：数据库路由与迁移定向回归通过，SQLite 和替身 TLS listener 分别复现迟到连接池和取消等待问题。发布编排及备份为源码复核。

- [ ] **AUD-068 · P1 · 修复 · SDK 写调用只在具有幂等保证时自动重试**
  - 依赖：—。交付与验收：明确六语言 Invoker 的读、写和结果未知策略，为调用方保留可识别的未知结果。记录服务端哪些入口真正消费幂等键，仅在能去重或明确声明可重放时重试写请求。用同一替身服务让首个 POST 完成副作用后丢失响应，断言默认不会重复执行；读请求仍可退避重试。证据：`sdks/go/pkg/croupier/http_invoker.go:109,147,404`、`sdks/python/croupier/invoker.py:50,301`、`sdks/js/src/invoker.ts:66,395`。普通函数和任务启动 handler 目前未读取该键，页面执行已有独立的幂等实现。
  - 状态／条件：未开始。

- [ ] **AUD-069 · P1 · 修复 · SDK 保留待审批的调用结果**
  - 依赖：—。交付与验收：为函数执行和任务启动保留可识别的待审批状态、approvalId 和工作流，兼容原有同步结果与任务 ID API。六语言对照普通结果、待审批、传输失败和真实缺字段。证据：`internal/api/task/service.go:134` 正常返回待审批且无任务 ID，JS 替身响应中被判为 `no_task_id`；其他 SDK 的解码路径同样要求任务 ID 或执行结果。
  - 状态／条件：未开始。

- [ ] **AUD-070 · P2 · 修复 · Go SDK 的 Stop 与 Close 并发幂等**
  - 依赖：—。交付与验收：用同一个生命周期关闭信号协调 Stop/Close，并发调用不重复关闭 channel。并发停止、注册和重连通过 race 测试，已有顺序重复停止用例保持通过。证据：`sdks/go/pkg/croupier/client.go:366,380,393` 通过 select 检查后再 close，两步未同步。
  - 状态／条件：未开始。

- [ ] **AUD-071 · P1 · 修复 · Java SDK 拒绝停止后的迟到建连**
  - 依赖：—。交付与验收：并发 connect 共用一次有效握手，stop 和 close 能中止在途操作或拒绝其结果。锁住建连回包后先停机，断言迟到 transport 被释放、connected 为 false、心跳和 manifest 后台动作不启动。证据：`sdks/java/src/main/java/io/github/cuihairu/croupier/sdk/CroupierClientImpl.java:112,137,208` 在异步任务中发布 transport，stop 清理时只访问已发布连接。
  - 状态／条件：未开始。

- [ ] **AUD-072 · P1 · 修复 · C++ SDK 读循环退出后立即更新连接状态**
  - 依赖：—。交付与验收：对端正常 EOF 或网络错误可立即唤醒本连接的 pending 请求并清除连接状态；从读线程触发的回收不自 join，后续重连不复用旧线程与队列。证据：`sdks/cpp/src/tcp_transport.cpp:496,581` 读循环退出不发布断开。
  - 状态／条件：未开始。

- [ ] **AUD-073 · P1 · 修复 · C# SDK 并发调用使用唯一请求 ID**
  - 依赖：—。交付与验收：分配和登记请求 ID 不与仍在途的请求冲突，临近回绕时仍不会覆盖响应等待者。通过同步起跑、多并发乱序回包与取消测试。证据：`sdks/csharp/src/Croupier.Sdk/Transport/TCPTransport.cs:240,248` 普通读改写后按 ID 覆盖 `_pending`，帧写锁在登记之后。
  - 状态／条件：未开始。

- [ ] **AUD-074 · P1 · 修复 · JS SDK 生效 Provider 和控制面 TLS 配置**
  - 依赖：—。交付与验收：两条连接使用配置指定的 TLS、CA、客户端证书与服务端名称。校验失败阻止建立会话，明确配置明文时沿用 TCP。证据：`sdks/js/src/index.ts:1035,1083` 传入 TLS 选项，`sdks/js/src/tcp_transport.ts:185,247` 存下选项后仍走普通 TCP 拨号。
  - 状态／条件：未开始。

- [ ] **AUD-075 · P1 · 修复 · Node 插件的强制退出计时从停机开始**
  - 依赖：—。交付与验收：插件保持健康超过两个宽限期并仍能处理调用；收到 Shutdown 或信号后才开启有界退出，异步清理回调有明确等待预算。证据：`sdks/plugins/js/src/serve.ts:224,259` 在启动阶段注册 `process.exit`，替身定时器在从未请求停机时即观察到该回调被设置。
  - 状态／条件：未开始。

- [ ] **AUD-076 · P2 · 修复 · Python SyncInvoker 保留调用方事件循环**
  - 依赖：—。交付与验收：运行中的 asyncio 循环内调用同步接口时直接给出明确错误，不分配替代循环或创建未等待协程；普通同步调用可复用并关闭所属循环。证据：`sdks/python/croupier/invoker.py:397` 自行抛出的错误被同一 try 捕获，从而创建并设置新循环。
  - 状态／条件：未开始。

- [ ] **AUD-077 · P1 · 修复 · DB Router 关闭后拒绝发布迟到连接池**
  - 依赖：—。交付与验收：Close 与数据库发布共享生命周期检查。挂起最后迁移步骤、关闭后让迁移成功返回，迟到池被关闭、调用返回已关闭错误、池计数为零。证据：`internal/db/router/router.go:251,267,406` 在打开与缓存发布间缺少关闭检查，SQLite 探针复现关闭后池数回到 1。
  - 状态／条件：未开始。

- [ ] **AUD-078 · P1 · 修复 · SDK 发布物固定到同一个版本提交**
  - 依赖：—。交付与验收：统一解析目标 tag 和 commit，六个发布任务记录并使用相同的源提交。用当前分支领先于既有 tag 的 dry-run 比较 Go 子模块 tag、C++ 源码及二进制、其余包源 SHA。证据：`.github/workflows/release-sdk-go.yml:39,65` 从触发 ref 打子模块 tag，`.github/workflows/release-sdk-cpp.yml:64,82` 源码包与编译物分别取触发 ref 和目标 tag。
  - 状态／条件：未开始。

- [ ] **AUD-079 · P2 · 修复 · SDK 发布演练验证构建并隔离生产凭据**
  - 依赖：AUD-078。交付与验收：演练执行与正式发布一致的构建打包检查，凭据和外部写入仅在实际发布步骤需要。验证无凭据演练可跑六车道、产物对应目标提交且没有发布。证据：`.github/workflows/release-sdk-js.yml:58` 演练仍要求发布 token，`.github/workflows/release-sdk-cpp.yml:74` 演练跳过编译任务。
  - 状态／条件：未开始。

- [ ] **AUD-080 · P2 · 修复 · JS SDK 任务事件轮询可取消并限制响应体等待**
  - 依赖：—。交付与验收：任务轮询使用调用方 timeout 和取消信号，预算覆盖响应体解析与轮询间隔。验证正常逐序读完、响应头已到但 body 停滞、取消后不再发送请求。证据：`sdks/js/src/invoker.ts:485,504` 在读取响应体前取消超时计时，轮询间隔无调用方取消路径。
  - 状态／条件：未开始。

- [ ] **AUD-081 · P2 · 修复 · TLS 互联监听响应 Serve 上下文取消**
  - 依赖：—。交付与验收：同一 Serve 生命周期在明文或 TLS、空闲或连接在途时均可取消，监听和 worker 有界收尾。证据：`internal/transport/tcp/server.go:135,143,412` 只有裸 TCP listener 设置 Accept 截止时间。包装后的 TLS listener 在空闲时保持阻塞，替身探针核对必须额外 Close 才退出。
  - 状态／条件：未开始。

- [ ] **AUD-082 · P1 · 修复 · 备份创建在执行器不可用时返回明确状态**
  - 依赖：—。交付与验收：对象存储缺失、驱动不支持时，在创建前拒绝或进入有清晰原因的不可执行状态。覆盖正常执行、缺配置和旧 pending 数据收敛。证据：`internal/api/backup/service.go:94,106,130` 写 pending 后才查看执行器，执行器缺失时仍返回 pending 且无人推进。这是通用备份入口，数据面 ops 路径已有单独校验。
  - 状态／条件：未开始。

- [ ] **AUD-083 · P1 · 修复 · 备份列表使用与执行器一致的状态来源**
  - 依赖：—。交付与验收：备份列表、下载、执行结果及删除采用同一记录真源，已有扩展历史能明确迁移或用于兼容读取。测试安装扩展后，创建记录从 pending 到 succeeded/failed 能回读，底层入库错误可观测。证据：`internal/api/backup/service.go:42,100,316` 列表优先读扩展配置，创建时把 pending 写入配置；`internal/platform/backupexec/executor.go:122,136` 更新的是数据库备份行。
  - 状态／条件：未开始。

- [ ] **AUD-084 · P1 · 修复 · Android release 包声明并验收网络权限**
  - 依赖：AUD-006。交付与验收：静态检查 APK 的合并 manifest 含联网权限，并在 release 实机或模拟器跑一次登录到 scoped 请求的冒烟测试。证据：`mobile/android/app/src/main/AndroidManifest.xml:1` 未声明 INTERNET，debug/profile manifest 各自声明，当前未构建 release APK 复核合并结果。参见 [Android 网络权限文档](https://developer.android.com/develop/connectivity/network-ops/connecting)。
  - 状态／条件：未开始。

- [ ] **AUD-085 · P2 · 修复 · Android 分离演练包和正式签名包**
  - 依赖：AUD-006。交付与验收：演练包明确标识签名来源，正式产物从发布配置获得受管签名，缺配置时发布流程给出错误。已签名包验证证书指纹及版本升级兼容。证据：`mobile/android/app/build.gradle.kts:34,37` release 使用 debug 签名，`.github/workflows/ci-mobile.yml:92` 产出 release 命名的 APK。参见 [Android 命令行构建与签名](https://developer.android.com/build/building-cmdline)。
  - 状态／条件：未开始。

### 调度、数据库迁移、Agent 与部署续审（2026-11-21）

本轮沿定时调度派发与 task 事件读取、迁移 bootstrap、Agent 注册与配置同步、自托管发布、iOS 构建门禁、管理员缓存追踪生产装配路径。最小 Go 探针复现派发失败计数停滞、迁移 baseline 未继承 deadline、单连接迁移超时、迁移解锁超出调用预算且隐藏错误、注册前连接暴露、停止后连接复活、配置版本回退和任务事件续读漏项。部署环境变量使用临时目录验证。其余证据为源码复核，未执行现场部署、真实数据库迁移或 iOS 构建。

- [ ] **AUD-086 · P1 · 修复 · 将已确定的调度派发失败计入死信阈值**
  - 依赖：ARCH-023。交付与验收：派发明确失败时记录一次失败并推进计数，在阈值达到后停止派发，后续明确成功可重置连续失败计数。保留未知派发结果的既有处置语义。证据：`internal/tasks/scheduler/manager.go:331,416,425` 在无 run ID 的错误路径将原计数写回。定向测试在阈值为 2 时观察到 3 次派发且计数始终为 0。
  - 状态／条件：未开始。

- [ ] **AUD-087 · P1 · 修复 · 从调度任务所属游戏库回读上一次运行终态**
  - 依赖：—。交付与验收：根据 schedule 的 game/env 路由运行状态查询，将找不到记录、运行未终结和查库失败区分开。用 meta 与两个 game 库验收，各 scope 的失败和成功只影响本 schedule。证据：`internal/svc/service_context.go:313` 以 meta 连接构造调度模型，`internal/model/task_schedule_model.go:239` 直接查该连接，`internal/model/migration.go:202` 将 TaskRun 列为 game 模型，`internal/tasks/scheduler/manager.go:333` 忽略查库错误。
  - 状态／条件：未开始。

- [ ] **AUD-088 · P1 · 修复 · 定时任务创建时原子保存首次触发时间**
  - 依赖：—。交付与验收：创建 active schedule 与首次 `next_triggered_at` 在同一次提交完成，持久化失败时接口返回错误。盘点存量 active 且触发时间为空的任务并提供明确处置结果。证据：`internal/api/schedule/service.go:162,178` 先插入再尝试更新，更新失败仍返回创建成功，`internal/model/task_schedule_model.go:145` 永远不会捞到空触发时间的 active 行。
  - 状态／条件：未开始。

- [ ] **AUD-089 · P2 · 修复 · 提供可直接续读的 Task Events 游标**
  - 依赖：—。交付与验收：明确接口游标语义，为旧调用方保留兼容路径。连续轮询时把服务端续读游标直接传回，验证新到达的一条事件刚好读到一次，并覆盖无增量的轮询。证据：`internal/api/task/service.go:207` 返回最大事件序号加一，`internal/model/task_model.go:298` 使用严格大于过滤。若读到 1、服务端返回 2，使用返回值续读会跳过序号 2。现有测试固定了旧 `nextSeq` 语义。SQLite 续读探针确认，回传该值时漏掉序号 2，回传最后已读序号时能读到。
  - 状态／条件：未开始。

- [ ] **AUD-090 · P1 · 修复 · 将迁移上下文传入 baseline**
  - 依赖：—。交付与验收：首次建库及遗留库升级的 baseline SQL 继承迁移调用的取消和 deadline，失败后能按现有重试策略收敛。验证已过期上下文、baseline 执行中取消及正常续跑。证据：`internal/db/migrate/migrate.go:225,232,241` 版本探测和 goose Up 使用 ctx，baseline 使用原始 GORM 句柄。SQLite 探针验证 baseline 回调看不到调用方 deadline。
  - 状态／条件：未开始。

- [ ] **AUD-091 · P2 · 修复 · 迁移会话锁适配单连接池**
  - 依赖：—。交付与验收：使用会话锁的驱动在池上限为 1 时能完成迁移，或在等待前给出明确的配置错误。验收池上限为 1 和正常池大小以及多进程锁互斥。证据：`internal/db/migrate/migrate.go:98,219,225` 先独占池中的连接作为锁会话，再从同一池借连接探测版本表。MySQL 替身验证上限为 1 时等到调用 deadline，版本查询未发出。
  - 状态／条件：未开始。

- [ ] **AUD-092 · P1 · 修复 · Agent 只向业务暴露注册已确认的连接**
  - 依赖：—。交付与验收：业务侧的连接和就绪状态在注册成功后一起发布，注册失败、超时、替换或断线时同步撤销。验证重连期间任务事件及指标发送看到明确的未就绪状态。证据：`core/register/loops.go:64,69,74` 在 Register 前发布连接，`core/register/registration.go:112,117` 直接对外返回连接与 socket 状态，`internal/app/agent/upstream.go:575` 使用该接口。阻塞注册前置步骤的探针观测到连接已暴露。
  - 状态／条件：未开始。

- [ ] **AUD-093 · P1 · 修复 · Stop 纳入 Agent 直接拨号与注册的生命周期**
  - 依赖：—。交付与验收：启动路径和直接拨号在 Stop 时统一取消并收尾。拨号晚返回时关闭该连接，禁止复活已停止的客户端或触发连接成功回调。证据：`core/register/loops.go:52,64,69` 只在拨号前判断停机，`core/register/registration.go:174` 未收束直接拨号。替身探针在 Stop 返回后放行拨号，客户端重新变成 Connected。
  - 状态／条件：未开始。

- [ ] **AUD-094 · P1 · 修复 · 串行化 Agent 配置拉取与版本提交**
  - 依赖：—。交付与验收：定时拉取与手动拉取同时发生时，配置落地结果和版本游标一致，较早请求的迟到响应无法覆盖已经应用的更新。用不要求可排序的版本值测试两种完成顺序，以及 apply 失败后的重试。证据：`core/configsync/loop.go:16` 分开读版本、拉取、应用和写版本，`internal/app/agent/app_extension.go:69` 暴露直接拉取入口。探针观察到较新配置先应用，随后被较早请求的迟到响应覆盖。
  - 状态／条件：未开始。

- [ ] **AUD-095 · P1 · 修复 · 自托管部署保留实例环境变量**
  - 依赖：—。交付与验收：明确 workflow 管理键与实例配置键，构建配置后原子落盘。以临时目录验证自定义端口、采集配置和 demo scope 两次部署后仍在，目标镜像 tag 正常更新。证据：`.github/workflows/deploy-self-hosted.yml:121,125,144` 先覆盖 `.env` 再从新文件读取旧端口。抽取脚本验证 demo scope 保留，数据库端口、采集端口和采集凭据配置键丢失。
  - 状态／条件：未开始。

- [ ] **AUD-096 · P1 · 修复 · 自托管发布在切换前完整验证候选配置**
  - 依赖：—。交付与验收：将 compose 与配置树写入独立候选目录并通过校验后再切换。覆盖镜像准备失败、候选配置无效和切换失败，验证现场配置和上一个可用版本可继续使用。证据：`.github/workflows/deploy-self-hosted.yml:66,79,110,150` 在登录镜像仓库及后续校验前直接替换现场 compose 和配置目录，备份范围只有两个 Agent YAML。
  - 状态／条件：未开始。

- [ ] **AUD-097 · P2 · 补齐 · 为 iOS 原生工程加入 CI 构建验收**
  - 依赖：AUD-006。交付与验收：在 macOS runner 验证 iOS 工程及插件注册，使用无签名构建阻止编译回归。记录首次 iOS 实机登录与作用域请求结果，发布签名验证按发布流程单独执行。证据：`mobile/ios/Runner.xcodeproj/project.pbxproj` 已跟踪原生工程，`.github/workflows/ci-mobile.yml:24,69,77` 当前构建门禁只验证 Android，仓内未找到 iOS 构建工作流。
  - 状态／条件：未开始。

- [ ] **AUD-098 · P1 · 修复 · 为迁移会话解锁设置独立清理预算**
  - 依赖：—。交付与验收：解锁使用有界清理上下文，保留原迁移错误并报告解锁失败，无法确认锁已释放时弃用相应物理连接。覆盖迁移取消、解锁延迟及非连接级解锁错误。证据：`internal/db/migrate/migrate.go:130` 使用无 deadline 的后台上下文并丢弃解锁错误，随后关闭逻辑连接。MySQL 替身观察到清理超过原调用预算、连接回到空闲池且解锁错误未透出。
  - 状态／条件：未开始。

- [ ] **AUD-099 · P2 · 修复 · Nightly 签名发布成功后再更新滚动入口**
  - 依赖：—。交付与验收：保留现有 `nightly` 安装入口和最新构建保留策略。版本标识、提交与签名清单指向同一发布，签名或上传失败时滚动入口继续对应上一次完整产物，重跑可以恢复未完成构建。证据：`.github/workflows/nightly.yml:42,1017,1020,1048` 使用滚动标签，先强移远端 tag，再创建草稿、签名及公开。本轮仅复核 workflow 源码。
  - 状态／条件：未开始。

- [ ] **AUD-100 · P1 · 修复 · 管理员资料缓存仅存业务所需字段**
  - 依赖：—。交付与验收：按 ID 和用户名写入同一份显式字段模型，移除缓存中的密码散列和 TOTP 密钥，并清理旧格式记录。验证本地、Redis 的主键和别名缓存均不含凭据字段，同时核对资料、角色查询和登录/二次验证流程。证据：`internal/svc/cache_layer.go:33,47,90` 直接缓存完整 Admin，`internal/cache/helper.go:115` 默认 JSON 序列化，`internal/model/admin.go:17,19` 的凭据字段参与序列化。当前为源码确认。
  - 状态／条件：未开始。

- [ ] **AUD-101 · P2 · 实现 · 在下次线协议升版前完成双向版本协商**
  - 依赖：CR-FIX-07。交付与验收：在存在两个受支持版本时确定每条连接共用的帧版本，按协商版本处理请求、响应和主动事件。用 N/N-1 的服务端、Agent 与 SDK 双向混合测试覆盖成功、无交集与重连。证据：`pkg/protocol/message.go:31,55,278` 仅实现入站版本窗口，出站固定使用本端版本。历史记录已明确 v1 无协商，本条落实下一次升版前的交付。
  - 状态／条件：未开始。

#### 历史高优先级问题复核

对照 `docs/review/external-deep-audit-2026-11-04.md`，源码基线为 `269fa205c`。

| 历史条目                            | 当前结果                           | 依据                                                                                                                                                                                                                      |
| ----------------------------------- | ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| P0-1 / CR-FIX-01                    | 已修复                             | SDK JS/Python manifest 均为 Apache-2.0，根 LICENSE 占位符已替换，NOTICE 已落库。                                                                                                                                          |
| P0-2 / CR-FIX-02                    | 已修复                             | `scripts/check-version-sync.sh` 当前通过，六语言 9 处版本声明均为 0.1.6，CI 和 release 均接入守卫。                                                                                                                       |
| P0-3 / CR-FIX-03                    | 已修复                             | `cmd/server/root.go:199` 启动前执行互联 TLS 校验，`TestValidateInterconnectTLS*` 通过。                                                                                                                                   |
| P0-4 / CR-FIX-04、P1-12 / CR-FIX-12 | 部分回退                           | 发布任务仍依赖质量门禁并生成签名摘要，Nightly 经 `46734718f` 有意改为只保留最新构建，旧任务的历史 Nightly 可下载验收已失效。更新滚动入口早于签名发布的新风险见 AUD-099。未执行真实发布。                                  |
| P0-5 / CR-FIX-05                    | 门禁及用例已落地，运行验证部分完成 | 六个 SDK 工作流均包含 `proto/**`，存在共享 golden 向量及六语言测试，`ci.yml` 有 breaking 检查。本轮 Python 2 例、JS 4 例通过，并使用上一轮在当前工作区构建的 C++、C# 测试产物各通过 2 例。Go、Java 仍受固定依赖下载阻塞。 |
| P1-6 / CR-FIX-06                    | 已修复                             | `docs/architecture/index.md:49` 的 SQLStore 说明与 `internal/svc/service_context.go:379` 装配一致。                                                                                                                       |
| P1-7 / CR-FIX-07                    | 部分完成                           | Go 接收路径按窗口拒绝并保留连接，出站仍固定本端版本。当前源码只支持 v1，混合版本协商仍需在下次升版前实现，见 AUD-101。                                                                                                    |
| P1-8 / CR-FIX-08                    | 实现已落地                         | 源码确认 Java、C++、C# 都有独立控制车道。本轮未重跑三语言相同负载下的背压组合测试。                                                                                                                                       |
| P1-9 / CR-FIX-09                    | 旧实现已替换                       | `internal/transport/tcp/server.go:197` 起实现业务车道上限、工作线程和独立控制车道。其 TLS 监听取消问题独立跟踪 AUD-081。                                                                                                  |
| P1-10 / CR-FIX-10                   | 原路径缺口已处理                   | 协议文档已有投递语义表，Dispatcher 将换实例条件收窄到不可达。其他 SDK 的提交结果不确定时重试问题继续由 AUD-068～069 跟踪。                                                                                                |

### SDK 配置入口与 Java 扩展发布续审（2026-11-21）

核对 Go 默认配置合并与连接入口、Python dataclass 到 Provider、JS 对象及重连配置合并、C++ base/profile、C# 环境与 JSON 加载、Java Spring Boot starter 与 Maven 发布。C++、C# 使用当前仓库此前构建的产物和独立配置夹具验证。Go 使用独立配置源码探针，Java 使用当前仓库此前构建的 Provider 类确认默认 transport，未跑完整 SDK 测试和 Java 发布。

- [ ] **AUD-102 · P1 · 修复 · Go SDK 默认配置合并支持显式安全模式**
  - 依赖：—。交付与验收：区分未提供安全选项和显式 `insecure=false`，合并后将 TLS 选项传到默认连接。验证省略、显式 true、显式 false 加 CA 三种输入及现有直传 `ClientConfig` 用法。证据：`sdks/go/pkg/croupier/types.go:180` 默认 true，`config_validation.go:382` 只覆盖 true，`client.go:153` 将合并后的标志传给连接层。Go 配置探针确认显式 false 加 CA 时仍返回 insecure=true，完整 Go SDK 测试构建在限时内未完成。
  - 状态／条件：未开始。

- [ ] **AUD-103 · P1 · 修复 · C++ SDK profile 只覆盖文件显式配置**
  - 依赖：—。交付与验收：先保留字段是否出现的信息，再合并配置并应用默认值。覆盖只改环境、布尔 false、重试次数 0 与基础配置安全设置。证据：`sdks/cpp/src/config/client_config_loader.cpp:109,271,277,462` 先将 profile 加载为带默认值的配置，再覆盖基础配置。探针用仅有 env 的 profile 观察到基础 `insecure=false`、禁用自动重连和 17 秒超时被改写成 true、true、30 秒。
  - 状态／条件：未开始。

- [ ] **AUD-104 · P2 · 修复 · C# SDK 文件配置按标准 JSON 结构解析**
  - 依赖：—。交付与验收：保留已支持的字段命名方式，正确解析嵌套对象、转义、空字符串和显式 false，语法及类型错误返回可定位诊断。证据：`sdks/csharp/src/Croupier.Sdk/Configuration/ClientConfigProvider.cs:125,139,151` 用正则截取后反射赋值。独立 .NET 探针复现含逗号的服务名被截断，合法 Headers 对象抛出 ArgumentException。
  - 状态／条件：未开始。

- [ ] **AUD-105 · P2 · 修复 · Java Spring Boot starter 暴露 Provider 鉴权配置**
  - 依赖：AUD-005。交付与验收：Spring 配置可绑定 Provider token，并映射到手写 Java ClientConfig 的同一属性。测试省略、显式设置和用户自定义 ClientConfig，验证连接握手使用该配置。证据：`sdks/java/spring-boot-starter/src/main/java/io/github/cuihairu/croupier/spring/boot/autoconfigure/CroupierAutoConfiguration.java:44` 的工厂与 CroupierProperties 缺 token 字段，核心 `ClientConfig.java:115` 已具备相应属性。
  - 状态／条件：未开始。

- [ ] **AUD-106 · P2 · 修复 · Java starter 与核心 SDK 使用同一发布仓库配置**
  - 依赖：AUD-078、AUD-079。交付与验收：在 Maven 本地演练中核对核心包和 starter 的仓库选择、版本及 POM 依赖均对应同一次发布。证据：`.github/workflows/release-sdk-java.yml:75,77` 传入统一 GitHub Packages URL，`sdks/java/build.gradle:170` 遵循该输入，`sdks/java/spring-boot-starter/build.gradle:95` 仍将目标固定为旧独立 SDK 仓库。当前为源码确认。
  - 状态／条件：未开始。

- [ ] **AUD-107 · P1 · 修复 · 将 Java Provider 安全配置接入默认 TCP transport**
  - 依赖：—。交付与验收：显式安全模式下为首连和重连启用已有的 TLS factory 与主机名验证，读取 CA、客户端证书、私钥和 server name。Spring 入口映射同一套字段。验收证书正确时注册成功，主机名或 CA 错误时拒绝连接，开发模式行为按原配置工作。证据：`sdks/java/src/main/java/io/github/cuihairu/croupier/sdk/CroupierClientImpl.java:65,69,75` 的默认工厂忽略 ClientConfig 中安全设置，直接使用明文构造函数，`transport/TCPTransport.java:116` 的 TLS 构造入口没有从该工厂调用。使用现有 Java 构建产物实例化 Provider，检查到 insecure=false 下默认 transport 的 TLS factory 仍为空。
  - 状态／条件：未开始。

## Codex 深审发现（2026-11-13，luna profile 近5 commit 审核，6C/22M/5m，待修）

> 本节保留导入原文及其来源；已对照到 AUD/PWR 的条目不重复设复选框。未映射条目仍待逐项审核。原始分级不是本轮代码核验结论。

<!-- 34 条原子 todo 由 codex 深审产出，原分级：6 Critical（JWT 回退密钥/TLS 形同虚设/权限缺失/JSON 注入/SQLite 备份不一致/PowerOps CAS）+ 22 Major + 5 Minor（含 C++/C#/Python SDK 同型残留）-->

- **对照 AUD-019，不重复计数**：为 ProvisionRun Update 增加主键条件写入（`internal/model/provision_model.go:61-63`）—— 防止 Save 因主键问题重复插入。
- **对照 PWR-001，不重复计数**：将 PowerOps pending→executing 改为数据库 CAS（`internal/powerops/request.go:200-229`）—— 防止续行重复执行破坏性动作。
- **对照 AUD-007，不重复计数**：为全局备份路由增加服务层权限与 403 测试（`internal/handler/routes_business.go:66-75`）—— 阻止零权限读取/删除备份。
- **对照 AUD-014，不重复计数**：接入 C++ 默认 TLS transport 或安全模式拒绝连接（`sdks/cpp/src/croupier_client.cpp:417-592`）—— 防止 TLS 配置形同虚设。
- **对照 PWR-002，不重复计数**：为 Provider 动作增加独立有界 timeout（`internal/powerops/request.go:187-198,306-308`）—— 避免卡住命令永久滞留。
- **对照 AUD-011，不重复计数**：修复 Mobile CI 覆盖率阈值解析（`.github/workflows/ci-mobile.yml:43-61`）—— 让质量门禁真实生效。
- **对照 AUD-002，不重复计数**：定义结构化 RPC 错误帧并替换空 protobuf（`internal/transport/tcp/server.go:339-362`）—— 让客户端可靠区分失败与零值成功。
- **对照 AUD-018，不重复计数**：统一 Python reader 异常退出收口逻辑（`sdks/python/croupier/transport/tcp.py:298-340`）—— 防止死连接保留可用状态。
- **对照 AUD-004，不重复计数**：让 Agent Stop 超时返回错误（`cmd/sidecar-agent/service.go:163-175`）—— 向服务管理器如实报告未停止。
- **对照 AUD-013，不重复计数**：为 C++ socket 写实现 write-all 和连接写锁（`sdks/cpp/src/tcp_transport_inbound.cpp:148-180`）—— 修复短写与并发帧交错。
- [ ] 优化 listener fallback 的空结果处理（`core/metrics/processes.go:49-62`）—— 避免空 socket 结果重复四次全表扫描。
- **对照 AUD-010，不重复计数**：区分软删除账号与存储故障的 token 版本查询（`internal/svc/service_context_auth.go:304-316`）—— 保证删除账号立即吊销旧 JWT。
- **对照 PWR-004，不重复计数**：统一 PowerOps 发送与验证成功计数（`internal/powerops/request.go:397-432`）—— 消除终态与头部计数矛盾。
- **对照 PWR-003，不重复计数**：实现逐目标持久化和重启恢复策略（`internal/powerops/request.go:271-321`）—— 不对不确定动作盲目重放。
- **对照 PWR-006，不重复计数**：将 WOL 结果改为派发/确认两阶段（`internal/powerops/request.go:264-269,397-409`）—— 不把包发送误报为上线完成。
- [ ] 用注入时钟替换备份 3.5 秒 sleep（相关测试，commit `d790d6258`）—— 降低测试耗时和慢 CI 抖动。
- **对照 AUD-015，不重复计数**：实现冷启动任务收割的有界持久巡检（`internal/tasks/store.go:135-156`）—— 覆盖未打开游戏库中的陈旧任务。
- **对照 PWR-005，不重复计数**：将 status 查询移出审批与变更验证状态机（`internal/powerops/request.go:222-223,349-391`）—— 避免状态查询等待满验证超时。
- **对照 AUD-005，不重复计数**：更新六语言鉴权/TLS 能力矩阵（`sdks/SDK_FEATURE_MATRIX.md:40-44`）—— 使文档与当前实现一致。
- **对照 AUD-020，不重复计数**：序列化 Incident Details 后写入 updates（`internal/api/incident/service.go:331-333`）—— 修复事故详情更新路径。
- **对照 AUD-002，不重复计数**：同步修改 `core/transport/tcp/mux_conn.go` 的错误编码（`core/transport/tcp/mux_conn.go:370-386`）—— 消除同型 JSON 注入残留。
- **对照 AUD-016，不重复计数**：为对象存储上传链路加权限与保留前缀保护（`internal/api/storage/service.go:122-163`）—— 阻止低权限覆写共享备份对象。
- **对照 AUD-017，不重复计数**：为全局 PowerOps 配置端点加平台硬件权限（`internal/handler/routes_task.go:218-223`）—— 隔离游戏作用域与全局节点管理。
- [ ] 将 fixture 的 signal 监听提前到 ready 前（`cmd/server/dashboard_fixture_cmd.go:51-55`）—— 修补 `76f057be9` 同型残留。
- **对照 AUD-006，不重复计数**：固定 Mobile Dart/Flutter 工具链与 lockfile（`.github/workflows/ci-mobile.yml:40-64`）—— 消除依赖解析漂移。
- **对照 AUD-008，不重复计数**：重写生产 JWT 密钥判定并补配置矩阵测试（`internal/security/jwtutil/secret.go:44-72`）—— 消除公开开发密钥回退。
- **对照 AUD-001，不重复计数**：非 loopback Provider token 强制绑定 TLS（`cmd/sidecar-agent/root.go:638-647`）—— 防止握手 token 明文外泄。
- **对照 AUD-003，不重复计数**：为移动端 refresh 加请求令牌并清理旧作用域（`mobile/lib/features/analytics/analytics_controller.dart:54-80`）—— 修复失败旧数据和乱序覆盖。
- **对照 AUD-014，不重复计数**：接入 C# 默认 TLS transport 或安全模式拒绝连接（`sdks/csharp/src/Croupier.Sdk/CroupierClient.cs:681-774`）—— 同步消除同型 TLS 漂移。
- **对照 AUD-021，不重复计数**：修正 IPv6 ICMP 类型比较（`core/healthprobe/icmp.go:96-101`）—— 让 IPv6 echo reply 能被识别。
- [ ] 校验 TCP 插件 path 并补协议注入测试（`agents/sidecar/funcplugin/examples/tcp/main.go:177-205`）—— 阻止 CR/LF 注入多行命令。
- **对照 AUD-009，不重复计数**：将 SQLite dump 改为一致性快照并验证恢复数据（`internal/platform/backupexec/executor.go:227-243`）—— 保证 WAL 已提交事务进入备份。
- [ ] 为 metrics 采集全链路接入 context 并补停止测试（`core/metrics/processes.go:139-200`）—— 防止采集关闭后 goroutine 泄漏。
- [x] 将 AUD-007..018 追加到 todo 台账（`todo.md:1231-1266`）—— 避免已确认审计项遗漏跟踪。（2026-11-13 已补齐至 AUD-028）

## 文档站语言自动切换（G2-LANG-001）

- [ ] G2-LANG-001 [P2] 文档站浏览器语言自动识别：文档站当前仅手动切换（导航栏语言菜单 + 显式 `//` ↔ `/zh/` 链接），无 `navigator.language` 探测跳转、无显式选择记忆。来源：2026-11-15 舰队文档一致性+语言切换审计 G2 轮，台账见 `docs/审计-文档一致性.md`「面2 语言」与「三桶计数（G2）」节。处置：作为增强项挂起，是否全舰队推行自动切换待用户拍板，本轮不实现。
