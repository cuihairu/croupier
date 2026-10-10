---
title: 事故报表体系——登记、归因、报表与 leader 分发（设计简档，已批开工）
---

# 事故报表体系（责任人报告 × 排行榜 × 趋势 × 统计）

## 状态

- 状态: **已批开工（2026-10-10 设计令落档；同日多批补充令并入，见各节标注）**——文档先落盘，落盘即按 §10 分批开工，每批测试绿才推送。
- 需求令四件：①责任人报告（周/月，事故归属到人：agent/操作员/变更引入，来源=execlog 审计 + herald 事件 + incident 登记）②排行榜（周/月/季/年四档，事故数/影响时长/修复速度）③事故趋势（时间序列，分类含 bug 与运维故障：编译红/服务不可用/探活失败/部署失败）④事故统计（总量/分类占比/MTTR/复发率）。
- 补充令：职能域类别（六类默认 + 设置可配存库 + 子类 + 删除归并未分类）→ §2.1/§3；每类配 leader → §2.1/§6；报告按类聚合分发 → §6；对比维度（环比/月度对比/同比三列 + 去年同期虚线 + 无数据不算假数）→ §5.4；对外开放 API（定案 REST + Webhook 两条，SDK/OpenAPI 客户端生成不做，OpenAPI 仅文档）→ §9；出口 Provider 链（站内默认必有 → 外部可选降级，默认 no-op 第一原则）与通知分级强化（level/type/source/ref/scope + 紧急度排序 + 升级去重）与通知可见范围（audience 定向 + 管理员全量 + 兜底组）→ §2.5/§6，机制面见[插件机制](plugin-mechanism.md) §5.1；配套[服务器状态 Provider](server-status-provider.md)（维护窗口 gate）独立简档。
- 数据源现状探查基线：alerts（`internal/model/alert.go`，meta DB，无 game 归库列）、execution_logs（`internal/model/execution_log.go:14`，Actor/FunctionID/Status/DurationMs，per-game DB）、bugs（`internal/model/bug.go:18`，含 Source/Assignee/CrashFingerprint）、cicd_builds（0038 迁移建表）、supervisor 事件（内存环，经 MetricsStore hook 出 herald）、probe 故障窗口（availability 品类）。

## 1. 定位与职责边界

| 职责                                | 归属       | 说明                                                                                      |
| ----------------------------------- | ---------- | ----------------------------------------------------------------------------------------- |
| 事故登记与归因（Record）            | croupier   | incidents 表是真值存储；人工登记 / 外部 API / 自动源一键转换三入口                        |
| 类别体系管理（职能域 + leader）     | croupier   | incident_categories 配置表，设置页可增删改，API 校验=配置表合法值                         |
| 报表聚合（周/月/季/年 + 环比同比）  | croupier   | 报告生成器 server-local 执行，payload 存 incident_reports                                 |
| 投递分发（leader 分片/渠道/收件人） | **herald** | courier 式：croupier 按类聚合后交 herald 出口，接收地址/频道全在 herald 侧（M2 既有出口） |
| 缺陷工单（bug）本体                 | croupier   | bugs 表既有体系不动（#25 工单关联、崩溃聚合照旧）；报表侧只加类别列并联查                 |
| 非目标                              | —          | 不做跨系统告警收敛/降噪（沿 alerts.md 边界）；v1 不做全自动事故归并（见 §4.3 与边界清单） |

## 2. 数据模型

### 2.1 incident_categories（类别配置表）

```go
type IncidentCategory struct {
    gorm.Model
    Name          string `gorm:"size:64;not null;uniqueIndex:uidx_incident_categories_name"` // 前端/客户端/美术/策划/测试/运维/未分类
    Slug          string `gorm:"size:64;not null;uniqueIndex:uidx_incident_categories_slug"` // frontend/client/art/design/qa/ops/uncategorized
    Sort          int    // 排序（面板/报表展示顺序）
    Leader        string `gorm:"size:64;index"` // 责任 leader（账号或人名，可空=该类无 leader，分发跳过）
    Subcategories JSON   `gorm:"type:json"`     // 子类白名单（字符串数组，可空=不限子类）
    Audience      JSON   `gorm:"type:json"`     // 可见范围映射 {roles:[], users:[]}：该类通知/报表对哪些部门角色+人可见（设置页可改）
    Enabled       bool   // 停用后：登记不可选、报表历史保留、动态出图不再出该线
    Builtin       bool   // 未分类行=true（不可删、slug 不可改）；播种六类亦标 builtin（名称/leader/子类可改）
}
```

- **首启播种**（迁移内幂等：缺行才插）：`未分类(uncategorized, builtin 不可删)` + 六类 `前端(frontend)/客户端(client)/美术(art)/策划(design)/测试(qa)/运维(ops)`，子类初始白名单示例：客户端 `[崩溃, 性能, 网络]`、运维 `[部署, 配置, 探活, 编译]`——播种值是样例，后续全在设置页改。
- incident 按 **category_id 关联**（改名不断链）；API 层类别校验 = `category_id 必须是本表 Enabled 行`（枚举来自配置表，非硬编码）。
- **删除语义（两段）**：删除请求先返回该类别下 incident/bug 计数（级联提示）；确认删除 = 事务内 `UPDATE incidents/bugs SET category_id=未分类` 后物理删行。推荐路径是 `Enabled=false` 停用（历史报表不受影响）；物理删是给"建错了"的场景。

### 2.2 incidents（事故登记表）

```go
type Incident struct {
    gorm.Model
    IncidentKey string `gorm:"size:128;uniqueIndex:uidx_incidents_key"` // 幂等键：自动源=派生指纹；外部 API=Idempotency-Key/eventId；人工=可空（自增主键兜底）
    Title       string `gorm:"size:255;not null"`
    CategoryID  uint   `gorm:"index"` // → incident_categories；删除类别时归并未分类
    Subcategory string `gorm:"size:64"` // 须 ∈ 类别的 Subcategories 白名单（该类配置了白名单时强校验）
    Severity    string `gorm:"size:16;index"` // info|warning|critical
    Status      string `gorm:"size:16;index"` // open|acknowledged|resolved
    Source      string `gorm:"size:32;index"` // manual|external|alert|cicd|probe|supervisor|bug
    // 责任归因
    ResponsibleType string `gorm:"size:16;index"` // agent|operator|change|unknown
    ResponsibleID   string `gorm:"size:255;index"` // agentId / 账号（operator=操作人；change=execlog.Actor 即变更引入人）
    // 时间线（时长口径基准）
    DetectedAt time.Time  `gorm:"index"` // 发生/发现时间（允许补登过去时间，默认=创建时刻）
    ResolvedAt *time.Time // resolved 时写入；影响时长与 MTTR 口径见 §5.2
    // 关联
    GameID    string `gorm:"size:64;index"` // 可空（平台级事故）
    Env       string `gorm:"size:64;index"`
    ExecLogIDs JSON  `gorm:"type:json"` // 关联 execution_logs.ID 列表（变更引入链路证据）
    RefType    string `gorm:"size:32"`  // 关联源对象类型：alert|cicd_build|bug|probe_window|supervisor_event
    RefID      string `gorm:"size:128;index"` // 关联源对象主键（alert_id / build id / …）
    Details    JSONMap `gorm:"type:json"` // 自由明细（受影响玩家数、根因、复盘链接…）
    CreatedBy  string `gorm:"size:64"`  // manual=面板操作者；external=调用方 token 名（§9.3）
}
```

- **meta DB 存储**（与 alerts 同款：跨 game 聚合报表是主用例，game_id 仅作过滤列，不做 per-game 分库）。
- 状态机：`open → acknowledged → resolved`（acknowledged 可跳过）；resolved 允许重开（重开清 ResolvedAt，复发统计不受重开影响——复发口径看 §5.3）。
- **kind 说明**：bug 与 incident 是两张表（bugs 是完整工单体系，不重复建行）；报表层联查两表统一聚合（§5）。incidents.Source=bug 的形态仅用于"从 bug 手动转出独立事故"的边角场景，v1 不提供该转换。

### 2.3 bugs 对齐（加列，不动既有体系）

- 0040 迁移给 bugs 表加列：`category_id (uint, index)` + `subcategory (varchar 64)`（`Migrator().HasColumn` + `AddColumn` 模式，0015/0016/0025/0027 先例；**禁止整模型 AutoMigrate**）。
- bug 登记页类别必选（与 incident 同一类别选择器）；存量 bug category_id=0 → 报表层归入「未分类」线。
- 报表口径：bug 的 detected_at=created_at、resolved_at=状态进入 resolved/closed 时刻（沿用 bugs 既有状态闭集）。

### 2.4 incident_reports 与外部调用方（0041）

```go
type IncidentReport struct {
    gorm.Model
    PeriodType  string `gorm:"size:16;uniqueIndex:uidx_incident_reports_period"` // week|month|quarter|year
    PeriodStart string `gorm:"size:16;uniqueIndex:uidx_incident_reports_period"` // 2026-W41 | 2026-10 | 2026-Q4 | 2026
    ReportKind  string `gorm:"size:16"`  // responsibility|leaderboard|trend|summary
    Payload     JSON   `gorm:"type:json"` // 完整报表 payload（§5 指标+对比+矩阵）
    GeneratedAt time.Time
    PushStatus  JSON   `gorm:"type:json"` // 按类别的分发结果 [{slug,leader,pushedAt,eventId,ok}]
}

type ExternalToken struct {
    gorm.Model
    Name       string `gorm:"size:64;not null;uniqueIndex"` // 调用方标识（审计/限流维度）
    TokenHash  string `gorm:"size:64;not null;uniqueIndex"` // sha256(token) 十六进制，明文只在创建响应返回一次
    Scope      JSON   `gorm:"type:json"`                    // 可见面 {categories:[slug...]}；null=全量：GET 过滤 + webhook 事件过滤
    Enabled    bool
    LastUsedAt *time.Time
    CreatedBy  string `gorm:"size:64"`
}
```

### 2.5 迁移与三处同步

- **0040**：建 `incident_categories`（含 audience 列）+ `incidents`（HasTable 检查后 CreateTable，0019 先例；新表无存量约束名漂移）+ bugs 加两列（HasColumn+AddColumn）+ 播种七行类别（幂等：按 slug 缺行才插）。
- **0041**：建 `incident_reports` + `external_tokens` + **messages 加列**（level/source/ref_type/ref_id/scope；type 复用既有列；level 历史回填 info——`Migrator().HasColumn`+`AddColumn`+回填 UPDATE，随 §6 报告分发与 §9 对外 API 批次落地）。

**站内通知字段强化（0041 同批）**：按插件机制 §5.1 通知强化令——`level`（info|warn|critical，历史回填 info）/ `source`（agent/状态源）/ `ref_type`+`ref_id`（关联 incident/服务器）/ `scope`（可见范围必备字段）；紧急度排序、视觉分级、重复触发升级去重（重复 warn→critical）与**通知可见范围规则**（按 incident_categories.audience 定向 + leader + 管理员全量 + 兜底接收组不落空）均见插件机制 §5.1。

- 三处同步：`internal/svc/migrations.go`（函数 + `registerSvcMigrations` 注册 + 文件头版本清单注释）、`internal/db/migrate/migrate.go`（MinimumRequiredVersion bump）、`internal/db/migrate/migrate_test.go`（合成清单 probe 条目 + 版本断言）。回归用例复刻存量形态（bug 加列用手工 CREATE TABLE 老式约束名先例 `TestComponentTemplateColumnsMigration_LegacyTableShape`）。

## 3. 类别管理（设置可配，存库不硬编码）

- **设置页**（面板「运维中心 → 事故类别」）：名称/slug/排序/leader/子类白名单/启停 全可配；slug 建行后不可改（外部 API 与报表 payload 按 slug 稳定引用），名称/leader/子类随意改。
- **校验规则**：incident/bug 登记与外部 API 提交时，category_id 必须命中 `Enabled` 行；subcategory 在该类配置了白名单时必须命中白名单，未配置白名单时 ≤64 字符即可。
- **报表动态出图**：趋势线/饼图/排行榜类目从 incident_categories 动态读取（Enabled 行 + 未分类兜底线），新增类别即出现新线，停用类别从当期图消失但历史报表 payload 不回改。
- **leader 字段**：账号或人名字符串（v1 不做账号外键——herald 侧受众映射按该字符串路由）；每类至多一个 leader，多 leader 场景在 herald 侧组受众解决（courier 式边界沿用）。

## 4. 事故登记与归因

### 4.1 登记入口（三入口，类别必选）

| 入口          | 路径                                | 类别                           | 身份留痕                           |
| ------------- | ----------------------------------- | ------------------------------ | ---------------------------------- |
| 面板登记      | `/ops/incidents` 登记表单           | 必选（Enabled 类别枚举选择器） | CreatedBy=JWT 操作者               |
| 外部 REST API | `POST /api/v1/incidents`（§9）      | 必选（slug 或 id）             | CreatedBy=token 名；执行留痕 audit |
| 自动源转换    | 告警/构建/探活/熔断页「转事故」按钮 | 必选（预填建议值可改）         | CreatedBy=操作者；Ref* 自动带上    |

### 4.2 归因字段语义（责任人 = 谁）

| ResponsibleType | 语义                  | ResponsibleID 取值               | 数据来源                                    |
| --------------- | --------------------- | -------------------------------- | ------------------------------------------- |
| `agent`         | 某台 agent/游戏服引入 | agentId                          | 转换时从源事件带出（supervisor/probe 事件） |
| `operator`      | 某操作员的执行引发    | 账号                             | 关联 execlog 时取 `execution_logs.Actor`    |
| `change`        | 某次变更引入          | 变更操作人账号（=execlog.Actor） | ExecLogIDs 关联链：注册/发布/审批执行留痕   |
| `unknown`       | 未归因                | 空                               | 登记缺省；报表有「未归因占比」提醒归因补全  |

- 归因可后补（登记时 unknown → 复盘时改 operator/change 并挂 ExecLogIDs）；责任人报告按报告期结束时刻的归因快照统计。

### 4.3 自动源与转换映射（v1 一键转换，不全自动）

| 自动源                        | 触发面                    | 转换建议值（可改）                                   | Ref* 关联                |
| ----------------------------- | ------------------------- | ---------------------------------------------------- | ------------------------ |
| 告警（capture/dbmon/入站）    | alerts 列表行「转事故」   | severity=alert level；detected_at=告警 fired 时刻    | RefType=alert+RefID      |
| cicd 构建失败（编译红）       | 构建列表失败行「转事故」  | category=运维/编译；ResponsibleType=change（触发人） | RefType=cicd_build       |
| probe 故障窗口（探活失败）    | 探活页故障窗口「转事故」  | category=运维/探活；ResponsibleType=agent            | RefType=probe_window     |
| supervisor 熔断（服务不可用） | 监管事件/节点页「转事故」 | category=运维；ResponsibleType=agent                 | RefType=supervisor_event |

- 诚实边界：v1 **不做自动归并**（阈值告警噪音大，全自动会把告警洪水灌进事故报表）；一键转换保证登记成本足够低。全自动规则（如 critical 级 probe 窗口自动立事故）留 P2，挂 plugin-mechanism §3.3 检测器规则位。
- **数据源现状锚点**（2026-10-10 探查，落点影响实现）：
  - cicd `TriggeredBy` 是 `api|webhook` 不是人——变更归因取**触发链路 actor**（REST 触发走 execlog invoke 记 `Actor`）或 integration `CreatedBy` 兜底；
  - probe 故障窗口（`core/healthprobe` Window 有 StartTS/EndTS/DurationMS，MTTR 口径完美）与 supervisor 事件（事件闭集完整）**server 侧均不落库**（agent 内存环/本地文件）——转换时必须把时间线快照进 incident 行（DetectedAt/ResolvedAt/RefID），不能事后反查；
  - `alerts` 表无 game/env/agent 列（scope 在 Details JSON）——incidents 表带 GameID/Env 列补此缺口；herald 出口事件不落本地 alerts 表，故 supervisor/probe 源的事故以转换为唯一入口；
  - audit_records 永久保留（唯一长周期流）但「部署/变更」类写点不全（函数注册/pack 导入操作人被丢弃，`internal/api/extension/service.go:596` 显式 `_ = operator`）——归因主锚点是 execlog.Actor（恒非空）；
  - **execlog 默认保留期 7 天**（`internal/config/config.go:736-742`）——归因在登记/转换时**快照**（ResponsibleID 落库），ExecLogIDs 仅作溯源引用，过期后报表口径不受影响。

## 5. 报表口径（全部指标可复算）

### 5.1 周期定义

| 档位 | 周期                         | 标识       | 环比对象      | 同比对象        |
| ---- | ---------------------------- | ---------- | ------------- | --------------- |
| 周   | ISO 周（周一 00:00 起 7 天） | `2026-W41` | 上一个 ISO 周 | 去年同 ISO 周号 |
| 月   | 自然月                       | `2026-10`  | 上个自然月    | 去年同月        |
| 季   | 自然季                       | `2026-Q4`  | 上个自然季    | 去年同季        |
| 年   | 自然年                       | `2026`     | 去年          | （无同比列）    |

- 时区口径：**server 进程本地时区**（部署即定，文档随部署说明）；库内时间戳一律 UTC，周期边界计算在本地时区做。
- 月度对比 = 月档的环比（本月 vs 上月），不做独立第五档。

### 5.2 指标口径

| 指标       | 算法                                                                                                  | 备注                                                     |
| ---------- | ----------------------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| 事故数 N   | `detected_at ∈ [期初, 期末)` 的 incident 数 + bug 数（两表联查）                                      | 含未解决；归因/类别按当期快照                            |
| 影响时长 D | `Σ(resolved_at − detected_at)`；期末仍 open 的计到 `min(now, 期末)`                                   | 未解决事故拖尾口径，报告 payload 记录截断时刻            |
| MTTR       | 已解决样本的 `Σ(resolved − detected) / 已解决数`                                                      | 无已解决样本 → MTTR=null（显示「暂无修复样本」，不算 0） |
| 修复速度榜 | MTTR 升序；仅纳入已解决样本 ≥1 的责任人/类别                                                          | 同上，空样本不进榜                                       |
| 复发率     | 复发数 / 已解决数。复发 = 同 `签名` 在 resolved 后 **7 天内**（固定口径 v1 不配置）再次出现同签名事故 | 签名见 §5.3                                              |
| 责任人次数 | 按 ResponsibleID 分组计数（unknown 单列一行）                                                         | 责任人报告主列                                           |

### 5.3 复发签名

- 自动源：`source + RefID 派生`（alert=规则名+目标；cicd=workflow+job；probe=探活目标；supervisor=agent+进程名）。
- 人工登记：`category + subcategory`（无子类用 category）。
- 同签名 7 天窗口内 resolve→再发 = 复发一次；连续多次按链首记 1 次复发链（链长可查）。

### 5.4 对比维度（环比 / 月度对比 / 同比三列）

- 每个指标（事故数/责任人次数/各类别占比/MTTR/影响时长/复发率）输出结构：

```json
{
  "value": 12,
  "prev": { "value": 8, "delta": 4, "pct": 50.0 },
  "yoy": { "value": null, "delta": null, "pct": null, "missing": true }
}
```

- 环比 = 上一同档周期；同比 = 去年同档周期（去年同 ISO 周号/同月/同季）。
- **数据不足规则**：同比周期无数据（上线不满一年）→ `missing: true`，前端显示「无去年同期数据」灰字占位；**禁止置 0、禁止算假百分比**（0 基数同理：上期为 0 时 pct=null 只给 ±delta）。
- 趋势图：当前周期实线 + **去年同期虚线叠加**（yoy 序列缺数据则整线不画，图例注明「无去年同期数据」）。

### 5.5 排行榜（四档 × 三榜 × 两视图）

- 档位：周/月/季/年（§5.1 对象）。
- 榜单：①事故数榜（降序）②影响时长榜（降序）③修复速度榜（MTTR 升序，空样本不进榜）。
- 视图：按责任人 / 按类别（职能域）两个 tab——哪个职能引入多少事故一眼可见（补充令②）。
- 每行带三列：本期值、环比 ±值与 %、同比 ±值与 %（missing 规则同 §5.4）。

### 5.6 责任人报告（周/月）

- 结构：报告头（周期/生成时间/总览三列指标）→ 类别×责任人矩阵（哪个职能多少事故、每类 leader 是谁）→ 责任人明细行（事故数按类别分布/影响时长/MTTR/复发数 + 环比同比）→ 未归因清单（提醒补全）。
- 生成时机：周报每周一 09:00（本地时区）生成上周；月报每月 1 日 09:00 生成上月；面板可随时按周期**按需重算**（重算不覆盖已分发记录，生成新 payload 版本）。

## 6. 报告生成 → 按类聚合 → leader 分发链路

```
TaskSchedule（task_schedules 表，五字段 cron：
  周报 `0 9 * * 1` / 月报 `0 9 1 * *`，run-log 触发槽幂等窗口防重）
  → 触发报告生成器（server-local 执行，不经 agent 派发链）
  → 聚合：incidents + bugs（§5 口径）→ payload（指标 + 环比/同比 + 类别矩阵）
  → 落 incident_reports（Payload + GeneratedAt）
  → 分发（出口链 = 站内默认必有 → 外部 Provider 链可选降级，插件机制 §5.1）：
     ① 站内通知（内建默认出口，默认必有，零外发）：
        · 管理员收【总聚合（全量事故摘要）】——不是人人一份
        · 各类别 leader 收【本类别分片】；该类别 audience（部门角色+人）按 scope 可见
        · 通知带 level/type/source/ref/scope 全套字段（§2.5）
     ② 外部出口链（可选 Provider，配置显式启用；主出口失败滑下一个，链尾失败计数不阻塞）：
        · kind=incident-report / severity 按分级（见下）
        · target=category-leader:<slug>（herald 侧映射到该 leader 的接收地址/频道/IM；
          未配 leader 的类别分片不外投，PushStatus 记 skipped(no leader)，面板仍可见）
        · event_id=report:<PeriodType>:<PeriodStart>:<slug>（确定性幂等，出口侧 dedup 兜底）
  → 每片分发结果写 incident_reports.PushStatus（slug/leader/channel/eventId/ok/错误）
```

- **推送分级**（站内通知与外部出口同规则）：常规报告 `info`；某类别事故数环比翻倍或复发率超阈值 → `warn`；连续两期恶化或含 critical 未解决事故 → `critical`——阈值 v1 固定口径（环比×2 / 复发率>20% / 连续两期），不配置化。
- **可见范围**：通知带 scope（类别上下文），可见性=read 时按 incident_categories.audience（`{roles, users}`，设置页可改）解析——该类受众+该类 leader 可见，管理员全量；**无归属类别的通知归「运维」或配置的兜底接收组，不落空**。
- 已读跟踪边界：站内信已读在平台内闭环（messages.ReadAt/MarkRead，既有机制）；herald 投递状态 v1 不回写 croupier（herald 简档 §6 边界沿用），已读与否看 herald 侧。
- 手动重推：报表详情页「重推」按钮按同 event_id 重发（幂等折叠），不改 payload。

## 7. 趋势与统计面板（数据面）

- 趋势：按天/周/月分桶（**Go 侧分桶扫时间戳**，`internal/api/analytics/invocations.go:152-194` 先例——不用 DATE_FORMAT/strftime，postgres/sqlite 方言安全）；系列=各类别一条线（可切换堆叠柱/折线）+ 总量线 + 去年同期虚线。
- 统计卡：总量/分类占比（饼）/MTTR/复发率，每卡带环比±与同比±（missing 占位规则同 §5.4）。
- 周期粒度：趋势图桶粒度 day/week/month 可选；榜单与报告档位 week/month/quarter/year。

## 8. 面板页设计（静态页路径，agent 类别规矩）

三页全部走**静态页**（仿 `web/src/pages/Analytics/Invocations/index.tsx` 整页模式；不走 PageSpec 动态页——图表类型与布局需自定义，发布链闭环仅在动态页路径强制，见 dashboard-page-model.md）：

| 页面         | 路径                       | 结构                                                                                                                                                | access       |
| ------------ | -------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| 事故登记列表 | `/ops/incidents`           | 统计卡行 + 筛选（类别/责任人/状态/来源/时间 RangePicker）+ ProTable 明细 + 登记表单 + 详情抽屉（SupervisorDrawer 先例：列表→抽屉→Tabs）             | canOpsRead   |
| 事故报表     | `/ops/incident-reports`    | 档位 Tabs（周/月/季/年）→ 统计卡 Row（三列对比）→ 趋势图（Line/堆叠 Column，去年同期虚线）→ 排行榜 Table（责任人/类别两视图）→ 周期责任人报告 Table | canOpsRead   |
| 事故类别设置 | `/ops/incident-categories` | 类别 CRUD 表格（名称/slug 只读/排序/leader/子类白名单编辑/启停）+ 删除级联提示弹窗                                                                  | canOpsManage |

- 后端路由：scoped 组 + `FlagOps` 双门（routes.go :210-212 模式：L2 物理不注册 + L3 softFlags.guard 403）；类别设置写接口要求 `ops:manage`。
- 图表：`@ant-design/charts`（唯一图表库，guard 强制）；统计卡 antd Statistic + 环比/同比箭头；列配置多时拆 `columns.tsx`（Nodes/columns.tsx 先例）。
- API 层：`web/src/services/api/incident.ts` 新建，scope 经 `getScope()`，X-Game-ID/X-Env 由请求拦截器自动注入。
- i18n：`web/src/locales/{zh-CN,en-US}/incidentReports.ts` per-page 模块，聚合文件 import+spread；id 惯例 `pages.incidentReports.*`。
- 既有页面接入：告警页/构建列表/探活页各加「转事故」按钮（§4.3），最小侵入。

## 9. 对外开放 API（定案：REST + Webhook 两条）

> 定案（用户，2026-10-10）：只做 Webhook + REST API；SDK / openapi-generator 客户端生成**不做**；OpenAPI 规范仅作接口说明文档保留。

### 9.1 REST 契约

| 端点                     | 说明                                                                                                                                                                                                                                                           |
| ------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST /api/v1/incidents` | 外部登记事故。头 `Authorization: Bearer <token>` + `Idempotency-Key`（或 body.eventId）；字段与事故模型对齐：title/category（slug 或 id，必选）/subcategory/severity/detectedAt/responsible{type,id}/execLogIds/ref/details/source 标记 `external:<tokenName>` |
| `GET /api/v1/incidents`  | 外部查询（分页 + 类别/状态/时间过滤；**按 token scope 限可见面**：token 未配 scope=全量只读，配了 `{categories:[...]}` 则结果与该 token 的 webhook 事件都限于类别集）                                                                                          |
| `POST /api/v1/ext/bugs`  | 外部建 bug：**内部复用既有 bug Service**（`internal/api/bug`），独立对外路由做 token 鉴权 + 字段白名单（不暴露内部全部字段）；bug 补 category_id 后契约与事故模型对齐                                                                                          |

- 响应契约沿 CLAUDE.md API 响应规范（成功裸 payload / 错误 `{error,message,details}`）；幂等：同 Idempotency-Key 重放返回首建结果（`incident_key` 唯一索引兜底）。
- curl 可通是验收标准之一（§10 批 5）。

### 9.2 鉴权 / 限流 / 审计

- **token**：`external_tokens` 表（§2.4，sha256 哈希存库、明文只回一次、可启停可吊销）；中间件校验 + `LastUsedAt` 更新；**scope 限可见面**：REST 查询结果与 webhook 事件推送都按 token 的类别集收窄（与站内通知 scope 同一可见范围哲学）。
- **限流**：`internal/platform/ratelimit` tokenbucket，按 token 名分桶（默认 30 req/min，可配）。
- **审计**：写 `execution_logs`（Source=`external`，Actor=`ext:<tokenName>`，FunctionID=操作名，RequestPayload/ResponseBody 落载荷）+ incidents.CreatedBy 双留痕；敏感字段脱敏沿 API 既有 masking。

### 9.3 反向 Webhook（复用 herald 出口）

- 事件：`incident.created` / `incident.escalated`（severity 升级或 acknowledged 超时）/ `incident.resolved`。
- 信封走 M2 事件信封（plugin-mechanism §5.2），kind=`incident-lifecycle`，event_id=`incident:<id>:<event>` 确定性幂等；外部系统的接收地址/频道在 herald 侧配置（courier 式，croupier 不配收件人）。
- 出站失败只记日志 + 失败计数（Outlet Manager 既有语义），不阻塞事故主流程。

### 9.4 OpenAPI 规范（文档性质）

- `docs/openapi/incidents.yaml`（OpenAPI 3.0）：上述 REST 契约的机器可读说明，供外部自助阅读；**不接代码生成管线**。

## 10. 分批交付计划（每批测试绿 → commit → push）

| 批   | 内容                                                                                                                                                                                     | 迁移 | 状态       |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---- | ---------- |
| 批 1 | 模型（categories/incidents）+ 0040 迁移 + bugs 加列 + 类别 CRUD API + 事故登记/列表/状态流转 API + Go 测试                                                                               | 0040 | ✅ 2a092ba |
| 批 2 | 报表聚合 API：summary / trend / leaderboard / responsibility-report（按需重算，含环比同比三列与 missing 语义）+ Go 测试                                                                  | —    | ✅ 11be163 |
| 批 3 | 面板三页 + 「转事故」按钮接入 + i18n 双语 + jest 用例 + tsc + guard                                                                                                                      | —    | ✅ 见下    |
| 批 4 | 调度生成（task_schedules cron）+ incident_reports/external_tokens/messages 加列 0041 + 站内通知分发（管理员总聚合/leader 分片/scope 可见性/兜底组）+ 外部出口链 leader 分片 + PushStatus | 0041 | ⏳ 待做    |
| 批 5 | 对外 REST（token/限流/execlog 外部留痕）+ 生命周期 webhook + `docs/openapi/incidents.yaml` + curl 冒烟                                                                                   | —    | ⏳ 待做    |

### 批 3 交付说明（2026-10-10）

- 三页：`/ops/incidents`（登记/筛选/详情抽屉/认领/解决/重开）、`/ops/incident-reports`（四档位 + 周期步进 + 汇总/趋势/排行榜/责任人报告）、`/ops/incident-categories`（类别 CRUD + 内建保护 + slug 校验）。
- 转事故接入：告警页操作列（refType=alert，refId=alertname+instance 合成签名，detectedAt=startsAt）+ CI/CD 构建失败行（refType=cicd_build，refId=externalId，responsibleType=change）。共享组件 `web/src/components/ConvertToIncidentModal`。
- 静态页不走 PageSpec（§8），发布链闭环不强制；tsc 0 错 + 5 个 jest 套件 23 用例 + guard PASSED。
- 已知边界：probe 故障窗口与 supervisor 事件 server 侧不落库（agent 内存环/本地文件，§4.3 锚点），当前无列表面可挂转换入口——转换入口待数据面落地后接入。

## 已知边界（诚实清单）

- v1 无全自动归并：自动源全部走一键转换（§4.3），全自动规则留 P2。
- 复发窗口 7 天固定不配置；同签名判定对人工登记较粗（category+subcategory 粒度）。
- leader 是字符串不是账号外键：无账号体系校验；多 leader 靠出口侧受众组。
- 归因快照依赖登记时刻数据：execlog 7 天保留期外的旧执行无法事后挂链（探查锚点见 §4.3）。
- 通知可见范围按 audience 解析于读时：角色/映射变更即时生效，但已分发的报表分片内容不回改。
- 推送分级阈值（环比×2 / 复发率>20% / 连续两期恶化）为固定口径 v1 不配置化。
- herald 投递状态/已读不回写（herald 简档 §6 边界）；平台内已读仅站内信闭环；外部出口未配置=仅站内（no-op 默认，零外发）。
- 同比在上线满一年前恒为 missing：「无去年同期数据」占位，不算假数。
- 对外 API v1 不做按 token 的数据范围收窄（全量只读）；OpenAPI 仅文档，无 SDK 生成。
- 周期口径依赖 server 本地时区：多实例跨时区部署会造成周期边界漂移（当前单实例部署模型，scheduler 同款假设）。
- 存量 bug 无类别（category_id=0）：报表归入「未分类」线，不做回填向导。
- probe 故障窗口 / supervisor 事件不落库（agent 侧内存环/本地文件）：无列表页可挂「转事故」入口，转换时须把时间线快照进 incident 行（§4.3）；待数据面落地后接入。
- 告警无行 ID（alertmanager 代理）：转事故 refId 用 alertname+instance 合成签名，与告警侧去重口径一致。
