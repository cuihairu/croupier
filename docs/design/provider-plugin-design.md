---
title: Provider 插件设计——外部平台接入的契约、加载与权限（#66 立项）
---

# Provider 插件设计：外部平台接入插件

## 状态

- 状态: Proposed（2026-10-08，OPEN-ISSUES #66 立项；批一两份调研已归档，本文为批二设计产出，实现代码按 §9 分期另行立项）
- **拍板记录（2026-10-10，用户授权代拍，效果后审）**：P0 准予动工（契约先行=无风险结构件：drivers/driver.go 骨架 + Registry 处置 + manifest provider 块定稿）；P2 pack descriptor 顺延；P3 维持远期不承诺。
- 决策摘要（拍板授权令下按推荐方案定，供后审）:
  1. **不引入动态代码加载**——driver 编译期内置（init 自注册），provider 是「配置实例」而非可执行包；Grafana 式子进程/gRPC 与 Go plugin `.so` 均不采用（§2、§6）
  2. **统一注册中心 = extension runtime bindings**——`internal/platform/provider.Registry` 归档不接线，避免双注册中心（§5.3）
  3. **`platform/provider` 的 Provider 接口语义保留为 driver 契约蓝本**，quicksdk 实现按 mapping 文档迁 `official.external-platform`（§8）
  4. **cicd / identity 两处抽象保留内置不迁移**（§8）
  5. 版本协商复用 release 体系（`min_core_version` + 依赖图校验），provider manifest 不另立版本字段（§3.4）
- 关联: [商业游戏后台功能调研](../research/commercial-backend-survey.md) · [插件架构调研](../research/plugin-architecture-survey.md) · [扩展安装模型](../architecture/extension-installation-model.md) · [官方扩展统一模式](../architecture/official-extension-unified-pattern.md) · [扩展域 API 契约基线](../architecture/extensions-api-contract-baseline.md) · [Core/Extension Mapping](../architecture/core-extension-mapping.md)

## 1. 问题与目标

### 1.1 现状

- 三处 provider 抽象并存、互不相通：`internal/platform/provider`（第三方平台集成，Registry 未接生产——生产链路已改走 extension externalfunc dispatcher）、`internal/cicd`（CI/CD，工厂注册表 + init 自注册）、`internal/security/identity`（身份源，配置驱动）。
- extension 体系（五表 + 安装生命周期 + pack 导入 + Agent 同步）已具备完整的「包管理器」能力，但 provider 侧没有成文的插件契约：binding spec 字段（`provider/type/operations/enabled/config`）只有代码消费（`externalfunc.ParseProviderBinding`），manifest 增量字段、driver 分层、生命周期映射均未定稿。
- Core/Extension Mapping 已定方向：`official.external-platform` 为迁移第一优先，`internal/platform/openapi` 转 `drivers/openapi`——本文补齐该方向的契约细节。

### 1.2 目标

1. 「外部平台/系统接入」（第三方平台、外部 API、webhook 类系统）统一为一种插件形态：**extension 安装实例 × 编译期内置 driver × provider 配置实例**。
2. provider 插件复用 extension 体系全部既有机制：catalog/release 分发、install/enable/disable/upgrade/uninstall 生命周期、config/secrets 拆分、三层 capability 权限、事件记账、Agent 运行时副本、依赖图校验。零新表、零新安装状态机。
3. 三处既有 provider 抽象各有明确裁决（保留/迁移/归档），不留悬而未决的双重语义。

### 1.3 非目标（本期明确不做）

- 动态加载外部代码（`.so`、子进程 connector、脚本沙箱）——见 §2 威胁模型论证。
- 任意 UI 注入——插件页面仍走 runtime `page` bindings / PageSpec 既有链路，不引入前端组件分发。
- cicd / identity 的扩展化改造（§8 裁决为保留内置）。
- 插件市场/公网目录分发（Grafana Catalog / wordpress.org 形态）——单公司自托管场景，Store（catalog 表）即分发面。

## 2. 调研结论 → 设计决策

两份调研的输入直接映射到四个设计决策：

| 调研结论                                                                                                                                                             | 设计决策                                                                                                                                                                                  |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 插件架构调研：契约强度与隔离强度成正比；进程外隔离（Grafana 子进程 + gRPC）是重量级选项，仅面向不受信第三方代码；WordPress 模型=同进程 + 「信任边界=管理员能装什么」 | Croupier 是自托管单公司部署，插件来源为内部 pack 导入（sha256 校验 + 对象存储落盘）——威胁模型不支持「不受信代码」假设，选同进程配置实例化，不立项子进程隔离                               |
| 插件架构调研：强契约平台标配三角=分发可信、能力声明、版本协商                                                                                                        | 能力声明复用统一模式三层 capability 与 config schema 声明（已落地）；版本协商复用 release 体系 `min_core_version` + 依赖图（已落地）；分发可信已有 checksum，签名分级登记为 P3 远期不承诺 |
| 商业后台调研：商业后台自身以「目录 + 安装 + 运行时绑定」形态接第三方能力（PlayFab Add-ons、Nakama Runtime Modules、Photon Plugins）                                  | 形态同构印证：extension 五表已实现同构机制，不另发明；差异仅在 driver 是否编译期内置                                                                                                      |
| 商业后台调研：RM/公告/客服等属业务面，权限/审计/风控属平台底座                                                                                                       | provider 插件只做「外部系统接入」这一件事，不带业务模块——业务能力走 `official.*` 全扩展（统一模式），两类载体不混装                                                                       |

## 3. 核心概念与契约

### 3.1 三层模型

| 层        | 是什么                                                 | 生命周期载体                   | 对应 Grafana 术语         |
| --------- | ------------------------------------------------------ | ------------------------------ | ------------------------- |
| extension | 可安装的插件包（catalog + release）                    | 五表（catalogs/releases/…）    | app（plugin.json + 分发） |
| driver    | 一种调用协议的编译期内置实现（openapi/webhook）        | Go 二进制版本                  | backend plugin 可执行文件 |
| provider  | 一个 driver 的配置实例（指向哪个外部系统、用什么凭据） | installation + runtime binding | datasource 实例           |

一个 extension release 声明 provider 能力（operations/configSchema），安装后派生 provider 绑定；一个绑定在运行时对应一个 driver 实例。Grafana 的 datasource 同构：grafana 内置 postgres/mysql driver，「添加 datasource」即配置实例。

### 3.2 manifest provider 块（增量契约）

release `manifest_json` 在既有字段（`ui.pages`/`configSchema`/三层权限声明）上新增 `provider` 块：

```json
{
  "provider": {
    "type": "openapi",
    "operations": ["day_report", "user_live", "order_list"],
    "permissions": { "operate": "external-platform.operate" }
  }
}
```

- `type`：driver 类型，闭集起步 `openapi | webhook`（`webhook` 为 P2 预留；`type` 缺省 `openapi`，与 `ParseProviderBinding` 现状一致）。
- `operations`：该 provider 暴露的方法闭集；调用面 function ID 沿用 `external.<provider>.<method>`（`externalfunc.BuildFunctionID` 现状，key 经 `SanitizeKey` 归一）。
- `permissions`：可选覆盖；缺省按统一模式三层权限（`<domain>.read/operate/admin`），不声明即继承。
- **与 binding spec 同构**：manifest provider 块与 runtime binding 的 `spec_json` 字段集一致（`provider/type/operations/enabled/config`）——安装时从 manifest 派生默认绑定，运营可在 installation 上覆写 `config`（经 config/secrets 既有拆分）。同构是防漂移约束：两侧字段集变更必须同步，守卫测试对齐 official seed 先例。

### 3.3 运行时绑定契约（现状确认，无新机制）

- `binding_type ∈ {provider, openapi}` 的绑定经 `DiscoverProviderOperations` 聚合为 provider→operations；`function` 绑定以 `external.*` 前缀解析并入。
- 调用链：`POST platform call`（或通用函数调用）→ dispatcher 按 `external.<provider>.<method>` 寻址 → driver 实例执行。
- capability 命名：`external.<provider>`（`externalfunc.Capability` 现状）——这是**调用面** capability；**管理面**（安装/启停/配置）capability 按统一模式挂 extension domain（如 `external-platform.admin`）。两层各司其职，不合并。

### 3.4 版本与依赖

- release 已有 `min_core_version`（semver 校验）与依赖图校验（`missing_dependency`/`dependency_cycle`/`version_mismatch`），provider 插件直接复用，**不新增版本字段**。
- 升级 = 版本切换 + config 对新 schema 校验 + reconcile 重建绑定（既有机制）；回滚 = upgrade 到旧版本。

## 4. 目录结构

对齐 Core/Extension Mapping 的目标形态（增量部分）：

```text
internal/
  core/extension/            # 既有六子包（manifest/catalog/installation/runtime/sync/externalfunc），不动
  drivers/
    driver.go                # Driver 接口 + Factory 注册表（cicd 模式参照：init 自注册 + 重复注册 panic）
    openapi/                 # openapi 调用协议 driver（internal/platform/openapi 迁入，mapping §2.3 已定）
    webhook/                 # P2 预留
  extensions/official/
    externalplatform/        # 官方外部平台扩展：quicksdk 实现迁入 + seed manifest（mapping §2.3 已定）
```

pack 结构（`.tgz` 导入物，现状仅整体存储不解析）：

```text
manifest.json          # §3.2 契约
descriptors/*.json     # 函数 descriptor（P2 起解析入 release）
schemas/*.json         # JSON Schema（P2 起）
```

## 5. 注册与发现

### 5.1 编译期（driver 层）

driver 是窄闭集（openapi/webhook），采用 `internal/cicd` 的注册表模式：包级 `Register(kind, factory)`，各 driver `init()` 自注册，重复注册 panic fail-fast，消费方 import 完成注册后按 kind 构造。不使用任何运行时代码发现。

### 5.2 运行时（provider 层）

既有链路，无新机制：install → bindings 写入（`ReplaceForInstallation` 全量替换）→ enable → Agent 同步 payload 按 target 过滤 → dispatcher 可调用。operations 聚合已由 `DiscoverProviderOperations` 实现，capabilities 端点已返回 `type/key/capability/provider/operations/permissions/configKeys`。

### 5.3 注册中心统一（裁决）

- `internal/platform/provider.Registry`（`NewRegistry/Register/Unregister`）**归档不接线**：它与 extension runtime bindings 构成双注册中心，接线等于让 provider 有两处真值；且其 `Init(ctx, config)` 语义与 installation 生命周期（enable/disable/upgrade）无对应关系。
- 处置：`Provider` 接口保留并注释为 driver 契约蓝本（`Call(ctx, method, []byte) ([]byte, error)` 的 JSON 透传语义正是 driver `Call` 的原型）；`Registry` 及其未接生产的测试代码在 P0 删除。包内 `ProviderNotFoundError/MethodNotSupportedError/ProviderDisabledError` 三个错误类型语义由 driver 层沿用。

## 6. 加载器

「加载」在无动态代码的前提下 = **配置实例化 + 生命周期映射**：

```text
installation.enable
  → 取 binding spec（provider/type/operations/config）
  → driverRegistry.New(type, DriverConfig{Endpoint, Token, Extra, HTTP, Now})
  → 实例进入运行时表（key = provider 名）；disable/uninstall → Close + 移除
  → upgrade → 旧实例 Close + 新 config 重建（复用 reconcile）
```

- Driver 接口以 `platform/provider.Provider` 语义为蓝本收窄：`Kind() string`、`Call(ctx, method, request []byte) ([]byte, error)`、`Close() error`（`Init` 并入构造工厂——配置不合法在工厂返回错误，对齐 cicd `Factory` 先例；`IsEnabled`/`SupportedMethods` 由 installation 状态与 binding spec 表达，不再要求实现方维护）。
- 出站 HTTP 统一走 secguard 守卫客户端（`Config.HTTP` 注入先例见 `internal/cicd` Config 注释——生产路径必须传守卫后的客户端）。
- 拨测复用 `POST /installations/:id/test-connection` 端点语义。
- 无热加载：config 修改 → disable/enable 或 reconcile 生效；driver 本体升级 = 平台发版。

## 7. 权限面

| 面     | 机制                                                                                                      | 载体                                                                 |
| ------ | --------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| 管理面 | 统一模式三层 capability：`<domain>.read/operate/admin`                                                    | extension manifest 权限声明 + capability 端点聚合（已落地）          |
| 调用面 | `external.<provider>` capability + operation                                                              | `externalfunc.Capability`（现状）；RBAC 按 capability/operation 裁决 |
| 配置面 | `config`（非敏感）/ `secret_refs`（引用非明文）拆分，schema 声明 `type/description/required/default/enum` | installation `config_json`/`secret_refs_json`（已落地）              |

- 事件前缀遵循统一模式：`<domain>_*`（如 `external_platform_call_failed`）。
- 审计走平台既有 audit 链（HTTP 层通用审计），不另建插件级审计表。

## 8. 与现有 provider 抽象对接（逐个裁决）

| 抽象                                              | 裁决            | 理由与动作                                                                                                                                           |
| ------------------------------------------------- | --------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/platform/provider`（Registry 未接生产） | 归档 + 语义继承 | Registry 删除（双注册中心，§5.3）；Provider 接口保留为 driver 蓝本；quicksdk 实现迁 `extensions/official/externalplatform`（P1，mapping 第一优先）   |
| `internal/platform/openapi`                       | 迁移            | → `internal/drivers/openapi`（mapping §2.3 已定「作为驱动复用，不再当业务模块」）；P1 执行                                                           |
| `internal/cicd`（Provider + 工厂注册表）          | 保留内置        | 窄闭集（4 kind）、无安装/分发诉求、工厂注册表已是良好形态；`CicdIntegration` 行与 installation config 同构，是否打通为扩展形态列为 P3 评估，不承诺   |
| `internal/security/identity`                      | 保留内置        | 登录链路不允许随扩展装卸：防锁死守卫（全关拒绝保存）依赖固定 provider 集，装卸语义与之冲突；新增身份源仍是「代码 + 配置」形态                        |
| `internal/api/platform`（HTTP 面）                | 保留过渡        | mapping 定位为过渡层；P1 后其 service 的 provider 寻址改从 driver 运行时表解析，dispatcher 语义（`external.<platform>.<method>`）不变，HTTP 契约不变 |

## 9. 分期实现计划

每期独立 PR，验收门按交付完成定义：`pnpm --dir web run tsc` + `go build ./... && go test ./...` + `scripts/dashboard_vnext_guard.sh` + 涉及渲染时走 accept-and-publish 发布链 + 文档三层同步。

### P0 契约定型与归档（纯代码小改 + 文档，零 schema 变更）

- 范围：`internal/drivers/driver.go` 落 Driver 接口 + 注册表（先空转，driver 实现随 P1 迁入）；`internal/platform/provider` 按裁决处置（Registry/删除、接口注释为蓝本）；manifest provider 块字段定稿并入统一模式文档；顺手修契约基线 §3.1 渠道闭集漂移（文档写 `stable|beta|experimental`、实现是 `stable/beta/alpha`——`service.go:153`，以实现为准改文档）。
- 验收：守卫测试锁 manifest provider 块字段集（对齐 official seed 守卫先例）；全量门禁绿。
- 状态：**已获准开工**（2026-10-10 用户授权代拍：P0 准予动工，契约先行=无风险结构件）。

### P1 external-platform 迁移（mapping 第一优先落地）

- 范围：`internal/platform/openapi` → `internal/drivers/openapi`；`internal/platform/quicksdk` → `internal/extensions/official/externalplatform`；`internal/api/platform` service 的 provider 寻址切 driver 运行时表（dispatcher 与 HTTP 契约不变）；official seed 补 `official.external-platform` 的 provider 块（该条目先于统一模式、未回溯——本批一并补齐）。
- 验收：`externalfunc` 既有测试零改动通过（wire 语义不变的证据）；api/platform 回归；迁移后旧目录删除（禁止兼容双路径，对齐「禁止兼容旧键」纪律）。
- 风险与对策：触碰生产调用链——dispatcher 寻址不变，只换实例来源；用既有 externalfunc/平台域测试对拍，发现行为差异即停。

### P2 pack descriptor 解析与 Store 深化

- 范围：pack 导入解析包内 `descriptors/`、`schemas/` 入 release（若需加列→**编号迁移**，按迁移契约三处同步 + 存量形态回归用例）；Store 详情展示 operations/configSchema；`webhook` driver 按需实现（无诉求则继续预留，不硬造）。
- 验收：导入含 descriptors 的 pack 后 capabilities 端点返回派生 operations；Store 页面渲染用例。

### P3 远期（登记不承诺）

- out-of-process connector 泛化（generic HTTP 外联形态，对应 Grafana 后端插件的最小等价物，仅当出现「外部团队提供 connector」的真实诉求）。
- 签名分级（Grafana Private/Community/Commercial 式；当前 pack 仅 checksum，单公司内部分发暂无签名诉求）。
- cicd / identity 扩展化评估（§8 保留裁决的再评估触发条件：出现第 5 个 cicd kind 或第 6 个身份源时重估）。

## 10. 已知边界（诚实登记）

- V1 无动态代码加载：新增 driver 类型必须平台发版——这是选型代价而非缺陷，由 §2 威胁模型决定。
- provider 实例健康检查沿用 V1 语义判定（enabled→healthy 推导），无主动探测（`extension_health` 表回补路径见安装模型文档 §5）。
- pack 包内 descriptors/schemas 在 P2 前不解析，仅随工件整体存储（契约基线 §7.6 已登记，本文延续）。
- `internal/platform/provider.Registry` 归档后，若有第三方代码已引用该包（仓外不存在；仓内仅测试引用），删除属破坏性变更——在 P1 变更说明中显式声明。

## 11. 设计依据索引

- 调研：[commercial-backend-survey](../research/commercial-backend-survey.md)（商业后台八维清单与插件形态旁证）· [plugin-architecture-survey](../research/plugin-architecture-survey.md)（四系统六问对比与标配三角）
- 架构（现状约束）：[extension-installation-model](../architecture/extension-installation-model.md)（五表/四态/Agent 副本）· [official-extension-unified-pattern](../architecture/official-extension-unified-pattern.md)（三层 capability/config schema/事件前缀）· [extensions-api-contract-baseline](../architecture/extensions-api-contract-baseline.md)（API/错误码基线）· [core-extension-mapping](../architecture/core-extension-mapping.md)（目录目标形态与迁移顺序）
- 代码参照：`internal/cicd/provider.go`（工厂注册表模式）· `internal/core/extension/externalfunc`（binding spec/function ID/capability 现状）· `internal/api/extension/service.go:153`（渠道闭集）
