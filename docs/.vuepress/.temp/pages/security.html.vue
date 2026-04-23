<template><div><h1 id="security" tabindex="-1"><a class="header-anchor" href="#security"><span>Security</span></a></h1>
<ul>
<li>mTLS for Server/Agent</li>
<li>OIDC/MFA for users</li>
<li>RBAC/ABAC, approvals, audit log chain</li>
</ul>
<p>Approvals (Two-person rule)</p>
<ul>
<li>Enable: set <code v-pre>auth.two_person_rule: true</code> in function descriptor; HTTP 提交将返回 <code v-pre>202</code> 和 <code v-pre>approval_id</code>。</li>
<li>Storage:
<ul>
<li>Memory: 默认（不提供 DATABASE_URL）</li>
<li>PostgreSQL: 设置 <code v-pre>DATABASE_URL=postgres://...</code>，二进制以 <code v-pre>-tags pg</code> 构建；表结构参见 <code v-pre>database/schema.sql</code> 中 <code v-pre>approvals</code>。</li>
<li>SQLite (可选): 设置 <code v-pre>DATABASE_URL=sqlite:///path/to/croupier.db</code> 或 <code v-pre>file:/path/to/croupier.db</code>，二进制以 <code v-pre>-tags sqlite</code> 构建；首次启动会自动建表。</li>
</ul>
</li>
<li>API：
<ul>
<li>列表：<code v-pre>GET /api/approvals?state=pending&amp;function_id=&amp;game_id=&amp;env=&amp;actor=&amp;mode=&amp;page=1&amp;size=20&amp;sort=created_at_desc</code>
<ul>
<li>返回：<code v-pre>{ approvals: [...], total, page, size }</code></li>
</ul>
</li>
<li>详情：<code v-pre>GET /api/approvals/get?id=...</code>（含 <code v-pre>payload_preview</code> 脱敏快照）</li>
<li>同意：<code v-pre>POST /api/approvals/approve</code>，body <code v-pre>{ &quot;id&quot;: &quot;...&quot; }</code>（同意后立即执行原调用并返回结果/Job）</li>
<li>拒绝：<code v-pre>POST /api/approvals/reject</code>，body <code v-pre>{ &quot;id&quot;: &quot;...&quot;, &quot;reason&quot;: &quot;...&quot; }</code></li>
</ul>
</li>
<li>审计：<code v-pre>approval_approve</code>/<code v-pre>approval_reject</code> 事件记录在审计链中；调用审计包含 <code v-pre>trace_id</code> 与脱敏快照。</li>
</ul>
<p>Notes</p>
<ul>
<li>UI 审批页已提供（/gm/approvals）：待办列表（分页/筛选）→ 详情侧栏 → 同意/拒绝；对高危函数已支持二次确认与 MFA（OTP）。</li>
<li>生产建议：优先 PostgreSQL，并为 approvals 表添加备份策略与告警（待办积压/拒绝率异常）。SQLite 适用于单机/PoC/嵌入式部署。</li>
</ul>
<p>RBAC/ABAC</p>
<ul>
<li>RBAC：基于角色/用户的 permission 检查，支持 game 作用域（<code v-pre>game:&lt;game_id&gt;:permission</code>）。</li>
<li>ABAC（简易表达式）：在函数描述 <code v-pre>auth.allow_if</code> 中配置表达式（==、!=、&amp;&amp;、||、has_role('admin')）
<ul>
<li>可用变量：<code v-pre>user</code>、<code v-pre>game_id</code>、<code v-pre>env</code>、<code v-pre>function_id</code></li>
<li>示例：<code v-pre>env == &quot;prod&quot; &amp;&amp; has_role('admin')</code></li>
</ul>
</li>
</ul>
<p>Rate limit &amp; Concurrency</p>
<ul>
<li>在函数描述 <code v-pre>semantics.rate_limit</code>（例如 <code v-pre>10rps</code>）与 <code v-pre>semantics.concurrency</code>（整数）启用限流/并发限制。</li>
<li>触达限制时返回 HTTP 429。</li>
</ul>
</div></template>


