<template><div><h1 id="卡牌-card-ccg-数据采集与分析指引" tabindex="-1"><a class="header-anchor" href="#卡牌-card-ccg-数据采集与分析指引"><span>卡牌（Card/CCG）数据采集与分析指引</span></a></h1>
<p>目标：评估卡牌与卡组生态（使用率/胜率），稳定抽卡保底，优化付费。</p>
<h2 id="核心循环" tabindex="-1"><a class="header-anchor" href="#核心循环"><span>核心循环</span></a></h2>
<ul>
<li>收集-&gt;构筑-&gt;对战-&gt;迭代环境；卡池与通行证驱动变现。</li>
</ul>
<h2 id="埋点清单" tabindex="-1"><a class="header-anchor" href="#埋点清单"><span>埋点清单</span></a></h2>
<ul>
<li>match.start / end：deck_id, deck_archetype, deck_cards, match_result</li>
<li>deck.set_active：deck_id, deck_archetype, card_ids</li>
<li>gacha.pull：pool_id, pulls, rarity, pity_counter, item_ids</li>
</ul>
<h2 id="核心指标" tabindex="-1"><a class="header-anchor" href="#核心指标"><span>核心指标</span></a></h2>
<ul>
<li>card_usage_rate（卡牌使用率）</li>
<li>card_win_rate（卡牌胜率）</li>
<li>deck_archetype_share / deck_archetype_win_rate（原型占比/胜率）</li>
<li>arpu / arppu、retention_d7</li>
</ul>
<h2 id="维度与分群" tabindex="-1"><a class="header-anchor" href="#维度与分群"><span>维度与分群</span></a></h2>
<ul>
<li>card_id、deck_archetype、game_mode、mmr_bracket、region。</li>
</ul>
<h2 id="质量校验" tabindex="-1"><a class="header-anchor" href="#质量校验"><span>质量校验</span></a></h2>
<ul>
<li>保底计数递增一致；卡组与对局卡牌一致性；极端胜率波动告警。</li>
</ul>
</div></template>


