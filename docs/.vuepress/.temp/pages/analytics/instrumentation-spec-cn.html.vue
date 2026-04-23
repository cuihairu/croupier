<template><div><h1 id="croupier-游戏数据埋点与指标规范-中文" tabindex="-1"><a class="header-anchor" href="#croupier-游戏数据埋点与指标规范-中文"><span>Croupier 游戏数据埋点与指标规范（中文）</span></a></h1>
<p>本文档说明本仓库的标准化埋点与指标体系，并给出不同游戏类型的特征与建议分析点。对应的机器可读配置：</p>
<ul>
<li>埋点事件清单：configs/analytics/events.yaml</li>
<li>指标定义清单：configs/analytics/metrics.yaml</li>
<li>游戏类型与推荐项：configs/analytics/game_types.yaml</li>
</ul>
<h2 id="设计目标" tabindex="-1"><a class="header-anchor" href="#设计目标"><span>设计目标</span></a></h2>
<ul>
<li>标准化：统一事件命名、属性与单位，跨端（客户端/服务端）一致。</li>
<li>可落地：对齐 OpenTelemetry，Collector 可按该规范进行 transform/脱敏/导出。</li>
<li>可扩展：新事件、新指标与新类型以增量方式扩展，向后兼容。</li>
<li>合规：用户/设备标识默认“假名化”（salted hash），不存储原始 PII。</li>
</ul>
<h2 id="命名与公共属性" tabindex="-1"><a class="header-anchor" href="#命名与公共属性"><span>命名与公共属性</span></a></h2>
<ul>
<li>事件命名：<code v-pre>category.object.action</code>（例：<code v-pre>session.start</code>、<code v-pre>progression.complete</code>）。</li>
<li>属性命名：<code v-pre>snake_case</code>（例：<code v-pre>event_time</code>、<code v-pre>match_result</code>）。</li>
<li>公共属性（节选）：<code v-pre>user_id</code>、<code v-pre>session_id</code>、<code v-pre>device_id</code>、<code v-pre>platform</code>、<code v-pre>region</code>、<code v-pre>country</code>、<code v-pre>app_version</code>、<code v-pre>server</code>、<code v-pre>game_type</code>、<code v-pre>event_time</code>。</li>
<li>隐私策略：<code v-pre>user_id</code>、<code v-pre>device_id</code> 采用加盐哈希；敏感字段尽量在 Collector 端脱敏或删除。</li>
</ul>
<h2 id="核心事件-节选" tabindex="-1"><a class="header-anchor" href="#核心事件-节选"><span>核心事件（节选）</span></a></h2>
<ul>
<li>会话：<code v-pre>session.start</code>、<code v-pre>session.end</code>（含 <code v-pre>duration_ms</code>、<code v-pre>cause_of_end</code>）。</li>
<li>用户：<code v-pre>user.register</code>、<code v-pre>user.login</code>。</li>
<li>进度：<code v-pre>progression.start</code>、<code v-pre>progression.complete</code>、<code v-pre>progression.fail</code>（含 <code v-pre>level_id</code>、<code v-pre>retries</code>）。</li>
<li>对局：<code v-pre>match.start</code>、<code v-pre>match.end</code>（含 <code v-pre>match_result</code>、<code v-pre>kills/deaths/assists</code>）。</li>
<li>经济：<code v-pre>economy.earn</code>、<code v-pre>economy.spend</code>（<code v-pre>currency_kind</code>=soft/hard/real）。</li>
<li>变现：<code v-pre>monetization.purchase_{attempt,success,fail}</code>（<code v-pre>price_usd</code>）。</li>
<li>广告：<code v-pre>ad.impression/click/reward</code>（<code v-pre>ad_format</code>=rewarded/interstitial/banner）。</li>
<li>性能/网络：<code v-pre>performance.frame/device</code>、<code v-pre>network.rtt</code>、异常：<code v-pre>error.crash/anr</code>。</li>
<li>其他：<code v-pre>gacha.pull</code>、<code v-pre>craft.complete</code>、<code v-pre>ui.screen_view/click</code>、<code v-pre>social.guild_{join,leave}</code>、<code v-pre>combat.stats</code>（<code v-pre>shots/hits</code>）。</li>
</ul>
<p>详细字段与单位见 <code v-pre>configs/analytics/events.yaml</code>。</p>
<h2 id="指标规范-节选" tabindex="-1"><a class="header-anchor" href="#指标规范-节选"><span>指标规范（节选）</span></a></h2>
<ul>
<li>活跃与留存：<code v-pre>dau/wau/mau</code>、<code v-pre>retention_d1/d7/d30</code>。</li>
<li>会话：<code v-pre>session_length_p50/p95</code>。</li>
<li>稳定性：<code v-pre>crash_rate</code>、<code v-pre>crash_free_users_rate</code>、<code v-pre>anr_rate</code>。</li>
<li>商业化：<code v-pre>arpu/arppu/pur</code>、广告：<code v-pre>ad_arpu/ad_impressions_per_dau</code>。</li>
<li>对局：<code v-pre>win_rate</code>、<code v-pre>kda</code>、匹配：<code v-pre>queue_time_p95</code>。</li>
<li>射击精度：<code v-pre>accuracy_rate</code>。</li>
<li>关卡：<code v-pre>level_completion_rate</code>、<code v-pre>retries_avg</code>。</li>
<li>扭蛋：<code v-pre>pity_counter_avg</code>。</li>
</ul>
<p>计算口径、窗口、维度见 <code v-pre>configs/analytics/metrics.yaml</code>。</p>
<h2 id="游戏类型与建议分析点-摘要" tabindex="-1"><a class="header-anchor" href="#游戏类型与建议分析点-摘要"><span>游戏类型与建议分析点（摘要）</span></a></h2>
<ul>
<li>休闲解谜（casual_puzzle）：关卡完成率、会话时长、广告 ARPU。</li>
<li>超休闲（hyper_casual）：D1 留存、会话时长、广告曝光/DAU。</li>
<li>放置（idle_incremental）：D7 留存、ARPU、长会话 p95。</li>
<li>RPG/MMORPG：D7/30 留存、ARPU/ARPPU、胜率/KDA、公会参与。</li>
<li>SLG/4X：长期留存、收入与消耗结构、联盟活跃。</li>
<li>MOBA：胜率/KDA、对局时长、崩溃率。</li>
<li>射击/Battle Royale：命中率、胜率、时延/崩溃。</li>
<li>体育/竞速：胜率/成绩、对局/比赛时长。</li>
<li>卡牌：胜率、ARPU、抽卡保底均值。</li>
<li>Roguelike：关卡完成率、重试均值、D7 留存。</li>
<li>模拟经营/沙盒生存：长会话、崩溃率、产消循环指标。</li>
<li>派对/音游/平台动作/叙事：留存、会话长度、核心漏斗（谱面/章节/关卡）。</li>
</ul>
<p>完整类型清单与推荐项见 <code v-pre>configs/analytics/game_types.yaml</code>。</p>
<h2 id="数据治理与合规" tabindex="-1"><a class="header-anchor" href="#数据治理与合规"><span>数据治理与合规</span></a></h2>
<ul>
<li>采集边界：注册/登录/支付/进度等权威数据以服务端为准；客户端补充行为与性能。</li>
<li>脱敏：Collector 中对 <code v-pre>user_id</code>/<code v-pre>device_id</code> 做加盐哈希；移除 IP/精确地理等敏感字段。</li>
<li>单位：时间统一 ms，金额统一 USD（保留原币种），帧率 FPS。</li>
<li>质量：落地一致性检查与异常检测（空会话、重复订单、极端值）。</li>
</ul>
<h2 id="与-opentelemetry-的映射" tabindex="-1"><a class="header-anchor" href="#与-opentelemetry-的映射"><span>与 OpenTelemetry 的映射</span></a></h2>
<ul>
<li>事件类数据→ Logs；数值分布→ Metrics（直方图）；链路视角→ Traces（可选）。</li>
<li>Collector 侧完成：属性重命名、单位转换、标签补充、敏感字段清理、分流（ClickHouse/Redis/Kafka/Prom）。</li>
</ul>
<h2 id="版本与扩展" tabindex="-1"><a class="header-anchor" href="#版本与扩展"><span>版本与扩展</span></a></h2>
<ul>
<li>使用 <code v-pre>version</code> 字段管理配置版本；新增事件与属性保持向后兼容；破坏性变更需 bump 次版本并提供迁移脚本。</li>
</ul>
<h2 id="快速使用" tabindex="-1"><a class="header-anchor" href="#快速使用"><span>快速使用</span></a></h2>
<ol>
<li>客户端/服务端按 <code v-pre>events.yaml</code> 上报事件。</li>
<li>Collector 根据本规范进行 transform 与导出。</li>
<li>ClickHouse 按 <code v-pre>metrics.yaml</code> 的口径进行查询聚合，或由离线任务生成宽表。</li>
</ol>
<h3 id="game-types-yaml-字段说明" tabindex="-1"><a class="header-anchor" href="#game-types-yaml-字段说明"><span>game_types.yaml 字段说明</span></a></h3>
<ul>
<li>id：类型标识（英文小写，代码引用）</li>
<li>name：类型名称（英文）</li>
<li>summary：一句话简介（中文，面向业务/产品）</li>
<li>description：更详细的类型特征（中文）</li>
<li>characteristics：标签化特征（英文枚举，便于检索）</li>
<li>recommended_events / recommended_metrics：该类型的建议事件与指标</li>
<li>breakdowns：默认切片维度</li>
</ul>
<p>注：自本次更新起，game_types.yaml 中的 summary 字段用于“名称解释”（中英对照与缩写展开），例如：</p>
<ul>
<li>MMORPG → &quot;MMORPG 大型多人在线角色扮演游戏（Massively Multiplayer Online RPG）&quot;</li>
<li>MOBA → &quot;MOBA 多人在线战术竞技（Multiplayer Online Battle Arena）&quot;</li>
</ul>
<h3 id="description-写作规范" tabindex="-1"><a class="header-anchor" href="#description-写作规范"><span>description 写作规范</span></a></h3>
<ul>
<li>两句话：1) 玩法/核心循环与系统要素；2) 变现方式与关键分析关注点。</li>
<li>建议每句 &lt;= 40~50 字，避免堆叠术语，尽量业务可读；需要时括号补充英文术语。</li>
<li>不写具体指标或事件 id（这些放在 recommended_* 中）；保持通用性，不含产品专有名词。</li>
<li>单位与口径不要在 description 中声明，放入 metrics.yaml 或文档专节。</li>
</ul>
<h2 id="类型别名与传统分类对照" tabindex="-1"><a class="header-anchor" href="#类型别名与传统分类对照"><span>类型别名与传统分类对照</span></a></h2>
<ul>
<li>新增文件：configs/analytics/taxonomy.yaml，包含传统分类代码（RPG/ARPG/SRPG/FPS/RTS/SLG/RAC/ACT/SIM/EDU/FLY/TAB/SPG/FTG/SFTG/PUZ/STG/AVG/ETC/TD）到本规范 game_types 的映射，以及中英文名称与常见别名。</li>
<li>events.yaml 允许可选字段 <code v-pre>genre_code</code>（单值）指向 taxonomy 代码，<code v-pre>game_type</code> 仍作为主标识，用于聚合与默认配置。</li>
<li>game_types.yaml 补充了 <code v-pre>aliases</code> 与 <code v-pre>examples</code> 字段，便于检索与沟通。</li>
</ul>
<h3 id="metrics-yaml-字段补充" tabindex="-1"><a class="header-anchor" href="#metrics-yaml-字段补充"><span>metrics.yaml 字段补充</span></a></h3>
<ul>
<li>zh_name：指标中文名（用于报表展示）。</li>
<li>zh_desc：中文描述（简述口径、用途与解读要点）。</li>
<li>其余字段含义保持不变（type/window/source/formula/numerator/denominator/dimensions）。</li>
</ul>
</div></template>


