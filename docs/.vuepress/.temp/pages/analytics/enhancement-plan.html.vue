<template><div><h1 id="路线图" tabindex="-1"><a class="header-anchor" href="#路线图"><span>路线图</span></a></h1>
<p>M1 稳定性（1-2 周）</p>
<ul>
<li>Ingestion：限流/熔断、超时与并发配置、全链路日志与 SLO</li>
<li>Worker：批处理参数自适应、重试与死信队列、消费延迟告警</li>
<li>ClickHouse：分区/排序键检查、物化视图落地 P95/P99 聚合</li>
</ul>
<p>M2 可观测性与治理（2-3 周）</p>
<ul>
<li>入口/消费/写入全链路指标与告警（QPS/429/积压/延迟）</li>
<li>事件 schema 注册与校验、维度白名单与高基数采样</li>
<li>数据生命周期：TTL、冷热分层、自动归档</li>
</ul>
<p>M3 规模化（3-4 周）</p>
<ul>
<li>Kafka 替代 Redis Streams、Worker 水平扩展与分组</li>
<li>多租户与限额管理（按游戏/环境/渠道）</li>
<li>预置看板模板与 A/B 评估组件</li>
</ul>
</div></template>


