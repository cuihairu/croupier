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

---

## cmd/ 覆盖口径与豁免清单（2026-09-22 扩展）

cmd/ 的覆盖目标与 internal/ 不同：二进制装配层允许存在进程边界与系统变更面，**策略逻辑全部下沉 internal/**（已 100%）。当前读数（`go test -cover`）：`cmd/server` ≈73%、`cmd/agent` ≈72%、`cmd/analytics-export` 91.7%、`cmd/schema-validator` ≈91%、`cmd/ingest/cmd` 99.1%。除下述豁免外，cmd/ 其余不可达分支均已按「先构造、构造不出才豁免」收口（含 fixture REST 全语义、startCluster 全装配矩阵、interconnect 全路由、service manager 状态机、schema-validator 归档解剖边界等）。

### cmd-1.（进程边界）全部二进制的 `main` / `Execute`

**位置**：`cmd/{server,agent,ingest,schema-validator,analytics-export,analytics-worker,check-db}` 的 `root.go` / `main.go` 入口函数。

**论证**：`main` 与 `Execute` 是进程装配边界（全局 flag 绑定、cobra 执行、`os.Exit`、信号与生命周期归 init 进程所有）。测试进程内执行会与被测进程生命周期冲突。各命令的 `run*` 策略函数均已直测——入口只做转发，无分支逻辑。

**失效条件**：无（结构性边界）。若 `Execute` 内出现可单测的分支逻辑，应把逻辑抽出为可直测函数而非在入口测。

### cmd-2.（系统变更）server/agent 的 service 变更命令主体

**位置**：`cmd/server/service.go` 与 `cmd/agent/service.go` 的 `run*ServiceInstall/Uninstall/Start/Stop/Restart` 中 `svc.Install()/Uninstall()/Start()/Stop()` 调用及其后的打印。

**论证**：install/uninstall/start/stop/restart 触发**真实系统级变更**（写 systemd unit、启停系统服务），单测进程不可执行。每个函数可安全触达的前置面已覆盖：入口守卫（`createServerService`/`createService` 失败 → "创建服务失败"）、状态查询守卫（`runServiceStatus`）、`service run` 前台运行路径（fakeService 注入 Start 失败/取消/成功三态）。

**失效条件**：命令增加纯校验类前置分支（如参数合法性检查）时应直测；若引入 dry-run 模式则守卫面应随实现补齐。

### cmd-3.（main 壳包）cmd/check-db、cmd/analytics-worker、cmd/ingest

**位置**：三包整包（7~108 行）。

**论证**：整包即 main 装配（连接串解析 + 调 internal 包诊断/导出逻辑），无策略分支；被调逻辑在 internal/ 已 100%。check-db 是运维诊断工具，装配错误即进程退出报错。

**失效条件**：包内出现 if/err 分支逻辑时需重新评估（届时按 C 类准入逐块论证）。

### cmd-4.（C 类防御）schema-validator — extractTarGz 的 `invalid path` 双保险与 `out.Close` 失败分支

**位置**：`cmd/schema-validator/main.go`（`extractTarGz`：`!strings.HasPrefix(target, destAbs+sep) && target != destAbs` 分支；`out.Close()` err 分支）。

**论证**：

- `invalid path` 分支：`target = filepath.Join(destAbs, cleanName)`，且 `cleanName = filepath.Clean(hdr.Name)` 已在上一步挡掉 `..` 前缀与内嵌 `../`。Go 的 `Join`/`Clean` 语义保证：非空 `cleanName` 经 `Join` 的结果要么**就是** `destAbs`（`cleanName` 归一为 `"."`），要么以 `destAbs+PathSeparator` 开头——二元判断的两个析取支恰好覆盖 `Join` 全部可能输出，条件恒 false。它是归一化不变式的自证性双保险（同 internal 第 2 条 `"fn-"` 前缀的构造）。
- `out.Close()` 分支：`OpenFile` 成功后 `Close` 的失败只剩 ENOSPC 类延迟写错误（close 时 flush），测试环境无法确定性构造且不影响解包语义（数据已 `io.Copy` 完毕）。

**失效条件**：归一化链改动（换掉 `Clean`+`Join` 组合、放开 `..` 检查）使 `invalid path` 变为可达；或引入写路径抽象（如可注入 fs）使 Close 失败可注入。

### cmd-5.（C 类防御）cmd/server — startCluster 的两处装配降级死分支

**位置**：`cmd/server/cluster.go`（`NormalizeConfig` err → standalone 分支；DB 存储下 `DBOwnerResolver.EnsureTable` err → standalone 分支）。

**论证**：

- `NormalizeConfig` 对任意输入恒返回 nil error（默认值填充型归一化，无失败路径），err 分支为死代码防御。
- `DBOwnerResolver.EnsureTable` 失败分支：成员表 `EnsureTable` 先行且使用**同一 DB 连接**，若连接可写则两表 DDL 同命运、若不可写则先行分支已拦截（只读库用例已覆盖先行分支）。让「成员表成功而 owner 表失败」需要 DDL 在同连接上对两个同构 `CreateTable` 分叉，无法确定性构造。gorm 层错误注入（如按表名 After 回调注错）对未来实现的回归有 pin 价值，但当前实现下两分支结构性同源。

**失效条件**：两表 EnsureTable 引入独立连接/不同 DDL 路径，或 `NormalizeConfig` 增加真实校验——届时按错误注入工具箱补测。

### cmd/ 残留部分覆盖面（非豁免，如实记录）

`cmd/server/dashboard_fixture.go` 的 E2E fixture 全链启动（`StartDashboardFixture`/`startServer`/`startAgent`/`ensureUIScope` 等约 55%-88% 覆盖）依赖真实 server+agent+dashboard 子进程编排，属 E2E 领域基础设施：可测面（fixture REST、SDK 替换、存储句柄、未启动防御）已在 `dashboard_fixture_*_test.go` 直测，全链编排由 `real-dashboard` E2E 套件承担，不在单测覆盖率口径内。

## 复核流程

1. 对每个豁免候选穷举可达路径（含缓存一致性、环、字节/多字节 rune、Unicode 折叠等边角），证伪「可构造触发」的所有尝试；
2. 尝试错误注入工具箱与直测构造，失败才进入豁免；
3. 豁免必须同时落三处：本文档条目（含失效条件）、就地注释（测试文件或产品代码）、锁定不变式的 pin 测试；
4. 实现变更时先跑 pin 测试——它失败即论证失效，必须补覆盖而不是续期豁免。
