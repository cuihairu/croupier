# 版本化审计摘要与失败策略

> 2026-10-30 实现复核：D1–D3 的摘要/兼容实现已有定向证据；D4 仍是目标约束：AuditFunctionInvoke/AuditApprovalCreated 仅打印错误，recordApprovalAudit 忽略错误，尚不能保证调用方返回 5xx 或阻断。ARCH-008 已重新打开。

> ARCH-007 · P2 · 方案决策 · 版本化审计摘要与失败策略
>
> Scope：`internal/audit` 审计链——`calculateHash` / `hashBytes` /
> `ValidateChain`（audit.go）、`audit_records` 的链字段
> （`chain_hash` / `chain_prev_hash` / `chain_sequence` / `chain_signer_id` /
> `chain_signature`，store.go）、可选 `AuditSigner`、以及 `Log` 的写失败路径
> （3 次 `ErrChainSequenceConflict` 重试后返回错误）。明确排除：链字段之外的
> 审计记录 schema、UI、保留/导出、多签名 quorum。
>
> Status：**DECIDED**（2026-10-29）。D1–D5 已定：规范化哈希字段集 +
> 跨数据库往返稳定的编码、SHA-256 取代 DJB2、显式 `chain_version` 列
> （迁移 0049，存量行回填 = 1，绝不改写既有哈希）、默认 strict 的写失败策略
> （显式 opt-in best_effort）、外部签名保持独立且单独校验的关注点。
> 驱动 ARCH-008。本文档不改任何运行时行为。已于 ARCH-008 落地（2026-10-29）。

## 1. 问题

审计链对记录的大多数字段检测不到篡改、使用非密码学哈希、且跨数据库往返
不稳定。写失败策略是隐式的、未决的。

证据：

- **字段集不完整。** `calculateHash`（audit.go）只哈希
  `timestamp`、`event_type`、`actor_id`、`resource`（ID）、`outcome`、
  `prev_hash`、`sequence`、`details`。篡改 `category`、`severity`、`action`、
  `changes`、`context`、`errorMessage`、`resource.gameId` /
  `resource.environment`、或任何 `actor` / `resource` metadata 字段对链不可见：
  记录重算出相同哈希。
- **非密码学哈希。** `hashBytes`（audit.go）是 DJB2——32 位 `uint32` 累加、
  格式化为 `%08x`，极易碰撞，非密码学摘要。其上注释字面写着 "Using SHA256
  for hashing / In production, you'd use crypto/sha256"，即意图与实现不符。
  既有 `chain_hash varchar(64)` 列恰好容纳 SHA-256 十六进制摘要（64 字符）——
  无需改列宽。
- **跨数据库往返不稳定。** 哈希输入用 `record.Timestamp.UnixNano()`
  （纳秒精度），但 postgres `timestamptz` 以微秒精度持久化、MySQL
  `datetime(3)` 以毫秒精度持久化。`Log` 哈希全精度时间；`ValidateChain` 在
  DB 截断后的时间上重算，于是带亚列精度时间戳的记录报假
  `hash_mismatch`。另外 `Details` / `Changes` 的数值经 JSON 往返后重编码为
  `float64`，大 `int64` 值可能在 Log 时与 Validate 时输入间发散。
- **无版本字段。** 无法在不破坏既有行校验的前提下演进字段集或算法；
  算法与字段集隐式存在于代码中，逐行无法区分。
- **隐式写失败策略。** `Log` 在 `ErrChainSequenceConflict` 上重试 3 次后返回
  错误。这对每类事件都是 fail-closed，无显式决策、无按类别区分、除传播的
  5xx 外无任何面向运维的信号。

外部签名（`AuditSigner` / `ChainInfo.Signature` / `SignerID`，列
`chain_signer_id` / `chain_signature`）已存在，并在 `ValidateChain` 中独立
校验。它是一个独立关注点，本文档必须相对哈希给出定位。

## 2. 决策

### D1 — 规范化哈希字段集与跨往返稳定编码

哈希输入是**全部**语义字段的固定、版本稳定集合——绝非易遗漏的子集。
空值与零值显式包含（不用 `omitempty`），故字段集闭合：将来任何改动都是
版本升级（D3），而非静默漂移。

规范化输入（版本 2）：

```
{
  "id":           record.ID,
  "timestamp":    record.Timestamp.UTC().UnixMilli(),
  "event_type":   record.EventType,
  "category":     record.Category,
  "severity":     record.Severity,
  "actor":        { id, type, name, email, roles, ipAddress, userAgent,
                    sessionId, mfaUsed, delegatedBy, metadata },
  "action":       record.Action,
  "resource":     { type, id, name, gameId, environment, metadata },
  "outcome":      record.Outcome,
  "error_message":record.ErrorMessage,
  "changes":      { before, after, diffFields } 或 null,
  "context":      { requestId, traceId, correlationId, service, version,
                    environment, tags },
  "details":      record.Details 或 {},
  "prev_hash":    record.ChainInfo.PrevHash,
  "sequence":     record.ChainInfo.Sequence
}
```

编码规则，目的：Log 时输入与 Validate 时输入逐字节相同。

1. **毫秒精度时间戳。** `Log` 在哈希前（及存储前）将 `record.Timestamp`
   截断到 `time.Millisecond`，使哈希所见与 postgres/MySQL 列往返返回的
   值一致。
2. **映射做 JSON 往返。** `Details` 与 `Changes.before`/`.after` 在 Log 时
   过一次 `json.Marshal` → `json.Unmarshal`，使其数值为 DB 归一后的
   `float64` 形态再哈希。消除大 `int64` 发散。
3. **确定性序列化。** `json.Marshal` 对映射按键字母序排序；嵌套映射同理。
   无 `omitempty`、无本地化格式、UTC 归一时间。

**决策：** 完整性指「任意字段改动都被检测」，而非「7 字段子集被链化」。
哈希有效的记录，其每个语义字段与签名者/哈希者承诺的内容逐位一致。

### D2 — SHA-256 取代 DJB2

`hashBytes` 改为对规范化输入字节取 `crypto/sha256.Sum256`，格式化为小写
十六进制（64 字符——恰好 `chain_hash varchar(64)`）。

- DJB2 **仅**保留为版本 1 行的旧算法（D3）。新行永不使用。
- 不改列宽；`chain_hash` 保持 `varchar(64)`。

### D3 — 显式 `chain_version` 列

编号 goose 迁移 **0049**（当前 48 之后）向 `audit_records` 加
`chain_version smallint NOT NULL DEFAULT 1` 并把存量行回填为 `1`。
新记录写版本 `2`。

- `ValidateChain` 按存储版本分派：`v1` → 旧字段集 + DJB2，`v2` → 规范化
  字段集 + SHA-256。
- **绝不改写既有哈希。** 后续每行的 `prev_hash` 引用前一哈希*值*；改写 v1
  值会断链。v1 行以旧算法保持可校验，链无缝延续进 v2 行。
- 版本被存储，而非按哈希长度推断。长度推断在两版本共享摘要长度时即歧义，
  未来算法也可能复用同一长度；显式列无歧义且经得起两者。
- 迁移遵循仓库规则：幂等（`HasColumn` 守卫后再 `AddColumn`）、注册进
  `registerSvcMigrations`、`MinimumRequiredVersion` 48 → 49、
  `migrate_test.go` 合成清单同步。

### D4 — 写失败策略：默认 strict，显式 opt-in best_effort

默认保持 **fail-closed**（`strict`）：3 次链冲突重试后，审计写失败返回错误，
调用方上抛（如 HTTP 5xx）。理由：审计系统不得静默越过痕迹缺口前进；运维
必须得知痕迹断了。当前行为已是如此；D4 将其固化为明文决策并增设一个
审慎的逃生门。

显式 opt-in `best_effort` 模式（配置旋钮
`audit.writeFailurePolicy: strict | best_effort`，默认 `strict`）只改失败分支：
`Log` 以 WARN 记录缺口、经既有 notifier 触发**限速**告警、并返回记录
（`ChainInfo` 带缺口标记）而非报错。它是 opt-in，因为静默越过审计缺口是
安全反模式；选择它的运维方在接受「已表面化、已告警的缺口」的同时换取
审计库故障期间控制面可用性。

**决策：** 默认完整性优先；可用性是审慎、显式、带告警的 opt-in。不存在
静默 fail-open。

### D5 — 外部签名是独立、可选关注点

`AuditSigner` 签名**不**并入哈希字段集，**不**因哈希算法或版本变更而重算。
它对 D1 的同一规范化载荷字节签名（或，若部署方偏好，对所得哈希签名——
部署方自选，置于算法路径之外），`ValidateChain` 照旧独立校验。

- 因此版本升级（D2/D3）不会使既有签名失效，既有签名无需重签。
- 保留单一签名列对（`chain_signer_id` / `chain_signature`）；多签名 quorum
  排除在外。
- 哈希与签名是两层独立完整性层；彼此不替代。

## 3. 影响 / 审查表

| 维度                                                                      | 之前（现状）            | 之后（ARCH-007 决策）                                           |
| ------------------------------------------------------------------------- | ----------------------- | --------------------------------------------------------------- |
| 篡改 category/severity/action/changes/context/errorMessage/scope/metadata | 不检测                  | 检测（D1）                                                      |
| 哈希算法                                                                  | DJB2 32 位非密码学      | SHA-256，64 十六进制，恰好容纳于 `chain_hash varchar(64)`（D2） |
| DB 往返（时间戳精度、JSON 数字重编码）                                    | 假 `hash_mismatch`      | 逐字节稳定，无假失配（D1）                                      |
| 算法 / 字段集演进                                                         | 隐式、无版本、破坏旧行  | 显式 `chain_version`，v1 旧路径保留，哈希绝不改写（D3）         |
| 审计库故障                                                                | 隐式 fail-closed（5xx） | 默认 strict；best_effort opt-in + WARN + 限速告警（D4）         |
| 外部签名                                                                  | 可选、独立校验          | 不变；独立于哈希版本（D5）                                      |
| 旧（v1）行                                                                | —                       | 旧算法仍可校验（D3）                                            |

## 4. ARCH-008 实现要点

1. 迁移 0049：加 `chain_version smallint NOT NULL DEFAULT 1`，带 `HasColumn`
   守卫；存量行回填 1；注册进 `registerSvcMigrations`；
   `MinimumRequiredVersion` 48 → 49；`migrate_test.go` 合成迁移清单同步。
2. 抽出 `canonicalHashInput(record, version) ([]byte, error)`：v1 返回当前旧
   输入 + DJB2；v2 返回 D1 规范化输入 + SHA-256。`calculateHash` 与
   `ValidateChain` 均按记录版本分派。
3. `Log` 中在 `buildChainInfo` 前（及存储前）将 `record.Timestamp` 截断到
   `time.Millisecond`；哈希前对 `Details` 与 `Changes.before`/`.after` 做 JSON
   往返。
4. 经模型写新记录版本（`2`，`chain_version` 列），使 `ValidateChain` 可逐行
   分派。
5. 重试后写失败按策略分支：`strict` 返回错误（现状）；`best_effort` WARN +
   限速 notifier 告警 + 带缺口标记返回记录。
6. 测试：逐字段篡改检测（翻转每个语义字段 → 哈希失配）、往返稳定（写 →
   读 → `ValidateChain` 无假失配）、v1 旧行以 DJB2 校验且 v2 行以 SHA-256
   校验、多实例 `ErrChainSequenceConflict` 仍被解决、`best_effort` 记录 +
   告警 + 无错返回、`strict` 返回错误。

本文档不改任何运行时行为。验收（逐字段篡改检测、旧链可验证、多实例写入
一致、按已批准策略处理审计库故障）属 ARCH-008 实现门禁。
