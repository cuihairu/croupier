# AUD-COV-002 收尾审计：迁移回滚、真实驱动故障、任务恢复全链（2026-11-28）

范围承接 todo.md AUD-COV-002 的进度记录：此前轮次已覆盖 GameDB 首开/关闭、Redis 消费进度、
任务留痕（DATA-019～023、AUD-057～058、AUD-077）、调度状态/事件游标/会话锁（AUD-086～091、
AUD-098）、缓存序列化（AUD-100）、分析发布与读仓（DATA-043～049）、Worker 批量/确认/重试/
跨组检查点（DATA-050～055）。本报告补齐最后三项遗留并收口。

## 1. 迁移回滚（migration rollback）

**结论：无自动 down 路径是有意设计；失败重跑安全性由逐迁移幂等契约承担。无新缺陷。**

- 生产代码不存在任何 down 迁移调用：`internal/db/migrate/migrate.go` 全文仅 `provider.Up`
  （migrate.go:292 一带），`goose.Down/GoDown/UpByOne` 全仓零命中（生产代码）。回滚手段 =
  重新部署旧版本二进制；`MinimumRequiredVersion` 门（migrate.go:331-333 一带）拒绝低于
  要求版本的库启动，即「库停旧版、修复版重跑」策略。
- 执行链（`ensureUpToDate`，migrate.go:270-340）：跨进程 session lock（AUD-098 语义，释放
  失败 `errors.Join` 不吞迁移错误）→ 版本表探测 → baseline 桥（唯一 sanctioned AutoMigrate
  路径，仅无版本表时执行，失败不记版本、下轮启动重探测重跑）→ goose `Up` 逐迁移前进 →
  版本 ≥ MinimumRequiredVersion 校验。
- **事务形态（本轮新核对）**：`internal/svc` 全部 70 个 Go 迁移均为
  `goose.NewGoMigration(n, &goose.GoFunc{RunDB: ...}, nil)` 形态（RunTx 零命中）——goose 对
  Go 迁移不包事务，单个迁移中途失败可能留下半套 DDL。安全性依赖仓库迁移契约
  （CLAUDE.md「迁移必须幂等」：逐列 HasColumn+AddColumn、HasTable+CreateTable、缺表跳过）
  使重跑收敛到同一终态；0023 事故（postgres 索引对齐 panic）后契约进一步禁止整模型
  AutoMigrate。0001_baseline.sql 本体是幂等占位（`SELECT 1`），真实 schema 由桥建。
- 版本记录语义与 CLAUDE.md 运维要点一致：迁移失败不记 goose 版本（库停旧版），修复版
  同版本号干净重跑。

## 2. 真实驱动故障（real driver failures）

**结论：router 故障路径已由 ARCH-009/AUD-077 轮次硬化；本轮复核未发现新缺陷。**

- `internal/db/router/router.go` `GameDB`（router.go:225-325）：closed 拒服务（AUD-077）、
  已取消调用方零 I/O（ARCH-009）、读锁快路径、singleflight `DoChan` 去重首开、回调入口
  缓存重查、发布期 close 竞态三重防护（closed 后发布→关池返 ErrClosed；Forget 竞态→择旧
  关新）、失败不产生负缓存（singleflight 完成即清，下个调用方重试），`openFailCount` 计数
  经 `Stats()` 披露。
- `openGameDB`（router.go:518-580）：首开预算（默认 5 分钟）四相位检查（ensure/open/migrate
  及取消检查）；EnsureDatabase→Open→PingContext（补偿 gorm.Open 不吃 context）→MigrateGame
  全链每步失败都 `closeQuietly` 回收连接池；`openWithContext` 对超时后迟到的 Open 结果做
  延迟关闭防泄漏。
- 残留小观察（不立新项）：db.DB() 解包失败时 ping 被静默跳过（router.go:564 `err == nil`
  才 ping）——新开连接池上该错误实际不可达（驱动已注册），风险可忽略；如未来引入懒加载
  驱动需回看此行。

## 3. 任务恢复全链（task recovery across restart）

**结论：服务端恢复链完整且语义诚实；「timed_out/unknown = 无进一步证据」边界与既有
ARCH-023..027 未决项一致。无新缺陷。**

- 看门狗（ARCH-018 D3）：`internal/server/control_background.go` `runTaskWatchdog` —
  活动行 `updated_at` 早于预算 → `timed_out`（终态，含义是「无进一步证据」而非「游戏侧
  失败」），30s 上限间隔 ticker 驱动。
- Agent 断连收割（ARCH-014 D3）：`cmd/server/root.go:658-662` 绑定
  `ReapAgentNonTerminal` → 该 agent 全部活动状态（`ReapableStatuses` = queued/dispatching/
  running/cancel_requested，`internal/tasks/types.go:42-50`）→ `unknown`（执行状态未确立，
  不得自动重跑）；收割失败仅记日志不阻塞断连清理。
- 作用域感知：`internal/tasks/store.go` `ReapStale`（store.go:145-158）经 `reapAcrossScopes`
  扇出全部 OPEN 作用域 + `reapDormantScopes`（AUD-015）限流补扫注册未开作用域——多库模式
  下从未被用户打开的 game DB 的陈旧任务同样收敛；失败聚合计数不静默。
- 进程重启路径：重启后由看门狗按预算把上一进程遗留的非终态行收敛到 timed_out——没有
  「重启即重置」或「自动重跑」路径，与「执行状态未知不自动重跑」原则一致。游戏侧凭据/
  回执契约（跨进程 exactly-once）仍是已记录的未决边界（CLAUDE.md 幂等节 + ARCH-023..027）。

## 收口判定

AUD-COV-002 声明的全部子范围（meta/game 路由、事务边界、迁移升级/回滚、异步任务状态与
进程重启恢复、缓存、队列/MQ）至此均已复核：本轮三项（迁移回滚、真实驱动故障、任务恢复
全链）无新增缺陷；既有未决边界均有编号条目跟踪（ARCH-023..027、AUD-015 等），不以
「扫描器无告警」替代人工复核。
