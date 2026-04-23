<template><div><h1 id="不同类型的指标重点" tabindex="-1"><a class="header-anchor" href="#不同类型的指标重点"><span>不同类型的指标重点</span></a></h1>
<p>总览</p>
<table>
<thead>
<tr>
<th>类型</th>
<th>关键指标</th>
<th>典型事件</th>
<th>采集要点</th>
</tr>
</thead>
<tbody>
<tr>
<td>休闲</td>
<td>留存、关卡漏斗、广告收益</td>
<td>session/level/purchase</td>
<td>设备维度多、碎片化时段；关注首日/三日</td>
</tr>
<tr>
<td>RPG</td>
<td>成长、社交、付费深度</td>
<td>progression/guild/purchase</td>
<td>长周期、经济系统；反作弊与资产一致性</td>
</tr>
<tr>
<td>竞技</td>
<td>匹配质量、平衡性、网络</td>
<td>match/result/latency</td>
<td>反作弊、网络体验、ELO 匹配参数</td>
</tr>
<tr>
<td>策略</td>
<td>时长、元游戏、决策变量</td>
<td>session/meta/purchase</td>
<td>长时段留存、跨天活跃；元数据追踪</td>
</tr>
</tbody>
</table>
<p>实施建议</p>
<ul>
<li>事件标准：统一基本字段（uid、game_id、env、ts、platform、version、country）</li>
<li>指标映射：从 ./game-metrics-overview.md 选择基础 + 类型特有子集</li>
<li>看板模板：按类型提供预制仪表板，减少上手成本</li>
</ul>
</div></template>


