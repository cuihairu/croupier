<template><div><h1 id="挂机-放置-idle-incremental-数据采集与分析指引" tabindex="-1"><a class="header-anchor" href="#挂机-放置-idle-incremental-数据采集与分析指引"><span>挂机/放置（Idle/Incremental）数据采集与分析指引</span></a></h1>
<p>目标：衡量离线收益与长期留存，平衡产消，定位核心付费节点。</p>
<h2 id="核心循环" tabindex="-1"><a class="header-anchor" href="#核心循环"><span>核心循环</span></a></h2>
<ul>
<li>在线/离线收益积累 -&gt; 解锁建筑/功能 -&gt; 升级/加速 -&gt; 指数式成长。</li>
</ul>
<h2 id="埋点清单" tabindex="-1"><a class="header-anchor" href="#埋点清单"><span>埋点清单</span></a></h2>
<ul>
<li>session.start / end</li>
<li>economy.earn / spend
<ul>
<li>amount, currency_kind, source(offine/quest/ad_reward/kill_enemy), sink(upgrade/unlock/boost)</li>
<li>offline_duration_ms（当 source=offline）</li>
</ul>
</li>
<li>progression.complete（node_id 或 feature_id 表达解锁）</li>
</ul>
<h2 id="核心指标" tabindex="-1"><a class="header-anchor" href="#核心指标"><span>核心指标</span></a></h2>
<ul>
<li>retention_d7 / retention_d30</li>
<li>idle_offline_income_share（离线收益占比）</li>
<li>economy_balance_ratio（经济产消比）</li>
<li>session_length_p95</li>
</ul>
<h2 id="维度与分群" tabindex="-1"><a class="header-anchor" href="#维度与分群"><span>维度与分群</span></a></h2>
<ul>
<li>platform、region、channel、node_id/feature_id、payer_segment（新/老/付费）。</li>
</ul>
<h2 id="质量校验" tabindex="-1"><a class="header-anchor" href="#质量校验"><span>质量校验</span></a></h2>
<ul>
<li>离线时长与收益上限；负收益/负余额拦截；异常尖峰识别。</li>
</ul>
</div></template>


