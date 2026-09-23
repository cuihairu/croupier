# Agent 能力 → 成熟库盘点矩阵

> 状态：**待拍板**（本文只做盘点与迁移路径设计，未动任何代码）。
> 方法：以代码实际实现为准逐文件梳理（`internal/agent/`、`internal/app/agent/`、`internal/transport/`、`pkg/protocol/`、`internal/platform/{agentlocal,openapi,ratelimit,tlsutil}`、`internal/devcert`、`cmd/agent/`），候选库逐一核实维护活跃度（star/最近 release/下游采用/打包状态），核实时间 2026-09-23。
> 结论口径：**换**（有明确更优的成熟库，附迁移路径）、**留**（自研是合理终态，附理由）、**补**（能力缺失且该用成熟库引入）。
> 「三个月无维护 = 不合格，宁可留自研」规则严格执行；对「完成态小库」的例外讨论见各项说明。

## 为什么做这次盘点（总述）

agent 测试与线上事故暴露的 bug 集中在**自研基础设施的边角**：transport 层 JSON/proto 混层错误（19 函数同步失败事故，942e1bf4e/4a9a67cd6 两次修复）、五处注释自证的历史数据竞争（app.go:47-50、upstream.go:31-32、ops_server.go:42-44/410-414、tcp_local_listener.go:197-201）加一处 RWMutex 误用 fatal、重连退避三套写法行为不一、限流窗口慢机 flaky。这些 bug 的共同形态是：**手写轮子把成熟领域的问题（退避调度、并发停机、限流边界）重新做了一遍，然后在自己的实现里重新踩了业界踩过的坑**。

但盘点同样发现：agent 的**协议层自研（分帧/协议头/MuxConn）不是轮子，是产品契约**——六语言 SDK 都实现了这套 wire 协议（`docs/architecture/sdk-wire-protocol.md`），换掉它等于全 SDK 重写。所以本次盘点的正确产出不是「全面换库」，而是**把「领域问题」交给成熟库（退避/限流/校验），把「产品契约」留在自己手里（协议/会话/调度语义）**，同时清理自研内部的已知瑕疵。

## 一、矩阵总表

| #   | 能力                            | 当前实现（文件:行）                                                                                                                               | 成熟库候选                                           | 活跃度证据                                                                                | 许可证            | 结论                               |
| --- | ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------- | ----------------------------------------------------------------------------------------- | ----------------- | ---------------------------------- |
| 1   | 重连/重试退避                   | `internal/app/agent/upstream.go:317-349`（固定 5s 轮询）、`464-497`（手写 200ms→2s 翻倍）、`openapi/provider.go:843-856`（线性 i+1 秒）——三套写法 | cenkalti/backoff/v5                                  | v5.0.3（2025-07-23），约 3 万仓库依赖，Debian/Guix 打包中                                 | MIT               | **换**                             |
| 2   | 令牌桶限流                      | `internal/platform/ratelimit/tokenbucket.go`（73 行）+ `advanced.go`（570 行），自写 channel 实现                                                 | golang.org/x/time/rate                               | Go 官方扩展仓库（golang.org/x），Google 维护，官方 wiki 推荐的限流实现                    | BSD-3             | **换**                             |
| 3   | OpenAPI schema 推导             | `internal/platform/openapi/input_schema.go`（207 行）+ `rest_classifier.go`（155 行）+ `validator.go`（179 行），手写 3.0.3 推导                  | getkin/kin-openapi                                   | v0.144（本仓 go.mod 现用），支持 3.0/3.1/3.2，CVE-2025-30153 已修                         | MIT               | **换（对拍后）**                   |
| 4   | JSON Schema 校验                | 无（InvokeResponse 数据不校验直接透传）                                                                                                           | santhosh-tekuri/jsonschema/v6                        | v6.0.2（本地 go.mod），draft 4–2020-12 全覆盖，oapi-codegen 采用                          | Apache-2.0        | **补**                             |
| 5   | 本地网关 TLS                    | 配置死项：`cmd/agent/root.go:124-131` 定义 `AgentTLSConfig`，但 `internal/agent/tcp_local_listener.go` 未接线                                     | （无需库，stdlib `crypto/tls` 已在用）               | —                                                                                         | —                 | **补（接线，非引库）**             |
| 6   | TCP 分帧 + 协议头               | `internal/transport/tcp/framing.go:14-50`（50 行）、`pkg/protocol/message.go`（296 行）                                                           | 无合格候选（protobuf-framed 自定义属产品契约）       | —                                                                                         | —                 | **留**                             |
| 7   | 双向多路复用（MuxConn）         | `internal/transport/tcp/mux_conn.go:62-489`（489 行，双车道派发）                                                                                 | hashicorp/yamux、xtaci/smux                          | yamux 0.1.2（2025-11 Debian 标记新版）、libp2p 标配；smux v1.5.x 活跃（2026-03 引用更新） | MPL-2.0 / MIT     | **留（候选记录在案）**             |
| 8   | 负载均衡（粘性哈希）            | `internal/agent/local_handler.go:373-426,749-759`（FNV-1a 取模，~70 行）                                                                          | buraksezer/consistent 等一致性哈希库                 | 未专项核实（弱候选）                                                                      | —                 | **留**                             |
| 9   | crontab/systemd timer 解析      | `internal/app/agent/systemd_parse.go`（237 行，只读枚举）                                                                                         | robfig/cron/v3 parser                                | **v3.0.1 后 2020 年至今零 release**，50+ open PR 积压——按规则不合格                       | MIT               | **留**                             |
| 10  | YAML 配置加载 + 旧键兼容        | `cmd/agent/root.go:78-465`（~390 行，四套 UnmarshalYAML 兼容层）                                                                                  | knadh/koanf v2（MIT，活跃）、spf13/viper（活跃，重） | koanf v2 活跃（pkg.go.dev 文档持续更新）                                                  | MIT               | **留（只减不增）**                 |
| 11  | 日志                            | stdlib slog + lumberjack 轮转（`internal/cli/common/logging.go`）                                                                                 | 现用组合已是生态标准                                 | lumberjack 为 Go 生态日志轮转事实标准                                                     | MIT（lumberjack） | **留**                             |
| 12  | TLS/开发证书                    | stdlib crypto/tls + `internal/devcert/devcert.go`（227 行自签 CA）                                                                                | 无嵌入式等价物（mkcert 是 CLI 非库）                 | —                                                                                         | —                 | **留**                             |
| 13  | 进程管理（OpsServer）           | `internal/app/agent/ops_server.go`（604 行，os/exec + allowlist + 300s 上限 + 托管进程启停）                                                      | 无统治级 Go 进程管理库                               | —                                                                                         | —                 | **留（内部瑕疵单独修）**           |
| 14  | HTTP 客户端（openapi provider） | `internal/platform/openapi/provider.go`（1115 行，stdlib net/http）                                                                               | go-resty/req                                         | 活跃（生态通用）                                                                          | MIT               | **留（重试部分随 #1 换 backoff）** |
| 15  | 会话/注册表                     | `internal/agent/provider_session.go`（210 行）与 `internal/transport/session/*`（378 行）**两套并存**                                             | （内部收敛问题，非换库）                             | —                                                                                         | —                 | **留 + 内部收敛**                  |
| 16  | 序列化                          | protobuf（google.golang.org/protobuf v1.36.12）+ 零散 JSON 混层点（`mux_conn.go:193,422`、`local_handler.go:422,509-519`）                        | 现用 protobuf 即正解                                 | protobuf 官方 Go 运行时                                                                   | BSD-3             | **留（混层点随 #4 收口）**         |

agent **不存在**且**不应引入**的能力（用户清单里与 croupier agent 定位无关的项）：浏览器自动化、截图/视觉、跑测试并解析、Git 操作、任意文件读写 API。croupier agent 是 GM 基础设施代理（游戏网络内的函数注册/调用/任务/运维），上述能力属于通用自动化 agent 的能力面，引入只会扩大攻击面（ExecuteCommand 已有 allowlist 门控就是为此）。这是边界决策，不是能力缺口。

## 二、逐项论证

### 1. 重连/重试退避 → 换（cenkalti/backoff/v5）

**为什么是问题**：同一份代码里存在三种退避——上游重连是固定 5s 轮询（无指数、无 jitter），注册同步是手写 200ms→2s 翻倍，OpenAPI provider 重试是线性 i+1 秒。三种写法行为不一致本身就是维护负担，而固定 5s 轮询有一个具体的故障放大器风险：server 重启时，N 个 agent 会以同相位 5s 间隔同时重拨（无 jitter 防惊群），这正是记忆中「双实例 registry 视图分裂」「cpp demo 重连挂起」一类现场的重连压力来源。手写翻倍退避则没有上限抖动，长故障下退避到 2s 后开始固定频率打点。

**为什么是这个库**：cenkalti/backoff 是 Go 退避的事实标准（本仓 go.mod 已有 v5 间接依赖，零新增引入成本），NewExponentialBackOff 给出初始间隔/倍率/最大间隔/随机化四个旋钮，`NextBackOff()` + `ctx` 组合覆盖现有三处场景。约 3 万个仓库依赖、2025-07 仍在发版、Debian/Guix 打包中，活跃度无争议。

**迁移路径**（风险低，接口内聚）：

1. `upstream.go` 的 `reconnectLoop`：`backoff.NewExponentialBackOff`（InitialInterval=5s，Multiplier=1.5，MaxInterval=60s）+ `backoff.WithContext` 保留 ctx 取消语义；重连成功后 Reset。jitter 默认随机化即获得防惊群。
2. `upstream.go` 的 `syncWithRetry`：同库替换手写翻倍（InitialInterval=200ms，MaxInterval=2s，保持现有上限语义不变）。
3. `openapi/provider.go:843-856` 重试循环：线性 i+1 秒换 `backoff.ConstantBackOff` 起步（行为等价），后续若要指数化单独评估。
4. 测试只增不减：现有退避相关用例保留，新增「退避序列符合指数上界」「ctx 取消即时退出」两组。

**风险**：重连时序变化可能触发既有依赖「5s 固定」假设的用例/运维脚本——迁移 PR 中 grep 掉所有硬编码 5s 断言；`TestGoMigrations` 类冒烟不受影响。

### 2. 令牌桶限流 → 换（golang.org/x/time/rate）

**为什么是问题**：`ratelimit/tokenbucket.go` 73 行自写 channel 令牌桶 + `advanced.go` 570 行。本仓已知 `TestSlidingWindowWaitVariants` 在慢机上反复 flaky（记忆档案在案）——自写限流的等待语义对时序敏感，这是「领域问题被手写后重踩业界坑」的直接证据。x/time/rate 的 Limiter 用单调时钟记账而非 channel 计数，慢机上不存在同样的等待窗漂移形态。

**为什么是这个库**：golang.org/x 是 Go 官方扩展标准库，Google 维护、永续活跃，BSD-3 无许可负担；官方 Go wiki 明确推荐 token bucket 场景用它。`Limiter.WaitN/Reserve/AllowN` 覆盖 agent openapi provider 的三类调用形态（阻塞等待/预检/直接放行）。它不覆盖滑动窗口语义——`advanced.go` 里若有窗口类逻辑（如每分钟配额），保留窗口层、只把桶层换成 x/time/rate，两层职责反而更清晰。

**迁移路径**：

1. 盘点 tokenbucket/advanced 的全部调用方（openapi provider 限流、ops 面），列出现有语义矩阵（burst、等待、拒绝）。
2. 桶层替换为 `rate.NewLimiter(rate.Limit(rps), burst)`，等待路径用 `Wait(ctx)`，非阻塞路径用 `Allow()`。
3. 新增用例对拍：同参数下新旧实现的前 N 次放行序列一致（锁行为契约），慢机 flaky 用例改注入点驱动（对齐 `prefer-injection-over-timing-tests` 记忆准则）。
4. 旧实现删除前用废弃注释挂一个版本周期。

**风险**：行为差异集中在「边界突发」形态——对拍用例就是为此设的；窗口语义保留在原层，不影响外部可观测节奏。

### 3. OpenAPI schema 推导 → 换（对拍后；getkin/kin-openapi）

**为什么是问题**：手写 207+155 行推导 3.0.3 spec 的 schema→函数签名映射。OpenAPI 的边角（allOf/oneOf 组合、nullable、递归 $ref、default 与 example 优先级）手写推导必然覆盖不全；游戏方 spec 的多样性不可控，每接入一家新游戏就可能新增一个推导 bug。kin-openapi 是本仓 go.mod **已有**依赖（server 侧在用，本地 cache 有 v0.137/v0.144），支持 3.0/3.1/3.2，2025 年 CVE 修复响应快（CVE-2025-30153 已出补丁版）。

**为什么标「对拍后」**：这不是纯内部实现替换——推导结果直接决定注册到平台的函数契约（inputSchema），行为变化会传导到 UI 表单生成与 SDK 校验。必须先建对拍集：收集现有全部 providers.yaml 对应的 spec，双实现对拍推导结果，差异逐个人工裁定（旧对/新错→新实现补，旧错/新对→记录为修复），对拍全绿再切换。

**迁移路径**：建对拍集（现有 spec 快照进 testdata）→ 引 kin-openapi 解析层重写 input_schema 推导 → 对拍 → 切换 → 旧推导代码删除。validator.go 的业务校验（平台约束）保留，只替换「spec 解析+schema 遍历」层。

### 4. JSON Schema 校验 → 补（santhosh-tekuri/jsonschema/v6）

**为什么缺**：agent 把 provider 返回的 InvokeResponse 数据直接透传 server/web，无 schema 级校验；proto/JSON 混层点（`mux_conn.go:193,422` 把 `{"error":...}` JSON 串塞进 proto payload）说明「结构化输出的边界校验」本来就缺位——19 函数同步事故的根因之一就是错误形态混进了正常数据通道。OpenAPI 3.1 支持落地后（#3），response schema 校验有了权威来源，补上即闭环。

**为什么是这个库**：v6 是 Go 生态唯一活跃维护且覆盖 draft 4–2020-12 的实现（gojsonschema 停在 draft-07 已实质死项目），oapi-codegen 生产采用，本仓 go.mod 已有（server 侧在用），Apache-2.0。与 kin-openapi v0.136+ 内部同源（kin 的 3.1 支持就构建在它上面），两库组合无额外概念成本。

**迁移路径**：InvokeResponse 落 schema 校验（有 response schema 时启用、无 schema 时跳过——兼容既有 provider）→ 校验失败走既有错误上报通道（不静默吞）→ 新增正反用例。

### 5. 本地网关 TLS → 补（接线，非引库）

`AgentTLSConfig` 配置面存在（root.go:124-131）但 `tcp_local_listener.go` 只消费 Address/RecvTimeout/SendTimeout——配置项是死的，用户配置了也不生效，这属于「配置承诺与实现不符」，边界诚实原则要求补接线或显式移除配置项。实现只需 stdlib crypto/tls（出站方向已有同款用法，client.go:165-196 可照抄模式）。二选一：接线（推荐，SDK↔Agent 本地明文 TCP 在共享主机上确有暴露面），或从配置结构删除该字段并在文档标注边界。**建议接线**，因为协议帧格式不受影响（TLS 在 TCP 之上、分帧之下），六语言 SDK 只需支持 TLS 拨号选项。

### 6. TCP 分帧 + 协议头 → 留

50 行分帧 + 296 行协议头看似「轮子」，实为**产品契约**：六语言 SDK（go/python/java/js/cpp/csharp）各自实现了同一 wire 协议，`docs/architecture/sdk-wire-protocol.md` 是它的规范文档。替换候选不存在——任何第三方分帧（gRPC framing、msgpack-rpc 等）都是协议变更，成本是全 SDK 重写+灰度兼容期，收益是负的（现有协议简单、已被六种语言验证）。留，且这部分的历史 bug（19-sync）修在混层语义而非帧格式本身。

### 7. MuxConn 双向多路复用 → 留（候选记录在案）

yamux（hashicorp，libp2p 标配，0.1.2 活跃）和 smux（xtaci，v1.5.x 活跃）都合格且成熟。**仍建议留**的理由：MuxConn 不是 stream 多路复用，是 **request/response 语义**复用——pending map + reqID 关联、控制车道 1 worker 永不拒、业务车道有界队列饱和时回 busy 帧触发对端 failover，这套语义与「server 广播/agent 单播/failover」的 GM 调用模型深度耦合。yamux/smux 提供 stream 抽象，用它意味着在 stream 之上再建一层 request/response 关联——等于把 MuxConn 的复杂度换个地方重写，还要六语言 SDK 同步。线上验证成本（19-sync 事故后已收敛）会全部清零重来。候选记录在案：若未来出现「单连接带宽瓶颈」或「百级 stream 并发」需求，再评估 smux v2 作为底座。顺手清理：`mux_conn.go:44-54` 自写 `errorAs` 删除，直接用 `errors.As`。

### 8. 负载均衡 FNV 粘性哈希 → 留

70 行 FNV-1a 取模选实例。一致性哈希库（buraksezer/consistent 等）解决的是「节点池动态伸缩时键的大规模迁移」，agent 场景是**单游戏服务几个实例**（注册表按 `(game_id, function_id)` 索引），粘性即可，无需迁移最小化。换库是拿大炮打蚊子，还引入依赖。30s 健康窗口+全 stale 降级的语义保留。

### 9. crontab/systemd timer 解析 → 留

robfig/cron/v3 按规则**不合格**：v3.0.1（2020）后零 release、50+ open PR 积压、社区已在出 fork。且细看需求：agent 只做**只读枚举展示**（读 /etc/crontab、/etc/cron.d 与 systemd timer 列表报给运维面），不是调度执行——不依赖 parser 的调度正确性，只依赖字段拆分。237 行自研解析器覆盖的恰好是「展示用宽松解析」，引入一个死项目库反而绑定其 CVE 风险。若未来需要「表达式下次触发时间」等调度语义，届时评估活跃 fork 或 systemd 原生 `NextElapse`（经由 D-Bus，LXC 环境可用性另议）。

### 10. YAML 配置兼容层 → 留（只减不增）

koanf v2 活跃（MIT、模块化、轻依赖）、viper 活跃但重。**不建议引入**的理由：390 行兼容层的主体不是「YAML 解析」（yaml.v3 已在做），而是**业务规则**——PascalCase/snake_case 旧写法的双读映射（root.go 四套 UnmarshalYAML）。koanf 不自带这些规则，引入后同样要写映射代码，迁移只是换壳不换脑子。真正的治理路径是 CLAUDE.md 已定的配置契约：**新键一律 canonical lowerCamelCase，兼容层只减不增**；等旧键全部自然淘汰后，兼容层整体删除。届时若要现代配置层（分层/环境变量/文件热载），koanf 是当时的首选候选（记录在案）。

### 11-14. 日志 / TLS+devcert / 进程管理 / HTTP 客户端 → 留

- **日志**：slog（stdlib）+ lumberjack 已是 Go 生态标准组合，无动作。
- **TLS/devcert**：出站用 stdlib crypto/tls 是正解；devcert 227 行自签 CA 是开发体验件，mkcert 等是 CLI 工具不可作为库嵌入，无合格替换。
- **进程管理**：Go 无统治级进程管理库；OpsServer 的真实瑕疵（AutoRestart 在锁内 `time.Sleep`、无进程树终止、输出不可分流）是**自研内可修的 bug**，列入「留+内部瑕疵清单」随常规迭代修，不构成换库理由。
- **HTTP 客户端**：provider 的复杂度在 spec→HTTP 语义映射（1115 行的主体），不在 HTTP 本体——net/http 没有可被 resty/req 改善的部分。唯一该换的是其重试循环的线性退避，随 #1 的 backoff 迁移一并处理。

### 15. 会话/注册表双抽象并存 → 留 + 内部收敛

`internal/agent/provider_session.go`（210 行）与 `internal/transport/session/*`（378 行）功能重叠、两套并存。这不是换库问题，是**内部收敛**：定一份为标准（transport/session 更通用，agentlocal 注册表已依赖它），另一份迁移合并后删除。列入内部瑕疵清单，独立 PR 处理（涉及 `registry import cycle` 记忆中的边界约束，需小心）。

### 16. 序列化混层点 → 留 + 随 #4 收口

protobuf 官方运行时即正解。两处 JSON 塞 proto payload 的混层点（`mux_conn.go:193,422`、`local_handler.go:422,509-519`）是既成事实约定（六语言 SDK 已对齐该形态），不能单方面改协议——收口方式是 #4 的 schema 校验在边界拦截非法形态，协议本身不动。

## 三、迁移路径汇总（拍板后执行顺序）

| 批次 | 内容                                                                                     | 风险                                     | 测试策略                                      |
| ---- | ---------------------------------------------------------------------------------------- | ---------------------------------------- | --------------------------------------------- |
| A    | #1 退避三处换 cenkalti/backoff/v5 + #14 重试循环随行                                     | 低（接口内聚，行为参数等价+jitter 增强） | 保留旧用例+新增退避序列/取消两组              |
| B    | #2 限流桶层换 golang.org/x/time/rate（窗口层保留）                                       | 中（边界突发对拍）                       | 新旧放行序列对拍用例；慢机 flaky 用例改注入点 |
| C    | #4 补 jsonschema/v6 响应校验（无 schema 跳过）                                           | 低（增量能力，不破坏既有）               | 正反用例+无 schema 兼容用例                   |
| D    | #3 OpenAPI 推导换 kin-openapi（对拍后切换）                                              | 高（契约传导）                           | spec 快照对拍集全绿才切                       |
| E    | #5 本地网关 TLS 接线（或配置项移除，二选一拍板）                                         | 中（SDK 侧需支持 TLS 拨号选项）          | TLS 握手集成用例                              |
| F    | 内部瑕疵批：errorAs 删除、AutoRestart 锁内 sleep、session 双抽象收敛、RWMutex fatal 复查 | 低-中                                    | 只增不减；-race -count=3                      |

每批独立 PR、独立 commit、CI 绿后进下一批。D 批若对拍差异过大可单独叫停，不影响 A-C。

## 四、活跃度核实证据存档

| 库                            | 证据（2026-09-23 核实）                                                                       |
| ----------------------------- | --------------------------------------------------------------------------------------------- |
| cenkalti/backoff/v5           | v5.0.3 发布 2025-07-23（pkg.go.dev/ecosyste.ms）；~30k 仓库依赖；Debian ITP 2026-05           |
| hashicorp/yamux               | 0.1.2 新版（Debian tracker 2025-11-27 标记）；libp2p 默认 muxer，有 Rust/JS 移植              |
| xtaci/smux                    | Debian 打包 v1.5.57；pkg.go.dev/Sourcegraph 引用更新至 2026-03                                |
| golang.org/x/time/rate        | golang.org/x 官方扩展仓库（Google 维护）；Go wiki 官方推荐                                    |
| getkin/kin-openapi            | 本仓 go.mod 现用 v0.144；支持 3.0/3.1/3.2；CVE-2025-30153 已出补丁版                          |
| santhosh-tekuri/jsonschema/v6 | v6.0.2（本地 go.mod）；draft 4–2020-12；oapi-codegen 生产依赖；kin-openapi 3.1 支持构建于其上 |
| knadh/koanf                   | v2 活跃（pkg.go.dev 文档持续更新）；MIT；轻依赖                                               |
| robfig/cron/v3                | **v3.0.1（2020-01）后零 release**，50+ open PR 积压，社区 fork 出现——不合格                   |
| gojsonschema（对照项）        | 停在 draft-07、v1.2.0 后无 release——不合格，故 #4 选 santhosh-tekuri                          |
