# TODO

需求清单：一条一句，只列需求。完成项保留一行证据（commit hash / 测试规模），过程叙述见 git log 与 docs/ 对应设计文档。

## 未完成

- [x] 4862 ops 告警列表按 game/env 过滤：Alert 加 game_id/env 列（迁移 0043）+ opsAlerts 按 GameScopeFromContext 过滤（scope 告警 + 平台级告警）+ 打戳三点（evaluator 继承 rule scope / inbound generic payload scope / control_handler capture req scope）+ 面板告警页全局视野确认（/api/v1/alerts 无 scope 不过滤）（4862 迁移 0043 + 三处同步 + 测试，2026-10-13）
- [ ] 4310 MongoDB Connector：CDC 内置源收口项，排期待用户立项；SQL Server/Oracle/TiDB/CockroachDB/国产库明确不做，更多数据源由使用方按 SourceConnector 契约自行实现
- [ ] capture 链部署环境验收：4301 六项在部署机复验，补浏览器截图取证（2026-10-13 卡点：站点可达（:8000 dashboard / :18780 server 均 200），但登录缺凭据——部署库 admins 仅 admin/admin2 密码未知（哈希不可逆），/auth/register 被部署版 auth 中间件拦截（与仓库路由不同，仓库版 register 在公开组），Playwright+Chromium 已就绪、SSH 隧道已验证可通，只差可用账号即可逐项复验+截图入 docs/证据）
- [ ] Agent 面板三页（agent-dashboard-design 简档）：Proposed 待用户过目批准，零代码；sidecar/devops/capture 每 type 一页+探针卡+职责数据+日志下钻，scope 联动
- [ ] Provider 插件 P1（#66 批二）：openapi driver 迁移 internal/platform/openapi → internal/drivers/openapi + externalplatform 迁入；按 §9 分期另行立项
- [ ] 插件机制正式设计落地（plugin-mechanism 简档）：Proposed 待拍板后按 §8 分期；含 LAN 资产接入（cockpit 优先）、配置热更双轨等 12 项决策

## 已完成

### 平台与面板

- [x] 上传即成页契约与绑定正交化 T1-T13（T1 65f88486b 至 T13 545477a5e，2026-09-18 全部完成）
- [x] 菜单管理系统 T-M1 至 T-M10（menu_items 表、CRUD、权限过滤、默认菜单种子，2026-09-18）
- [x] 风控面板：/risk 分类、capture gates 表单/JSON 双模式编辑、策略版本历史与行级回滚、风险事件人工处理四列（迁移 0042）、告警中心迁移 /risk/alerts（2026-10-13）
- [x] 密码箱抽象与可替换提供商：vault.Provider 接口、local/env 双实现、vault:// 引用接入 settings/DSN/JWT 三 seam（internal/platform/vault，2026-10-13）
- [x] 服务器状态源：默认 atlas、通用 http 源可切换（切换六用例 + http 源 12 用例，2026-10-12）
- [x] 设置页外部服务只读清单 S3-web（92b6cfc，2026-10-11）
- [x] 三 agent 体系：sidecar 定名、devops 堡垒机代理、capture CDC；使用场景文档、一键安装脚本、逐字段配置文档（2026-10-12）
- [x] 插件机制 M2-M4：Outlet 告警出口、webhook 告警入口、外部服务接入模板（2026-10-10）
- [x] 系统公告 Markdown 渲染补齐（#50）与运维七页全量核对（#51 至 #56，2026-10-08）
- [x] Provider 插件设计立项 #66、Agent Supervisor 立项 #67（设计简档加 S1-S3 交付，2026-10-10）

### CDC 捕获链

- [x] 统一契约：ChangeEvent 模型、FilterPolicy 策略头、SourceConnector 六方法接口、能力闭集（agents/capture/connector，2026-10-11）
- [x] 三闸与告警链：reconcile/cep/lineage 加 CaptureRuleHitReport 上报（fb189bb、db31e20、4f9584e、01646d4、b9ddc43，2026-10-11）
- [x] 中心策略管理：config_versions 资产化乐观锁、发布依赖与能力感知 lint、appliedRevision 上报、回滚端点、位点-策略联动四拍板（2026-10-13）
- [x] 过滤策略落地：event filter 热换、失败语义与统计计数入 Stats（runner_stats_test.go 三用例，2026-10-13）
- [x] MySQL/MariaDB Connector：flavor 双方言、GTID 变量分叉、CI capture-matrix（2026-10-13）
- [x] PostgreSQL P0 闭环：pgoutput 解码层、流式 Connector、runner 多源装配、活栈全链端到端、六阶段集成测试（2026-10-13）
- [x] 4301 验证清单六项本地活栈跑通：规则热生效、断线恢复、位点恢复、新表启用、过滤错误可观测、跨 game/env 隔离（22ffb40 修复 Poll 启动失败 walsender 泄漏，2026-10-13）

### 质量工程

- [x] Go 覆盖率巡检 56 轮：internal 多包收口至 100%（2026-09-27 至 10-08）
- [x] web 覆盖率巡检 40 余轮：全站零测试页归零，全量 jest 360+ suites 绿（2026-09-28 至 10-01）
- [x] Dependabot/audit advisory 收口：axios 12 条、dompurify（2026-09-30）
- [x] PageSpec/渲染守卫复验七批（dashboard_vnext_guard，2026-09-30）
