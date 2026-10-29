# 审计链完整性：版本化摘要与失败策略

> ARCH-007 · P2 · 方案决策 · 版本化审计摘要与失败策略
>
> 范围：`internal/audit/audit.go` 的审计链（`AuditService.Log` →
> `buildChainInfo` → `calculateHash`/`hashBytes`、`ValidateChain`）及其持久化
> （`internal/audit/store.go` `AuditModel`：`audit_records`）。审计保留/导出
> （`Archive`、`Export`）与通知器（`AuditNotifier`）不在本决策范围。
>
> 状态：**DECIDED**（2026-10-29）。D1–D7 已定：规范化全载荷摘要、升级 SHA-256、
> `hash_version` 兼容旧链、覆盖 scope/action/changes、失败开放+强制呈现的写失败
> 策略并附失败关闭安全白名单、确认外部签名已独占列。驱动 ARCH-008（落地审计
> 完整性规则）。本文档不建立生产就绪性。
> 前置：ARCH-008。

## 1. 问题

审计链本应让审计记录的篡改显形，但当前摘要只覆盖每条记录的窄摘要，且用非
密码学 32 位函数——记录的大部分含义（以及大部分篡改）都在完整性保证之外。

证据（均在 `internal/audit/audit.go`）：

- **窄哈希字段**（`calculateHash`，628 行）：摘要输入仅 `timestamp`、
  `event_type`、`actor_id`（仅 `Actor.ID`）、`resource`（仅 `Resource.ID`）、
  `outcome`、`prev_hash`、`sequence`、`details`（若设）。**未入哈希**：
  `action`、`category`、`severity`、`error_message`、`resource.type/name/
gameId/environment`、整个 `changes`（before/after/diffFields）、整个
  `context`、整个 `actor`（type/name/roles/ip/userAgent/sessionId/mfaUsed/
  delegatedBy/metadata）。改写 `changes.before/after`、`action` 或 scope
  （`resource.gameId`/`resource.environment`）的篡改因此原样通过链校验。
- **非密码学摘要**（`hashBytes`，807 行）：djb2，32 位，`%08x`。32 位非密钥
  哈希不是承诺——碰撞伪造廉价（~2^16 条记录即生日碰撞），对能重算的攻击者无
  原像抵抗。代码注释甚至写「生产应用 crypto/sha256」，但实现从没改。
- **无版本化**：没有 `hash_version` 列。一旦改动哈希字段集或算法，会静默
  失效所有既有记录的摘要；`ValidateChain` 会对整段历史报 `hash_mismatch`。
- **外部签名休眠**：生产把 signer 接为 `nil`（`internal/svc/
service_context.go:354` `NewAuditService(auditStore, nil)`）。列
  `chain_signer_id`/`chain_signature` 存在但恒空。
- **写失败处理不一致**：`Log` 返回 error，但调用点分裂为 `_, err := ...Log(...)`
  （可能上抛）与 `_, _ = ...Log(...)`（静默丢弃）——如 `internal/api/approval/
stepup_otp.go:149`、`service.go:142/371`、`internal/api/configexplorer/
service.go:84/171` 丢弃；`internal/platform/functioninvoke/approval.go:50`、
  `internal/api/console/service.go:1197`、`internal/api/page/service.go:1848`、
  `internal/api/openapi/service.go:2072` 捕获。无统一策略，且丢行时无可观测信号
  （指标/响应头）。
- **数字往返隐患**：`ToRecord` 用 `json.Unmarshal`（默认 → `float64`）解码
  JSON 列。`details`/`changes` 中 >2^53 的整数（如 id `9007199254740993`）
  以 `int64` 写入并哈希为 `"9007199254740993"`，读回变 `float64` 再序列化为
  `"9007199254740992"` ——对完好链条产生**假 `hash_mismatch`**。摘要必须能从
  存储表示复算，而非内存表示。

## 2. 决策

### D1 — 规范化全载荷摘要

摘要输入改为覆盖**每个完整性相关字段**的规范化 JSON 对象，取代当前窄列表：

```json
{
  "id":          "<audit_id>",
  "timestamp":   <unix_nano>,
  "event_type":  "<string>",
  "category":    "<string>",
  "severity":    "<string>",
  "action":      "<string>",
  "outcome":     "<string>",
  "error_message": "<string>",
  "sequence":    <int64>,
  "prev_hash":   "<string>",
  "actor":       <raw actor_json bytes>,
  "resource":    <raw resource_json bytes>,
  "details":     <raw details_json bytes>,
  "changes":     <raw changes_json bytes>,
  "context":     <raw context_json bytes>
}
```

- JSON 子对象（`actor`、`resource`、`details`、`changes`、`context`）以**存储的
  原始列字节**嵌入，而非重新解码值——写时与验时的摘要按构造逐字节一致，彻底
  消除数字精度隐患。（备选：用 `json.Decoder.UseNumber()` 解码；两者都满足
  「旧链可验证」，但原始字节为主选，因它对任意 JSON 值都精确。）
- 类型化标量字段按标量序列化；`sequence` 与 `prev_hash` **留在**摘要内，使
  链本身可防篡改。
- 排除：`chain.hash`、`chain.signature`、`chain.signer_id`（自指/密钥覆盖字段）
  与 `created_at`（存储插入时间戳，非语义字段）。
- `action` 恒存在（未设时默认 `event_type`，`setDefaults`），故绝不成为
  不确定性缺失。

### D2 — SHA-256 摘要函数

摘要函数改为 `crypto/sha256`，hex 编码（64 字符）。此为 **v2** 算法。v1 djb2
32 位仅保留用于校验既有记录（见 D3），绝不用于新记录。

### D3 — `hash_version` 列兼容旧链

- `audit_records` 加 `hash_version int` 列。
- 新记录写 `hash_version = 2`（SHA-256，D1 载荷）。
- goose 迁移把既有行回填为 `hash_version = 1`（旧窄字段 djb2 摘要）。
- `ValidateChain` 按记录分发：v1 重算旧摘要（算法与字段集不变——旧记录保持
  可验，且**检测的篡改与之前相同**）；v2 重算 D1/D2 摘要。
- 旧记录**不**重哈希：改写 `chain_hash` 会破坏所有后代行的 `prev_hash` 链，
  且重签历史是带自身完整性含义的刻意操作。链是线性单一全局链（见 D5），
  v1 记录只是 v2 记录链接其上的链头。

### D4 — scope、action 与 changes 覆盖（本条的「规范化字段」）

D1 已覆盖；此决策把意图明示。旧缺口——`action`、`resource.gameId`/
`resource.environment`（运营 scope）、`changes.before/after` 未入哈希——已关闭。
改写其中任一项现在都会断链。

### D5 — 单一全局链

链是单一全局序号（`chain_sequence`），非按 scope/game 分链。理由：审计 UI
全局列示；跨 scope 篡改相关性只在单一有序链上可见；按 scope 分链会放大校验并
复杂化「哪条链」问题。多实例并发沿用既有**唯一约束 + 重试**机制
（`store.go` `ErrChainSequenceConflict`、`Log` 重试循环）——不加咨询锁、不用
DB 序号。

### D6 — 写失败策略：失败开放+强制呈现，附失败关闭安全白名单

- **默认：失败开放。** 审计存储写失败时业务操作照常进行。理由：链是 append-only
  证据；丢行只是证据缺席（可观测），而阻断操作把审计故障变成功能故障且无完整性
  增益（行照样写不进）。失败必须**呈现**，绝不静默：`slog.Error("audit write
failed", ...)`、单调计数器 `audit_write_failures_total{event_type, category}`、
  调用方 HTTP handler 尽力写 `X-Audit-Degraded: true` 响应头。
- **失败关闭白名单**（不可逆安全变更，缺留痕比故障更危险）：
  `admin.user_delete`、`admin.user_lock`、`admin.user_unlock`、
  `admin.password_change`、`auth.mfa_enabled`、`auth.mfa_disabled`、
  `system.backup_restore`。对这些，`Log` 返回哨兵 `ErrAuditWriteFailed`，
  调用方**必须**中止操作（记录未建 → 操作不执行）。身份事件（`auth.login*`、
  `authz.access_*`）刻意失败开放：无法失败关闭（审计库不可用时没有身份可鉴），
  故封锁登录的分裂策略不可行。
- **单一咽喉点。** 所有调用点迁到同一 helper（`AuditService.Log` 保持签名；
  新增 `AuditService.LogOrDegrade(ctx, opts...)` 应用上述策略）。移除现有
  `_, err` / `_, _` 分裂。

### D7 — 外部签名：独占列已存在（无需改动）

`chain_signer_id`/`chain_signature` 已是每条记录的独立列，非嵌入摘要 JSON、
非独立表。**保留此布局。** 签名覆盖与摘要相同的规范化字节（哈希与签名一致），
以 `signer_id` 为键。生产当前传 `signer=nil`（列休眠）；接入外部签名器
（HSM/TPM/KMS）时填充既有列——无需 schema 变更。签名列对不是「要加的一列」，
它已经是每条记录单列。

## 3. 评审表

| #   | 场景（当前代码）                                         | 采纳 D1–D7 后                                               |
| --- | -------------------------------------------------------- | ----------------------------------------------------------- |
| 1   | 篡改 `changes.before/after`、`action`、`resource.gameId` | 断链（`hash_mismatch`）——D1/D4                              |
| 2   | 篡改 `details` 含 >2^53 整数                             | 今日验过；**完好链条假 mismatch**——原始字节嵌入修复（D1）   |
| 3   | 伪造 32 位 djb2 碰撞                                     | 新记录排除（SHA-256，D2）；旧记录保留既有暴露面（明示，D3） |
| 4   | 常规操作时审计库故障                                     | 操作继续，故障呈现（日志+计数器+响应头）——D6                |
| 5   | `admin.user_delete` 时审计库故障                         | 操作中止（`ErrAuditWriteFailed`）——D6                       |
| 6   | v2 链中夹 v1 旧记录                                      | 按 v1 摘要可验；后代链接其 v1 hash——D3                      |
| 7   | 多实例并发 `Log`                                         | 唯一约束重试；单一全局链——D5                                |
| 8   | 接入外部签名器                                           | 填充既有 `chain_signer_id`/`chain_signature`；无迁移——D7    |

## 4. 完整性下限与边界（诚实限制）

- 哈希链是**无密钥的防篡改**。它检测意外损坏与不重算链的偶发/回溯编辑（含直写
  DB）。具备 DB 写**且**能重算摘要的攻击者可伪造整条链——密钥化签名（D7）才把它
  升级为密码学不可抵赖。这是下限而非保证，如此明示以让 ARCH-008 的验证预期诚实。
- 检测依赖**周期性 `ValidateChain`** 全范围跑；无持续验证。ARCH-008 定节奏与告警。
- `hash_version` 回填（D3）让既有行*可验*，而非*更强*：v1 行仍受弱摘要保护。
  重签历史刻意排除（见 D3）。
