# 覆盖率豁免论证档案

internal/ 目标的逐包语句覆盖率为 100%。本文档是**唯一豁免清单**：穷尽复核后确认「在当前实现下不可构造真实触发」的防御性分支，逐条给出结构化论证与失效条件。豁免块均配有 pin 测试锁定其依赖的不变式——若未来实现变化使论证失效（分支变为可达），对应测试会失败并提醒补错误路径用例，届时必须补覆盖而不是续期豁免。

## 分类与准入

只接受 **C 类（防御性兜底）** 豁免：分支防御的是「标准库/上游不变式之外」的假想异常，且可给出穷尽的结构性论证。以下情形**不构成**豁免理由：

- 调用链上游恰好不会传某值（对私有函数直测即可构造）——先例：`createUnboundContractsForSource` 的空 operationId 分支，生产链路上游整源拒绝，但直测传空白元素即真实可达，已补直测而非豁免。
- 错误注入工具箱（gorm After 回调按表名/SQL 注错、sqlite RAISE 触发器、只读库 PRAGMA、关闭连接池、`exporterFunc` 通道确认等）能确定性触达的分支。
- fire-and-forget goroutine「调度运气」型覆盖抖动——先例：`TracerProvider.EndSpan` 逐 exporter 起的 goroutine，用通道等待交付信号后确定性覆盖，而非豁免。
- 锁相位对齐/概率轮次的时序编排——先例：`Router.GameDB` 的 singleflight re-check 分支，以回调入口测试接缝 `gameDBInflightHook` 确定性触发，替代已退役的 24 轮锁泊车 + TryLock 自旋编排（该编排在全量负载下会整轮落空，正是覆盖抖动来源；时序分支用注入点）。

## 当前豁免清单（共 3 条）

### 1. `internal/api/menu` — filterAccessibleTree 的 `!check(parent)` 分支

**位置**：`internal/api/menu/service.go`（`filterAccessibleTree` 主循环，`!check(parent) { continue }`）。

**论证**：对任意条目 `item`，主循环走到该分支的前提是 `check(item)=true` 且 `item.ParentID != nil` 且 `byID` 中存在该父节点。而 `check` 的递归结构保证：`check(item)=true` 时若父节点存在，则 `check` 内部已执行 `ok = check(parent)` 且返回 true（否则最终 `ok=false`）。主循环再次 `check(parent)` 命中 `accessible` 缓存返回同一 final 值，故 `!check(parent)` 恒为 false。

关键不变式（pin 测试锁定）：

- `check` 对每个节点只计算一次 final 值并缓存，后续调用全部命中缓存（值一致性）；
- 成环节点的 final 值恒为 false（递归前先落 false 作环守卫，false 沿环传播）；
- `ParentID`、`byID`、权限集合在整个函数执行期间不可变（主循环与递归看到同一世界）。

**失效条件**：`check` 改为可重入计算（如引入动态可见性、去掉缓存）或主循环/递归使用不同的父节点解析。

### 2. `internal/api/openapi` — unboundFunctionID 的 `"fn-"` 前缀分支

**位置**：`internal/api/openapi/service.go`（`unboundFunctionID` 末尾 `return "fn-" + out`）。

**论证**：函数先把输入归一到字符集 `[a-z0-9._-]`（小写折叠 + 非字符集 rune 逐个替换为 `-`，多字节 rune 不可能存活到输出），再用 `strings.Trim(out, ".-_")` 去首尾。cutset `".-_"` **恰好是字符集内全部非字母数字成员**，因此非空结果的首字符必属 `[a-z0-9]`，前一行 `if out[0]` 的字母数字判断恒为真、恒走 `return out`。`"fn-"` 分支是字符集不变式的自证性兜底。

**失效条件**：builder 字符集扩入其他符号（如 `~`），或改用非 Trim 的首字符归一策略。

### 3. `internal/platform/monitoring/certificates` — fetchCertificateInfo 的 `len(certs)==0` 分支

**位置**：`internal/platform/monitoring/certificates/certificates.go`（`fetchCertificateInfo`，论证同时就地注释在产品代码）。

**论证**：`tls.DialWithDialer` 握手成功即保证 `ConnectionState().PeerCertificates` 非空——Go 标准库 TLS 客户端不实现任何匿名（aNULL）套件，服务器不出示证书则握手必然失败并进入上方 err 分支；Go 客户端亦不支持客户端侧 PSK，TLS 1.3 会话恢复场景下服务器仍发送 Certificate 消息。该分支是对标准库行为之外的防御性兜底，且 dialer/config 均为函数内构造、无注入缝，无法用 fake server 触达（Go 的 `tls.Server` 同样要求出示证书才能完成握手）。

**失效条件**：Go 标准库引入客户端侧 PSK/匿名套件，或函数改为可注入 `tls.Config`/拨号器。

## 复核流程

1. 对每个豁免候选穷举可达路径（含缓存一致性、环、字节/多字节 rune、Unicode 折叠等边角），证伪「可构造触发」的所有尝试；
2. 尝试错误注入工具箱与直测构造，失败才进入豁免；
3. 豁免必须同时落三处：本文档条目（含失效条件）、就地注释（测试文件或产品代码）、锁定不变式的 pin 测试；
4. 实现变更时先跑 pin 测试——它失败即论证失效，必须补覆盖而不是续期豁免。
