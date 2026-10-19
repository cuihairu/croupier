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

sidecar 可用同一宿主机制提供可选函数适配插件，将函数声明/调用转换成游戏已有 HTTP/TCP/管理接口；探测插件与函数插件分别定义契约及权限，均不要求游戏 SDK 引入 go-plugin/gRPC；额外 IPC 时延需独立测量。

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

本片明确不做（照旧列明）：Python/JS 插件 SDK、ICMP/TCP 探测插件、`no_go_plugin` build tag 裁剪与运行时开关、并发背压与崩溃退避熔断、targetCategory=game/third_party 类别绑定、真实游戏协议目标验收——均为 todo:36 剩余范围。

关联：[Agent 清单](../design/agents-inventory.md)、[健康评审](/archive/legacy-en/design/agent-health-recovery-review)、根目录 `todo.md`。
