---
title: 游戏类型适配
---
# 不同类型的指标重点

总览

| 类型 | 关键指标 | 典型事件 | 采集要点 |
|-----|---------|---------|---------|
| 休闲 | 留存、关卡漏斗、广告收益 | session/level/purchase | 设备维度多、碎片化时段；关注首日/三日 |
| RPG | 成长、社交、付费深度 | progression/guild/purchase | 长周期、经济系统；反作弊与资产一致性 |
| 竞技 | 匹配质量、平衡性、网络 | match/result/latency | 反作弊、网络体验、ELO 匹配参数 |
| 策略 | 时长、元游戏、决策变量 | session/meta/purchase | 长时段留存、跨天活跃；元数据追踪 |

实施建议
- 事件标准：统一基本字段（uid、game_id、env、ts、platform、version、country）
- 指标映射：从 ./game-metrics-overview.md 选择基础 + 类型特有子集
- 看板模板：按类型提供预制仪表板，减少上手成本
