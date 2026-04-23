<template><div><div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line">================================================================================</span>
<span class="line">                   Croupier 函数管理架构 - 现状 vs 建议</span>
<span class="line">================================================================================</span>
<span class="line"></span>
<span class="line">【现状菜单结构】</span>
<span class="line">└─ 主菜单</span>
<span class="line">   ├─ 数据分析 (Analytics)</span>
<span class="line">   ├─ 游戏运营 (Operations) ────────────┐</span>
<span class="line">   │  ├─ 审批管理                        │</span>
<span class="line">   │  ├─ 审计日志                        │  ← Registry 在这里</span>
<span class="line">   │  ├─ 操作日志                        │</span>
<span class="line">   │  ├─ 服务注册表 ◄─────────────────┘</span>
<span class="line">   │  └─ 服务列表</span>
<span class="line">   ├─ 游戏管理 (GameManagement) ────────┐</span>
<span class="line">   │  ├─ 游戏环境                        │</span>
<span class="line">   │  ├─ 实体管理                        │</span>
<span class="line">   │  ├─ 函数管理 (GmFunctions) ◄────────┼─ 函数调用在这里</span>
<span class="line">   │  ├─ 功能分配 (Assignments) ◄────────┼─ 函数分配在这里</span>
<span class="line">   │  └─ 功能包管理 (Packs) ◄───────────┘  ← 包管理在这里</span>
<span class="line">   ├─ 后台用户 (AdminUsers)</span>
<span class="line">   ├─ 支持系统 (Support)</span>
<span class="line">   └─ 运维管理 (Ops)</span>
<span class="line"></span>
<span class="line">【问题分析】</span>
<span class="line">1. 函数相关功能散落在 2 个顶级菜单下 (GameManagement + Operations)</span>
<span class="line">2. Registry 与 Assignments/Packs 强相关但位置远离</span>
<span class="line">3. 新用户难以快速找到函数管理相关页面</span>
<span class="line">4. 缺乏统一的入口和导航概念</span>
<span class="line"></span>
<span class="line">================================================================================</span>
<span class="line"></span>
<span class="line">【建议菜单结构 - 统一函数管理】</span>
<span class="line">└─ 主菜单</span>
<span class="line">   ├─ 数据分析 (Analytics)</span>
<span class="line">   ├─ 函数管理 (FunctionManagement) [新菜单] ────┐</span>
<span class="line">   │  ├─ 函数目录 (Catalog)                      │</span>
<span class="line">   │  │   └─ 搜索、分类、版本、权限管理          │</span>
<span class="line">   │  ├─ 函数调用 (Invoke)                        │</span>
<span class="line">   │  │   ├─ 快速调用 (Simple)                   │</span>
<span class="line">   │  │   ├─ 高级调用 (Advanced)                 │</span>
<span class="line">   │  │   └─ 调用历史                            │</span>
<span class="line">   │  ├─ 函数分配 (Assignments)                  │</span>
<span class="line">   │  │   └─ 增强版：细粒度权限 + 变更历史       │</span>
<span class="line">   │  ├─ 实例管理 (Instances) ◄── 从 Registry   │</span>
<span class="line">   │  │   ├─ Agent 管理                         │</span>
<span class="line">   │  │   ├─ 覆盖率分析                         │</span>
<span class="line">   │  │   └─ 健康检查                           │</span>
<span class="line">   │  └─ 函数包 (Packs)                          │</span>
<span class="line">   │      ├─ 包清单                             │</span>
<span class="line">   │      ├─ 导入/导出                          │</span>
<span class="line">   │      └─ 包内容详情 [新功能]                │</span>
<span class="line">   │                                             │</span>
<span class="line">   ├─ 游戏运营 (Operations) ─────────┬───────────┘  ← Registry 迁移</span>
<span class="line">   │  ├─ 审批管理</span>
<span class="line">   │  ├─ 审计日志</span>
<span class="line">   │  ├─ 操作日志</span>
<span class="line">   │  └─ 服务列表</span>
<span class="line">   ├─ 游戏管理 (GameManagement)</span>
<span class="line">   │  ├─ 游戏环境</span>
<span class="line">   │  └─ 实体管理</span>
<span class="line">   ├─ 后台用户 (AdminUsers)</span>
<span class="line">   ├─ 支持系统 (Support)</span>
<span class="line">   └─ 运维管理 (Ops)</span>
<span class="line"></span>
<span class="line">【优势】</span>
<span class="line">✓ 集中化：所有函数操作在一个菜单下</span>
<span class="line">✓ 清晰化：按功能分页面，逻辑清晰</span>
<span class="line">✓ 可扩展：易于添加新功能（版本、权限、监控等）</span>
<span class="line">✓ 易发现：用户快速找到函数管理功能</span>
<span class="line"></span>
<span class="line">================================================================================</span>
<span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div><div class="line-number"></div></div></div><p>【页面布局对比】</p>
<p>当前 GmFunctions 页面 (650+ 行代码)
┌────────────────────────────────────────────────────────────┐
│ ▢ GM Functions                                    [dev]     │
├────────────────────────────────────────────────────────────┤
│ [游戏选择器]                                               │
│ [Select Function: ▼] [Render Mode: ▼] [Route: ▼]         │
│ [Target: ▼] [Hash Key: ▼]                                │
│                                                            │
│ [表单区域 - 3 种渲染模式可切换]                          │
│ - Enhanced UI: XUI Schema 基础                          │
│ - Form-Render: 第三方库                               │
│ - Legacy: 经典 Ant Form                              │
│                                                            │
│ [调用] [启动任务] [取消任务]                             │
│                                                            │
│ [输出/事件区域]                                          │
│ [视图区域 - 支持多个自定义 renderer]                      │
└────────────────────────────────────────────────────────────┘</p>
<p>问题：
✗ 单页面承载过多功能
✗ 表单渲染逻辑过复杂
✗ UI 堆砌，缺乏层次感
✗ 没有搜索、排序、过滤
✗ 没有调用历史</p>
<hr>
<p>改进后的页面布局
┌────────────────────────────────────────────────────────────┐
│ 函数管理 (FunctionManagement)                              │
├─────────────────────────────────────────────────────────────┤
│ ▶ 函数目录 │ ▶ 函数调用 │ ▶ 函数分配 │ ▶ 实例 │ ▶ 包    │
└─────────────────────────────────────────────────────────────┘</p>
<p>【函数目录页面】
┌────────────────────────────────────────────────────────────┐
│ 搜索: __________ [分类 ▼] [版本 ▼] [状态 ▼]              │
├────────────────────────────────────────────────────────────┤
│ [卡片视图] [列表视图] [详情视图]                          │
├────────────────────────────────────────────────────────────┤
│ 函数名 │ 分类 │ 版本 │ 实例数 │ 状态 │ 操作               │
├────────────────────────────────────────────────────────────┤
│ fn1    │ ...  │ ...  │ ...    │ ✓   │ [调用] [分配]      │
│ fn2    │ ...  │ ...  │ ...    │ ✓   │ [调用] [分配]      │
│ fn3    │ ...  │ ...  │ ...    │ ⚠   │ [调用] [分配]      │
└────────────────────────────────────────────────────────────┘</p>
<p>优势：
✓ 清晰的分页设计
✓ 支持搜索和高级过滤
✓ 快速操作按钮
✓ 视觉反馈清晰（状态指示）</p>
<p>【函数调用页面 - 简化版】
┌────────────────────────────────────────────────────────────┐
│ 选择函数: [fn_name ▼]  [搜索 ____________]                 │
├────────────────────────────────────────────────────────────┤
│ 参数                    │ 最近调用           │ 结果         │
│ ─────────────────────────────────────────────────────────  │
│ [参数表单区域]         │ · 2024-01-15      │ 结果展示    │
│                        │ · 2024-01-14      │             │
│                        │ · 2024-01-13      │ [重新运行]  │
│                        │                   │             │
│ [快速调用] [高级]      │ [查看全部]        │             │
└────────────────────────────────────────────────────────────┘</p>
<p>优势：
✓ 更清晰的布局（参数 | 历史 | 结果）
✓ 调用历史集成
✓ 快速操作
✓ 易用性提升</p>
<p>================================================================================</p>
<p>【数据流对比】</p>
<p>当前数据流：
Web UI (GmFunctions)
├─ listDescriptors()         获取所有函数
├─ fetchAssignments()         过滤显示函数
├─ listFunctionInstances()    查询实例列表
└─ invokeFunction()           调用函数</p>
<p>Web UI (Registry)
├─ fetchRegistry()            获取注册表信息
└─ (没有与 GmFunctions 的联动)</p>
<p>Web UI (Assignments)
├─ listDescriptors()          列出可分配的函数
├─ fetchAssignments()         获取当前分配
└─ setAssignments()           保存分配</p>
<p>问题：多个页面各自加载数据，没有共享状态管理</p>
<hr>
<p>改进的数据流（建议）：
全局 Function Context (新增)
├─ selectedGame
├─ selectedEnv
├─ descriptors (缓存所有函数)
├─ assignments (缓存分配关系)
├─ registry (缓存注册信息)
└─ recentCalls (调用历史)</p>
<p>使用这个上下文：
├─ FunctionCatalog 页面
├─ FunctionInvoke 页面
├─ FunctionAssignments 页面
├─ FunctionInstances 页面
└─ FunctionPacks 页面</p>
<p>新增 API 端点：
├─ GET /api/functions/summary      一次获取所有信息
├─ GET /api/function_calls/:id     调用历史
├─ GET /api/coverage/analysis      覆盖率分析
└─ GET /api/agents/:id/metrics     Agent 指标</p>
<p>优势：
✓ 减少 API 调用次数
✓ 数据一致性
✓ 更好的性能
✓ 页面间共享状态</p>
<p>================================================================================</p>
<p>【权限模型对比】</p>
<p>当前权限模型（粗粒度）:
可授予权限:
├─ functions:read      (读函数列表)
├─ registry:read       (读注册表)
├─ assignments:read    (读分配)
├─ assignments:write   (写分配)
├─ packs:read          (读包)
├─ packs:reload        (重新加载包)
└─ packs:export        (导出包)</p>
<p>问题：
✗ 不支持按函数粒度的权限
✗ 不支持按角色/环境的权限
✗ Assignments 只有白名单，没有细粒度权限</p>
<hr>
<p>改进的权限模型（细粒度）:</p>
<p>资源粒度：
├─ function:{id}:read           读特定函数
├─ function:{id}:invoke         调用特定函数
├─ function:{id}:view_history   查看调用历史
├─ assignments:{game_id}:read   读特定游戏的分配
├─ assignments:{game_id}:write  修改特定游戏的分配
├─ registry:read                读注册表
└─ packs:manage                 管理包</p>
<p>角色绑定：
├─ game_operator            可以调用已分配的函数
├─ game_admin               可以修改分配关系
├─ function_developer       可以上传/更新函数包
├─ ops_engineer             可以查看 Registry 和 Instances
└─ system_admin             完全权限</p>
<p>优势：
✓ 更安全的权限控制
✓ 更灵活的角色定义
✓ 审计日志更清晰</p>
<p>================================================================================</p>
<p>【组件复用策略】</p>
<p>当前：每个页面各自实现逻辑</p>
<p>改进：提取通用组件</p>
<p>┌─ FunctionFormRenderer 组件
│   ├─ 支持 JSONSchema
│   ├─ 支持 UISchema
│   ├─ 支持简化/高级模式
│   └─ 使用场景：GmFunctions, Assignments, Approvals
│
├─ FunctionListTable 组件
│   ├─ 搜索、排序、过滤
│   ├─ 列表/卡片/详情视图
│   ├─ 批量操作
│   └─ 使用场景：Catalog, Assignments, Approvals
│
├─ RegistryViewer 组件
│   ├─ Agent 表格
│   ├─ 覆盖率分析
│   ├─ CSV 导出
│   └─ 使用场景：Instances, Dashboard, Reports
│
├─ FunctionCallHistory 组件
│   ├─ 时间线视图
│   ├─ 参数/结果对比
│   ├─ 重新运行功能
│   └─ 使用场景：Invoke, Detail, Dashboard
│
└─ FunctionDetailPanel 组件
├─ 元数据展示
├─ 权限信息
├─ 实例分布
└─ 使用场景：Catalog, Invoke, Detail</p>
<p>================================================================================</p>
<div class="language-text line-numbers-mode" data-highlighter="prismjs" data-ext="text"><pre v-pre><code class="language-text"><span class="line"></span></code></pre>
<div class="line-numbers" aria-hidden="true" style="counter-reset:line-number 0"></div></div></div></template>


