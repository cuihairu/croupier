<template><div><h1 id="棋牌-桌面-board-table-数据采集与分析指引" tabindex="-1"><a class="header-anchor" href="#棋牌-桌面-board-table-数据采集与分析指引"><span>棋牌/桌面（Board/Table）数据采集与分析指引</span></a></h1>
<p>目标：保障公平与稳态经济，监控对局质量与抽水合理性。</p>
<h2 id="核心循环" tabindex="-1"><a class="header-anchor" href="#核心循环"><span>核心循环</span></a></h2>
<ul>
<li>房间/桌子 -&gt; 对局 -&gt;（可选）回合/一手 -&gt; 结算。</li>
</ul>
<h2 id="埋点清单" tabindex="-1"><a class="header-anchor" href="#埋点清单"><span>埋点清单</span></a></h2>
<ul>
<li>match.start / end：room_id/table_id、game_variant、stakes、seat_id、result、rake、pot</li>
<li>round.start / end：round_id/hand_id、seat_id、duration_ms、result、rake</li>
</ul>
<h2 id="核心指标" tabindex="-1"><a class="header-anchor" href="#核心指标"><span>核心指标</span></a></h2>
<ul>
<li>avg_round_duration（回合时长P50）</li>
<li>win_rate_by_seat（按座位胜率）</li>
<li>rake_rate（抽水率）</li>
<li>afk_leave_rate（中途离场率）</li>
<li>retention_d7</li>
</ul>
<h2 id="维度与分群" tabindex="-1"><a class="header-anchor" href="#维度与分群"><span>维度与分群</span></a></h2>
<ul>
<li>game_variant、stakes、seat_id、region、platform。</li>
</ul>
<h2 id="质量校验" tabindex="-1"><a class="header-anchor" href="#质量校验"><span>质量校验</span></a></h2>
<ul>
<li>异常对局（极短时长/超高胜率/协同异常）；断线/重连链路；rake 与 stakes 匹配。</li>
</ul>
</div></template>


