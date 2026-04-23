<template><div><h1 id="游戏后台角色权限体系设计" tabindex="-1"><a class="header-anchor" href="#游戏后台角色权限体系设计"><span>游戏后台角色权限体系设计</span></a></h1>
<h2 id="角色层级架构" tabindex="-1"><a class="header-anchor" href="#角色层级架构"><span>角色层级架构</span></a></h2>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">超级管理员 (super_admin)</span>
<span class="line">├── 系统管理员 (admin)</span>
<span class="line">├── 开发人员 (developer)</span>
<span class="line">├── 测试人员 (tester)</span>
<span class="line">├── 运维人员 (ops)</span>
<span class="line">├── 数据分析师 (analyst)</span>
<span class="line">└── 客服人员 (support)</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="权限域说明" tabindex="-1"><a class="header-anchor" href="#权限域说明"><span>权限域说明</span></a></h2>
<table>
<thead>
<tr>
<th>权限域</th>
<th>说明</th>
<th>示例权限</th>
</tr>
</thead>
<tbody>
<tr>
<td><code v-pre>system:*</code></td>
<td>系统级操作</td>
<td><code v-pre>system:config</code>, <code v-pre>system:restart</code>, <code v-pre>system:backup</code></td>
</tr>
<tr>
<td><code v-pre>user:*</code></td>
<td>用户管理</td>
<td><code v-pre>user:create</code>, <code v-pre>user:update</code>, <code v-pre>user:delete</code>, <code v-pre>user:view</code></td>
</tr>
<tr>
<td><code v-pre>game:*</code></td>
<td>游戏配置管理</td>
<td><code v-pre>game:config</code>, <code v-pre>game:deploy</code>, <code v-pre>game:restart</code></td>
</tr>
<tr>
<td><code v-pre>player:*</code></td>
<td>玩家管理</td>
<td><code v-pre>player:query</code>, <code v-pre>player:update</code>, <code v-pre>player:ban</code></td>
</tr>
<tr>
<td><code v-pre>function:*</code></td>
<td>函数管理</td>
<td><code v-pre>function:register</code>, <code v-pre>function:deploy</code>, <code v-pre>function:test</code></td>
</tr>
<tr>
<td><code v-pre>job:*</code></td>
<td>任务管理</td>
<td><code v-pre>job:create</code>, <code v-pre>job:view</code>, <code v-pre>job:cancel</code>, <code v-pre>job:retry</code></td>
</tr>
<tr>
<td><code v-pre>audit:*</code></td>
<td>审计查看</td>
<td><code v-pre>audit:view</code>, <code v-pre>audit:export</code></td>
</tr>
<tr>
<td><code v-pre>monitor:*</code></td>
<td>监控数据</td>
<td><code v-pre>monitor:view</code>, <code v-pre>monitor:alert</code></td>
</tr>
<tr>
<td><code v-pre>data:*</code></td>
<td>数据分析</td>
<td><code v-pre>data:query</code>, <code v-pre>data:export</code>, <code v-pre>data:report</code></td>
</tr>
<tr>
<td><code v-pre>support:*</code></td>
<td>客服功能</td>
<td><code v-pre>support:ticket</code>, <code v-pre>support:chat</code></td>
</tr>
</tbody>
</table>
<h2 id="角色详细权限" tabindex="-1"><a class="header-anchor" href="#角色详细权限"><span>角色详细权限</span></a></h2>
<h3 id="_1-超级管理员-super-admin" tabindex="-1"><a class="header-anchor" href="#_1-超级管理员-super-admin"><span>1. 超级管理员 (super_admin)</span></a></h3>
<p><strong>权限</strong>：<code v-pre>[&quot;*&quot;]</code> - 所有权限</p>
<p><strong>职责</strong>：</p>
<ul>
<li>系统最高权限持有者</li>
<li>紧急情况下的故障恢复</li>
<li>重要配置变更的最终审批</li>
<li>新角色和权限的创建</li>
</ul>
<p><strong>使用场景</strong>：</p>
<ul>
<li>系统初始化配置</li>
<li>重大故障处理</li>
<li>安全事件响应</li>
</ul>
<h3 id="_2-系统管理员-admin" tabindex="-1"><a class="header-anchor" href="#_2-系统管理员-admin"><span>2. 系统管理员 (admin)</span></a></h3>
<p><strong>权限</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">[</span></span>
<span class="line">  <span class="token string">"system:config"</span><span class="token punctuation">,</span> <span class="token string">"system:restart"</span><span class="token punctuation">,</span> <span class="token string">"system:backup"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"user:*"</span><span class="token punctuation">,</span> <span class="token string">"game:*"</span><span class="token punctuation">,</span> <span class="token string">"function:*"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"audit:view"</span><span class="token punctuation">,</span> <span class="token string">"monitor:view"</span></span>
<span class="line"><span class="token punctuation">]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>职责</strong>：</p>
<ul>
<li>日常系统管理和维护</li>
<li>用户账号和权限管理</li>
<li>游戏环境配置管理</li>
<li>系统备份和恢复</li>
</ul>
<p><strong>使用场景</strong>：</p>
<ul>
<li>新用户账号创建</li>
<li>角色权限分配调整</li>
<li>游戏配置更新</li>
<li>定期系统维护</li>
</ul>
<h3 id="_3-开发人员-developer" tabindex="-1"><a class="header-anchor" href="#_3-开发人员-developer"><span>3. 开发人员 (developer)</span></a></h3>
<p><strong>权限</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">[</span></span>
<span class="line">  <span class="token string">"function:register"</span><span class="token punctuation">,</span> <span class="token string">"function:update"</span><span class="token punctuation">,</span> <span class="token string">"function:test"</span><span class="token punctuation">,</span> <span class="token string">"function:view"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"job:create"</span><span class="token punctuation">,</span> <span class="token string">"job:view"</span><span class="token punctuation">,</span> <span class="token string">"job:cancel"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"game:config:read"</span><span class="token punctuation">,</span> <span class="token string">"player:query"</span><span class="token punctuation">,</span> <span class="token string">"monitor:view"</span></span>
<span class="line"><span class="token punctuation">]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>职责</strong>：</p>
<ul>
<li>新功能开发和部署</li>
<li>功能测试和调试</li>
<li>代码质量保证</li>
<li>技术文档编写</li>
</ul>
<p><strong>使用场景</strong>：</p>
<ul>
<li>注册新的游戏功能函数</li>
<li>测试功能逻辑和性能</li>
<li>查看玩家数据进行调试</li>
<li>监控功能运行状态</li>
</ul>
<h3 id="_4-测试人员-tester" tabindex="-1"><a class="header-anchor" href="#_4-测试人员-tester"><span>4. 测试人员 (tester)</span></a></h3>
<p><strong>权限</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">[</span></span>
<span class="line">  <span class="token string">"function:test"</span><span class="token punctuation">,</span> <span class="token string">"function:view"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"job:create"</span><span class="token punctuation">,</span> <span class="token string">"job:view"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"player:create:test"</span><span class="token punctuation">,</span> <span class="token string">"player:query"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"game:config:read"</span></span>
<span class="line"><span class="token punctuation">]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>职责</strong>：</p>
<ul>
<li>功能测试执行</li>
<li>Bug验证和报告</li>
<li>测试用例设计</li>
<li>质量保证流程</li>
</ul>
<p><strong>使用场景</strong>：</p>
<ul>
<li>执行功能测试用例</li>
<li>创建测试用户和数据</li>
<li>验证Bug修复结果</li>
<li>性能和压力测试</li>
</ul>
<h3 id="_5-运维人员-ops" tabindex="-1"><a class="header-anchor" href="#_5-运维人员-ops"><span>5. 运维人员 (ops)</span></a></h3>
<p><strong>权限</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">[</span></span>
<span class="line">  <span class="token string">"system:monitor"</span><span class="token punctuation">,</span> <span class="token string">"system:restart"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"job:view"</span><span class="token punctuation">,</span> <span class="token string">"job:cancel"</span><span class="token punctuation">,</span> <span class="token string">"job:retry"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"monitor:*"</span><span class="token punctuation">,</span> <span class="token string">"audit:view"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"function:deploy"</span><span class="token punctuation">,</span> <span class="token string">"game:deploy"</span></span>
<span class="line"><span class="token punctuation">]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>职责</strong>：</p>
<ul>
<li>系统运行状态监控</li>
<li>故障诊断和处理</li>
<li>性能优化和调优</li>
<li>部署和发布管理</li>
</ul>
<p><strong>使用场景</strong>：</p>
<ul>
<li>监控系统性能指标</li>
<li>处理系统告警和故障</li>
<li>执行功能发布部署</li>
<li>故障恢复和回滚</li>
</ul>
<h3 id="_6-数据分析师-analyst" tabindex="-1"><a class="header-anchor" href="#_6-数据分析师-analyst"><span>6. 数据分析师 (analyst)</span></a></h3>
<p><strong>权限</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">[</span></span>
<span class="line">  <span class="token string">"data:*"</span><span class="token punctuation">,</span> <span class="token string">"player:query"</span><span class="token punctuation">,</span> <span class="token string">"player:export"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"audit:view"</span><span class="token punctuation">,</span> <span class="token string">"monitor:view"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"job:create:readonly"</span></span>
<span class="line"><span class="token punctuation">]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>职责</strong>：</p>
<ul>
<li>游戏数据分析和挖掘</li>
<li>用户行为分析</li>
<li>运营数据报告</li>
<li>业务指标监控</li>
</ul>
<p><strong>使用场景</strong>：</p>
<ul>
<li>生成日常运营报告</li>
<li>分析玩家行为模式</li>
<li>导出数据进行深度分析</li>
<li>制作数据可视化图表</li>
</ul>
<h3 id="_7-客服人员-support" tabindex="-1"><a class="header-anchor" href="#_7-客服人员-support"><span>7. 客服人员 (support)</span></a></h3>
<p><strong>权限</strong>：</p>
<div class="language-json line-numbers-mode" data-highlighter="prismjs" data-ext="json"><pre v-pre><code class="language-json"><span class="line"><span class="token punctuation">[</span></span>
<span class="line">  <span class="token string">"player:query"</span><span class="token punctuation">,</span> <span class="token string">"player:update:basic"</span><span class="token punctuation">,</span></span>
<span class="line">  <span class="token string">"support:ticket"</span><span class="token punctuation">,</span> <span class="token string">"audit:view:player"</span></span>
<span class="line"><span class="token punctuation">]</span></span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p><strong>职责</strong>：</p>
<ul>
<li>玩家问题处理</li>
<li>客服工单管理</li>
<li>玩家信息查询和更新</li>
<li>客服质量保证</li>
</ul>
<p><strong>使用场景</strong>：</p>
<ul>
<li>查询玩家账号信息</li>
<li>处理玩家投诉和建议</li>
<li>更新玩家基础信息</li>
<li>查看玩家操作历史</li>
</ul>
<h2 id="权限控制机制" tabindex="-1"><a class="header-anchor" href="#权限控制机制"><span>权限控制机制</span></a></h2>
<h3 id="_1-基于角色的权限继承" tabindex="-1"><a class="header-anchor" href="#_1-基于角色的权限继承"><span>1. 基于角色的权限继承</span></a></h3>
<ul>
<li>用户通过角色获得权限</li>
<li>支持多角色分配</li>
<li>角色权限可以动态调整</li>
</ul>
<h3 id="_2-权限检查流程" tabindex="-1"><a class="header-anchor" href="#_2-权限检查流程"><span>2. 权限检查流程</span></a></h3>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">请求 → JWT认证 → 提取用户角色 → RBAC权限检查 → 业务逻辑执行</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div></div></div><h3 id="_3-审计和监控" tabindex="-1"><a class="header-anchor" href="#_3-审计和监控"><span>3. 审计和监控</span></a></h3>
<ul>
<li>所有权限操作都会记录审计日志</li>
<li>敏感操作支持双人审批机制</li>
<li>实时权限使用情况监控</li>
</ul>
<h2 id="配置文件说明" tabindex="-1"><a class="header-anchor" href="#配置文件说明"><span>配置文件说明</span></a></h2>
<h3 id="rbac配置-configs-rbac-game-roles-json" tabindex="-1"><a class="header-anchor" href="#rbac配置-configs-rbac-game-roles-json"><span>RBAC配置 (<code v-pre>configs/rbac.game-roles.json</code>)</span></a></h3>
<p>定义了所有角色及其对应的权限列表。</p>
<h3 id="用户配置-configs-users-game-roles-json" tabindex="-1"><a class="header-anchor" href="#用户配置-configs-users-game-roles-json"><span>用户配置 (<code v-pre>configs/users.game-roles.json</code>)</span></a></h3>
<p>定义了预设的用户账号及其角色分配。</p>
<h3 id="使用方法" tabindex="-1"><a class="header-anchor" href="#使用方法"><span>使用方法</span></a></h3>
<ol>
<li>将配置文件复制到相应位置</li>
<li>根据实际需要调整权限配置</li>
<li>重启服务生效</li>
</ol>
<h2 id="安全建议" tabindex="-1"><a class="header-anchor" href="#安全建议"><span>安全建议</span></a></h2>
<ol>
<li><strong>最小权限原则</strong>：只分配完成工作所需的最小权限</li>
<li><strong>定期权限审查</strong>：定期检查和清理不必要的权限</li>
<li><strong>强密码策略</strong>：所有账号使用强密码和双因子认证</li>
<li><strong>权限分离</strong>：避免单人拥有过多权限，实施职责分离</li>
<li><strong>审计监控</strong>：建立完善的权限使用审计和监控机制</li>
</ol>
</div></template>


