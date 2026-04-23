<template><div><h1 id="通用指标" tabindex="-1"><a class="header-anchor" href="#通用指标"><span>通用指标</span></a></h1>
<p>活跃与留存</p>
<ul>
<li>DAU/WAU/MAU：去重用户数；以 uid 或设备 id 去重</li>
<li>次日留存/7 日留存：次日/第 7 日回访用户 / 当日新增</li>
<li>Session 时长：单次会话在线时长；异常短/长会话需剔除</li>
</ul>
<p>转化与付费</p>
<ul>
<li>新增付费率：新增用户中发生首付费的比例</li>
<li>ARPU/ARPPU：总收入/DAU、总收入/付费用户数</li>
<li>付费留存：发生过付费的用户在 X 日的留存</li>
</ul>
<p>性能体验</p>
<ul>
<li>启动时长：应用从启动到首屏可用的时长（P50/P95）</li>
<li>FPS：平均帧率；低帧率占比（&lt; 30fps）</li>
<li>崩溃率：崩溃会话数 / 总会话数</li>
</ul>
<h1 id="维度与口径建议" tabindex="-1"><a class="header-anchor" href="#维度与口径建议"><span>维度与口径建议</span></a></h1>
<ul>
<li>统一时间粒度：分钟/小时/日；跨时区用 UTC 存储、展示按本地时区</li>
<li>维度白名单：platform、channel、country、version、game_type</li>
<li>高基数字段（uid、ip、trace_id）谨慎做聚合维度，避免卡方炸裂</li>
</ul>
</div></template>


