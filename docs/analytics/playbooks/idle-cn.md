# 挂机/放置（Idle/Incremental）数据采集与分析指引

目标：衡量离线收益与长期留存，平衡产消，定位核心付费节点。

## 核心循环
- 在线/离线收益积累 -> 解锁建筑/功能 -> 升级/加速 -> 指数式成长。

## 埋点清单
- session.start / end
- economy.earn / spend
  - amount, currency_kind, source(offine/quest/ad_reward/kill_enemy), sink(upgrade/unlock/boost)
  - offline_duration_ms（当 source=offline）
- progression.complete（node_id 或 feature_id 表达解锁）

## 核心指标
- retention_d7 / retention_d30
- idle_offline_income_share（离线收益占比）
- economy_balance_ratio（经济产消比）
- session_length_p95

## 维度与分群
- platform、region、channel、node_id/feature_id、payer_segment（新/老/付费）。

## 质量校验
- 离线时长与收益上限；负收益/负余额拦截；异常尖峰识别。
