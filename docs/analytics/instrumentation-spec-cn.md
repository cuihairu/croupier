# Croupier 游戏数据埋点与指标规范（中文）

本文档说明本仓库的标准化埋点与指标体系，并给出不同游戏类型的特征与建议分析点。对应的机器可读配置：
- 埋点事件清单：configs/analytics/events.yaml
- 指标定义清单：configs/analytics/metrics.yaml
- 游戏类型与推荐项：configs/analytics/game_types.yaml

## 设计目标
- 标准化：统一事件命名、属性与单位，跨端（客户端/服务端）一致。
- 可落地：对齐 OpenTelemetry，Collector 可按该规范进行 transform/脱敏/导出。
- 可扩展：新事件、新指标与新类型以增量方式扩展，向后兼容。
- 合规：用户/设备标识默认“假名化”（salted hash），不存储原始 PII。

## 命名与公共属性
- 事件命名：`category.object.action`（例：`session.start`、`progression.complete`）。
- 属性命名：`snake_case`（例：`event_time`、`match_result`）。
- 公共属性（节选）：`user_id`、`session_id`、`device_id`、`platform`、`region`、`country`、`app_version`、`server`、`game_type`、`event_time`。
- 隐私策略：`user_id`、`device_id` 采用加盐哈希；敏感字段尽量在 Collector 端脱敏或删除。

## 核心事件（节选）
- 会话：`session.start`、`session.end`（含 `duration_ms`、`cause_of_end`）。
- 用户：`user.register`、`user.login`。
- 进度：`progression.start`、`progression.complete`、`progression.fail`（含 `level_id`、`retries`）。
- 对局：`match.start`、`match.end`（含 `match_result`、`kills/deaths/assists`）。
- 经济：`economy.earn`、`economy.spend`（`currency_kind`=soft/hard/real）。
- 变现：`monetization.purchase_{attempt,success,fail}`（`price_usd`）。
- 广告：`ad.impression/click/reward`（`ad_format`=rewarded/interstitial/banner）。
- 性能/网络：`performance.frame/device`、`network.rtt`、异常：`error.crash/anr`。
- 其他：`gacha.pull`、`craft.complete`、`ui.screen_view/click`、`social.guild_{join,leave}`、`combat.stats`（`shots/hits`）。

详细字段与单位见 `configs/analytics/events.yaml`。

## 指标规范（节选）
- 活跃与留存：`dau/wau/mau`、`retention_d1/d7/d30`。
- 会话：`session_length_p50/p95`。
- 稳定性：`crash_rate`、`crash_free_users_rate`、`anr_rate`。
- 商业化：`arpu/arppu/pur`、广告：`ad_arpu/ad_impressions_per_dau`。
- 对局：`win_rate`、`kda`、匹配：`queue_time_p95`。
- 射击精度：`accuracy_rate`。
- 关卡：`level_completion_rate`、`retries_avg`。
- 扭蛋：`pity_counter_avg`。

计算口径、窗口、维度见 `configs/analytics/metrics.yaml`。

## 游戏类型与建议分析点（摘要）
- 休闲解谜（casual_puzzle）：关卡完成率、会话时长、广告 ARPU。
- 超休闲（hyper_casual）：D1 留存、会话时长、广告曝光/DAU。
- 放置（idle_incremental）：D7 留存、ARPU、长会话 p95。
- RPG/MMORPG：D7/30 留存、ARPU/ARPPU、胜率/KDA、公会参与。
- SLG/4X：长期留存、收入与消耗结构、联盟活跃。
- MOBA：胜率/KDA、对局时长、崩溃率。
- 射击/Battle Royale：命中率、胜率、时延/崩溃。
- 体育/竞速：胜率/成绩、对局/比赛时长。
- 卡牌：胜率、ARPU、抽卡保底均值。
- Roguelike：关卡完成率、重试均值、D7 留存。
- 模拟经营/沙盒生存：长会话、崩溃率、产消循环指标。
- 派对/音游/平台动作/叙事：留存、会话长度、核心漏斗（谱面/章节/关卡）。

完整类型清单与推荐项见 `configs/analytics/game_types.yaml`。

## 数据治理与合规
- 采集边界：注册/登录/支付/进度等权威数据以服务端为准；客户端补充行为与性能。
- 脱敏：Collector 中对 `user_id`/`device_id` 做加盐哈希；移除 IP/精确地理等敏感字段。
- 单位：时间统一 ms，金额统一 USD（保留原币种），帧率 FPS。
- 质量：落地一致性检查与异常检测（空会话、重复订单、极端值）。

## 与 OpenTelemetry 的映射
- 事件类数据→ Logs；数值分布→ Metrics（直方图）；链路视角→ Traces（可选）。
- Collector 侧完成：属性重命名、单位转换、标签补充、敏感字段清理、分流（ClickHouse/Redis/Kafka/Prom）。

## 版本与扩展
- 使用 `version` 字段管理配置版本；新增事件与属性保持向后兼容；破坏性变更需 bump 次版本并提供迁移脚本。

## 快速使用
1. 客户端/服务端按 `events.yaml` 上报事件。
2. Collector 根据本规范进行 transform 与导出。
3. ClickHouse 按 `metrics.yaml` 的口径进行查询聚合，或由离线任务生成宽表。

### game_types.yaml 字段说明
- id：类型标识（英文小写，代码引用）
- name：类型名称（英文）
- summary：一句话简介（中文，面向业务/产品）
- description：更详细的类型特征（中文）
- characteristics：标签化特征（英文枚举，便于检索）
- recommended_events / recommended_metrics：该类型的建议事件与指标
- breakdowns：默认切片维度

注：自本次更新起，game_types.yaml 中的 summary 字段用于“名称解释”（中英对照与缩写展开），例如：
- MMORPG → "MMORPG 大型多人在线角色扮演游戏（Massively Multiplayer Online RPG）"
- MOBA → "MOBA 多人在线战术竞技（Multiplayer Online Battle Arena）"

### description 写作规范
- 两句话：1) 玩法/核心循环与系统要素；2) 变现方式与关键分析关注点。
- 建议每句 &lt;= 40~50 字，避免堆叠术语，尽量业务可读；需要时括号补充英文术语。
- 不写具体指标或事件 id（这些放在 recommended_* 中）；保持通用性，不含产品专有名词。
- 单位与口径不要在 description 中声明，放入 metrics.yaml 或文档专节。

## 类型别名与传统分类对照
- 新增文件：configs/analytics/taxonomy.yaml，包含传统分类代码（RPG/ARPG/SRPG/FPS/RTS/SLG/RAC/ACT/SIM/EDU/FLY/TAB/SPG/FTG/SFTG/PUZ/STG/AVG/ETC/TD）到本规范 game_types 的映射，以及中英文名称与常见别名。
- events.yaml 允许可选字段 `genre_code`（单值）指向 taxonomy 代码，`game_type` 仍作为主标识，用于聚合与默认配置。
- game_types.yaml 补充了 `aliases` 与 `examples` 字段，便于检索与沟通。

### metrics.yaml 字段补充
- zh_name：指标中文名（用于报表展示）。
- zh_desc：中文描述（简述口径、用途与解读要点）。
- 其余字段含义保持不变（type/window/source/formula/numerator/denominator/dimensions）。
