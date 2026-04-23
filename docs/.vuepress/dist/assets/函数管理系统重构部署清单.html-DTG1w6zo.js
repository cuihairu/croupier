import{_ as n,c as a,a as e,o as i}from"./app-C_dHcy8Q.js";const l={};function p(c,s){return i(),a("div",null,[...s[0]||(s[0]=[e(`<h1 id="croupier-函数管理系统重构部署清单" tabindex="-1"><a class="header-anchor" href="#croupier-函数管理系统重构部署清单"><span>Croupier 函数管理系统重构部署清单</span></a></h1><h2 id="📋-部署前检查清单" tabindex="-1"><a class="header-anchor" href="#📋-部署前检查清单"><span>📋 部署前检查清单</span></a></h2><h3 id="✅-代码准备" tabindex="-1"><a class="header-anchor" href="#✅-代码准备"><span>✅ 代码准备</span></a></h3><ul><li>[x] 所有组件开发完成</li><li>[x] 单元测试编写完成</li><li>[x] 文档和示例完成</li><li>[x] 类型定义完整</li><li>[x] 向后兼容性验证</li></ul><h3 id="✅-环境准备" tabindex="-1"><a class="header-anchor" href="#✅-环境准备"><span>✅ 环境准备</span></a></h3><ul><li>[ ] 开发环境测试通过</li><li>[ ] 测试环境部署验证</li><li>[ ] 生产环境配置准备</li><li>[ ] 数据库备份完成</li><li>[ ] 回滚方案准备</li></ul><h2 id="🚀-部署步骤" tabindex="-1"><a class="header-anchor" href="#🚀-部署步骤"><span>🚀 部署步骤</span></a></h2><h3 id="第一阶段-基础设施部署-预计30分钟" tabindex="-1"><a class="header-anchor" href="#第一阶段-基础设施部署-预计30分钟"><span>第一阶段：基础设施部署（预计30分钟）</span></a></h3><h4 id="_1-1-文件部署" tabindex="-1"><a class="header-anchor" href="#_1-1-文件部署"><span>1.1 文件部署</span></a></h4><div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 备份现有文件</span></span>
<span class="line"><span class="token function">cp</span> <span class="token parameter variable">-r</span> dashboard/src/pages/GmFunctions dashboard/src/pages/GmFunctions.backup</span>
<span class="line"><span class="token function">cp</span> dashboard/config/routes.ts dashboard/config/routes.ts.backup</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 部署新组件</span></span>
<span class="line"><span class="token function">mkdir</span> <span class="token parameter variable">-p</span> dashboard/src/components/FunctionComponents</span>
<span class="line"><span class="token function">mkdir</span> <span class="token parameter variable">-p</span> dashboard/src/components/FunctionComponents/utils</span>
<span class="line"><span class="token function">mkdir</span> <span class="token parameter variable">-p</span> dashboard/src/components/FunctionComponents/__tests__</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 复制所有新组件文件</span></span>
<span class="line"><span class="token comment"># (需要手动复制或使用部署脚本)</span></span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="_1-2-依赖检查" tabindex="-1"><a class="header-anchor" href="#_1-2-依赖检查"><span>1.2 依赖检查</span></a></h4><div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre><code class="language-bash"><span class="line"><span class="token comment"># 检查package.json依赖</span></span>
<span class="line"><span class="token function">npm</span> list @ant-design/pro-components</span>
<span class="line"><span class="token function">npm</span> <span class="token function">install</span> @ant-design/pro-components@latest <span class="token parameter variable">--save</span></span>
<span class="line"></span>
<span class="line"><span class="token comment"># 检查TypeScript配置</span></span>
<span class="line"><span class="token function">npm</span> run type-check</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="_1-3-路由配置更新" tabindex="-1"><a class="header-anchor" href="#_1-3-路由配置更新"><span>1.3 路由配置更新</span></a></h4><div class="language-typescript line-numbers-mode" data-highlighter="prismjs" data-ext="ts"><pre><code class="language-typescript"><span class="line"><span class="token comment">// 验证 config/routes.ts 已更新</span></span>
<span class="line"><span class="token comment">// 确认新的函数管理菜单结构</span></span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="第二阶段-菜单和国际化-预计15分钟" tabindex="-1"><a class="header-anchor" href="#第二阶段-菜单和国际化-预计15分钟"><span>第二阶段：菜单和国际化（预计15分钟）</span></a></h3><h4 id="_2-1-菜单配置验证" tabindex="-1"><a class="header-anchor" href="#_2-1-菜单配置验证"><span>2.1 菜单配置验证</span></a></h4><div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre><code class="language-bash"><span class="line"><span class="token comment"># 检查中文菜单配置</span></span>
<span class="line"><span class="token function">grep</span> <span class="token parameter variable">-n</span> <span class="token string">&quot;FunctionManagement&quot;</span> dashboard/src/locales/zh-CN/menu.ts</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 检查英文菜单配置</span></span>
<span class="line"><span class="token function">grep</span> <span class="token parameter variable">-n</span> <span class="token string">&quot;FunctionManagement&quot;</span> dashboard/src/locales/en-US/menu.ts</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="_2-2-权限配置检查" tabindex="-1"><a class="header-anchor" href="#_2-2-权限配置检查"><span>2.2 权限配置检查</span></a></h4><div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre><code class="language-bash"><span class="line"><span class="token comment"># 确认权限配置正确</span></span>
<span class="line"><span class="token function">grep</span> <span class="token parameter variable">-n</span> <span class="token string">&quot;canFunctionsRead&quot;</span> dashboard/config/routes.ts</span>
<span class="line"><span class="token function">grep</span> <span class="token parameter variable">-n</span> <span class="token string">&quot;canAssignmentsRead&quot;</span> dashboard/config/routes.ts</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="第三阶段-页面部署-预计45分钟" tabindex="-1"><a class="header-anchor" href="#第三阶段-页面部署-预计45分钟"><span>第三阶段：页面部署（预计45分钟）</span></a></h3><h4 id="_3-1-新页面部署" tabindex="-1"><a class="header-anchor" href="#_3-1-新页面部署"><span>3.1 新页面部署</span></a></h4><div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre><code class="language-bash"><span class="line"><span class="token comment"># 创建新页面目录</span></span>
<span class="line"><span class="token function">mkdir</span> <span class="token parameter variable">-p</span> dashboard/src/pages/Functions/Directory</span>
<span class="line"><span class="token function">mkdir</span> <span class="token parameter variable">-p</span> dashboard/src/pages/Functions/Instances</span>
<span class="line"><span class="token function">mkdir</span> <span class="token parameter variable">-p</span> dashboard/src/pages/Functions/Invoke</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 部署页面文件</span></span>
<span class="line"><span class="token comment"># (需要手动复制对应的index.tsx文件)</span></span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="_3-2-组件集成验证" tabindex="-1"><a class="header-anchor" href="#_3-2-组件集成验证"><span>3.2 组件集成验证</span></a></h4><div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre><code class="language-bash"><span class="line"><span class="token comment"># 检查组件导入</span></span>
<span class="line"><span class="token function">grep</span> <span class="token parameter variable">-n</span> <span class="token string">&quot;FunctionComponents&quot;</span> dashboard/src/pages/Functions/*/index.tsx</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 验证API服务集成</span></span>
<span class="line"><span class="token function">grep</span> <span class="token parameter variable">-n</span> <span class="token string">&quot;functions-enhanced&quot;</span> dashboard/src/pages/Functions/*/index.tsx</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="第四阶段-api后端支持-预计60分钟" tabindex="-1"><a class="header-anchor" href="#第四阶段-api后端支持-预计60分钟"><span>第四阶段：API后端支持（预计60分钟）</span></a></h3><h4 id="_4-1-新api端点检查" tabindex="-1"><a class="header-anchor" href="#_4-1-新api端点检查"><span>4.1 新API端点检查</span></a></h4><div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre><code class="language-bash"><span class="line"><span class="token comment"># 验证以下API端点已实现：</span></span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> GET http://localhost:8080/api/functions/summary</span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> GET http://localhost:8080/api/function_calls</span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> GET http://localhost:8080/api/function_instances</span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> GET http://localhost:8080/api/registry/services</span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-X</span> GET http://localhost:8080/api/coverage/analysis</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="_4-2-api权限配置" tabindex="-1"><a class="header-anchor" href="#_4-2-api权限配置"><span>4.2 API权限配置</span></a></h4><div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre><code class="language-bash"><span class="line"><span class="token comment"># 确认以下权限已配置：</span></span>
<span class="line"><span class="token comment"># functions:read</span></span>
<span class="line"><span class="token comment"># function_calls:read</span></span>
<span class="line"><span class="token comment"># function_instances:read</span></span>
<span class="line"><span class="token comment"># registry:read</span></span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="第五阶段-测试验证-预计30分钟" tabindex="-1"><a class="header-anchor" href="#第五阶段-测试验证-预计30分钟"><span>第五阶段：测试验证（预计30分钟）</span></a></h3><h4 id="_5-1-功能测试" tabindex="-1"><a class="header-anchor" href="#_5-1-功能测试"><span>5.1 功能测试</span></a></h4><ul><li>[ ] 函数目录页面正常显示</li><li>[ ] 函数调用功能正常工作</li><li>[ ] 实例管理页面数据正确</li><li>[ ] 调用历史记录完整</li><li>[ ] 权限控制生效</li></ul><h4 id="_5-2-兼容性测试" tabindex="-1"><a class="header-anchor" href="#_5-2-兼容性测试"><span>5.2 兼容性测试</span></a></h4><ul><li>[ ] 旧版本GmFunctions页面仍可访问</li><li>[ ] URL重定向正常工作</li><li>[ ] 数据格式兼容</li><li>[ ] API向后兼容</li></ul><h4 id="_5-3-性能测试" tabindex="-1"><a class="header-anchor" href="#_5-3-性能测试"><span>5.3 性能测试</span></a></h4><ul><li>[ ] 页面加载时间 &lt; 3秒</li><li>[ ] 搜索响应时间 &lt; 500ms</li><li>[ ] 内存使用正常</li><li>[ ] 无内存泄漏</li></ul><h2 id="🔧-配置文件更新" tabindex="-1"><a class="header-anchor" href="#🔧-配置文件更新"><span>🔧 配置文件更新</span></a></h2><h3 id="路由配置-config-routes-ts" tabindex="-1"><a class="header-anchor" href="#路由配置-config-routes-ts"><span>路由配置 (config/routes.ts)</span></a></h3><div class="language-typescript line-numbers-mode" data-highlighter="prismjs" data-ext="ts"><pre><code class="language-typescript"><span class="line"><span class="token comment">// 确保包含以下配置：</span></span>
<span class="line"><span class="token punctuation">{</span></span>
<span class="line">  path<span class="token operator">:</span> <span class="token string">&#39;/game/functions&#39;</span><span class="token punctuation">,</span></span>
<span class="line">  name<span class="token operator">:</span> <span class="token string">&#39;FunctionManagement&#39;</span><span class="token punctuation">,</span></span>
<span class="line">  access<span class="token operator">:</span> <span class="token string">&#39;canFunctionsRead&#39;</span><span class="token punctuation">,</span></span>
<span class="line">  routes<span class="token operator">:</span> <span class="token punctuation">[</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      path<span class="token operator">:</span> <span class="token string">&#39;/game/functions/catalog&#39;</span><span class="token punctuation">,</span></span>
<span class="line">      name<span class="token operator">:</span> <span class="token string">&#39;FunctionCatalog&#39;</span><span class="token punctuation">,</span></span>
<span class="line">      component<span class="token operator">:</span> <span class="token string">&#39;./Functions/Directory&#39;</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      path<span class="token operator">:</span> <span class="token string">&#39;/game/functions/invoke&#39;</span><span class="token punctuation">,</span></span>
<span class="line">      name<span class="token operator">:</span> <span class="token string">&#39;FunctionInvoke&#39;</span><span class="token punctuation">,</span></span>
<span class="line">      component<span class="token operator">:</span> <span class="token string">&#39;./GmFunctions&#39;</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">{</span></span>
<span class="line">      path<span class="token operator">:</span> <span class="token string">&#39;/game/functions/instances&#39;</span><span class="token punctuation">,</span></span>
<span class="line">      name<span class="token operator">:</span> <span class="token string">&#39;FunctionInstances&#39;</span><span class="token punctuation">,</span></span>
<span class="line">      component<span class="token operator">:</span> <span class="token string">&#39;./Functions/Instances&#39;</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token punctuation">}</span><span class="token punctuation">,</span></span>
<span class="line">    <span class="token comment">// ... 其他路由</span></span>
<span class="line">  <span class="token punctuation">]</span><span class="token punctuation">,</span></span>
<span class="line"><span class="token punctuation">}</span></span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="环境变量配置" tabindex="-1"><a class="header-anchor" href="#环境变量配置"><span>环境变量配置</span></a></h3><div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre><code class="language-bash"><span class="line"><span class="token comment"># 检查以下环境变量：</span></span>
<span class="line"><span class="token assign-left variable">CROUPIER_SERVER_URL</span><span class="token operator">=</span>http://localhost:8080</span>
<span class="line"><span class="token assign-left variable">CROUPIER_API_TIMEOUT</span><span class="token operator">=</span><span class="token number">30000</span></span>
<span class="line"><span class="token assign-left variable">CROUPIER_REFRESH_INTERVAL</span><span class="token operator">=</span><span class="token number">30000</span></span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="📊-监控指标" tabindex="-1"><a class="header-anchor" href="#📊-监控指标"><span>📊 监控指标</span></a></h2><h3 id="部署后验证指标" tabindex="-1"><a class="header-anchor" href="#部署后验证指标"><span>部署后验证指标</span></a></h3><ul><li>页面加载时间 &lt; 3秒</li><li>API响应时间 &lt; 500ms</li><li>错误率 &lt; 1%</li><li>用户成功率 &gt; 95%</li></ul><h3 id="监控检查点" tabindex="-1"><a class="header-anchor" href="#监控检查点"><span>监控检查点</span></a></h3><div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 检查服务健康状态</span></span>
<span class="line"><span class="token function">curl</span> <span class="token parameter variable">-f</span> http://localhost:8080/health</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 检查前端构建</span></span>
<span class="line"><span class="token function">npm</span> run build</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 检查类型安全</span></span>
<span class="line"><span class="token function">npm</span> run type-check</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 4. 运行测试套件</span></span>
<span class="line"><span class="token function">npm</span> <span class="token builtin class-name">test</span></span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="🔄-回滚计划" tabindex="-1"><a class="header-anchor" href="#🔄-回滚计划"><span>🔄 回滚计划</span></a></h2><h3 id="快速回滚-5分钟内" tabindex="-1"><a class="header-anchor" href="#快速回滚-5分钟内"><span>快速回滚（5分钟内）</span></a></h3><div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre><code class="language-bash"><span class="line"><span class="token comment"># 1. 恢复路由配置</span></span>
<span class="line"><span class="token function">cp</span> dashboard/config/routes.ts.backup dashboard/config/routes.ts</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 恢复原始页面</span></span>
<span class="line"><span class="token function">cp</span> <span class="token parameter variable">-r</span> dashboard/src/pages/GmFunctions.backup dashboard/src/pages/GmFunctions</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 移除新页面</span></span>
<span class="line"><span class="token function">rm</span> <span class="token parameter variable">-rf</span> dashboard/src/pages/Functions</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 4. 重新构建</span></span>
<span class="line"><span class="token function">npm</span> run build</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="完整回滚-15分钟内" tabindex="-1"><a class="header-anchor" href="#完整回滚-15分钟内"><span>完整回滚（15分钟内）</span></a></h3><div class="language-bash line-numbers-mode" data-highlighter="prismjs" data-ext="sh"><pre><code class="language-bash"><span class="line"><span class="token comment"># 1. Git回滚到上一个版本</span></span>
<span class="line"><span class="token function">git</span> checkout HEAD~1</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 2. 重新安装依赖</span></span>
<span class="line"><span class="token function">npm</span> ci</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 3. 重新构建</span></span>
<span class="line"><span class="token function">npm</span> run build</span>
<span class="line"></span>
<span class="line"><span class="token comment"># 4. 重启服务</span></span>
<span class="line"><span class="token function">npm</span> restart</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="📱-用户通知" tabindex="-1"><a class="header-anchor" href="#📱-用户通知"><span>📱 用户通知</span></a></h2><h3 id="部署通知模板" tabindex="-1"><a class="header-anchor" href="#部署通知模板"><span>部署通知模板</span></a></h3><div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre><code class="language-text"><span class="line">主题：Croupier函数管理系统升级通知</span>
<span class="line"></span>
<span class="line">尊敬的用户：</span>
<span class="line"></span>
<span class="line">我们将于 [时间] 进行函数管理系统的升级维护。</span>
<span class="line"></span>
<span class="line">主要改进：</span>
<span class="line">• 全新的函数管理界面，提供更好的用户体验</span>
<span class="line">• 优化的函数目录，支持搜索和过滤</span>
<span class="line">• 增强的调用历史记录和状态监控</span>
<span class="line">• 改进的实例管理和覆盖率分析</span>
<span class="line"></span>
<span class="line">影响范围：</span>
<span class="line">• 函数管理界面将暂时不可用（预计5分钟）</span>
<span class="line">• 所有现有数据和配置将保持不变</span>
<span class="line">• 旧版本界面仍可通过特定路径访问</span>
<span class="line"></span>
<span class="line">如有问题，请联系技术支持。</span>
<span class="line"></span>
<span class="line">谢谢您的理解与配合！</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h3 id="用户培训材料" tabindex="-1"><a class="header-anchor" href="#用户培训材料"><span>用户培训材料</span></a></h3><ul><li>[x] 新界面使用指南</li><li>[x] 功能对比文档</li><li>[x] 视频教程链接</li><li>[x] 常见问题解答</li></ul><h2 id="📋-部署后检查清单" tabindex="-1"><a class="header-anchor" href="#📋-部署后检查清单"><span>📋 部署后检查清单</span></a></h2><h3 id="功能验证-✅" tabindex="-1"><a class="header-anchor" href="#功能验证-✅"><span>功能验证 ✅</span></a></h3><ul><li>[ ] 函数目录页面显示正常</li><li>[ ] 搜索功能工作正常</li><li>[ ] 函数调用执行成功</li><li>[ ] 实例状态监控正确</li><li>[ ] 调用历史数据完整</li><li>[ ] 权限控制生效</li><li>[ ] 国际化显示正确</li></ul><h3 id="性能验证-✅" tabindex="-1"><a class="header-anchor" href="#性能验证-✅"><span>性能验证 ✅</span></a></h3><ul><li>[ ] 页面加载时间 &lt; 3秒</li><li>[ ] API响应时间 &lt; 500ms</li><li>[ ] 内存使用稳定</li><li>[ ] CPU使用率正常</li><li>[ ] 网络请求优化</li></ul><h3 id="兼容性验证-✅" tabindex="-1"><a class="header-anchor" href="#兼容性验证-✅"><span>兼容性验证 ✅</span></a></h3><ul><li>[ ] Chrome浏览器兼容</li><li>[ ] Firefox浏览器兼容</li><li>[ ] Safari浏览器兼容</li><li>[ ] 移动端适配正常</li><li>[ ] 旧版本URL重定向</li></ul><h3 id="安全验证-✅" tabindex="-1"><a class="header-anchor" href="#安全验证-✅"><span>安全验证 ✅</span></a></h3><ul><li>[ ] 权限检查正确</li><li>[ ] 数据验证有效</li><li>[ ] XSS防护正常</li><li>[ ] CSRF防护生效</li><li>[ ] 敏感信息脱敏</li></ul><h2 id="🚨-故障处理预案" tabindex="-1"><a class="header-anchor" href="#🚨-故障处理预案"><span>🚨 故障处理预案</span></a></h2><h3 id="常见问题及解决方案" tabindex="-1"><a class="header-anchor" href="#常见问题及解决方案"><span>常见问题及解决方案</span></a></h3><h4 id="问题1-页面加载失败" tabindex="-1"><a class="header-anchor" href="#问题1-页面加载失败"><span>问题1：页面加载失败</span></a></h4><div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre><code class="language-text"><span class="line">症状：页面显示空白或加载错误</span>
<span class="line">原因：路由配置错误或组件导入失败</span>
<span class="line">解决：</span>
<span class="line">1. 检查routes.ts配置</span>
<span class="line">2. 验证组件文件路径</span>
<span class="line">3. 检查import语句</span>
<span class="line">4. 查看浏览器控制台错误</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="问题2-api请求失败" tabindex="-1"><a class="header-anchor" href="#问题2-api请求失败"><span>问题2：API请求失败</span></a></h4><div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre><code class="language-text"><span class="line">症状：数据不显示或错误提示</span>
<span class="line">原因：API端点不存在或权限不足</span>
<span class="line">解决：</span>
<span class="line">1. 检查API服务状态</span>
<span class="line">2. 验证权限配置</span>
<span class="line">3. 检查请求参数</span>
<span class="line">4. 查看网络面板错误</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="问题3-功能异常" tabindex="-1"><a class="header-anchor" href="#问题3-功能异常"><span>问题3：功能异常</span></a></h4><div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre><code class="language-text"><span class="line">症状：按钮点击无响应或结果错误</span>
<span class="line">原因：事件处理函数错误或API调用失败</span>
<span class="line">解决：</span>
<span class="line">1. 检查事件绑定</span>
<span class="line">2. 验证API调用参数</span>
<span class="line">3. 查看控制台日志</span>
<span class="line">4. 检查数据处理逻辑</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h4 id="问题4-性能问题" tabindex="-1"><a class="header-anchor" href="#问题4-性能问题"><span>问题4：性能问题</span></a></h4><div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre><code class="language-text"><span class="line">症状：页面响应缓慢或卡顿</span>
<span class="line">原因：组件渲染性能问题或数据量大</span>
<span class="line">解决：</span>
<span class="line">1. 检查组件重渲染</span>
<span class="line">2. 优化数据处理</span>
<span class="line">3. 实施虚拟滚动</span>
<span class="line">4. 添加数据缓存</span>
<span class="line"></span></code></pre><div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0;"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><h2 id="📈-成功指标" tabindex="-1"><a class="header-anchor" href="#📈-成功指标"><span>📈 成功指标</span></a></h2><h3 id="用户体验指标" tabindex="-1"><a class="header-anchor" href="#用户体验指标"><span>用户体验指标</span></a></h3><ul><li>新用户学习时间减少40%</li><li>功能发现效率提升50%</li><li>用户满意度 &gt; 4.5/5</li></ul><h3 id="技术指标" tabindex="-1"><a class="header-anchor" href="#技术指标"><span>技术指标</span></a></h3><ul><li>代码复用率提升35%</li><li>维护成本降低35%</li><li>开发效率提升50%</li></ul><h3 id="业务指标" tabindex="-1"><a class="header-anchor" href="#业务指标"><span>业务指标</span></a></h3><ul><li>函数调用成功率 &gt; 95%</li><li>平均响应时间 &lt; 500ms</li><li>系统可用性 &gt; 99.9%</li></ul><h2 id="📞-联系信息" tabindex="-1"><a class="header-anchor" href="#📞-联系信息"><span>📞 联系信息</span></a></h2><h3 id="技术支持" tabindex="-1"><a class="header-anchor" href="#技术支持"><span>技术支持</span></a></h3><ul><li>开发团队：dev-team@company.com</li><li>紧急联系：+86-xxx-xxxx-xxxx</li></ul><h3 id="相关链接" tabindex="-1"><a class="header-anchor" href="#相关链接"><span>相关链接</span></a></h3><ul><li>项目文档：/docs/函数管理系统重构实施指南.md</li><li>组件文档：/dashboard/src/components/FunctionComponents/README.md</li><li>API文档：/docs/api.md</li></ul><hr><p><strong>部署负责人：</strong> ____________ <strong>部署日期：</strong> ____________ <strong>审核人员：</strong> ____________ <strong>版本号：</strong> v2.0.0</p><p><strong>确认签名：</strong> ____________</p>`,90)])])}const r=n(l,[["render",p]]),d=JSON.parse('{"path":"/%E5%87%BD%E6%95%B0%E7%AE%A1%E7%90%86%E7%B3%BB%E7%BB%9F%E9%87%8D%E6%9E%84%E9%83%A8%E7%BD%B2%E6%B8%85%E5%8D%95.html","title":"Croupier 函数管理系统重构部署清单","lang":"zh-CN","frontmatter":{},"git":{"updatedTime":1763866570000,"contributors":[{"name":"cuihairu","username":"cuihairu","email":"chuihairu@gmail.com","commits":1,"url":"https://github.com/cuihairu"}],"changelog":[{"hash":"18fe1406b320ee57fcad82dff0dbf0fb39d1d0b0","time":1763866570000,"email":"chuihairu@gmail.com","author":"cuihairu","message":"chore: before migration"}]},"filePathRelative":"函数管理系统重构部署清单.md"}');export{r as comp,d as data};
