# Croupier 运维/安全与限制全量核对（需求清单 #56 核销归档）

## 状态

- 状态: 核对归档（2026-10-04，需求清单 #56）
- 日期: 2026-10-04
- 范围: 用户需求「允许的端口、IP 和域名过滤、允许的私有 IP、SSRF 保护之类的安全与限制配置」
- 结论: **已落地（2026-10-02，5b52d95）+ 已线上复证 + 记录与代码一致，零新缺口，OPEN-ISSUES #56 核销**；secguard 域覆盖率已由并行批次收口至 100%（97d6f8c，含 #56 契约的 `http.DefaultClient` 派生收敛与钳制矩阵用例）；本批纯文档零代码改动

## 一、需求 → 落地对照

| 用户需求      | 落地 | 证据                                                                                                                                                                                                                                                                                                            |
| ------------- | ---- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 允许的端口    | ✅   | `sec.allowPorts`（逗号分隔端口白名单，空=不限；写侧 1-65535 校验、读侧容错跳过非法片段）`layered.go:174` / `handler.go:213-220` / `secguard.go:113-121`                                                                                                                                                         |
| IP 过滤       | ✅   | `sec.allowIPs`（单 IP/CIDR 放行清单）`layered.go:175` / `handler.go:221-237` / `secguard.go ipAllowed:243-256`                                                                                                                                                                                                  |
| 允许的私有 IP | ✅   | 同 `sec.allowIPs`——SSRF 保护开启时内网依赖经此放行（CIDR.Contains / ParseIP.Equal）                                                                                                                                                                                                                             |
| 域名过滤      | ✅   | `sec.domainFilter`（后缀白名单，子域自动放行、`evil-example.com` 不撞 `example.com` 后缀）`layered.go:176` / `handler.go:238-242`+`validateDomainSuffixList:253` / `secguard.go domainAllowed:260-269`                                                                                                          |
| SSRF 保护     | ✅   | `sec.ssrfProtection`（bool，默认 false）双层拦截：`CheckURL` 静态校验（scheme 限 http(s)/端口/域名后缀/DNS 解析逐 IP 拒私有·回环·链路本地·未指定，`secguard.go:125-152`）+ `HTTPClient` 拨号 `Dialer.Control` 钩子（真实 connect 前复核对端 IP，消除 DNS 重绑定/解析 TOCTOU，`:158-182`/`dialControl:219-234`） |

## 二、既有记录 vs 代码核验（2026-10-04）

- 键族：`internal/platform/settings/layered.go:174-177` sec.\* 四键 ✓
- 守卫语义：`internal/security/secguard/secguard.go`——全关零开销直通（:126-128）、Resolve nil 安全（:48-51）、ipAllowed 非法 CIDR 跳过不中断（:243-256）、domainAllowed 后缀匹配（:260-269）✓
- **#56 契约·http.DefaultClient 收敛**：`HTTPClient` SSRF 开启时 Clone Transport 新实例（不改写 base）；`net.requestTimeoutMs > 0` 时即便守卫关闭也**派生副本**（`derived := &http.Client{}; *derived = *base`，:170-179）——webhook 接线正是以 `http.DefaultClient` 作 base（`notification.go:770`），原地改写会污染全进程共享实例 ✓（97d6f8c 已含「派生副本不污染共享 base」用例）
- **钳制矩阵**（#56 契约注记；实现属 net.\* 三键、需求归 #57）：`Retries()` 负值→0、>10→10（:197-205）；`Backoff()` ≤0→500ms 缺省（:211-216）；`TimeoutOrDefault` 0/负→调用方缺省（:185-190）✓ 代码与用例在册，行为面逐项核销留 #57 收口
- 接线两处（保存即热生效，外呼前每次 `secguard.Resolve(settings.Current())` 读 L3 快照）：通知 webhook POST `defaultPostJSONWithHeaders`（`approvals/notification.go:756-770`，覆盖钉钉/飞书/企微/通用 webhook）+「检查更新」拉取 `fetchRemoteVersion`（`ops/systeminfo.go:119-125`）✓
- 读视图：`OutboundSnapshot()`（`layered.go:1003-1021`，清单回显+逐键来源）→ 新端点 `GET /api/v1/site/outbound`（`sitesettings/handler.go:55`/`:71-74`）✓
- 写侧校验：validateValue 三键格式校验非法 400（`handler.go:212-248`：端口 1-65535 / IP·CIDR / 域名禁协议路径）✓
- 前端：账号安全 Tab「出站安全与限制」卡（`OutboundSecurityCard.tsx` 三清单逐键保存 + SSRF Switch + 边界 Alert，空值=清除覆盖回不限）+ `SecurityTab.tsx` 双卡并存查询按卡收窄 ✓；两套件 **19 用例 fresh 全绿**（本批复跑）
- 覆盖率（fresh，本批）：secguard **100%** / sitesettings **100%** / settings **100%** / approvals **100%** / ops **99.9%**（残余 `probe.go` 属 #57 域）✓
- 部署谱系（本批复证）：落地提交 `5b52d95` ∈ `444d7f0`（merge-base 断言通过）→ 线上复证 **2026-10-02 deploy run 36937558114**（gitCommit 444d7f0，双实例 healthy，迁移 completed）继续有效 ✓

## 三、边界（原登记四条，无变化）

1. 守卫仅覆盖 webhook 通知与检查更新两处外呼——agent/数据库/SDK 通道、固定 URL 外呼（GitHub OAuth 等）不经守卫。
2. 默认全关零行为变更（全关零开销直通）。
3. `domainFilter` 为允许清单语义（配置后仅清单内域名可出站，非黑名单）。
4. `CheckURL` 静态解析与真实连接间存在窗口，SSRF 保护开启时由 `Dialer.Control` 钩子 connect 前复核闭合。

## 四、核销动作

- OPEN-ISSUES.md #56 行追加「已核销（2026-10-04）」列（含本批复证证据）。
- todo.md 增 #56 核对段。
- vitepress 侧边栏登记本归档。
- **本批零代码改动**。门禁：`go build ./...` 0 错、`go test ./internal/...` 155 包零 FAIL、`tsc` 0 错、全量 jest 393 套件 4838 用例绿、guard PASSED。
