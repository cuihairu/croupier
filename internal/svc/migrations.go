package svc

import (
	"fmt"

	"github.com/pressly/goose/v3"
)

// This file turns the legacy compensation hooks (see
// docs/architecture/database-migration-strategy.md, phase 3) into numbered
// goose Go migrations. They reuse the exact same implementations as the
// baseline bridge in internal/model so the two paths can never drift.
//
// Version layout:
//   0001 (SQL)  baseline marker
//   0002 (Go)   functions openapi column backfill
//   0003 (Go)   legacy table/index/column cleanup
//   0004 (Go)   varchar→int enum column conversion
//   0005 (Go)   game-support context columns (docs/research/game-support-systems.md)
//   0006 (Go)   bug tracker baseline table (docs/research/bug-tracking-design.md)
//   0007 (Go)   tool registry baseline table (docs/research/tool-registry-design.md)
//   0008 (Go)   game release baseline table (docs/research/release-management-design.md)
//   0009 (Go)   config namespace column (docs/research/config-hot-reload-design.md)
//   0010 (Go)   ticket CSAT columns (docs/research/game-support-systems.md P2)
//   0011 (Go)   hotpatch baseline table (docs/research/hot-patch-design.md)
//   0012 (Go)   db source registry table (docs/research/db-monitoring-design.md)
//   0013 (Go)   platform settings table (docs/architecture/config-layering.md)
//   0015 (Go)   agent_sessions.addr column (HA 跨实例节点视图的 IP 展示)
//   0014 (Go)   task schedules tables (docs: cron 调度 task_schedules/run_logs)
//   0016 (Go)   function_contracts.timeout_ms column（声明式超时执行层接线）
//   0017 (Go)   admins 登录安全列（failed_attempts/locked_until/token_version，
//               见 todo.md T1/T2：登录失败锁定 + token 撤销）
//   0019 (Go)   announcements + announcement_reads（公告系统：markdown 公告 + 弹窗确认）
//   0018 (Go)   admins.otp_enabled 列（T3：MFA 按 provider 接线，local 账号
//               可启用 TOTP；ldap/oidc 登录跳过平台 MFA）
//   0020 (Go)   execution_logs 表（R1 执行留痕：payload 级请求/响应落库）
//   0021 (Go)   function_contracts.prev_input/output_schema 列（selector
//               一键同步的 prev schema 存储；sync-selectors 上线时漏配
//               存量库迁移，agent 注册 upsert 直接 SQL 报错）
//   0022 (Go)   term_dictionary.display JSON 列（57fac95df 双列→JSON 重构
//               只改了模型，存量库从未跑过 AutoMigrate，seed 持续报
//               column "display" does not exist）
//   0023 (Go)   component_templates.params/digest 列（U6 模板参数化加 Params、
//               U11 更新提醒加 Digest 时均只改了模型，存量 game 库过 baseline
//               后不再跑 AutoMigrate，创建/更新模板持续报 column "params"
//               does not exist）
//   0024 (Go)   软删除残留行清理（删除路径改硬删后，把 page_specs/
//               page_proposals/component_templates/openapi_source_bindings/
//               registration_warnings 里已软删的存量行物理清除——它们占着
//               物理唯一索引，同 key 重建 500）
//   0025 (Go)   function_contracts.execution_state 列（D2/T3：契约执行
//               状态 bound/unbound，存量行默认 bound，行为与现状一致）
//   0026 (Go)   roles/admins 软删除残留行清理（0024 同族漏网表：删除路径
//               改硬删后物理清除已软删的存量行——roles.name 与
//               admins.username 的物理唯一索引被软删行占位，同名重建 500）
//   0027 (Go)   menu_items 表 + page_specs.menu_id 列（T-M1/T-M4 菜单系统
//               落地时只改了模型，存量库过 baseline 后不再跑 AutoMigrate：
//               menus API 因表缺失 500，页面保存/发布链因 GORM 全字段
//               INSERT 报 column "menu_id" does not exist 中断——0021/0023
//               同族「模型改了迁移漏配」事故）
//   0028 (Go)   sdk_version_highwatermarks 表（SDK 滑动版本门槛的高水位
//               存储：per (game_id, env, sdk_language) 记见过的最高版本；
//               新建表无存量约束名漂移，0014/0027 建表同模式）
//   0031 (Go)   function_contracts.removal_pending_at 列 + 清扫候选
//               过滤索引 idx_function_contracts_removal_pending（摘除
//               宽限：瞬态摘除延迟真删，过期清扫；0021/0025 加列模式）
//   0030 (Go)   function_version_floors 表（函数级最低 SDK 版本门槛：
//               平台设置独立于注册物化，0030 同 0028/0029 新建表模式）
//   0029 (Go)   function_contract_versions 表（B2 函数契约变更历史：
//               per (game_id, env, function_id) 的内容变化快照流；
//               新建表无存量约束名漂移，0028 同模式）
//   0032 (Go)   admin_otp_recovery_codes 表（MFA 备用恢复码：TOTP 绑定时
//               一次性签发、逐条一次性消费；新表无存量数据，0028/0030 同模式）
//   0033 (Go)   admins 密码策略列（must_change_password 登录后必须改密 +
//               password_expires_at 密码有效期截止，NULL=长期有效；OPEN-
//               ISSUES #20；0017/0018 加列同模式）
//   0034 (Go)   provider_metadata 表（provider 自报元数据 EAV 持久化，
//               OPEN-ISSUES #11：元数据此前纯内存态重启即失；新表无存量
//               数据，0028/0030 同模式）
//   0035 (Go)   bug_ticket_links 表（bug↔工单多对多关联 #25：新表无存量
//               数据，0028/0030/0032 同模式；0034 已被 provider_metadata
//               占用故顺延）
//   0036 (Go)   announcement_games 表（公告↔游戏多对多绑定 #45：一公告可
//               绑定多游戏、未绑定=全服可见；新表无存量数据，0019/0035
//               同模式）
//   0037 (Go)   email_verifications 表（注册邮箱验证令牌 #51c 第二批：只存
//               SHA-256 哈希、一次性 used_at、重发即作废）+ admins 逐列补
//               email_verified（0033 密码策略列同模式）
//   0038 (Go)   cicd_integrations + cicd_builds 表（CI/CD 可插拔 provider
//               接入 #58：外部构建系统注册 + 构建记录；game-scoped，单库/
//               多游戏两 scope 都补齐；0020 execution_logs 同模式）
//   0039 (Go)   games 软删除残留行清理（0024/0026 同族：games 删除路径改
//               硬删后，把已软删的存量游戏行物理清除——game_id/alias_name
//               的物理唯一索引被墓碑行占位，同名重建 500，线上实证）
//   0040 (Go)   incident_categories + incidents 表 + bugs.category_id/
//               subcategory 列 + 播种默认七行类别（事故报表体系批 1，
//               docs/design/incident-reports.md；建表 0019/0038 模式、
//               加列 0025 模式、播种按 slug 幂等缺行才插）
//   0041 (Go)   incident_reports + external_tokens 表 + messages 加五列
//               （level/source/ref_type/ref_id/scope，level 历史回填 info）
//               + task_schedules.kind 列（回填 function）（事故报表体系
//               批 4 §2.4/§2.5：建表 0019/0038 模式、加列 0025 模式；
//               调度行播种不在迁移内——Manager 支持 kind 路由后才落行）
//   0042 (Go)   alerts 工作流状态四列 handle_status/handled_by/
//               handled_remark/handled_at（风险事件扩展，
//               risk-control-panel-design.md §3.3；独立于入站 lifecycle
//               status 列，加列 0033 模式；存量行回填 pending）
//   0043 (Go)   alerts scope 两列 game_id/env（4862 ops 告警列表按
//               game/env 过滤；空 = 平台级全局可见，非空 = game/env 级；
//               加列 0033 模式；存量行不回填——历史告警无 scope 归属，
//               诚实留空）
//   0044 (Go)   agent_metrics_history.probe_json 列（探测统计持久化，
//               agent-health-recovery-review.md §13.1 统计面；加列 0025
//               模式；存量行不回填——历史窗口无统计快照）
//   0045 (Go)   db_sources.owner_agent_id 列（DB 源周期探测归属 agent，
//               agent-health-recovery-review.md §12 R4；加列 0025 模式；
//               存量行不回填——历史源无归属=未归属，server 诊断端点兜底）
//   0046 (Go)   probe_events 建表（synthetics 探测事件持久化表；此前只在
//               AutoMigrate 路径存在，线上 postgres 缺表即写入报错，且
//               payload `type:blob` 为 MySQL 专有使 Migration Matrix 在
//               pg/mssql 直接失败；HasTable 检查后 CreateTable，新表无
//               存量约束名漂移；payload 走驱动 bytes 默认映射）
//   0047 (Go)   probe_events.target_category 列（外部探测目标分类 31：
//               synthetics-agent 新目标强制携带类别；结果行保存执行时
//               快照，便于不过 proto 反序列化过滤；存量行不回填——历史
//               未分类行只在「全部」视图出现，不猜归属）
//   0048 (Go)   page_execute_idempotencies 建表（D4 页面执行服务端幂等：
//               Idempotency-Key 头对页面绑定执行去重——同 key 已完成重放
//               结果、进行中判结果未知拒绝、超窗遗留可回收；新表无存量
//               约束名漂移，HasTable 检查后 CreateTable，0027/0038 模式；
//               建在 game scope 表集内，随 multiGame 路由到物理库）
//   0049 (Go)   audit_records.chain_version 列（ARCH-008：逐行钉住哈希算法
//               与字段集——1=旧 DJB2 7 字段子集、2=规范化 SHA-256 全记录；
//               存量行经列 DEFAULT 回填 1，既有哈希绝不改写；AddColumn
//               补列模式，0015/0016/0021/0025/0027）
//   0051 (Go)   agent_metrics_history.networks/processes/ports 列 +
//               agent_metrics_hourly 建表（OPS-010 节点资源指标持久化：
//               网络收发速率/进程清单/监听端口清单三面落原始表，另建
//               90 天小时聚合表；加列 0025 模式、建表 0027/0038 模式；
//               存量行不回填——历史窗口无这三面数据，诚实留空）
//   0052 (Go)   provision_runs 建表（OPS-001 云资源供给引擎运行记录：
//               provider/region/规格/状态/输出/agent id 快照；新表无存量
//               约束名漂移，HasTable 检查后 CreateTable，0027/0038 模式；
//               敏感字段（bootstrap token）不入库——只落非敏感快照）
//   0053 (Go)   provision_runs.destroy_protected 列（OPS-002 数据盘护栏：
//               true 时拒绝自动销毁；加列 0025 模式；缺表跳过（fanout
//               重放到无该特性的库不报错、不建空壳）；存量行默认 false）
//   0054 (Go)   provision_runs.destroy_approval_id 列（OPS-003 审批双模式
//               workflow：销毁申请单的审批记录 id；加列 0025 模式；缺表
//               跳过；存量行空=未申请销毁审批，行为与现状一致）
//   0059 (Go)   provision_runs.cloud_init_extra 列（ed541f6e 审计 #657：
//               ticket 续行丢失非敏感 cloud-init 自定义片段；加列 0025 模式；
//               缺表跳过；存量行空=未提供，行为与现状一致）
//   0060 (Go)   backups.game_id/env/source_id/source_name 列（OPS-008 数据面
//               备份恢复：游戏库备份归属经 capture 所见数据源登记；加列 0025
//               模式 + idx_backup_scope 复合索引；缺表跳过；存量行三列空=
//               平台级备份，行为与现状一致）
//   0062 (Go)   runbooks + runbook_runs 表（OPS-009 运维 runbook：参数化步骤
//               序列引用 OPS-004 部署/OPS-006 批处理能力；新表 CreateTable，
//               无存量约束名漂移问题）
//   0063 (Go)   agent_metrics_hourly.agent_id 统一 VARCHAR(64)（审核发现：修改
//               已存在的 0061 迁移不会在已执行过 0061 的库上重跑，存量库
//               agent_id 仍为无尺寸列与新建库 schema 分叉；新增版本迁移对
//               存量表 AlterColumn 回填，幂等；缺表跳过）

func init() {
	registerSvcMigrations()
}

// registerSvcMigrations 注册本包的 goose 全局迁移。抽为具名函数是为可测性：
// init 在包加载时仅执行一次，测试经 goose.ResetGlobalMigrations 清空后
// 重新调用，可覆盖重复注册的 panic 分支（见 migrations_register_test.go）。
// panic 是 Go init 对注册失败的唯一表达方式，保留 fail-fast。
func registerSvcMigrations() {
	if err := goose.SetGlobalMigrations(
		openapiBackfillMigration(),
		legacyCleanupMigration(),
		enumColumnsMigration(),
		supportContextMigration(),
		bugTrackerMigration(),
		toolRegistryMigration(),
		releaseMigration(),
		configNamespaceMigration(),
		ticketCSATMigration(),
		hotpatchMigration(),
		dbSourceMigration(),
		platformSettingsMigration(),
		taskSchedulesMigration(),
		agentSessionAddrMigration(),
		contractTimeoutMigration(),
		announcementTablesMigration(),
		adminLoginSecurityMigration(),
		adminMfaMigration(),
		executionLogsTableMigration(),
		contractPrevSchemaMigration(),
		termDictionaryDisplayMigration(),
		componentTemplateColumnsMigration(),
		softDeleteResidueCleanupMigration(),
		contractExecutionStateMigration(),
		roleAdminSoftDeleteCleanupMigration(),
		menuItemTablesMigration(),
		sdkVersionHighwatermarkMigration(),
		contractVersionTableMigration(),
		functionVersionFloorTableMigration(),
		contractRemovalPendingColumnMigration(),
		adminOtpRecoveryCodesMigration(),
		adminPasswordPolicyMigration(),
		providerMetadataTableMigration(),
		bugTicketLinkMigration(),
		announcementGamesTableMigration(),
		emailVerificationMigration(),
		cicdTablesMigration(),
		gameSoftDeleteResidueCleanupMigration(),
		incidentTablesMigration(),
		reportDistributionMigration(),
		alertHandlingMigration(),
		alertScopeMigration(),
		agentMetricsProbeJSONMigration(),
		dbSourceOwnerMigration(),
		probeEventsTableMigration(),
		probeEventTargetCategoryMigration(),
		pageExecuteIdempotencyTableMigration(),
		auditChainVersionMigration(),
		approvalDispatchClaimColumnsMigration(),
		agentMetricsHistoryEnrichmentMigration(),
		provisionRunsTableMigration(),
		provisionRunDestroyProtectionMigration(),
		provisionRunDestroyApprovalMigration(),
		provisionRunCloudInitExtraMigration(),
		deployTablesMigration(),
		agentLogTablesMigration(),
		batchOpTablesMigration(),
		costBudgetTablesMigration(),
		backupScopeColumnsMigration(),
		agentMetricsHourlyUniqueIndexMigration(),
		runbookTablesMigration(),
		agentMetricsHourlyAgentIDSizeMigration(),
	); err != nil {
		panic(fmt.Sprintf("svc: register goose go migrations: %v", err))
	}
}
