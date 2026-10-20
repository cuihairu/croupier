# 外部探测 Agent 与 HashiCorp go-plugin 可行性调研

日期：2026-10-17。状态：用户确认第四类 Agent 需求并指定调研 HashiCorp go-plugin；技术路线可行，尚未实现或执行跨语言原型验证。

## 需求与当前差距

第四类 Agent 沿用已有名称 `synthetics-agent`，在代表玩家或服务消费方的网络位置独立运行，从游戏进程外主动探测游戏及第三方服务；负责拉起、复用、监管开发者实现的探测插件，插件可使用支持所定 IPC 契约的语言，游戏地址、环境、凭证引用、频率与超时由平台配置。

此前记录并非完全缺失：[外部适配器评审](/review/chatgpt-2026-10-14-probe-adapter) 已采纳独立进程、多语言和进程常驻复用，[监管设计](../design/agent-supervisor-design.md) §10 已加入 `kind=probe-adapter` 元数据，但独立 Agent 分类、插件调用运行时和完整待办未落地；`core/healthprobe` 内置 Checker、声明式协议步骤与 Minecraft SLP 不等于外部程序插件实现。

## go-plugin 调研事实

- [官方仓库](https://github.com/hashicorp/go-plugin)：宿主启动子进程并通过本地 RPC 调用，提供握手、协议版本、日志及进程管理相关能力；官方明确定位于可靠的本地连接，不能作为跨机器 Agent→Server 通信框架。
- [官方非 Go 插件指南](https://github.com/hashicorp/go-plugin/blob/main/docs/guide-plugin-write-non-go.md)：跨语言接入使用 gRPC，开发者需实现约定的 protobuf service、gRPC health 服务及 stdout 启动握手，监听本机 TCP 或 Unix socket；“多语言”表示实现共同协议，不能把任意已有可执行文件直接当作插件。
- [官方 Python 示例](https://github.com/hashicorp/go-plugin/blob/main/examples/grpc/plugin-python/plugin.py)：除业务接口与 health 外，演示 Controller Shutdown 及 Stdio 服务；实现应按钉扎版本逐项验证宿主实际使用的控制服务，不能只复制业务 RPC 就宣称兼容。
- 插件健康仅表示插件通信可用，目标游戏或第三方服务健康必须来自业务探测返回；进程隔离本身不提供权限沙箱，取消请求也需要插件实现配合。

## 建议架构

```text
Server ← 共用 TCP/TLS session → synthetics-agent
                                  ├─ core/healthprobe：阈值、窗口、统计
                                  ├─ 内置 HTTP/TCP/DB/声明式 Checker
                                  └─ 插件宿主 ← 本地 go-plugin gRPC → 常驻探测程序
                                                                        ├─ 游戏握手/登录/业务探测
                                                                        └─ 第三方服务检查
```

所有 Agent 的远程网络通信继续共用既有底座；本地子进程 IPC 独立分层。外部插件实现只需提供探测能力，平台调度、权限、故障窗口和上报留在宿主侧；第三方服务按实际消费方视角部署，已有 Agent 仍可复用公共探测内核，新增独立实例用于需要独立网络视角或插件宿主的场景。

## 本仓约束与选型边界

2026-10-17 用户澄清：禁止 gRPC 的原意是避免游戏 SDK 携带过大的运行时与构建依赖，历史 debug 接入场景约 1.7GB；独立外部探测插件不属于该禁用范围，[传输决策](/archive/legacy-en/architecture/transport-no-grpc) 已据此修正，无需再把本地插件 IPC 视为待批准的例外。

按此边界采用 go-plugin 的本地 gRPC 多语言模式，把依赖集中在插件宿主与插件，保持 `core/healthprobe` 的 Checker 抽象及 Agent↔Server↔SDK 的 TCP 契约；实现时统一替换外部适配器 JSONL 候选，不同时维护两份等价协议，保证游戏 SDK 的依赖闭包及产物不引入 gRPC；`scripts/check-no-grpc.sh` 仍是旧全局规则，需随实现同步为依赖边界守卫，不能仅删除检查。

## 待定义的探测契约

- `Describe`：插件身份、版本、支持的探测能力与契约版本。
- `Check`：请求 ID、目标、显式目标类别 `targetCategory=game/third_party`、探测类型、截止时间、参数与限定范围的凭证；返回状态、阶段、时延、错误原因和有界详情，类别由平台目标配置确定，插件不得改写，两类均绑定 gameId/env，展示与统计规则见 [后台页面设计](../design/agent-dashboard-design.md) §4.4。
- 结果区分成功、目标失败、插件错误、未接入、超时与取消；插件错误不得伪装为目标不可用，TCP 可达不得自动提升为游戏登录成功。
- 宿主设置启动超时、并发上限、请求限额、崩溃退避与熔断、日志与输出限制、正常关闭及超时强制清理，插件进程常驻复用。
- 一个插件进程只有一个生命周期管理者，需复用或抽取既有监管能力，不能让 go-plugin 宿主和 supervisor 同时启动、重启同一进程。
- 插件只执行探测，不获得平台控制权；凭证按目标最小范围传递，不写握手、日志或结果详情。

## 最小验证与实施顺序

1. 按用户已澄清的依赖边界细化插件协议、同步守卫，钉扎 go-plugin 版本与插件契约版本。
2. 实现宿主与 Go、Python 两个参考插件，验证握手、复用、超时取消、崩溃恢复、关闭与协议不兼容行为；再按实际需要提供 C#/Java/C++ 模板。
3. 用一个真实游戏协议目标及一个真实第三方 HTTP 服务验证业务结果，覆盖网络故障、业务错误及插件故障三种不同情形。
4. 装配独立 synthetics-agent 的注册、配置、探测调度、结果上行与告警窗口，接入按 game/env 与探测点过滤的面板。

## 默认集成与禁用选项（2026-10-17 用户确定）

用户确定默认集成 go-plugin 与 OTel，均提供禁用选项，覆盖此前默认轻量版建议；默认构建包含插件宿主与 OTel 能力，运行时通过各自总开关控制，并保留插件级启停与 tracing/metrics 子开关；默认包含能力不等于无配置就启动任意插件或向远端发送遥测，插件仅按声明启动，遥测按配置的采集端导出。

编译裁剪采用反向 build tag：`no_go_plugin` 移除插件宿主，`no_otel` 裁剪 OTel SDK/exporter 装配，两者可组合；普通构建无需额外 tag。需明确 `no_otel` 下轻量 API 与 trace-context 保留范围，不能把只关闭 exporter 宣称为移除全部 OTel 依赖，业务指标和 Agent 心跳也不应随外部遥测一起关闭。

| 构建选项（待实现）     | 插件宿主           | OTel SDK/exporter  |
| ---------------------- | ------------------ | ------------------ |
| 默认，无 tag           | 包含，运行时可关闭 | 包含，运行时可关闭 |
| `no_go_plugin`         | 裁剪               | 包含               |
| `no_otel`              | 包含               | 裁剪               |
| `no_go_plugin,no_otel` | 裁剪               | 裁剪               |

运行时 enabled 总关闭必须优先于子开关，避免 tracing/metrics 子开关重新打开已禁用的 OTel；关闭后不得初始化 exporter 或周期导出任务。当前 `internal/telemetry/config.go` 子开关可将 Enabled 设回 true，sidecar CLI 也尚未装配完整遥测配置，需要随本项收口，不能视为现状已满足。

运行时关闭可避免启动子进程和连接资源，但不能据此消除已链接到二进制的实现体积；编译裁剪需将插件实现、注册装配、生成的 gRPC 代码均放在 `!no_go_plugin` 分支，关闭路径使用轻量实现，禁止无条件 blank import 或公共类型引用把依赖拉回；OTel 同样隔离 SDK/exporter 装配与公共调用接口。

未编译插件能力却配置启用时明确启动报错；在线停用时先停止接收新调用，再按定义的超时策略排空/取消在途调用并关闭插件，不能把开关关闭当成无条件杀进程。

sidecar 可用同一宿主机制提供可选函数适配插件，将函数声明/调用转换成游戏已有 HTTP/TCP/管理接口；探测插件与函数插件分别定义契约及权限，均不要求游戏 SDK 引入 go-plugin/gRPC；额外 IPC 时延已实测（见「插件路径 IPC 时延实测」）。

体积应分别记录 Agent 二进制、开发者插件程序及其 Python/.NET/JVM 等运行环境；历史 1.7GB debug 场景不作为 Go 宿主增量估值，跨平台与正式完整实现均需重新测量。

### 宿主体积对照实验（2026-10-17）

环境：Linux/amd64、Go 1.27.2、CGO_ENABLED=1、go-plugin v1.8.0、项目依赖解析得到 gRPC v1.83.2，两组均使用 `-trimpath -ldflags='-s -w'`；这是保留默认优化并去除符号信息的构建，未测 debug 构建。

| 对照对象                           |     不包含 go-plugin 宿主 |          包含宿主连接路径 |                           增量 |
| ---------------------------------- | ------------------------: | ------------------------: | -----------------------------: |
| 当前 sidecar（同源码、同临时模块） | 32,829,703 B / 31.309 MiB | 33,685,767 B / 32.125 MiB | 856,064 B / 0.816 MiB（2.61%） |
| 干净最小 main 程序                 |   1,507,488 B / 1.438 MiB | 13,758,727 B / 13.121 MiB |      12,251,239 B / 11.684 MiB |

实验通过 `/tmp/croupier-plugin-size` 临时 modfile 和 overlay 给 sidecar 增加受 `go_plugin` tag 控制的 `init`，保留 `plugin.NewClient`、`Client()` 和 `Kill()` 路径，仅在实验环境变量为 1 时执行；两个产物均未启动插件，项目源码与 go.mod/go.sum 未改。最小程序使用相同依赖版本，并设置 `-buildvcs=false`。

现有 sidecar 基线的静态导入链已经包含 gRPC：`cmd/sidecar-agent → internal/app/agent → internal/telemetry → otlpmetrichttp → internal/oconf → google.golang.org/grpc`，因此 0.816 MiB 是当前项目已有依赖基础上的增量，不能当作首次引入 go-plugin/gRPC 的成本；基线不包含 go-plugin 包。该导入链也不表示当前 OTel 实际使用 gRPC 上报，更不证明游戏 SDK 的依赖闭包包含它。

本次只测宿主连接代码的链接体积，未实现 Describe/Check 等业务服务，也未计插件程序、语言运行时、部署包压缩体积、运行内存与调用时延；最终默认版需在完整实现后重新测量。实验使用的正向 `go_plugin` tag 仅用于此次对照，不是用户已确定的产品选项；正式实现采用默认集成、反向 tag 裁剪和运行时可关，不构成所有平台固定增加约 12 MiB 的保证。

## 第二片实施记录（2026-10-20，todo:36 部分完成）

裁剪开关、并发背压与崩溃退避熔断已落地。交付物与登记：

| 交付物                                 | 位置                                                                                                |
| -------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `no_go_plugin` 裁剪：宿主 Behind tag   | `pluginhost/{plugin,probe,controller}.go` `//go:build !no_go_plugin`；`pluginhost/stub.go`（tag 内) |
| 裁剪 fail-closed 门禁                  | `config/plugingate_trim.go`（默认构建 `config/plugingate_default.go` 为 no-op）                     |
| 运行时开关（总开关 + 按插件启停）      | `config/config.go`（`pluginsEnabled` 缺省 true，`plugins[].enabled`）、`runner/runner.go`           |
| 并发背压（有界槽位 + 有界排队 + 拒绝） | `pluginhost/controller.go`（`gate` 信号量 + `pluginQueueWindow`）                                   |
| 崩溃退避 + 连续失败熔断                | `pluginhost/controller.go`（`failLocked`/`succeed`/`pluginHealth`）                                 |
| 故障注入旋钮                           | `pluginhost/examples/echoprobe/main.go`（HOLD/CONC/EXIT_AFTER_CHECKS）                              |
| 示例配置注记                           | `configs/synthetics-agent.example.yaml`                                                             |

语义与默认值（均可经 yaml 覆盖）：

- **总开关** `synthetics.pluginsEnabled`：缺省 true（默认集成口径不变）；false 不创建 controller、不拉起子进程，plugin 路由目标直接原生 HTTP 探测并在事件 details 记录 `plugin_disabled`。停用后的在途收尾策略：`Stop()` 先停止接受新调用，再以 5s 有界窗口等待在途调用按各自 per-check deadline 收尾，随后由单一生命周期管理者 kill 子进程——无传统"无条件杀进程"。
- **裁剪**：`no_go_plugin` 构建下 `config.Validate` 对 `synthetics.plugins` 声明直接启动报错（fail-closed）；`pluginhost` 仅编译 stub（`Check` 恒降级 `plugin_not_compiled`），API 与真实 controller 等形供 runner 两态编译。实测裁剪产物 `go list -deps` 闭包无 `hashicorp/go-plugin`、无 `google.golang.org/grpc`（默认构建两者均链接）。
- **背压**：`Check` 先取有界槽（默认 8），排队窗口默认 2s，超窗拒绝 `plugin_backpressure`；调用路径不生成任何 goroutine，排队者是他方调用函数本身，无界 goroutine 不可能出现。排队成功者必须在窗口内拿到释放的槽（有界排队，非必拒）。
- **退避与熔断**：失败（启动失败/调用错误/超时/崩溃）后下一次重启等待 `backoff * 2^(连续失败-1)`，封顶 `backoffMax`；选指数而非固定步进——一次性崩溃首个重试即窗口最小（默认 1s），彻底损坏的插件几何级拉开避免重启风暴，固定步进会让死插件以恒定节奏永久重启；封顶保证最坏恢复延迟有界。连续失败达 `breakAfter`（默认 5）开熔断：开窗 30s 内调用即刻降级 `plugin_circuit_open`、不触碰子进程；窗口到期允许单次探测路径，成功关断并清零计数，失败按原计数重开。
- 新降级原因 `plugin_disabled` / `plugin_not_compiled` / `plugin_backpressure` / `plugin_backoff` / `plugin_circuit_open` 与既有四类前缀同构，`runner.errorClass` 无改动即可分类。

测试（`-race` 全绿；裁剪用例如下带 `-tags no_go_plugin` 运行）：

| 用例                                                           | 面             |
| -------------------------------------------------------------- | -------------- |
| `TestControllerCrashBackoffAndCircuitBreaker`                  | 退避 + 熔断    |
| `TestControllerBackpressureSlotsBoundedRejectionAndQueueing`   | 背压拒绝/有界  |
| `TestControllerBackpressureQueuedCallerGetsFreedSlot`          | 有界排队成功   |
| `TestRunProbePluginRuntimeSwitchTwoStates`                     | 运行时开关两态 |
| `TestControllerPerPluginSwitchDegradesWithoutLaunch`           | 按插件启停     |
| `TestTrimmedRuntimeAlwaysDegrades`（-tags no_go_plugin）       | 裁剪 stub      |
| `TestTrimBuildRejectsPluginDeclarations`（-tags no_go_plugin） | 裁剪门禁       |
| `TestSyntheticsPluginKnobsYamlTwoStatesAndValidation`          | 旋钮 yaml/校验 |

本片明确不做（照旧列明）：Python/JS 插件 SDK、ICMP/TCP 探测插件、targetCategory=game/third_party 类别绑定、凭证边界细化、真实游戏协议目标跨语言验收——均为 todo:36 剩余范围。

## 裁剪构建同参数实测（2026-10-20，todo:32 收口）

第二片交付的真实反向 tag 裁剪补做同参数对照（兑现 2026-10-17 实验遗留的「最终默认版需在完整实现后重新测量」）：同一源码（8508540）、同一平台 linux/amd64、go1.27.2、相同 `-trimpath` 与固定 `-ldflags "-s -w -X …Version=…GitCommit=…BuildTime=…"`，唯一变量是 `-tags no_go_plugin`：

| 对照对象                 | 体积                                      |
| ------------------------ | ----------------------------------------- |
| 默认版（含插件宿主）     | 15,360,263 B / 14.652 MiB                 |
| 裁剪版（`no_go_plugin`） | 9,466,119 B / 9.029 MiB                   |
| 裁剪增量（默认 − 裁剪）  | 5,894,144 B / 5.622 MiB（占默认版 38.4%） |

- **裁剪版依赖闭包**：`go list -deps -tags no_go_plugin ./cmd/synthetics-agent` 无 `hashicorp/go-plugin`、无 `google.golang.org/grpc`（默认版两者均链接，grpc 相关依赖 75 个包）；裁剪产物二进制 strings 0 命中 `go-plugin` / `google.golang.org/grpc`（默认版分别 325 / 2319 命中），且裁剪版仍可正常启动（`--help` rc=0），证明 5.622 MiB 增量即插件宿主 + gRPC 传输栈本身。
- **插件程序与语言运行时独立体积**：Go 参考插件 `echoprobe`（`-trimpath -ldflags "-s -w"`）13,984,007 B / 13.336 MiB——Go 插件静态内嵌语言运行时，无独立解释器/VM 需要部署；Python 插件程序本体为 KiB 级脚本，解释器由部署环境提供，运行时依赖体积与常驻内存实测见下方「Python 插件 SDK 交付」；JS 插件 SDK 探测面与函数适配面均已交付（依赖体积与常驻内存实测见「Node.js 插件 SDK 交付」）。
- 2026-10-17 的正向 tag 对照表保留为历史实验记录；本节为正式实现（反向 tag + 运行时开关）的交付口径。

## 第一片实施记录（2026-10-20，todo:36 部分完成）

已落地最小可用插件链路：synthetics-agent 插件宿主 + Go 参考插件，经 go-plugin 本地 gRPC 打通 Describe/Check、结果进入既有 AlertEvent/probe_events 链路。交付物与登记：

| 交付物                                                           | 位置                                                                                  |
| ---------------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| 插件契约（proto + 生成代码）                                     | `agents/synthetics/pluginhost/protocol/`（独立于顶层 `proto/`，分轨版本化）           |
| 宿主：发现/加载/调用/超时/降级                                   | `agents/synthetics/pluginhost/controller.go`、`probe.go`、`plugin.go`                 |
| 插件配置与校验（显式声明，不自动发现可执行文件）                 | `agents/synthetics/config/config.go`                                                  |
| runner 路由与显式回退（插件故障 → 原生 HTTP 探测，原因记录在案） | `agents/synthetics/runner/runner.go`                                                  |
| Go 参考插件（echo/固定结果，环境变量注入故障）                   | `agents/synthetics/pluginhost/examples/echoprobe/`                                    |
| 全局禁用守卫 → 依赖边界守卫                                      | `scripts/check-no-grpc.sh`（gRPC import 仅允许 `agents/synthetics/pluginhost/` 子树） |

钉扎版本与许可（`go.mod` 登记）：

| 依赖                             | 版本    | 许可       | 用途                                   |
| -------------------------------- | ------- | ---------- | -------------------------------------- |
| `github.com/hashicorp/go-plugin` | v1.8.0  | MPL-2.0    | 插件宿主运行时（握手、子进程生命周期） |
| `github.com/hashicorp/go-hclog`  | v1.6.3  | MIT        | go-plugin 内部日志（indirect）         |
| `github.com/hashicorp/yamux`     | v0.1.2  | MPL-2.0    | go-plugin 内部多路复用（indirect）     |
| `google.golang.org/grpc`         | v1.83.2 | Apache-2.0 | 插件 IPC 传输（仅宿主边界内）          |
| `github.com/oklog/run`           | v1.1.0  | Apache-2.0 | go-plugin 内部（indirect）             |

本片语义决定：插件按 `synthetics.plugins[]` 显式声明，目标经 `plugin:` 字段路由；插件不可用/超时/调用错误时返回降级原因（`plugin_not_configured` / `plugin_unavailable` / `plugin_timeout` / `plugin_error`），同一目标自动回退原生 HTTP 探测并在事件 details 记录 `pluginFallback`/`pluginCause`，插件进程常驻复用、故障后由宿主单一管理者重启。测试见 `pluginhost_test.go`（加载成功/缺失降级/超时降级/结果回传）与 `runner_plugin_test.go`（插件结果进事件链/缺失回退）。

本片明确不做（照旧列明）：Python/JS 插件 SDK、ICMP/TCP 探测插件、`no_go_plugin` build tag 裁剪与运行时开关、并发背压与崩溃退避熔断、targetCategory=game/third_party 类别绑定、真实游戏协议目标验收——均为 todo:36 剩余范围（其中裁剪/开关/背压/退避已由上方第二片记录交付）。

## Python 插件 SDK 交付（2026-10-20，todo:38/40 共尾项）

探测面与函数面共用一个 Python 包 `croupier-plugin-sdk`（`sdks/plugins/python/`，与游戏 SDK 目录 `sdks/python/` 分离，互不携带依赖），首次给出跨语言插件链路的完整实测：

- **协议面**（`croupier_plugin/serve.py`）：按钉扎 go-plugin v1.8.0 逐项实现的子进程半协议——魔法 cookie 精确值门禁（缺失/不符即拒绝直接执行、exit 1）、`PLUGIN_PROTOCOL_VERSIONS` 协商（不在宿主可讲版本集合内即拒启）、stdout 单行握手 `1|1|unix|<sock>|grpc|`（`PLUGIN_MULTIPLEX_GRPC` 置位时追加 `|true`）、GRPCStdio 停泊流 / GRPCController Shutdown / grpc.health.v1 三个宿主必用控制服务、SIGTERM/SIGINT 有界排空（2s grace）。stdout 只承载握手行，插件日志走 stderr 由宿主回收。
- **契约面**（`funcplugin.py` / `probeplugin.py`）：`FunctionPlugin`（describe/list_functions/invoke）与 `ProbePlugin`（describe/check）两个 ABC；错误码闭集常量与 `error_result` 本地校验（越集即抛，先于宿主拒绝暴露）；`InvokeInput.context` / `CheckInput.context` 暴露 RPC 取消（长等待必须轮询 `is_active()`，与 Go 适配器自持超时同语义）。
- **跨语言实证**（`hostdriver/`）：Go 程序驱动**真实**宿主（`funcplugin.NewHost` 与 `pluginhost.NewController`，非桩非 mock）跑七个场景——func-register / func-invoke / func-envelope（五类业务失败全部留在闭集信封）/ func-timeout（短宿主 deadline 报 `plugin_timeout`）/ probe-check-ok / probe-check-fail / probe-version，全部 PASS。pytest 共 24 用例（握手子进程 7 + 契约映射 17 中含集成 7）。
- **CI**：`.github/workflows/ci-sdk-plugins-python.yml`（matrix 3.12/3.13，`pip install -e '.[dev]'` + setup-go 编译 hostdriver 跑集成）。
- **运行时体积实测**（兑现上方 todo:32 补记承诺；linux/amd64、CPython 3.14）：运行时依赖 `grpcio==1.84.0 + grpcio-health-checking + protobuf==7.36.2` 共 34 MiB site-packages（含各平台原生 .so 轮子，实际部署可裁剪）；插件进程握手就绪态常驻 RSS ≈ 2.1 MiB / VSZ ≈ 7.7 MiB（grpc 原生库按需分页，流量态会增长，此为空闲基线）。对照：Go 参考插件 13.336 MiB 静态自含、零部署依赖；Python 面把「单插件部署体积」换成「每机一次解释器 + 依赖环境」。
- **已知边界**：协议协商只讲 v1（宿主契约也只有 v1）；GRPCStdio 停泊流不走转发（插件日志直写 stderr，宿主同样回收）；multiplex 标志按协议回显但 Croupier 宿主不启用多路复用；stub 为提交态、由 `protos/gen.sh` 重生（不入 `make proto` 主链）；JS 插件 SDK 已交付探测与函数适配两面（见下节）。

## Node.js 插件 SDK 交付（探测面 + 函数适配面）（2026-10-20，todo:40 剩余子项）

`sdks/plugins/js/`（`croupier-plugin-sdk-node`，与游戏 SDK 目录 `sdks/js/` 分离，互不携带依赖）镜像 Python 面的探测与函数适配两个契约，jest 57 用例 + 真实宿主七场景全绿：

- **协议面**（`src/serve.ts`）：同一 go-plugin 子进程半协议——魔法 cookie 精确值门禁（直接执行拒绝、exit 1）、`PLUGIN_PROTOCOL_VERSIONS` 协商（v1）、stdout 单行握手、GRPCStdio 停泊流 / GRPCController.Shutdown（先回包再拆，50ms 间隔防误杀在途回复）/ grpc.health.v1 SERVING、SIGTERM/SIGINT 有界排空（2s grace）。**绑定走 TCP**：grpc-js 无 unix socket 支持，默认 `127.0.0.1:0`（`CROUPIER_PLUGIN_BIND` 可覆盖），握手行声明 `tcp|<addr>`——go-plugin 按握手行拨号，宿主侧插件配置与 Go/Python 插件完全一致。
- **契约面**：探测（`src/probeplugin.ts`）`ProbePlugin`（describe/check）与 `ProbeCall`（target/timeoutMs/payloadJson + isCancelled/remainingMs 取消视图）、`passResult`/`failResult`；函数适配（`src/funcplugin.ts`）`FunctionPlugin`（describe/listFunctions/invoke）与 `InvokeCall`（functionId/paramsJson/timeoutMs + 同一取消视图）、`okResult`/`errorResult`、错误码闭集 `target_unavailable|target_rejected|bad_response` 与风险级闭集 `low|medium|high|danger`（默认 low）、id 命名空间 `plugin.<plugin-name>.<function>`。两面 cookie 独立（`CROUPIER_SYNTHETICS_PLUGIN` / `CROUPIER_SIDECAR_FUNC_PLUGIN`），公共出口将函数面 cookie 别名为 `FUNC_COOKIE_KEY`/`FUNC_COOKIE_VALUE`。业务失败必须留在闭集信封、禁止抛异常（与 Python/Go 语义一致）；`check`/`invoke` 可异步——grpc-js 无线程池，长等待必须 `await` 分片定时器而非阻塞事件循环（语义与 Python 对齐，实现形态不同）。
- **运行时加载与防漂移**：契约经 `@grpc/proto-loader` 从 `protos/` 运行时动态加载（无代码生成步骤）；`scripts/verify-proto-drift.js` 断言 `probeplugin.proto`（权威源 `agents/synthetics/pluginhost/protocol/`）、`funcplugin.proto`（权威源 `agents/sidecar/funcplugin/protocol/`）、`plugin_stdio.proto`/`plugin_controller.proto`（权威源 Python SDK 镜像）字节一致，`health.proto`（grpc 上游拷贝，grpc-js 不带健康服务）做 shape 断言。
- **跨语言实证**（hostdriver 上移为共享 `sdks/plugins/hostdriver/`，Python 集成测试同 PR 改路径）：jest 57 用例 = 握手/协商/握手行单测 + 两面契约映射单测 + 进程内全协议 e2e（cookie 拒绝 → 握手行 → health/Describe/Check/Invoke 通过与失败双向 → Shutdown 有界终止）+ 真实宿主七场景：probe-check-ok / probe-check-fail / probe-version（真实 `pluginhost.NewController`）与 func-register / func-invoke / func-envelope（五类业务失败全部留在闭集信封）/ func-timeout（短宿主 deadline 报 `plugin_timeout`）（真实 `funcplugin.NewHost`），均驱动编译后的 node 参考插件；另有无 cookie 直接执行拒绝。Python 面 24 用例回归照常全绿。
- **CI**：`.github/workflows/ci-sdk-plugins-js.yml`（node 24，`pnpm/action-setup` + `--frozen-lockfile`，build/typecheck/verify:proto/test 四道，setup-go 编译 hostdriver 跑集成；paths 覆盖 js 与共享 hostdriver）。
- **运行时体积实测**（linux/amd64、Node 24.21.0）：运行时依赖 `@grpc/grpc-js + @grpc/proto-loader + protobufjs(+long)` 共约 2.7 MiB node_modules（对比 Python grpcio+protobuf 原生轮子 34 MiB；typescript/jest 等 dev 工具链另有 ~52 MiB 不随部署）；握手就绪态常驻 RSS ≈ 74 MiB / VSZ ≈ 1.49 GB（V8 虚拟地址空间预留，属 node 进程基线）。对照 Python 面 RSS ≈ 2.1 MiB——JS 面以更高的常驻内存与更重的运行时换来更小的依赖盘载。
- **已知边界**：协议协商只讲 v1（宿主契约也只有 v1）；multiplex 标志按协议回显但 Croupier 宿主不启用；绑定仅 TCP（见上）；GRPCStdio 停泊流不走转发（插件日志直写 stderr，宿主同样回收）；宿主侧 sidecar 函数插件配置的消费路径（配置面/下发链）不在本 SDK 范围。

## 非 HTTP 目标参数与 TCP 参考插件（2026-10-21，todo:40 剩余子项）

`synthetics.targets[].payloadJson` 现在是插件目标的显式参数面：YAML 解析后由真实 `pluginhost.NewController` 传入 `CheckRequest.payload_json`，并按 JSON 语法校验（`agents/synthetics/config/config.go`、`agents/synthetics/pluginhost/controller.go`）。该字段只服务插件；原生 HTTP 回退不解释它，降级目标仍使用既有 `url/method/expectedStatus/bodyContains` 回退契约。目标参数不是凭据：连接串、令牌和密钥必须继续由部署环境注入给插件进程，禁止写入提交配置或事件详情。

Python SDK 新增 `examples/probe_tcp.py` TCP 参考插件：接受 `{"host":"127.0.0.1","port":<1-65535>}`，用配置截止时间内的一次 TCP connect 返回连通/失败，长等待轮询 RPC cancellation。其语义刻意止步于网络可达，不把端口连通提升为游戏登录或业务成功；`sleep_ms` 旋钮仅用于跨语言注入取消用例。

跨语言实证扩展共享 `hostdriver -mode probe -scenario probe-tcp`：先监听临时本地端口断言成功结果与 detail，再释放端口断言 connection refused 是目标失败且不产生 `plugin_*` 降级，最后用 80ms 截止时间加 1s 插件等待断言 `plugin_timeout`。该场景由 Python 25 用例与 JS 58 用例的真实宿主集成套件共同驱动（宿主 Go、插件分别 Python/Node）。

## 插件路径 IPC 时延实测（2026-10-21，todo:38 剩余子项）

测量工具 `hostdriver -mode funcbench -scenario bench-invoke`（`sdks/plugins/hostdriver/`，共享给三语言 SDK 套件；jest 仅断言输出形态，不跑计时）：同一进程内先跑 **inline** 基线（参考插件 echo 语义的进程内 JSON parse/校验/marshal，代表零 IPC 的调用工作本身），再拉起**真实** `funcplugin.NewHost` 顺序驱动参考插件跑 **plugin**（宿主派发 + 本地 go-plugin gRPC 往返 + 插件 echo 工作），n=300 次顺序调用、20 次预热，输出每调用 mean/p50/p95/max。两条路径的上游链（注册/审批/审计/路由）同链（todo:38 第二片实证），故插件路径的额外时延即此 hop 差值。

实测（共享开发机 linux/amd64，顺序负载，两次独立运行展示抖动；Python 为 venv CPython 3.14.4 + grpcio 1.84 + protobuf 7.36，JS 为 Node 24.21.0）：

| 路径                                     | run1 mean / p50 / p95 / max             | run2 mean / p50 / p95 / max      |
| ---------------------------------------- | --------------------------------------- | -------------------------------- |
| inline 基线（进程内 echo，两语言同量级） | 0.002 / 0.001 / 0.004–0.005 / ≤0.031 ms | —                                |
| plugin hop（Node 24 + grpc-js）          | 0.573 / 0.534 / 0.789 / 2.282 ms        | 0.773 / 0.719 / 1.107 / 3.304 ms |
| plugin hop（Python + grpcio）            | 0.583 / 0.560 / 0.703 / 2.033 ms        | 0.593 / 0.578 / 0.726 / 1.484 ms |

结论与边界：**每次经插件适配器的函数调用额外 ≈0.5–0.8ms（p50 0.53–0.72ms）**，两语言差异在共享开发机运行间抖动内（开销由本地 gRPC hop 与宿主派发主导，不是解释器）；对照 inline 的 ~2µs，插件路径约放大 250–400 倍——对管理面/探活类低频调用无感，对高频批量路径应在插件侧做批量聚合或改走直连 SDK。边界：共享开发机、顺序调用、含宿主派发与框架开销，非吞吐上限（并发与专用硬件下另行测量）；未覆盖完整部署链端到端（SDK→agent→sidecar→插件的跨进程叠加），该验收归 todo:38 剩余的「真实游戏接口端到端验收」。

关联：[Agent 清单](../design/agents-inventory.md)、[健康评审](/archive/legacy-en/design/agent-health-recovery-review)、根目录 `todo.md`。
