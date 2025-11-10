# 棋牌/桌面（Board/Table）数据采集与分析指引

目标：保障公平与稳态经济，监控对局质量与抽水合理性。

## 核心循环
- 房间/桌子 -> 对局 ->（可选）回合/一手 -> 结算。

## 埋点清单
- match.start / end：room_id/table_id、game_variant、stakes、seat_id、result、rake、pot
- round.start / end：round_id/hand_id、seat_id、duration_ms、result、rake

## 核心指标
- avg_round_duration（回合时长P50）
- win_rate_by_seat（按座位胜率）
- rake_rate（抽水率）
- afk_leave_rate（中途离场率）
- retention_d7

## 维度与分群
- game_variant、stakes、seat_id、region、platform。

## 质量校验
- 异常对局（极短时长/超高胜率/协同异常）；断线/重连链路；rake 与 stakes 匹配。
