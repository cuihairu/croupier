# AUD-COV-008 · 历史审计回归总报告（2026-11-28）

- 基线：HEAD `d11dd4beb`。
- 范围：`docs/review/` 中 2026-11-01…10 的七份历史审计/交付记录，对照「历史高优先级问题复核」表（todo.md，P0-1..5、P1-6..10、P1-12 已于 2026-11-21 回归）之外的**全部剩余条目**。
- 方法：四路并行只读验证，每条发现逐条开文件取证（file:line 落到当前 HEAD），主审复核关键残留并补两处机械证据：C-list 29 个修复 commit 全部 `git merge-base --is-ancestor` 通过；wip-audit 4 个 commit 同法验证。
- 判定口径：已修 / 部分修复 / 仍存在 / 无法复核（动态运行面，归 COV-002..007 的运行验证范围）。

## 1. 结论总览

| 源文档                                                                                        | 判定分布                                                                                                                           |
| --------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| external-deep-audit-2026-11-04.md（本轮回归的 24 条：P1-11、P1-13..20、P2-21..30、P3-31..35） | 已修 19 · 部分修复 4（P1-18、P2-21、P2-22、P2-28）· 仍存在 1（P2-24）                                                              |
| 2026-11-01-agent-scope-isolation-matrix.md（AGENT-001）                                       | 4/4 检查点在位（含两处「已知缺口」按台账维持原状）                                                                                 |
| 2026-11-01-code-review.md（C-001..C-032）                                                     | 结论维持：29 修复 commit 全为 HEAD 祖先 + 6 处代码抽样在位；C-005 缓期维持；C-032 不适用维持                                       |
| wip-audit-2026-11-04.md                                                                       | 4 commit 在主链；`hostdriver` 构建产物仍以 ignore 形态留在根目录（`.gitignore:129`，与台账一致）                                   |
| 2026-11-01/02 ops 三份交付报告（OPS-001/010/002）                                             | 交付物全部在位；provision 面有增量演进（迁移 0054 destroy_approval_id、0059 cloud_init_extra、hourly 唯一索引 0061），属追加非漂移 |

P2-24…P3-35 范围与 todo.md 自身台账完全吻合：该区间仅 CR-FIX-24、CR-FIX-28 仍未勾，代码状态证实两条均真实未修；其余 CR-FIX-25/26/27/29/30/31/32/33/34/35 勾行与 HEAD 一致。

## 2. 外部深审剩余 24 条逐条判定

| 条目                        | 判定         | HEAD 证据                                                                                                                                                                                                                                                                                                                                                                             | 跟踪                              |
| --------------------------- | ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------- |
| P1-11 SDK→Agent 握手鉴权    | 已修         | 六语言 ProviderConnect metadata 注入 authToken（Go `tcp_manager.go:877-883`、Python `__init__.py:357-361`、Java `CroupierClientImpl.java:463-468`、JS `index.ts:1591-1596`、C++ `croupier_client.cpp:811-820`、C# `CroupierClient.cs:736-741`）；Agent 强制侧 `internal/agent/provider_auth.go:25-49` + `cmd/sidecar-agent/root.go:657-673`；AUD-001 cleartext 守卫 `root.go:669-671` | CR-FIX-11 [x]                     |
| P1-13 注册中心矩阵发布      | 已修         | 六语言 workflow 均有 workflow_call（如 `release-sdk-go.yml:14,29`）；tag 触发矩阵 `release-sdk-matrix.yml` ← `release.yml:306-310`（`v*` tag）；任一车道失败 → finalize 转 draft                                                                                                                                                                                                      | CR-FIX-13 [x]                     |
| P1-14 测试范围              | 已修         | `ci.yml:280` 含 `./pkg/... ./cmd/...`；`codecov.yml` informational 计数 0，SDK 绝对下限 target 85                                                                                                                                                                                                                                                                                     | CR-FIX-14 [x]                     |
| P1-15 SQLite 删列           | 已修         | `agent_session_db.go:86-100` 原生 `ALTER TABLE ... DROP COLUMN`（PG 用 IF EXISTS），错误 `slog.Warn` 不吞                                                                                                                                                                                                                                                                             | CR-FIX-15 [x]                     |
| P1-16 治理文件              | 已修         | CONTRIBUTING/CHANGELOG/CoC/NOTICE/CODEOWNERS/issue+PR 模板齐备；SECURITY.md 支持表 `0.1.x ✅ + main ✅` + backport 政策 + 安全邮箱。残留：PGP 指纹未配置（`SECURITY.md:27` 明示「发布后列出」；勾行已知边界注为 user-blocked，非矛盾）                                                                                                                                                | CR-FIX-16 [x]                     |
| P1-17 文档站死链            | 已修         | `config.mjs:16` ignoreDeadLinks 为白名单（非 true）；locale-parity CI `docs-check.yml:79-81` + `check-locale-parity.py`；`docs/zh/sdks/` 8 个真实中文页                                                                                                                                                                                                                               | CR-FIX-17 [x]                     |
| P1-18 README 一致性         | **部分修复** | EN 已纠（`README.md:112` 指现行 ADR）；ADR 在位（`docs/architecture/adr/0001-transport.md` + 0002）；但 `README.zh.md:103` 仍引用归档 `transport-no-grpc.md` 作权威，且 `check-docs-links.sh`（:37 文件集）不扫 README.zh.md——陈旧引用对 CI 不可见                                                                                                                                    | CR-FIX-18 [x]；残留立 **AUD-108** |
| P1-19 可访问性              | 已修         | `web/package.json:83,108` axe+jsx-a11y；`eslint.config.cjs:55-62` 挂插件；`a11y-dashboard.spec.ts:176-222` 关键页扫描 + 四组对比度 sweep；CI 门禁 `ci-dashboard.yml:336`                                                                                                                                                                                                              | CR-FIX-19 [x]                     |
| P1-20 仓库卫生              | 已修         | `git ls-files` 无 test-results；`.gitignore:250-251,4-5` 齐；Makefile 死引用清零。遗留 `cmd/analytics-export/main.go:113` 默认路径指向已删目录（CR-FIX-20 勾行已记录在案）                                                                                                                                                                                                            | CR-FIX-20 [x]                     |
| P2-21 internal 分层         | 部分修复     | 顶层包 43（原 42，基本未变）；但四层 import 守卫 `check_layer_dependency.sh` 入 CI（`ci.yml:89-92` + 自测）；svc/api 合并有意跳过（勾行已记录）                                                                                                                                                                                                                                       | CR-FIX-21 [x]                     |
| P2-22 技术债信号            | 部分修复     | 现：TODO/FIXME 151（平）、非测试 panic 23（升，须 `panic-ok:` 豁免）、api/handler `context.Background()` 5（原 18，降）、`_ =` 765（原 527，升、无门禁）；panic/ctx 两族已入 `check_context_panic_hygiene.sh` 门禁                                                                                                                                                                    | CR-FIX-22 [x]                     |
| P2-23 proto 破坏性门禁      | 已修         | `ci.yml:125-152` buf breaking（对照 push 基线，含 fetch 兜底）；`proto/buf.breaking.yaml`；SDK CI `paths` 含 `proto/**`                                                                                                                                                                                                                                                               | CR-FIX-23/05 [x]                  |
| P2-24 性能门禁              | **仍存在**   | `scripts/e2e/test-performance.sh` 阈值仅配色（`:58-66` 不传播失败，5s 均值也「过」）；仍 10 次串行无并发无 P95；不在任何 workflow（全 workflows 零引用）；默认凭据 `admin/admin123`（`:9-10`）在 CR-FIX-27 后只对 opt-in demo 配置有效                                                                                                                                                | CR-FIX-24 [ ]、PERF-001..003 [ ]  |
| P2-25 nightly 只打包不测试  | 已修（主诉） | `nightly.yml:111-200` nightly-test：全量 `-race -shuffle=on`、flaky `-count=3`、fuzz 12.5m×3 靶；quality-gate 前置打包。边界：nightly-test 有意**非阻断**（`:114-115` 注释，红不挡产物）；无 soak 长跑                                                                                                                                                                                | CR-FIX-25 [x]                     |
| P2-26 移动端质量            | 已修（主诉） | `ci-mobile.yml:49-80` coverage 门禁（阈值 80，低于即红）+ lcov artifact；`docs/operations/mobile-threat-model.md`（+zh）。golden 未做（勾行已记录跨 OS 基线漂移边界，225 用例 + 门禁替代）                                                                                                                                                                                            | CR-FIX-26 [x]                     |
| P2-27 默认凭据              | 已修         | 轮换：`service_context_seed.go:26-96`（AllowDefaultCredentials 门控 + 32 字符随机 bootstrap 口令打印一次）；黑名单 `admin123` 保留即轮换检测键；README:40-46 demo-only 表述 + 轮换说明。残留：发布归档仍打包 admins.json（服务端首 seed 轮换，勾行已记录）                                                                                                                            | CR-FIX-27 [x]                     |
| P2-28 SDK 版本统一          | 部分修复     | 单一 VERSION 源 + version-sync + 守卫在位；`SDK_FEATURE_MATRIX.md` 能力×SDK 矩阵 + 版本管理节；但六 SDK 仍同一 0.1.6、无 per-package 版本、无「能力×版本」发布映射（差异是政策化的：TLS 缺席 C++/C# 有 AUD-014 rationale）                                                                                                                                                            | CR-FIX-28 [ ]                     |
| P2-29 语言术语              | 已修         | `terms-and-layering.md` + `transport-glossary.md`（subprotocol/L1-L4 定义式条目）+ 入口链接（README:139、architecture index:90、sidebar EN+zh）                                                                                                                                                                                                                                       | CR-FIX-29 [x]                     |
| P2-30 可视化资产            | 已修         | `docs/screenshots/ui/` 1008K/10 文件全 WebP（原 4.3MB PNG）；再生脚本 `optimize-screenshots.sh` + `capture-interface-preview.mjs`；对比度声明 + a11y sweep（再生为手动脚本非 workflow）                                                                                                                                                                                               | CR-FIX-30 [x]                     |
| P3-31 ROADMAP               | 已修         | 根 `/ROADMAP.md`，README:152 链接                                                                                                                                                                                                                                                                                                                                                     | CR-FIX-31 [x]                     |
| P3-32 术语表                | 已修         | `transport-glossary.md` 12 术语 + zh 镜像 + sidebar 双语注册                                                                                                                                                                                                                                                                                                                          | CR-FIX-32 [x]                     |
| P3-33 golden 帧入口         | 已修         | `scripts/contract/run-golden-vectors.sh`（--only/--skip/--strict）+ `proto/contracts/golden_vectors.json`；按设计不进独立 workflow（各 ci-sdk 跑同一套件）                                                                                                                                                                                                                            | CR-FIX-33 [x]                     |
| P3-34 样式边界              | 已修         | `docs/development/frontend-styles.md`（+zh）四层表 + 决策规则 + sidebar                                                                                                                                                                                                                                                                                                               | CR-FIX-34 [x]                     |
| P3-35 workspace/module 策略 | 已修         | `docs/development/versioning.md`（+zh）：版本源表/守卫/Go modules/pnpm overrides 规则                                                                                                                                                                                                                                                                                                 | CR-FIX-35 [x]                     |

## 3. 2026-11-01/02 审计与交付记录回归

### 3.1 AGENT-001 scope 隔离矩阵

| 检查                            | 判定     | 证据                                                                   |
| ------------------------------- | -------- | ---------------------------------------------------------------------- |
| 四个节点动作处理器 scope 门     | 在位     | `helpers.go:792/852/905/940` 全部走 `requireAgentScope`                |
| node_action_scope_test.go       | 在位     | `:31/:55/:68`（restart+undrain 合为一例，命名微漂移）                  |
| 缺口 3.1 synthetics query scope | 维持原状 | `synthetics_results.go:38` 仍读 query gameId/env（已知边界，登记在案） |
| 缺口 3.4 NodeService 死实现     | 维持原状 | `nodes.go:15,19` 仍仅测试引用、无路由                                  |

### 3.2 11-01 代码深审 C-list

- **机械证据**：29 个修复 commit（`6cf5862c`…`f6b556d2`）全部为 HEAD 祖先。
- **代码抽样（6/30）**：C-001（`safeUrl.ts:53` 白名单 + Bugs 页三 sink）、C-008（已搬迁至 `ops_server_exec.go:51`，敏感键/解释器/工作目录三层守卫逻辑不变——唯一路径漂移）、C-022（`helpers.go:131` RLock + 修复注释）、C-024（`tcp_local_listener.go:244→257` mux.Wait）、C-012（Python varint 三重边界 `:713-724`）、C-005（`Login/index.tsx:173` 仍 localStorage——**缓期维持**，JWT→httpOnly cookie 须用户设计签字，见 c005-auth-cookie-deferral）。
- 结论：历史勾行「30 修 + 1 缓期 + 1 不适用」与 HEAD 一致；未抽样的 24 条以 commit 在主链 + 各自回归测试仍在树为证据（若后续重构删除守卫由对应测试挡下）。

### 3.3 wip-audit 2026-11-04

`23204f611`/`795a518bb`/`672420161`/`f0311e9f2` 四 commit 均为 HEAD 祖先；`hostdriver` 21MB 二进制仍以未跟踪 + `.gitignore:129` 形态留根目录（台账「保留工作区原样」维持）。

### 3.4 OPS-001 / OPS-010 / OPS-002 交付物

供给引擎（`internal/ops/provision/` 编排+模板+503 未配置面+迁移 0052/0053 注册 `migrations.go:249-250`）、销毁三闸（`provision.go:280` 护栏→排空→在途，fail-closed 500 `provision_load_unverifiable`）、`CountActiveByAgents`（`task_model.go:195`）、节点指标三面（`core/metrics/processes.go`、`agent_metrics_hourly` rollup/prune、15s 默认间隔 `ops_config.go:319`、迁移 0051）全部在位。已知边界（真机对账、Windows 采样、账号负载编排、多实例互斥）仍以交付时口径成立，未发现静默收窄。

## 4. 本轮新发现与残留（严重级别）

| #   | 级别 | 发现                                                                                                                                                              | 处置                                                                        |
| --- | ---- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| R1  | P2   | `README.zh.md:103` 仍以归档 legacy 文档为传输权威（EN 已在 CR-FIX-18 改指 ADR）；`scripts/check-docs-links.sh:37` 文件集不含 README.zh.md，该陈旧引用对 CI 不可见 | **立 AUD-108**                                                              |
| R2  | P3   | PGP 指纹未配置（`SECURITY.md:27`）                                                                                                                                | 无矛盾：CR-FIX-16 勾行已知边界已注 user-blocked（指纹须用户本人生成公布）   |
| R3  | P2   | P2-24 比原判更糟：性能脚本阈值纯配色不失败                                                                                                                        | 已有开放项 CR-FIX-24 / PERF-001..003 跟踪，本报告补充「阈值不传播失败」事实 |
| R4  | P3   | 能力×版本发布映射缺（P2-28）                                                                                                                                      | 已有开放项 CR-FIX-28 跟踪                                                   |
| R5  | 边界 | nightly-test 非阻断为**有意设计**（race/fuzz 红不挡 nightly 产物）                                                                                                | CR-FIX-25 勾行语义内，维持；如需阻断化应另立决策项                          |
| R6  | 边界 | 发布归档仍打包 admins.json（服务端首 seed 轮换兜底）                                                                                                              | CR-FIX-27 勾行已记录                                                        |
| R7  | 边界 | `cmd/analytics-export/main.go:113` 默认路径指向已删目录                                                                                                           | CR-FIX-20 勾行已记录（留后续批次）                                          |

## 5. 无法复核 / 未覆盖（诚实清单）

- **动态运行面不在本静态回归范围**：真实云 provider apply、真机/iOS 实机、真实部署与隔离库恢复演练、真实 Redis/ClickHouse 多进程对账——归 AUD-COV-002/003/004/006/007 的运行验证剩余范围，本报告不因其静态在位而宣称已覆盖。
- **C-list 未逐文件复验的 24 条**：以修复 commit 在 HEAD 祖先链 + 回归测试仍在树为证据；属抽样回归而非逐文件复跑。
- **11-12/11-13 两份深审**（`deep-audit-2026-11-12.md`、`code-logic-review-2026-11-13.md`）在 11-10 窗口外，其发现已分别落 AUD-007..028 与后续编号跟踪，不在本报告范围。
- 历史结论中依赖「运行验证部分完成」的条目（此前表内 P0-5 的 Go/Java 固定依赖阻塞、P1-8 背压组合负载复跑）维持原判定，本轮未新增动态证据。

## 6. 后续建议

1. **AUD-108**（新立，P2）：`README.zh.md:103` 权威引用改指 `docs/architecture/adr/0001-transport.md`（zh 镜像）；`check-docs-links.sh` 文件集补 README.zh.md（脚本有 mutation 自测先例，改动须带注入验证）。
2. CR-FIX-24 / CR-FIX-28 两条开放项照原验收口径推进（P2-24 需先修脚本阈值传播再谈入门禁）。
3. R5 阻断化、PGP 配置均涉用户决策，不擅动。
