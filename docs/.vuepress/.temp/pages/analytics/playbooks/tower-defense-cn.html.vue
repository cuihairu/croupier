<template><div><h1 id="塔防-tower-defense-数据采集与分析指引" tabindex="-1"><a class="header-anchor" href="#塔防-tower-defense-数据采集与分析指引"><span>塔防（Tower Defense）数据采集与分析指引</span></a></h1>
<p>适用范围：移动端塔防（TD）。核心目标是平衡关卡与塔型、优化广告与内购收益，并保障稳定性。</p>
<h2 id="核心循环与系统要素" tabindex="-1"><a class="header-anchor" href="#核心循环与系统要素"><span>核心循环与系统要素</span></a></h2>
<ul>
<li>关卡-波次：路径/格点布塔 -&gt; 抗波 -&gt; 资源滚雪球；BOSS波与特殊单位。</li>
<li>经济系统：金币/宝石产出来源（击杀/波次奖励/任务/广告），消耗（建造/升级/技能）。</li>
<li>变现：激励视频（波次间/复活/奖励加倍）+ 内购（增益/礼包/去广告）。</li>
</ul>
<h2 id="埋点清单-对齐-events-yaml" tabindex="-1"><a class="header-anchor" href="#埋点清单-对齐-events-yaml"><span>埋点清单（对齐 events.yaml）</span></a></h2>
<ul>
<li>progression.start / complete / fail
<ul>
<li>level_id, difficulty, attempt_index, wave_index, is_boss_wave, duration_ms, retries, hearts_remaining</li>
</ul>
</li>
<li>economy.earn / economy.spend
<ul>
<li>currency, amount, source(such as kill_enemy, wave_bonus, ad_reward), sink(such as tower_build, tower_upgrade), balance_after</li>
</ul>
</li>
<li>ad.impression / ad.reward
<ul>
<li>ad_network, placement_id, placement_type(between_waves, revive, booster, double_reward)</li>
</ul>
</li>
<li>td.tower.build / td.tower.upgrade
<ul>
<li>level_id, tower_id, tower_type, pos_x/pos_y, cost, wave_index</li>
</ul>
</li>
<li>性能与稳定
<ul>
<li>performance.frame、error.crash</li>
</ul>
</li>
</ul>
<h2 id="核心指标-对齐-metrics-yaml" tabindex="-1"><a class="header-anchor" href="#核心指标-对齐-metrics-yaml"><span>核心指标（对齐 metrics.yaml）</span></a></h2>
<ul>
<li>塔防：td_level_clear_rate、td_wave_fail_rate_by_wave、td_avg_hearts_remaining、td_tower_usage_rate_by_type、td_upgrade_rate</li>
<li>变现：ad_arpu、ad_impressions_per_dau</li>
<li>留存：retention_d1、retention_d7</li>
<li>会话：session_length_p50/p95</li>
</ul>
<h2 id="维度与分群" tabindex="-1"><a class="header-anchor" href="#维度与分群"><span>维度与分群</span></a></h2>
<ul>
<li>level_id、difficulty、wave_index、tower_type、map_id、placement_type、device_perf_grade、country/region、channel。</li>
</ul>
<h2 id="质量校验" tabindex="-1"><a class="header-anchor" href="#质量校验"><span>质量校验</span></a></h2>
<ul>
<li>波次 index 单调且连续；BOSS 波标记一致。</li>
<li>建造/升级 cost 与经济余额对应；负余额/负收益不得出现。</li>
<li>广告曝光位置与波次/复活逻辑一致；奖励与收益匹配。</li>
</ul>
<h2 id="实施提示-collector" tabindex="-1"><a class="header-anchor" href="#实施提示-collector"><span>实施提示（Collector）</span></a></h2>
<ul>
<li>统一时间单位 ms；对 user_id/device_id 做加盐哈希。</li>
<li>将 placement_id 规范命名并映射 placement_type，便于聚合与看板。</li>
<li>对 hearts_remaining 进行范围校验（0..max_hearts）。</li>
</ul>
</div></template>


