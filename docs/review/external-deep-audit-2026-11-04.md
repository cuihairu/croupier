# 外部深度审核落地（2026-11-04）

> **来源**：外部深度审核《Croupier 源码深度审核 · Issues 清单》（35 条：P0×5 / P1×15 / P2×10 / P3×5），由外部评审者基于源码逐项取证产出。
> **评审代码快照**：commit `a4bcfe4b1`（fix(metrics): make SetDB atomic to avoid data race），VERSION `0.1.6`，最新 tag `v0.1.5`。
> **核验与落地时当前 HEAD**：`8ed22de0`（快照之后新增 7 个提交：OPS-008 备份链路 ×5、follow-redirects/decode-uri-component 依赖 bump ×2）。
> **外部建议非仓内事实**：本页第三节为评审原文完整搬运（保留原限定语，包括原文自述「需运行确认/未验证」的表述，未升格为仓内已验证结论）；第二节为本仓在 HEAD `8ed22de0` 上的逐条复核对照表。以对照表为准判断哪些仍是现行事实、哪些已被快照后的工作覆盖。
> **核验方法**：对每条 P0/P1 及 P2/P3 关键声称在当前 HEAD 用 grep / file:line 实核，证据见对照表。

## 核验对照表（35 条逐条复核 @ 8ed22de0）

**结论汇总：属实 29 / 部分属实 6 / 不属实 0。行号漂移 2 处（P0-3、P1-9）已在下表更新现行位置；「部分属实」6 条为快照后工作或证据口径差异所致，逐条注明。**

| 编号 | 条目 | 结论 | 当前 HEAD（8ed22de0）证据 |
|---|---|---|---|
| P0-1 | 许可证双口径、版权主体未填 | **属实** | `LICENSE:189` 仍为 `Copyright [yyyy] [name of copyright owner]` 占位符；`sdks/python/pyproject.toml:13` `license = {text = "MIT"}`；`sdks/js/package.json:13` `"license": "MIT"` 且 `files` 含 `LICENSE`（:72）；README badge 为 Apache 2.0，双口径仍在 |
| P0-2 | 版本号多源漂移、CI 无守卫 | **属实** | `VERSION`=`0.1.6`；最新 tag `v0.1.5`；`sdks/js/package.json:3` `0.1.4`、`sdks/python/pyproject.toml:7` `0.1.4`；`release.yml:59-61` 自述 linux/darwin `--version` 恒 0.1.4 并以 `git describe` 重注；`ci.yml` 无 VERSION 一致性校验步骤；`scripts/sync-sdk-versions.sh` 纯手工触发 |
| P0-3 | 集群互联证书缺失静默降级明文（fail-open） | **属实（行号漂移已更新）** | 原引 `config.go:637-645` 现为 **`internal/config/config.go:670-676`**：`InsecureSkipTLS`（:670-671）、「三者都配置时互联走 mTLS；任一项缺失或 InsecureSkipTLS=true 时维持内网明文（向后兼容），并在就绪面记录降级」（:672-676，注释现行位置）；`configs/server.yaml:54-55` `certFile: ""`/`keyFile: ""`、:120 降级注释不变。fail-open 姿态未变 |
| P0-4 | 发布无测试门禁、产物无校验和/签名 | **属实** | `release.yml:193` `needs: [build-go, build-sdk-sources]`；全文 grep `go test`/`sha256`/`SHA256`/`cosign`/`attest` 零命中；`nightly.yml:814-815` `git tag -f nightly` + `git push origin nightly --force` |
| P0-5 | 六语言 SDK 无行为级契约验证、proto 不触发 SDK CI | **属实** | 6 个 `ci-sdk-*.yml` paths 均仅 `sdks/<lang>/**`（含 ci-sdk-csharp / ci-sdk-plugins-js 的多行 paths 列表，均无 `proto/`）；`scripts/check-sdk-matrix.sh:5` 自述 "only checks for *presence*, not naming consistency"；全仓 workflow 无 `buf breaking` |
| P1-6 | 架构文档称审批走 MemStore，代码已是 SQLStore（反向偏差） | **属实（行号与评审一致）** | `docs/architecture/index.md:49` 仍写 "MemStore in current ServiceContext wiring"；`internal/svc/service_context.go:366` `approvals.NewSQLStore(db)`、`:543` 注入。文档滞后于代码的事实未变 |
| P1-7 | 协议演进只有硬断开、无版本协商 | **属实** | `docs/architecture/sdk-wire-protocol.md:103` `Version` 固定 `0x01`；:381-383 unsupported Version / 不合 subprotocol / undecodable body 均 "disconnects immediately"；无 capability / min_supported_version 字段 |
| P1-8 | 双通道调度仅 3/6 语言落地 | **属实** | Go `tcp_client.go:52,152,405` `ctrlInbox`；JS `tcp_transport.ts:173,432` `controlQueue`；Python `tcp.py:133,350-355` `_control_pool` + `is_control_request` 分流；Java 主实现、C++、C# grep 无 control 队列 |
| P1-9 | 传输层读循环同步调用 handler、无 in-flight 上限 | **属实（行号漂移已更新）** | 原引 `server.go:197` 现为 **`internal/transport/tcp/server.go:200`**：`respBody, handleErr := s.handler.Handle(ctx, msgID, reqID, body)` 仍在 `serveConn` 读循环内同步执行；`internal/transport/` grep `inflight\|MaxInflight` 零命中。注：todo.md 所述 `mux_conn.go` 双车道 `dispatchInbound`（系统车道+业务车道）为 Agent↔Server / Agent↔Provider 链路的另一路径，server.go 读循环路径未变，评审声称成立 |
| P1-10 | 投递语义未定义，busy→failover 对非幂等命令有重复执行风险 | **部分属实** | 仍在：`sdk-wire-protocol.md` 派发表（:147 业务车道满时 busy error frame → peer failover；:166 同义）；全文档无 at-most-once / at-least-once 投递语义矩阵；`docs/architecture/index.md:38` exactly-once 免责仍在。已漂移：评审引证「todo.md ARCH-018 为 open」已过期——ARCH-018 现为 `[x]`（2026-10-31 完成，主题为执行中断与停机演练），但投递语义缺口本身未闭合 |
| P1-11 | SDK→Agent 握手鉴权缺失 | **部分属实** | `auth_token` 最全实现确在 Python L3（`invoker.py:78,340`）；Go `types.go:137,246` 已有 `AuthToken` 字段（自述 Bearer token for Server API authentication）但未见 Provider 握手注入；`internal/` 无 `require_provider_auth` 类配置。核心缺口（Provider 握手统一鉴权 + Agent 强制开关）仍成立，Go 字段基础比评审所述略多 |
| P1-12 | 发布产物无校验和/签名，nightly tag 可强推 | **属实** | 同 P0-4 证据：`release.yml` 无 sha256/cosign/attest；`nightly.yml:814-815` `tag -f` + force push |
| P1-13 | 六语言 SDK 均未进注册中心、发布原子性不成立 | **属实** | 6 个 `release-sdk-*.yml` 全部 `workflow_dispatch`；`release-sdk-python.yml:3`「仅手动执行（workflow_dispatch），绝不自动触发」 |
| P1-14 | 单测范围漏 ./pkg、./cmd；8 个 coverage flag 全 informational | **属实（行号与评审一致）** | `ci.yml:214` `./internal/... ./core/... ./agents/...`（无 `./pkg/...`、`./cmd/...`）；:218 race 仅 5 包；`codecov.yml:53,60,67,74,81,88,95,102` 共 8 处 `informational: true` |
| P1-15 | 破坏性迁移忽略错误，SQLite 下删列可能静默 no-op | **属实** | `internal/platform/registry/agent_session_db.go:85` `_ = db.Migrator().DropColumn(&AgentSessionDB{}, "rpc_addr")`；`internal/model/migration_term_dictionary.go:20` 明注 glebarez/sqlite DropColumn 静默 no-op、:82,:88,:93 已示范原生 `ALTER TABLE ... DROP COLUMN` 做法 |
| P1-16 | 开源治理文件缺失、SECURITY 支持范围冲突 | **属实** | CONTRIBUTING.md / CHANGELOG.md / CODE_OF_CONDUCT.md / NOTICE / CODEOWNERS / .github/ISSUE_TEMPLATE / PULL_REQUEST_TEMPLATE.md 全部不存在；`SECURITY.md` 支持表仅 `main ✅`，无安全邮箱与 PGP |
| P1-17 | 文档站关闭死链检查、中文 SDK 文档回指英文 | **属实** | `docs/.vitepress/config.mjs:12` `ignoreDeadLinks: true`；commit `45bba54b` "docs: sync sdk docs and repoint zh links to english pages" 在案 |
| P1-18 | README 与仓库实际状态不一致 | **部分属实** | 仍在：`README.md:98` 仍以归档 legacy 文档 `docs/archive/legacy-en/architecture/transport-no-grpc.md` 为传输取舍权威依据；`docs/architecture/adr/` 不存在（gRPC 取舍无现行 ADR）。已过期子项：`gameops-demo` 已于 2026-11-01 落地为 `sdks/go/examples/gameops-demo/`（todo.md UI-002/UI-003 LOCAL VERIFIED），根 `examples/` 现有 7 项（评审时 6 项）；「9 cases green」表述在 README/todo 未检得 |
| P1-19 | 可访问性工程零投入 | **属实** | `web/package.json`、`web/eslint.config.cjs` grep `jsx-a11y\|axe` 零命中；docs 无面向用户的 WCAG/键盘/读屏声明。注：`docs/review/2026-10-31-ui-audit-impeccable.md` 已做过一次性 WCAG 对比度采样审计（审计记录，非工程化 a11y 投入，评审结论不受影响） |
| P1-20 | 仓库与工作区卫生问题 | **部分属实** | 仍在：`test-results/.last-run.json` 仍被 git 跟踪；`Makefile:18,25,40,50,161-162,239` 仍引用 `clone-dashboard`/`dashboard/` 与 6 个 `croupier-sdk-*` 独立仓。已过期子项：`.gitignore` 现含 `*.exe`(:4)、`*.out`(:10,:57)、`web/test-results/`(:217)——评审「未覆盖 coverage_*.out」已不成立；`server.exe`/`coverage_*.out` 为评审者本地产物，仓内 checkout 无 |
| P2-21 | internal/ 42 个顶层包、四套分层并存 | **属实** | `ls -d internal/*/ \| wc -l` = 42，精确一致 |
| P2-22 | 代码级技术债信号偏高 | **属实（数字漂移）** | TODO/FIXME/XXX 151（评审 150）；非测试 `panic(` 21（评审 18）；`context.Background()` internal/api+handler 非测试 20（评审 18）；`_ =` 忽略错误计数与评审口径不同但量级同向偏高。方向与结论均成立 |
| P2-23 | 无 proto 破坏性变更门禁 | **属实** | `ci.yml:95` 仅 `buf lint proto`；全 workflow grep `buf breaking` 零命中 |
| P2-24 | 性能无门禁 | **属实** | `scripts/e2e/test-performance.sh` 存在且不被任何 workflow 引用；todo.md PERF-001/002/003 均 BLOCKED（2026-11-01 复核：缺容量目标/隔离环境） |
| P2-25 | nightly 只打包不测试 | **属实** | `nightly.yml` 为 6 平台构建矩阵；grep `-race`/`fuzz`/`shuffle`/`-count` 零命中，无长跑/fuzz/全量 race |
| P2-26 | 移动端质量信号薄弱 | **属实** | `ci-mobile.yml`：flutter analyze + flutter test + debug APK 冒烟；无 coverage、无 golden test |
| P2-27 | 演示/默认凭据暴露 | **属实** | `internal/logic/utils/password_helpers.go:27` `"admin123": true` 弱口令黑名单；`README.md:36` demo 凭据 `admin/admin123` |
| P2-28 | SDK 版本强行同一、能力不同步 | **部分属实** | 版本同为 0.1.4、能力差异主体成立：`instance_metadata` 为 Go+JS 双实现（`sdks/go/pkg/croupier/types.go`、`tcp_manager.go:269,560`；`sdks/js/src/index.ts:403` `instanceMetadata`），Java/Python/C++/C# 无；auth_token Python 最全、Go 有 Server API AuthToken。注：`sdks/SDK_FEATURE_MATRIX.md` 能力矩阵文档已存在，评审建议的「能力 × 版本」矩阵部分在案 |
| P2-29 | 文档语言与术语混杂 | **属实** | `docs/zh/审计-文档一致性.md` 等中文命名文件在案；docs 中英混排 |
| P2-30 | 可视化资产与文档呈现 | **属实** | `docs/screenshots/ui/` 共 4.3M；`01-login.png` 1,086,608 字节；webp 数量 0 |
| P3-31 | 缺面向用户的 ROADMAP.md | **属实** | 仓库根无 `ROADMAP.md` |
| P3-32 | 缺术语表 | **部分属实** | `docs/zh/architecture/dashboard-glossary.md`（Dashboard 术语表）已存在；但 session runtime / subprotocol / L1-L3 传输术语无术语表条目，评审指向的缺口部分成立 |
| P3-33 | 无跨 SDK 字节级 golden 帧测试入口 | **属实** | 无 `scripts/contract/` 目录 |
| P3-34 | 样式系统三分、无边界文档 | **属实** | `web/package.json`：`antd-style ^4.1.0` + `tailwindcss ^4.2.4`（+antd token）并存 |
| P3-35 | pnpm workspace 与 Go module 版本策略未文档化 | **属实** | docs/ 与 README 未检索到 pnpm workspace / Go module 版本策略说明 |

### 快照（a4bcfe4b1）→ 核验（8ed22de0）之间的新增工作

快照后 7 个提交为 OPS-008 备份链路（periodic backup schedule loop、Get read path、scope filter、真实 db source backup、todo 记录）与 2 个 npm 依赖 bump（follow-redirects、decode-uri-component，对应 VULN-CROUPIER-001）。**均不直接改变 35 条的结论**；「部分属实」6 条的差异来自评审写作时点与本仓更早已落地工作（UI-002/003 的 gameops-demo、.gitignore 补充、ARCH-018 关闭、SDK_FEATURE_MATRIX）被评审遗漏或评审者本地工作区状态，详见对照表逐条说明。

---

## 评审原文（完整搬运，保留原限定语）

> 以下为外部评审原文，未作结论性修改；原文中的 `[已证实]` 为评审者自证标记，其行号以评审快照 a4bcfe4b1 为准，现行位置见上方对照表。

# Croupier 源码深度审核 · Issues 清单

> 审核对象：`D:\workspaces\croupier`（GitHub: cuihairu/croupier）
> 代码快照：commit `a4bcfe4b1`（fix(metrics): make SetDB atomic to avoid data race），VERSION `0.1.6`，最新 tag `v0.1.5`
> 审核方式：源码逐项取证（grep / 文件阅读 / git 状态），每条问题均附 `文件:行号` 证据；标注 **[已证实]** 的结论可直接复核
> 审核视角：架构与安全、测试与质量、发布与工程效能、产品与文档、前端 UI/可访问性
> 生成时间：2026-11-04

---

## 0. 总览

| 严重度 | 数量 | 含义 |
|---|---|---|
| **P0** | 5 | 阻断对外交付/生产使用，或已造成事实错误 |
| **P1** | 15 | 重要缺陷，规模化使用前必须解决 |
| **P2** | 10 | 一般问题，影响可维护性与信任度 |
| **P3** | 5 | 改进建议 |
| 合计 | **35** | — |

**总体判定：CONCERNS（不阻断内部试用，阻断生产交付）**

这个仓库的工程密度（27 个 workflow、Codecov 实测 91.5% 的总覆盖率、详细的 todo 台账、诚实标注 BLOCKED）明显高于同规模开源项目，问题不在"能力"而在**一致性**：三套版本号互相矛盾、许可证两个口径、文档描述与代码实现已经分叉、同一条协议有 6 份实现却只有 1 份被真正验证。

**优先修复顺序**：P0-1/2/3（合规与安全默认值）→ P0-4/5（发布与协议验证）→ P1-9/10/11（运行时正确性）→ 其余。

---

## P0 · 阻断级

### P0-1 许可证口径自相矛盾，且版权主体未填 [已证实]

**证据**
- `LICENSE:1-3` 为 Apache License 2.0 全文，正文中 `Copyright [yyyy] [name of copyright owner]` 占位符**未替换**。
- `sdks/python/pyproject.toml`：`license = {text = "MIT"}`，并带 `License :: OSI Approved :: MIT License` classifier。
- `sdks/js/package.json`：`"version": "0.1.4"` 同文件 `"license": "MIT"`，且 `files` 会把根 Apache LICENSE 打进 npm 包。

**影响**：同一产品对外存在两份互斥的许可证声明；游戏公司采购/法务尽调第一步即卡住；无版权主体意味着无法主张权利，也无法做 CLA/DCO 治理。

**修复**：统一为 Apache-2.0（与根 LICENSE 及 README badge 一致），改两处 manifest 的 `license`/classifier；补全 LICENSE 版权行；按需新增 `NOTICE`。

**验收**：`grep -r '"license"' sdks/*/package.json` 与 `grep license sdks/python/pyproject.toml` 结果唯一；LICENSE 无占位符。

---

### P0-2 版本号多源漂移，CI 无一致性守卫 [已证实]

**证据**
| 事实源 | 值 |
|---|---|
| `VERSION` | **0.1.6** |
| 最新 git tag | **v0.1.5** |
| `sdks/js/package.json` | **0.1.4** |
| `sdks/python/pyproject.toml` | **0.1.4** |
| `release.yml:59-61` | 注释自述："linux/darwin 二进制 `--version` 恒 0.1.4，Windows 因 git describe 而正确"，随后用 `git describe --tags --always --dirty` 重新注入绕过 |

- `.github/workflows/ci.yml` 中**不存在**任何 `VERSION` / version-sync 校验步骤（grep 无命中）。
- 版本同步脚本 `scripts/sync-sdk-versions.sh` 为纯手工触发，脚本尾提示 "Don't forget to commit changes!"。

**影响**：已发生实证事故——v0.1.5 发布的 Linux/macOS 二进制自报 0.1.4；用户提 bug 的版本号不可信，依赖 pin 不可信；`make build` 开发路径至今注入陈旧值。

**修复**：确立 `VERSION` 为唯一源；CI 新增 `version-guard`（VERSION == 最新 tag 去 `v` == 6 个 SDK manifest）；`release.yml` 增加 `verify-version` job，不一致则拒绝发布；移除 `git describe` 补丁、修 `make build` 注入。

**验收**：故意改坏任一 manifest，CI 必须红；`croupier-server --version` 与 tag 一致。

---

### P0-3 集群互联证书缺失时静默降级为明文（fail-open） [已证实]

**证据**
- `internal/config/config.go:637-645`：
  > `InsecureSkipTLS 互联 TLS 校验跳过（仅开发联调）`
  > `三者都配置时互联走 mTLS；任一项缺失或 InsecureSkipTLS=true 时维持内网明文（向后兼容），并在就绪面记录降级`
- `configs/server.yaml:52-58`：`toAgentTLS.certFile: ""`、`keyFile: ""` —— 仓库模板默认即明文。
- `configs/server.yaml:120`：`insecureSkipTls=true 时维持内网明文（就绪面记录降级）`。

**影响**：运维漏配证书不会报错，集群节点间控制流量以明文传输（含会话令牌、高危指令）；"记录降级"仅体现在就绪面，不具备阻断能力。这是安全设计上的典型 fail-open。

**修复**：集群 `enabled: true` 且 TLS 面缺失时**拒绝启动**（或强制显式 `allowInsecureInterconnect: true` 二次确认）；就绪探针置 503；文档同步修订。

---

### P0-4 发布流程无测试门禁、产物无校验和与签名 [已证实]

**证据**
- `.github/workflows/release.yml`：作业仅 `build-go` → `build-sdk-sources` → `release`，`needs` 只依赖两个构建作业；全文无 `go test`、无 `workflow_run` 等待 CI 结论。
- 同文件 grep `sha256|SHA256|cosign|attest` **零命中**——`croupier-bin-*.tar.gz`、SDK 源码包、dashboard 包均无摘要文件、无签名、无 provenance。
- 对照：`nightly.yml` 用 `git tag -f nightly` 覆盖同一 tag。

**影响**：任意 commit 打 tag 即对外发布；用户无法验证下载物完整性；nightly 产物不可复现（昨天的包无法按 tag 找回）。产品自身在做"SHA-256 校验 + 灰度 + 回滚"，对自己发布的包却不校验，反差明显。

**修复**：release job 串跑 `go test ./...` + e2e + 契约守卫，红则不出包；全产物生成 `SHA256SUMS`；接入 `cosign sign-blob`（keyless）或 GitHub artifact attestation；nightly 改不可变 `nightly-YYYYMMDD` tag 并滚动保留。

---

### P0-5 六语言 SDK 无行为级契约验证，且 proto 变更不触发 SDK CI [已证实]

**证据**
- 6 个 `ci-sdk-*.yml` 触发路径一律为 `paths: ["sdks/<lang>/**"]`（已核对 `ci-sdk-go.yml:8`）——改 `proto/` 不会触发任何 SDK 构建或测试。
- `scripts/check-sdk-matrix.sh` 自述 "It only checks for *presence*, not naming consistency"，实现为 grep 符号名。
- 各语言仅有自写的 mock 契约测试（`http_invoker_test.go` / `test_invoker.py` / `ServerHttpInvokerTest` / `invoker.test.ts` / `test_invoker.cpp` / `CroupierInvokerTests`），**不是同一份测试向量**。
- `.github/workflows/*.yml` 无 `buf breaking` 门禁（grep 零命中）。

**影响**：6 份实现可以同时全绿而线上互不兼容——某 SDK 的 mock 复刻了自身 bug 时无法被发现。这是"多语言 SDK + 自研 wire protocol"这一核心卖点上最大的质量真空。

**修复**：在 `proto/` 旁放语言中立 golden 向量（握手帧字节、Register 编解码、task 事件、错误码映射），6 个 SDK 的测试读同一文件做 round-trip，CI matrix 扇出；补 `buf breaking --against '.git#branch=main'`。

---

## P1 · 重要

### P1-6 架构文档称审批走 MemStore，代码已改为 SQLStore [已证实]

**证据**：`docs/architecture/index.md:49` 仍写 `MemStore in current ServiceContext wiring | SQLStore exists but is not wired for restart/cross-instance persistence`；而 `internal/svc/service_context.go:366` 实际为 `approvalsStore, err := approvals.NewSQLStore(db)`，并在 `:543` 注入。

**影响**：这是**反向偏差**——代码已修好，文档还在自曝短板，导致外部安全评审依据错误信息下结论（上一轮线上评审即据此判了 P0）。同时也说明架构状态表没有与代码同步的机制。

**修复**：更正文档；把该状态表的关键行做成可执行断言（启动时打印实际 store 类型，或 CI 校验文档表格与 wiring 一致）。

---

### P1-7 协议演进只有"硬断开"，无版本协商 [已证实]

**证据**：`docs/architecture/sdk-wire-protocol.md:381`：`An unsupported Version disconnects immediately`；另有"undecodable protobuf body disconnects immediately"。Header `Version` 固定 `0x01`，握手无 capability / `min_supported_version` 字段。

**影响**：SDK 内嵌在游戏进程里，一次运行数周；服务端升版即全量断连且无降级路径，等于要求游戏服与平台同步发版——在真实游戏公司不可能。

**修复**：握手增加能力协商与最低版本窗口；"不支持版本"降级为"拒绝该请求、保留会话"；给出 N-1 兼容期与废止时间表。

---

### P1-8 双通道调度仅 3/6 语言落地，另 3 种仍是单队列 [已证实]

**证据**：控制通道实现存在于 `sdks/go/pkg/croupier/transport/tcp_client.go`（`ControlQLen`/`ctrlInbox`）、`sdks/js/src/tcp_transport.ts`、`sdks/python/croupier/transport/tcp.py:133,350-355`（`_control_pool` + `is_control_request` 分流）。Java 主实现 grep 无 control 队列；C++/C# 未见对应实现。

**影响**：Java/C++/C# 的心跳与业务共队列，正是作者在 `sdk-wire-protocol.md` 里警告的"业务洪峰挤掉心跳→对端判死→连接雪崩"模式。游戏服务端大量使用 Java/C#，风险面很大。

**修复**：三语言补齐控制通道并用同一 golden 场景验证（业务饱和下心跳延迟上界）。

---

### P1-9 传输层读循环同步调用 handler，无 in-flight 上限 [已证实]

**证据**：`internal/transport/tcp/server.go:197`：`respBody, handleErr := s.handler.Handle(ctx, msgID, reqID, body)` 直接在 `serveConn` 的 `for` 循环内同步执行；`internal/transport/` 下 grep `inflight|MaxInflight` **零命中**。

**影响**：一条连接上慢 handler 会队头阻塞该连接全部请求（含心跳/取消），与协议声称的"单连接多并发 in-flight"矛盾；无 per-request deadline、无并发上限，重放/积压时内存无界增长。

**修复**：读循环与执行解耦（worker pool + 有界队列），实现 `max_inflight_requests` 与逐请求 deadline；补"慢 handler 不影响控制帧"的回归测试。

---

### P1-10 投递语义未定义，busy→failover 对非幂等命令有重复执行风险 [已证实设计缺口]

**证据**：业务通道满时返回 busy 并"触发对端故障转移"（`sdk-wire-protocol.md`）；而 `todo.md` 中 ARCH-018（总体超时保证）为 open，`docs/architecture/index.md` 自认"数据库分页幂等不等于外部副作用 exactly-once"。

**影响**：重启服务、杀进程、改配置这类运维命令若在 busy 时自动换实例重试，等于给破坏性操作装了"自动重复执行"。

**修复**：定义投递语义矩阵（每类消息 at-most-once / at-least-once + 幂等键）；Ops 类默认 at-most-once + 人工确认，禁止 busy 自动 failover。

---

### P1-11 SDK→Agent 握手鉴权缺失 [已证实]

**证据**：`auth_token` 仅出现在 `sdks/python/croupier/invoker.py`（L3 HTTP Invoker 路径）；Go/Java 的 Provider 握手元数据未注入；SDK↔Agent 链路默认不启用 TLS。

**影响**：同机/内网任意进程可冒充 SDK 注册函数或截获调用；Agent 无法对 SDK 端口强制鉴权（强制即打挂 3 种语言）。

**修复**：握手元数据统一携带 `auth_token`（6 语言），Agent 侧可配置 `require_provider_auth`；与双通道补齐同期完成。

---

### P1-12 发布产物无校验和/签名，nightly tag 可强推 [已证实]

**证据**：`release.yml` 无 `sha256`/`cosign`/`attest`；`nightly.yml` 使用 `git tag -f nightly && git push --force`。

**影响**：安装包完整性不可验证、历史 nightly 不可复现、供应链攻击无检测手段。

**修复**：同 P0-4 的第 2、3 条（SHA256SUMS + 签名 + 不可变 nightly tag）。

---

### P1-13 六语言 SDK 均未进入注册中心，发布原子性不成立 [已证实]

**证据**：`.github/workflows/` 下 6 个 `release-sdk-*.yml` 全部 `workflow_dispatch` 手动触发（`release-sdk-python.yml:1-12` 明确"仅手动执行，绝不自动触发"）。

**影响**：用户拿到的只有 tar.gz 源码包，没有 `pip install` / `npm i`；一次协议变更要人工点 6 次，可漏、可乱序、可半发布——monorepo 单 tag 的原子发布优势被抵消。

**修复**：合并为一个 tag 触发的 reusable workflow 矩阵（npm / PyPI / Maven / NuGet / Go proxy / vcpkg），任一失败则 release 转 draft 并告警。

---

### P1-14 单测范围漏 `./pkg`、`./cmd`，8 个覆盖率 flag 全是 informational [已证实]

**证据**：`ci.yml:214`：`go test ... ./internal/... ./core/... ./agents/...`；`:218` race 仅点 5 个包；无 `./pkg/...`、无 `./cmd/...`。`codecov.yml` 第 53/60/67/74/81/88/95/102 行共 8 处 `informational: true`（含 6 个 SDK 与 dashboard）。

**影响**：`pkg/` 是跨 agent 共享层，其中单测可能永不执行；6 个 SDK 覆盖率跌到 50% 也不会红；`target: auto` 允许每次 PR 掉 1%，长期可累积下滑。

**修复**：纳入 `./pkg/...`、`./cmd/...`（或至少显式点名 `./cmd/server` 的 drill 测试）；SDK flag 去掉 informational，设绝对下限（如 85%）。

---

### P1-15 破坏性迁移忽略错误，SQLite 下删列可能是静默 no-op [已证实]

**证据**：`internal/platform/registry/agent_session_db.go:85`：`_ = db.Migrator().DropColumn(&AgentSessionDB{}, "rpc_addr")`；而同仓库 `internal/model/migration_term_dictionary.go:20-24` 明确写道"部分驱动（glebarez/sqlite）的 DropColumn 是静默 no-op"。

**影响**：`rpc_addr` 在 SQLite 部署下可能实际未删除，仅"模型不再映射"；滚动升级期间新旧版本共存时，旧版本读该列的行为未定义；错误被吞掉导致迁移"看起来成功"。

**修复**：不用 GORM DropColumn，改原生 `ALTER TABLE ... DROP COLUMN`（该仓库已在 `migration_term_dictionary.go:82` 采用此做法）；回填/回滚路径写入迁移文档。

---

### P1-16 开源治理文件缺失，SECURITY 支持范围与已发版本冲突 [已证实]

**证据**：仓库根无 `CONTRIBUTING.md`、`CHANGELOG.md`、`CODE_OF_CONDUCT.md`、`NOTICE`、`CODEOWNERS`、`.github/ISSUE_TEMPLATE/`、`PULL_REQUEST_TEMPLATE.md`；`SECURITY.md:7-9` 的支持表只有 `main | ✅`。

**影响**：外部贡献者零入口（分支策略、测试要求、评审 SLA 全无）；"已发布的 0.1.0–0.1.5 不提供安全更新"对采购方是硬伤；安全披露只写"发送邮件至项目维护者"却未给邮箱与 PGP。

**修复**：补 CONTRIBUTING / CoC / issue+PR 模板 / CODEOWNERS；SECURITY 支持表改 `0.1.x + main` 并写 backport 政策；补安全邮箱与 PGP 指纹。

---

### P1-17 文档站关闭死链检查，中文 SDK 文档回指英文 [已证实]

**证据**：`docs/.vitepress/config.mjs:12`：`ignoreDeadLinks: true`；提交 `docs: sync sdk docs and repoint zh links to english pages`。

**影响**：坏链被静默容忍；中文接入团队读 SDK 文档被迫切英文，双语承诺在关键路径断裂。

**修复**：`ignoreDeadLinks` 改白名单过渡；CI 增加 locale parity 检查（en/zh 页面集合 + sidebar 条目 diff）；补齐 SDK 中文页。

---

### P1-18 README 与仓库实际状态不一致 [已证实]

**证据**
- README 将 `docs/archive/legacy-en/architecture/transport-no-grpc.md` 作为"传输取舍"权威依据——该文已归档为 legacy，而 `docs/architecture/` 下无对应现行文档；`session-runtime-landscape.md` 的候选对比表**不含 gRPC/HTTP2 行**。
- README/todo 提及的 `examples/gameops-demo` **不存在**（`examples/` 仅 6 个子目录）；e2e 实际在 `scripts/e2e/`。
- 提交信息中的 "9 cases green" 与脚本断言数（happy-path 6 + task-lifecycle 5）对不上。

**影响**：最关键的技术决策没有现行 ADR（gRPC 从未被正式评估，唯一成立的反对理由"反向 RPC"未落纸）；外部读者按 README 找文件会 404，直接损伤可信度。

**修复**：写 `docs/architecture/adr/0001-transport.md`（含 gRPC 逐项对比与否决理由、go-plugin 豁免边界）；修正 README 引用；建立"README 链接可点"CI 检查。

---

### P1-19 可访问性工程零投入 [已证实]

**证据**：`web/package.json`、`web/eslint.config.cjs` 中 `jsx-a11y`/`axe` 零命中；docs 无 WCAG/键盘/读屏声明。

**影响**：RJSF 表单、自定义 ThemeToggle、公告弹窗的可达性完全依赖 antd 默认行为；运维人员长时间高密度作业场景下风险真实存在（也是政企采购的常见门槛）。

**修复**：`eslint-plugin-jsx-a11y` 入 lint 链；在既有 Playwright `real-dashboard` project 接 `@axe-core/playwright` 扫关键页；两套主题套 × 明暗共 4 组做 WCAG AA 对比度检查。

---

### P1-20 仓库与工作区卫生问题 [已证实]

**证据**
- `git ls-files` 命中 `test-results/.last-run.json`（Playwright 产物被提交）。
- 工作区存在 `server.exe`（**87 MB**）与 7 个 `coverage_*.out`（`coverage_lf.out`、`coverage_ops_final.out` 等），未入库但污染构建目录；`.gitignore` 只覆盖 `coverage.html` 类，未覆盖 `coverage_*.out`。
- `Makefile:25,40,50,161-162` 仍引用已废弃的 `dashboard/` 目录与 6 个已并入 monorepo 的 `croupier-sdk-*` 独立仓库。

**影响**：新贡献者按 Makefile 执行会失败；85MB 二进制误提交风险高；CI 缓存/构建产物混入源码树影响 grep 类守卫的准确性。

**修复**：`.gitignore` 补 `coverage_*.out`、`*.exe`、`test-results/`；`git rm --cached test-results/.last-run.json`；清理 Makefile 中的 `clone-dashboard`/`dashboard/`/`croupier-sdk-*` 引用。

---

## P2 · 一般

### P2-21 `internal/` 42 个顶层包，四套分层约定并存 [已证实]
`ls -d internal/*/ | wc -l` = 42；`service` 与 `svc`、`api` 与 `handler`、`logic`/`platform`/`core` 职责重叠。新人无法从目录推断归属。
**建议**：定四层（transport / core / platform / api）并用 `depguard` 在 CI 强制 import 方向，先合并 `svc→service`、`api→handler`。

### P2-22 代码级技术债信号偏高 [已证实]
`internal/ cmd/ pkg/ agents/` 中：`TODO/FIXME/XXX` **150** 处；非测试代码 `panic(` **18** 处；`_ =` 忽略错误 **527** 处；`internal/api`、`internal/handler` 内 `context.Background()` **18** 处（绕过请求取消）。
**建议**：`context.Background()` 全部改为从入参派生；panic 限定在 init/不可恢复路径并加注释；按目录分批消化 TODO。

### P2-23 无 proto 破坏性变更门禁 [已证实]
CI 只有 `buf lint`，无 `buf breaking`；`proto/**` 变更不影响任何 SDK workflow（见 P0-5）。

### P2-24 性能无门禁 [已证实]
`scripts/e2e/test-performance.sh` 存在但不进任何 workflow，且只是 10 次串行取均值、无 exit 1 阈值；`todo.md` 的 PERF-* 全 BLOCKED。
**建议**：改 k6/vegeta 并发压测 + P95 断言 + benchstat 基线，放 nightly。

### P2-25 nightly 只打包不测试 [已证实]
6 平台矩阵 + 版本注入校验很扎实，但无 `-race` 全量、无长跑、无 fuzz。夜间是长跑测试最该落的位置。
**建议**：新增 `nightly-test` job（全量 `-race -shuffle=on`、`-count=3` flaky 探测、10 分钟 fuzz）。

### P2-26 移动端质量信号薄弱 [已证实]
`mobile/` 仅 `flutter analyze` + `flutter test` + debug APK 烟测；无 coverage、无 golden test，却与 Web 共用同一高危执行 API，无独立威胁模型。

### P2-27 演示/默认凭据暴露 [已证实]
`internal/logic/utils/password_helpers.go:27` 将 `"admin123"` 列入弱口令黑名单（说明该默认凭据真实存在）；README 直接给出 demo 的 `admin/admin123`。
**建议**：初始化脚本现场生成随机口令并打印一次；README 明确"仅 demo 环境"。

### P2-28 SDK 版本强行同一，但能力并不同步 [已证实]
6 个 SDK 共用 `0.1.4`，而 `auth_token` 仅 Python L3 有、双通道仅 3 语言有、`instance_metadata` 仅 Go/JS 支持。
**建议**：要么能力对齐后再统一版本，要么允许 per-package 版本（changesets/release-please），并发布"能力 × 版本"矩阵。

### P2-29 文档语言与术语混杂 [已证实]
`internal/config/config.go` 注释中文、`docs/` 中英混排（存在 `审计-文档一致性.md` 等中文命名文件）；`L1/L2/L3 Provider`、`subprotocol` 等术语在用户文档入口无解释。

### P2-30 可视化资产与文档呈现 [已证实/部分待运行]
`docs/screenshots/ui/` 共 4.3 MB，`01-login.png` 达 1.1 MB 且无 WebP 版本；截图无自动重生成机制；两套主题的对比度无任何声明或测试（需实际运行确认）。

---

## P3 · 建议

- **P3-31** 缺面向用户的 `ROADMAP.md`（todo.md 是内部验收台账，30+ BLOCKED 项混在其中）。
- **P3-32** 缺术语表（glossary），`session runtime`/`subprotocol`/`L1-L3` 首次出现处无链接。
- **P3-33** 无跨 SDK 的字节级 golden 帧测试脚本入口（与 P0-5 配套，建议独立 `scripts/contract/`）。
- **P3-34** 样式系统三分（antd token + antd-style + Tailwind 4 + global.less）无边界文档，长期发散风险。
- **P3-35** `mobile/`、`web/`、`docs/` 的 pnpm workspace 与 Go module 版本策略未文档化，新贡献者需自行试错。

---

## 附录 A · 与上一轮线上评审的差异（本次以源码为准的更正）

| 上一轮结论 | 源码核验结果 | 说明 |
|---|---|---|
| 审批存于 MemStore，重启即失效（P0） | **已修正**：`service_context.go:366` 使用 `NewSQLStore(db)` | 是**文档滞后**，代码已修；风险降级为 P1-6 |
| `transport-no-grpc.md` 404、不存在 | **部分修正**：文件在 `docs/archive/legacy-en/`，README 链接有效 | 真正的问题是"归档 legacy 文档为现行架构背书"，仍保留在 P1-18 |
| `examples/gameops-demo` 不存在 | **维持**：`examples/` 下确无此目录 | e2e 实际在 `scripts/e2e/` |
| 双通道 3 语言缺失 | **维持并补强**：Java 主实现无 control 队列；Python 已实现（`tcp.py:350`） | Go/Python/JS 有，Java/C++/C# 无 |
| 读循环同步调用 handler | **维持**：`server.go:197` 同步调用，且无 inflight 上限 | 补充了"无 per-request deadline" |
| TLS 默认姿态多文档不一致 | **维持并具体化**：`config.go:641-645` 明文降级有代码注释为证 | 升级为 P0-3 |

---

## 附录 B · 建议的修复批次

| 批次 | 内容 | 对应条目 |
|---|---|---|
| 第 1 批（1 天） | 许可证统一、版本守卫接入 CI、仓库卫生 | P0-1、P0-2、P1-20 |
| 第 2 批（3 天） | TLS fail-closed、发布测试门禁 + 校验和签名 | P0-3、P0-4、P1-12 |
| 第 3 批（1 周） | 跨 SDK golden 向量、proto 变更触发 SDK CI、buf breaking | P0-5、P2-23 |
| 第 4 批（2 周） | 双通道补齐 3 语言、握手鉴权、读循环解耦、投递语义矩阵 | P1-8、P1-9、P1-10、P1-11 |
| 第 5 批（并行） | 治理文件、文档纠偏、a11y、注册中心矩阵发布 | P1-13、P1-16、P1-17、P1-18、P1-19 |

---

*本清单所有 `[已证实]` 条目均可通过所述文件与行号直接复核；未标注者为设计缺口或需运行验证的推断，已在条目内说明。*
