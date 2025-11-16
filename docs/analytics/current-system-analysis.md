---
title: 当前系统分析
---
# 架构现状

组件
- Public Ingestion（cmd/analytics-ingest）：HTTP/JSON + HMAC 校验，写入 Redis Streams/Kafka
- Analytics Worker（cmd/analytics-worker）：从 MQ 消费 → 清洗/聚合 → 写入 ClickHouse
- Server（internal/app/server）：OTel 接入、管理 API、可选业务事件直写 MQ
- 存储：ClickHouse（明细/聚合）、Redis/Kafka（缓冲）

数据流
- Client → Ingestion → MQ → Worker → ClickHouse → Grafana/报表
- Server traces/metrics → OTel Collector → ClickHouse/Prometheus

边界与约束
- 客户端仅走 Ingestion（公网/DMZ），不暴露核心控制面
- 事件 schema 灵活（JSON），通过 Worker 做口径收敛与维度控制

风险点
- 高基数维度导致 ClickHouse 表爆炸
- Ingestion/Worker 异常导致延迟和丢数风险
- 签名/时间戳偏差导致客户端报错

改进方向（概述）
- 指标与事件规范化：统一字段、维度白名单、版本化
- 可靠性：重试/死信、回放、端到端监控指标
- 成本：冷热分层、TTL/归档、聚合物化视图
