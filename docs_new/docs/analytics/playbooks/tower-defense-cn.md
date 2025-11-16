# 塔防（Tower Defense）数据采集与分析指引

适用范围：移动端塔防（TD）。核心目标是平衡关卡与塔型、优化广告与内购收益，并保障稳定性。

## 核心循环与系统要素
- 关卡-波次：路径/格点布塔 -> 抗波 -> 资源滚雪球；BOSS波与特殊单位。
- 经济系统：金币/宝石产出来源（击杀/波次奖励/任务/广告），消耗（建造/升级/技能）。
- 变现：激励视频（波次间/复活/奖励加倍）+ 内购（增益/礼包/去广告）。

## 埋点清单（对齐 events.yaml）
- progression.start / complete / fail
  - level_id, difficulty, attempt_index, wave_index, is_boss_wave, duration_ms, retries, hearts_remaining
- economy.earn / economy.spend
  - currency, amount, source(such as kill_enemy, wave_bonus, ad_reward), sink(such as tower_build, tower_upgrade), balance_after
- ad.impression / ad.reward
  - ad_network, placement_id, placement_type(between_waves, revive, booster, double_reward)
- td.tower.build / td.tower.upgrade
  - level_id, tower_id, tower_type, pos_x/pos_y, cost, wave_index
- 性能与稳定
  - performance.frame、error.crash

## 核心指标（对齐 metrics.yaml）
- 塔防：td_level_clear_rate、td_wave_fail_rate_by_wave、td_avg_hearts_remaining、td_tower_usage_rate_by_type、td_upgrade_rate
- 变现：ad_arpu、ad_impressions_per_dau
- 留存：retention_d1、retention_d7
- 会话：session_length_p50/p95

## 维度与分群
- level_id、difficulty、wave_index、tower_type、map_id、placement_type、device_perf_grade、country/region、channel。

## 质量校验
- 波次 index 单调且连续；BOSS 波标记一致。
- 建造/升级 cost 与经济余额对应；负余额/负收益不得出现。
- 广告曝光位置与波次/复活逻辑一致；奖励与收益匹配。

## 实施提示（Collector）
- 统一时间单位 ms；对 user_id/device_id 做加盐哈希。
- 将 placement_id 规范命名并映射 placement_type，便于聚合与看板。
- 对 hearts_remaining 进行范围校验（0..max_hearts）。
