<template><div><h1 id="游戏数据分析系统文档" tabindex="-1"><a class="header-anchor" href="#游戏数据分析系统文档"><span>游戏数据分析系统文档</span></a></h1>
<p>欢迎使用 Croupier 游戏数据分析系统！本文档将帮助您了解如何构建现代化的游戏数据分析平台。</p>
<h2 id="📚-文档导航" tabindex="-1"><a class="header-anchor" href="#📚-文档导航"><span>📚 文档导航</span></a></h2>
<h3 id="🎯-核心概念" tabindex="-1"><a class="header-anchor" href="#🎯-核心概念"><span>🎯 核心概念</span></a></h3>
<ul>
<li><RouteLink to="/analytics/game-metrics-overview.html">游戏指标全景图</RouteLink> - 完整的游戏数据指标体系</li>
<li><RouteLink to="/analytics/data-collection-architecture.html">数据采集架构</RouteLink> - 客户端vs服务器端采集策略</li>
<li><RouteLink to="/analytics/game-type-adaptation.html">游戏类型适配</RouteLink> - 不同游戏类型的指标选择</li>
</ul>
<h3 id="🛠️-技术实施" tabindex="-1"><a class="header-anchor" href="#🛠️-技术实施"><span>🛠️ 技术实施</span></a></h3>
<ul>
<li><RouteLink to="/analytics/opentelemetry-integration.html">OpenTelemetry集成指南</RouteLink> - 现代化遥测解决方案</li>
<li><RouteLink to="/analytics/current-system-analysis.html">当前系统分析</RouteLink> - 现有Analytics系统详解</li>
<li><RouteLink to="/analytics/enhancement-plan.html">系统增强方案</RouteLink> - 完整的系统升级路线图</li>
<li><RouteLink to="/analytics/clickhouse-schema.html">ClickHouse 表结构</RouteLink> - 事件/支付与按天聚合表设计</li>
</ul>
<h3 id="🚀-实施指南" tabindex="-1"><a class="header-anchor" href="#🚀-实施指南"><span>🚀 实施指南</span></a></h3>
<ul>
<li><RouteLink to="/analytics/quick-start.html">快速开始</RouteLink> - 5分钟上手指南</li>
<li><RouteLink to="/analytics/best-practices.html">最佳实践</RouteLink> - 生产环境部署建议</li>
<li><RouteLink to="/analytics/troubleshooting.html">故障排除</RouteLink> - 常见问题解决方案</li>
</ul>
<h3 id="📊-指标参考" tabindex="-1"><a class="header-anchor" href="#📊-指标参考"><span>📊 指标参考</span></a></h3>
<ul>
<li><RouteLink to="/analytics/metrics-dictionary.html">指标词典</RouteLink> - 所有指标的详细定义</li>
<li><RouteLink to="/analytics/api-reference.html">API参考</RouteLink> - 完整的API文档</li>
<li><RouteLink to="/analytics/sdk-reference.html">SDK参考</RouteLink> - 多语言SDK使用指南</li>
</ul>
<h2 id="🎮-支持的游戏类型" tabindex="-1"><a class="header-anchor" href="#🎮-支持的游戏类型"><span>🎮 支持的游戏类型</span></a></h2>
<table>
<thead>
<tr>
<th>游戏类型</th>
<th>核心指标</th>
<th>特殊需求</th>
<th>文档链接</th>
</tr>
</thead>
<tbody>
<tr>
<td><strong>休闲游戏</strong></td>
<td>留存率、广告收益、Session时长</td>
<td>碎片化指标、上手难度</td>
<td><RouteLink to="/analytics/casual-games.html">休闲游戏指标</RouteLink></td>
</tr>
<tr>
<td><strong>重度RPG</strong></td>
<td>成长体系、付费深度、社交粘性</td>
<td>养成投入度、数值平衡</td>
<td><RouteLink to="/analytics/rpg-games.html">RPG游戏指标</RouteLink></td>
</tr>
<tr>
<td><strong>竞技游戏</strong></td>
<td>竞技平衡、技能成长、匹配质量</td>
<td>作弊检测、网络体验</td>
<td><RouteLink to="/analytics/competitive-games.html">竞技游戏指标</RouteLink></td>
</tr>
<tr>
<td><strong>策略游戏</strong></td>
<td>策略深度、时间投入、META演化</td>
<td>决策分析、学习曲线</td>
<td><RouteLink to="/analytics/strategy-games.html">策略游戏指标</RouteLink></td>
</tr>
</tbody>
</table>
<h2 id="🔧-技术架构" tabindex="-1"><a class="header-anchor" href="#🔧-技术架构"><span>🔧 技术架构</span></a></h2>
<Mermaid code="eJx1kd1KAkEcxe97iqFbkXmBCEzDr8rNFoSWLjabNnHbjd31Iuim8GOzxA2FsiIyEoQyCSpFzadxZvXKV0hnTc1w7s5w/uf85j+Cwh8dAHZlAQyOGtu19GIvlerdJfH7Wb+l48Sr+X0FXevbwAZwMjEQ/db5Ip0YHgdH6nWiZ/HbE9Fr5kt1B9jty8DLOSRePNYiYRV4JQGpWkSWlnYVuOxhWQb6tgIbNrPSxkbG1isY5PN0Z5zoHCWS+wxOF3GhbCW6uACLRLDl8kOHgCTNGkDS3sIMPX6Om8YvfbZqNuPY0P8wu2jgqhXolEURhTVZoXQBdo2BQpBxngw5J1BeOhLk1jeBHQTRXkSFfn4/yk8cq5ZjrINUh6b2EJKVKLJqSD1BPq5h9/R2wAZxooQbufkPqtzgs/LoQf9/IESLPJxTjISjHjmmIlrRaVx0ml+jim6xPEvKcIwiHyLtAMVUi+kyRR5Tszafm/PxSEAKZNHhkUydvVy7W6vO523kzfyQd8LooVl+DutJ8mCQdGkABN0Kv89LUytkLNdY+9xTF8OiH323/ts="></Mermaid><h2 id="🌟-核心优势" tabindex="-1"><a class="header-anchor" href="#🌟-核心优势"><span>🌟 核心优势</span></a></h2>
<h3 id="标准化数据模型" tabindex="-1"><a class="header-anchor" href="#标准化数据模型"><span>标准化数据模型</span></a></h3>
<ul>
<li>基于 OpenTelemetry 行业标准</li>
<li>统一的 Traces、Metrics、Logs 数据格式</li>
<li>多语言SDK支持 (Go/Java/C++/JavaScript/Python)</li>
</ul>
<h3 id="灵活的部署方式" tabindex="-1"><a class="header-anchor" href="#灵活的部署方式"><span>灵活的部署方式</span></a></h3>
<ul>
<li>云原生架构，支持容器化部署</li>
<li>多种后端存储选择 (ClickHouse/Prometheus/Elasticsearch)</li>
<li>水平扩展，支持高并发数据处理</li>
</ul>
<h3 id="游戏业务深度优化" tabindex="-1"><a class="header-anchor" href="#游戏业务深度优化"><span>游戏业务深度优化</span></a></h3>
<ul>
<li>游戏特有的语义标准化</li>
<li>完整的用户行为链路追踪</li>
<li>实时性能监控和异常检测</li>
</ul>
<h2 id="📈-业务价值" tabindex="-1"><a class="header-anchor" href="#📈-业务价值"><span>📈 业务价值</span></a></h2>
<h3 id="运营优化" tabindex="-1"><a class="header-anchor" href="#运营优化"><span>运营优化</span></a></h3>
<ul>
<li><strong>用户获取成本降低 15-25%</strong> - 精准渠道投放分析</li>
<li><strong>用户留存提升 10-15%</strong> - 流失预警和干预</li>
<li><strong>活动ROI提升 30-50%</strong> - 数据驱动的活动优化</li>
</ul>
<h3 id="技术提升" tabindex="-1"><a class="header-anchor" href="#技术提升"><span>技术提升</span></a></h3>
<ul>
<li><strong>开发效率提升 70%</strong> - 标准化SDK和工具链</li>
<li><strong>故障定位速度提升 80%</strong> - 分布式链路追踪</li>
<li><strong>运维成本降低 50%</strong> - 自动化监控和告警</li>
</ul>
<h2 id="🌐-公网采集入口-analytics-ingestion" tabindex="-1"><a class="header-anchor" href="#🌐-公网采集入口-analytics-ingestion"><span>🌐 公网采集入口（Analytics Ingestion）</span></a></h2>
<p>面向“游戏客户端”的事件上报，推荐使用独立的 Analytics Ingestion 服务（与控制面解耦），具备如下特性：</p>
<ul>
<li>安全：前置 CDN/WAF、基于 HMAC-SHA256 的签名校验、时间戳+nonce 防重放、按游戏/环境限流</li>
<li>可靠：写入 MQ（Redis Streams/Kafka），消费失败可重放；支持 MaxLen 背压</li>
<li>灵活：完全业务事件模型（login/progression/purchase…），字段白名单与灰度开关</li>
</ul>
<p>最小化落地示例（已在本仓库提供 <code v-pre>services/ingest</code>）：</p>
<ul>
<li>端点：
<ul>
<li>POST /api/ingest/events（通用事件数组）</li>
<li>POST /api/ingest/payments（支付事件数组）</li>
</ul>
</li>
<li>认证与防重放（请求头）：
<ul>
<li>X-Timestamp：Unix 秒；与服务器允许的时间偏差默认 300s</li>
<li>X-Nonce：客户端生成随机字符串</li>
<li>X-Signature：Base64(HMAC-SHA256(secret, <code v-pre>${timestamp}\n${nonce}\n${sha256(body)}</code>))</li>
<li>共享密钥通过环境变量 <code v-pre>ANALYTICS_INGEST_SECRET</code> 配置</li>
</ul>
</li>
<li>环境变量：
<ul>
<li>ANALYTICS_MQ_TYPE=redis</li>
<li>REDIS_URL=redis://user:pass@host:6379/0</li>
<li>ANALYTICS_REDIS_STREAM_EVENTS=analytics:events</li>
<li>ANALYTICS_REDIS_STREAM_PAYMENTS=analytics:payments</li>
<li>ANALYTICS_INGEST_SECRET=your-secret</li>
<li>ANALYTICS_INGEST_SKEW=300（可选，允许时间偏差秒）</li>
</ul>
</li>
</ul>
<p>与 OTel Collector 的关系：</p>
<ul>
<li>客户端若不便引入 OTLP，可直接用 Ingestion（HTTP/JSON）</li>
<li>服务端/Agent 的 traces/metrics 建议走 OTel Collector；业务事件可通过 SDK/Bridge 写 MQ</li>
<li>二者可并存，最终在 ClickHouse/Grafana 汇聚</li>
</ul>
<h2 id="🚀-快速开始" tabindex="-1"><a class="header-anchor" href="#🚀-快速开始"><span>🚀 快速开始</span></a></h2>
<ol>
<li>
<p><strong>环境准备</strong></p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 克隆项目</span></span>
<span class="line"><span class="token function">git</span> clone https://github.com/cuihairu/croupier.git</span>
<span class="line"><span class="token builtin class-name">cd</span> croupier</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 启动基础服务</span></span>
<span class="line"><span class="token function">docker-compose</span> up <span class="token parameter variable">-d</span> clickhouse redis</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div></li>
<li>
<p><strong>启动分析服务</strong></p>
<div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre v-pre><code class="language-bash"><span class="line"><span class="token comment"># 编译并启动 server (go-zero)</span></span>
<span class="line"><span class="token function">make</span> server</span>
<span class="line">./bin/croupier-server <span class="token parameter variable">-f</span> services/server/etc/server.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 启动 Analytics Ingest 服务 (事件采集)</span></span>
<span class="line"><span class="token function">make</span> ingest</span>
<span class="line">./bin/croupier-ingest <span class="token parameter variable">-f</span> services/ingest/etc/ingest.yaml</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 启动 Analytics Worker (消费处理)</span></span>
<span class="line"><span class="token function">make</span> worker</span>
<span class="line">./bin/croupier-worker</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div></li>
<li>
<p><strong>查看分析面板</strong></p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">打开浏览器访问: http://localhost:8080/analytics</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div></li>
</ol>
<h2 id="📞-获取支持" tabindex="-1"><a class="header-anchor" href="#📞-获取支持"><span>📞 获取支持</span></a></h2>
<ul>
<li><strong>文档问题</strong>: <a href="https://github.com/cuihairu/croupier/issues" target="_blank" rel="noopener noreferrer">文档Issue</a></li>
<li><strong>技术讨论</strong>: <a href="https://github.com/cuihairu/croupier/discussions" target="_blank" rel="noopener noreferrer">GitHub Discussions</a></li>
<li><strong>Bug报告</strong>: <a href="https://github.com/cuihairu/croupier/issues/new?template=bug_report.md" target="_blank" rel="noopener noreferrer">Bug Report Template</a></li>
</ul>
<hr>
<blockquote>
<p>💡 <strong>提示</strong>: 建议先阅读<RouteLink to="/analytics/game-metrics-overview.html">游戏指标全景图</RouteLink>了解整体框架，再根据您的游戏类型选择对应的实施方案。</p>
</blockquote>
</div></template>


