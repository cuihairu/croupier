---
title: sidecar 外部函数适配插件设计
---

# sidecar 外部函数适配插件

## 状态与结论

- 状态：第一片（最小闭环）已实现——共享常驻宿主引擎上收 `core/pluginhost`，函数插件契约、宿主、声明校验与参考插件落地 `agents/sidecar/funcplugin`，sidecar 装配与派发钩子接通（todo.md:35，2026-10-20）。
- 结论：函数插件与探测插件共享同一 go-plugin 常驻宿主机制（懒启动、常驻复用、每调用 deadline、崩溃分类、有界停机），但**契约、权限面与错误闭集完全分离**；游戏没有管理接口时平台不能凭空造入口，开发者仍须提供 HTTP/TCP/管理接口或 SDK provider，插件只做已有接口的适配。

## 背景与目标

todo.md:35（2026-10-17 用户提出复用 go-plugin 模式）：开发者插件声明函数/schema，并把调用映射到游戏已有 HTTP/TCP/管理接口；sidecar 复用函数注册、权限审批、审计及路由链；插件宿主与探测共享基础机制但接口/权限分离，且均受可选构建（`no_go_plugin`）与运行时开关控制。

设计依据：`docs/zh/research/synthetics-agent-go-plugin.md`（「sidecar 可用同一宿主机制提供可选函数适配插件……探测插件与函数插件分别定义契约及权限」）、`docs/zh/development/agent-runtime-layout.md`（「将通用进程监管/插件宿主能力抽为独立公共包」）。

## 共享宿主机制（core/pluginhost）

探测宿主（`agents/synthetics/pluginhost`）原有的通用常驻引擎——并发槽位背压、崩溃指数退避、连续失败熔断、懒启动、Describe 握手、有界停机排空——原样上收到 `core/pluginhost`（泛型 `Controller[Impl]` + `Contract[Impl]`），探测控制器与函数宿主都消费同一实现，不复制第二份生命周期代码。gRPC 依赖边界从「仅 synthetics 插件宿主子树」扩展为「插件宿主子树（synthetics + sidecar funcplugin + core/pluginhost）」，`scripts/check-no-grpc.sh` 同步收口；游戏 SDK 与 Agent↔Server TCP 链路依旧零 gRPC。

探测宿主保留探测语义（`CheckInput/CheckOutput`、`probe.Result` 映射、原生 HTTP 回退）；函数宿主只取所需子集并在语义上不同：探测是自驱 tick 循环，必须自带退避/熔断；函数调用是同步请求响应，重试节奏由任务派发层（TaskRunner/审批）持有，宿主不做静默重试。

## 函数插件契约（funcplugin.v1）

`agents/sidecar/funcplugin/protocol/funcplugin.proto`，与 `probeplugin.proto` 同轨分版本（本地 go-plugin gRPC IPC，不进顶层 proto/）：

- `Describe`：name/version 握手（与探测一致，宿主启动后即发，元数据随注册上报）。
- `ListFunctions`：插件声明函数集——每条含 `id`、`summary`、`param_schema`/`result_schema`/`error_schema`（JSON Schema Draft 2020-12，请求/响应恒为 JSON 对象）、`risk_level`（low|medium|high|danger 闭集，空按 low）。
- `Invoke(function_id, params_json, timeout_ms)`：单次调用。**业务失败走响应信封（ok=false + error_code/error_message），不返回 gRPC error**；gRPC error 仅用于插件进程级故障（崩溃/断连），由宿主分类成 `plugin_*` 原因。

### 声明校验（declare 子包，纯 Go，两种构建都编译）

- 命名空间：函数 id 必须形如 `plugin.<插件名>.<函数名>`——与 SDK provider 函数、平台函数空间隔离，插件不可能 shadow 既有路由。
- schema 必须是合法 JSON、顶层 `type: object`、单个 ≤64KiB；error_schema 可选，存在时同样校验。
- 插件名匹配 `[a-z0-9][a-z0-9-]*`；函数 id 全局唯一——重复声明（含跨插件撞 id）注册期拒绝并清空本次注册，不做静默 upsert。
- 校验通过的声明经 `declare.Metadata` 构造为 `functionv1.FunctionMetadata`（resource=`plugin.<名>`、tags 带 `plugin`/插件名、execution=sync、audit_log=true、risk 映射），供既有注册/审批/审计链消费。

## 调用映射与错误翻译

映射逻辑在**插件侧**：宿主保持协议无关（不认识 HTTP/TCP），插件是协议适配器，把 `Invoke(params_json)` 翻译成游戏已有接口调用。目标配置（URL/地址、每函数方法与路径、目标超时）由插件自己的启动参数/环境持有，不经 sidecar 配置下发，插件也拿不到 agent 凭据或 server 地址。参考插件 `httpget` 把调用映射为本地 HTTP GET（`GET <base><path>`）；`tcp`（第四片）把调用映射为游戏已有 TCP 管理/文本接口——一次调用一条连接，请求帧 `<path>[ <紧凑 JSON body>]` + 换行，应答读一行（换行或 EOF 结束、1 MiB 封顶）。TCP 协议没有状态码：应答不是 `{"ok":...}` 对象时原样作为数据返回（不臆造无状态协议没有的成功语义），`{"ok":false}` 才译为 `target_rejected`。

错误闭集（设计约束：调用方拿到的永远是可判别、可审计的有限形态）：

| error_code                                                          | 含义                                        | 典型来源 |
| ------------------------------------------------------------------- | ------------------------------------------- | -------- |
| （ok=true）                                                         | 成功，result_json 为游戏接口响应            | —        |
| `target_unavailable`                                                | 游戏接口连不上/超时/5xx                     | 插件翻译 |
| `target_rejected`                                                   | 游戏接口明确拒绝（4xx/业务错误码）          | 插件翻译 |
| `bad_response`                                                      | 接口应答不符合声明契约（不可解析/缺字段）   | 插件翻译 |
| `plugin_unavailable` / `plugin_timeout` / `plugin_error`            | 插件进程崩溃、deadline 内未答、插件内部错误 | 宿主分类 |
| `plugin_not_configured` / `plugin_disabled` / `plugin_not_compiled` | 未声明、运行时/按插件开关关闭、裁剪构建     | 宿主分类 |

自持目标超时：适配器的 socket 等待取 ctx 取消、宿主每调用 deadline、插件自持 `-target-timeout`（同 httpget 的 client timeout）三者最早。没有自持超时的适配器里，卡死目标永远先撞宿主 deadline 报 `plugin_timeout`（宿主级降级），适配器无法报告「游戏接口超时未答」这一业务事实——这是两处超时各司其职的原因。

翻译规则：目标 2xx 且可解析→成功；目标侧连接失败/超时→`target_unavailable`；目标 4xx/业务拒绝→`target_rejected`（携带状态码与响应摘要）；目标 5xx→`target_unavailable`（目标故障，非调用方过错）。宿主侧每调用 deadline = min（调用方预算，插件配置 timeout）；超时经 ctx 取消传播到插件，插件应中止目标调用。

## 权限与作用域边界（不绕开 8508540 防线）

- **scope 归属唯一来源是 agent 配置**。函数插件不声明、不携带自己的 game/env；宿主启动时取 sidecar 已校验的 `gameId/env`，空白即拒绝启动（fail-closed，与 `internal/app/agent.ValidateAgentScope` 同口径）——不存在「插件自带 scope」或「跨 scope 函数」这条路径。
- **函数随 agent 会话 scope 上报与路由**。插件函数与 SDK 函数走同一条注册/派发链：server 按会话的 game/env 绑定派发，sidecar 本地派发钩子只认本 agent 已注册的函数 id，注册表按 id 精确匹配、拒绝跨插件/未注册 id——没有通配路由。
- **命名空间隔离即权限边界的一半**。`plugin.<名>.<fn>` 前缀由宿主强制校验，SDK/平台函数不可能被插件声明覆盖；插件声明冲突（重复 id）在注册期拒绝，不做静默 upsert。
- **权限/审批/审计沿用函数元数据既有面**。声明的 risk_level 映射进 `FunctionSecurity.risk_level`，`audit_log=true` 恒置；高危函数由 server 侧既有审批链把关，插件不提供绕过审批的旁路（宿主只实现 Invoke，不做鉴权判断）。
- **插件是最小信任进程**：由 sidecar 显式配置拉起（不自动发现）、单一管理者（宿主）、不持有 croupier agent 凭据/server 地址/TLS 材料；映射目标由插件本地配置，策略上限定本机/内网游戏接口。gRPC IPC 仅宿主↔插件本机通道，不出主机。

## 失败与降级语义

- 插件进程崩溃：宿主观察 `client.Exited()`，本次调用报 `plugin_unavailable`，进程由宿主重建（函数面复用核心引擎的退避/熔断状态机时同样适用；最小实现按调用分类报错，重试由调用方/任务层决策）。
- 超时：每调用 deadline 到期报 `plugin_timeout`；调用方 context 取消（任务取消、断连）向插件传播，语义与直连 SDK 的 ctx 取消一致，额外开销仅为一次本机 gRPC 往返。
- 注册漂移：宿主每 30s 重跑 `ListFunctions` 并整组替换该插件的注册集（`Start` 为严格变体）；注册集只在拿到完全合法应答时才变化——进程不可达或声明非法都保留上一组、下次派发可见降级；server 侧注册快照整体重建，缺席即撤销（第三片）。
- 停机：`Stop()` 有界排空（5s 窗口）后由唯一管理者 kill 全部常驻插件，不留孤儿子进程。
- 可选构建：`no_go_plugin` 下函数宿主编译为 stub，配置仍声明函数插件时启动即报错（fail-closed，同探测宿主裁剪口径）；运行时总开关 `functions.enabled`（缺省 true，默认集成；`functions:` 段未声明则不托管任何插件）+ 按插件 `enabled`，关闭时不拉起子进程、派发显式报 `plugin_disabled`。

## 与直连 SDK 的对比

功能上插件路径与直连 SDK 等价（同一注册/派发/审批/审计链），差别在：取消/错误语义（插件面为错误闭集信封 + ctx 传播，SDK 面为 proto 响应/错误）、额外 IPC 时延（每次调用多一跳本机 gRPC，参考 go-plugin 实测量级为亚毫秒；精确数字待真实游戏接口基准测试，本切片不伪造测量结论）。游戏没有管理接口时，开发者仍须提供入口（游戏侧 HTTP/TCP 管理接口或 SDK provider 函数），插件只适配不代理不存在的接口。

## 最小实现范围与本片交付

已交付（2026-10-20）：

1. `core/pluginhost`：通用常驻宿主引擎上收，探测控制器改为消费（既有探测测试全数保持通过即行为未漂移的证据）。
2. `agents/sidecar/funcplugin`：契约 proto、宿主（含 no_go_plugin stub）、declare 校验/元数据构造。
3. sidecar 装配与派发：`internal/app/agent` 装配/校验钩子、`internal/agent.LocalHandler` 插件函数派发分支、`cmd/sidecar-agent` 配置段；`App.Stop` 作为宿主唯一生命周期出口。
4. 参考插件 `agents/sidecar/funcplugin/examples/httpget`（HTTP GET 映射）。
5. 配置校验、gRPC 依赖边界守卫扩展、通用引擎与函数宿主集成测试。
6. 第二片：插件函数并入注册上报快照与 server 侧注册表/审批/审计真实接线（`UpstreamClient.SetPluginFunctionsSource` 晚绑定快照源 + `Registered.Descriptor()` risk 归一；store 同名 id SDK/provider 恒赢；server 注册边界锁定插件命名空间与 presentation 拒绝，链路零 server 改动即闭环）。
7. 第三片：注册 reconcile（`Host.Reconcile` + 30s 宿主自持循环，tick 不带自有 deadline；注册集只在完全合法应答时变化，撤销链经 server 快照整体重建闭环）。
8. 第四片：TCP 参考插件 `examples/tcp`（一次调用一条连接的行协议帧约定、错误闭集翻译、自持 `-target-timeout` 与宿主 deadline 双向有界；映射成功、五类失败翻译、双超时均有真实子进程测试）。

剩余（后续切片）：多语言（Python/JS）插件 SDK、IPC 时延与直连 SDK 对比实测、真实游戏接口端到端验收。
