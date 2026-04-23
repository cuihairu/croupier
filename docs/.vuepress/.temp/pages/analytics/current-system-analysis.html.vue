<template><div><h1 id="架构现状" tabindex="-1"><a class="header-anchor" href="#架构现状"><span>架构现状</span></a></h1>
<p>组件</p>
<ul>
<li>Public Ingestion（services/ingest）：HTTP/JSON + HMAC 校验，写入 Redis Streams/Kafka</li>
<li>Analytics Worker（cmd/analytics-worker）：从 MQ 消费 → 清洗/聚合 → 写入 ClickHouse</li>
<li>Server（internal/app/server）：OTel 接入、管理 API、可选业务事件直写 MQ</li>
<li>存储：ClickHouse（明细/聚合）、Redis/Kafka（缓冲）</li>
</ul>
<p>数据流</p>
<ul>
<li>Client → Ingestion → MQ → Worker → ClickHouse → Grafana/报表</li>
<li>Server traces/metrics → OTel Collector → ClickHouse/Prometheus</li>
</ul>
<p>边界与约束</p>
<ul>
<li>客户端仅走 Ingestion（公网/DMZ），不暴露核心控制面</li>
<li>事件 schema 灵活（JSON），通过 Worker 做口径收敛与维度控制</li>
</ul>
<p>风险点</p>
<ul>
<li>高基数维度导致 ClickHouse 表爆炸</li>
<li>Ingestion/Worker 异常导致延迟和丢数风险</li>
<li>签名/时间戳偏差导致客户端报错</li>
</ul>
<p>改进方向（概述）</p>
<ul>
<li>指标与事件规范化：统一字段、维度白名单、版本化</li>
<li>可靠性：重试/死信、回放、端到端监控指标</li>
<li>成本：冷热分层、TTL/归档、聚合物化视图</li>
</ul>
</div></template>


