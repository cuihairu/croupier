---
title: 数据库迁移策略 — 版本化迁移替代 AutoMigrate 演进
---

# 数据库迁移策略:版本化迁移替代 AutoMigrate 演进

## 状态

Proposed(方向已定,分阶段落地中)。**不要再让生产库在启动时执行 AutoMigrate。**

## 决策

1. Schema 生命周期与 ORM 运行时职责分离:**GORM 只做 CRUD,不做 schema 演进**。
2. 采用**版本化迁移**(versioned migration)作为唯一 schema 变更事实源,迁移文件入库受 review。
3. 因 croupier 存在**运行时动态创建的游戏库**,采用混合模型:
   - 全新游戏库首次创建 → 运行时引导建库并应用基线版本(仅一次);
   - 已存在的库 → 按版本号追平待应用迁移;
   - 版本升级 → 运维触发的批量滚动工具覆盖 N 个存量游戏库。
4. 启动只做**兼容性校验**(库版本 ≥ 程序要求的最低版本),失败直接拒绝启动,绝不静默 ALTER。

## 背景与现状(实证)

当前实现是"GORM AutoMigrate + 手写补偿钩子 + 散落 SQL"形态,补偿成本已经出现:

- `internal/model/migration.go`:AutoMigrate 前必须先跑 `migrateEnumColumns`(注释原话:_AutoMigrate would alter their types which would destroy values_——即不处理会毁数据)、`renameLegacyTables`、`dropLegacyPageUniqueIndexes`,postgres 还需要约束自愈重试循环;之后还有 `CleanupAllLegacy`。这些全是**没有版本化迁移后手写的补偿层**,且会永久存在、持续膨胀。
- `internal/db/router/router.go`:按 `(game_id, env)` 运行时懒创建物理游戏库(`game_<id>_<env>`),`MigrateGame` 在首开时执行;singleflight 仅进程内去重,**多进程同时启动仍会对同一张表并发 ALTER**。
- `migrations/001_openapi_schema.sql`、`002_function_openapi_backfill.sql`:已经开始出现游离的 SQL 文件,但没有版本表、没有统一执行器。

## 方案对比

| 方案             | 评价                                                                                                                   | 结论                                  |
| ---------------- | ---------------------------------------------------------------------------------------------------------------------- | ------------------------------------- |
| GORM AutoMigrate | 新库引导方便;但对已存在的库反复执行是危险操作:无审计、无回滚、DDL 不可审查、大表 MDL 锁、多实例竞态、schema 与代码漂移 | 仅保留给**全新库基线**                |
| Atlas(versioned) | diff 生成迁移、lint、test、CI 生态完整;支持 MySQL/MariaDB/Postgres/SQLite                                              | **首选工具链**(可用于生成基线与 lint) |
| Goose            | 成熟、SQL + Go 双模式,适合复杂数据回填                                                                                 | 可接受的执行器备选                    |
| golang-migrate   | 经典纯版本执行器                                                                                                       | 同上                                  |

工具不是重点,**治理模型才是**:迁移必须是"带版本的、进 git 的、可 review 的、幂等追平的"。在此前提下执行器选 Atlas 或 Goose 均可。

## croupier 特殊性:动态游戏库

通用教程假设 schema 目标集合已知且有限,CI 直接对目标库执行即可。croupier 的游戏库在**首次被访问时才创建**,未来的游戏库今天不存在,CI 无法对其执行迁移。因此采用混合模型:

```text
全新库(首次 EnsureDatabase)
    └─ 运行时引导:CREATE DATABASE + 应用基线版本一次(仅 CREATE 语义)
        └─ 写入 schema_migrations 版本记录

已存在的库(server 启动 / 首次访问)
    └─ 按 schema_migrations 追平未应用的版本
    └─ 跨进程锁:MySQL GET_LOCK / Postgres advisory lock(sqlite 单文件无此问题)

新版本发布
    └─ 运维触发 fan-out 工具,按 game_envs 注册表批量滚动所有存量游戏库
```

规则:

- AutoMigrate **仅允许**出现在"检测到全新空库"的分支里作为基线生成手段;对已有数据的库一律走版本化迁移。
- 迁移文件命名 `NNN_<描述>.sql`（必要时配 goose Go 迁移做数据回填/方言探测，如 0002–0004），嵌入二进制，保证 fan-out 工具离线可用。
- 每条迁移必须幂等或在事务内与版本写入原子提交；MySQL DDL 不支持事务，因此探测式幂等（HasTable/HasColumn 探测后执行）是必需形态。
- 修改 `internal/db/migrate/migrations/*.sql` 后运行 `make migrate-hash` 更新 `atlas.sum`，CI 会用 `atlas migrate validate` 校验。

## 数据变更规范(Expand → Contract)

改字段/含义时禁止一步到位 DROP:

```text
Expand      扩展:ADD COLUMN 新列(带默认值)
Migrate     回填:UPDATE 迁移旧值(大数据量分批,避开高峰)
Switch      切换:代码双读写过渡
Contract    收缩:确认全部数据迁移完成后才 DROP 旧列
```

`migrateEnumColumns` 就是该模式的现有手工范例,后续应改写为标准版本化迁移条目而非启动钩子。

## 启动校验

Server/Agent 启动时读取 `schema_migrations` 当前版本:

- 库版本 < 程序要求最低版本 → 打印明确错误(`database schema version 181, required >= 182`)并拒绝启动;
- 禁止任何组件在请求路径上隐式触发 DDL。

## 与"数据再生"的边界

**Schema 迁移和数据再生是两类问题,不要混用工具。**

- 列类型/结构变化 → 本文的版本化迁移。
- 业务/spec 内容重生成(例如页面生成器升级后,旧 PageProposal/PublishedPageSpec 中残留 Title Case 兜底标签)→ 应用层按 `generatorVersion` 升版触发再生成或清理,不写 DDL。
- 已发布的页面快照永不静默变更;再生成必须显式(升 generator version 或运维命令),与"契约变化 → stale → 重发布"既有链路保持一致。

## 分阶段落地

1. ✅ 引入 `schema_migrations` 版本表与统一执行器（goose，`internal/db/migrate`，版本表 `goose_db_version`）；AutoMigrate 收敛到全新空库分支（baseline 桥，仅一次性）。
2. ✅ 启动兼容校验（`MinimumRequiredVersion`，追平后低于最低版本即拒绝启动）；跨进程会话锁（MySQL `GET_LOCK` / PG advisory lock）覆盖「版本表探测 + baseline + 追平」全程，接入 `EnsureDatabase`（PG 建库竞态容错）与 `MigrateGame`；单库模式（`multiGame: false`）同样经 `EnsureUpToDate` 执行，不再每次启动裸跑 AutoMigrate。
3. ✅ 存量补偿钩子（enum 回填、legacy 表/列/索引清理、openapi 回填）改写为编号迁移 0002–0004（goose Go 迁移，注册于 `internal/svc/migrations.go`，与 baseline 桥共用同一实现）；钩子从「每次启动执行」收敛为「仅 baseline 一次性」。根目录游离 SQL（001/002）已删除（002 收编为 0002）。
4. ✅ CI 增加三方言迁移测试（`ci.yml` 的 `migrate-matrix` job：真实 MySQL 8.4 + Postgres 16 services，含并发首开锁验证；SQLite 由单测覆盖）与 Atlas 校验（社区版 `atlas migrate validate` 校验 `atlas.sum` 与文件顺序；注意 `atlas migrate lint` 自 v0.38 起 Pro 付费，规则化 lint 暂不可用）。

   **SQL Server 状态（进行中）**：迁移执行器侧已就绪——`internal/db/migrate` 支持 sqlserver 方言映射、`sp_getapplock` 会话锁、`sys.objects` 版本表探测，集成测试 case 与 CI mssql 容器已接线（见 `ci.yml` 中被注释的 `TEST_SQLSERVER_DSN`）。当前阻塞在模型层：`datatypes.JSON` 对 sqlserver 无 DB 类型映射，gorm sqlserver 驱动对未知类型返回字面量 `json`，SQL Server 无此类型导致 AutoMigrate 建表失败。落地路径：引入方言感知的 JSON 列类型（sqlserver 用 `nvarchar(max)`），属独立的模型层改造，完成后取消 CI env 注释即可进矩阵。

5. ✅ fan-out 批量滚动工具：`croupier-server db fanout`（读 `game_envs`，meta + 逐游戏库追平，输出报告表格；`--dry-run` 仅报告版本不执行 DDL；注册表引用但物理缺失的库标记 `missing-database`，运行时懒建不算错误）。

## Review Checklist

合并前检查:

```bash
rg -n 'AutoMigrate\(' internal cmd --glob '!**/*_test.go'
```

新增 AutoMigrate 调用必须位于"全新空库引导"分支,否则视为 review failure;任何 DDL 变更必须附带编号迁移文件,禁止只在 model struct 上改 tag 了事。

## 已知债务

构造期 AutoMigrate 已基本收敛（audit / metrics 表并入 `internal/svc` 的 baseline 回调 `migrateAuxModels`，构造函数不再执行 DDL）。当前剩余的 AutoMigrate 调用及其定性：

- `internal/model/migration*.go` — baseline 桥本体，合规。
- `internal/svc/service_context.go`（autoMigrate / autoMigrateMeta / migrateAuxModels）— baseline 回调，合规。
- `internal/platform/approvals/` — `ApprovalModel` 及 workflow 表；PG/SQLite store 使用**自有独立 DSN**（非 server meta 库），自建自管属合理形态。
- `internal/platform/registry/agent_session_db.go` — 由 svc baseline 显式调用（表结构在 platform 包，避免 model→platform 反向依赖）。
- `internal/platform/monitoring/certificates/` — 模型已在 `MetaModels` baseline；`Store.AutoMigrate` 方法保留为测试 fixture 入口。
