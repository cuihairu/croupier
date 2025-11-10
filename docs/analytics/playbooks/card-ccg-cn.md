# 卡牌（Card/CCG）数据采集与分析指引

目标：评估卡牌与卡组生态（使用率/胜率），稳定抽卡保底，优化付费。

## 核心循环
- 收集->构筑->对战->迭代环境；卡池与通行证驱动变现。

## 埋点清单
- match.start / end：deck_id, deck_archetype, deck_cards, match_result
- deck.set_active：deck_id, deck_archetype, card_ids
- gacha.pull：pool_id, pulls, rarity, pity_counter, item_ids

## 核心指标
- card_usage_rate（卡牌使用率）
- card_win_rate（卡牌胜率）
- deck_archetype_share / deck_archetype_win_rate（原型占比/胜率）
- arpu / arppu、retention_d7

## 维度与分群
- card_id、deck_archetype、game_mode、mmr_bracket、region。

## 质量校验
- 保底计数递增一致；卡组与对局卡牌一致性；极端胜率波动告警。
