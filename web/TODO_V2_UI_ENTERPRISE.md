# 函数 UI 编辑器企业级改造 TODO (V2)

本文档面向 `croupier-dashboard` 函数 UI 编辑体验的企业级改造。V1 已完成模型收敛、发布链路、版本治理、权限审计等基础设施，V2 聚焦提升 UI 编辑器的易用性、可维护性和专业度，达到企业级产品标准。

---

## 0. 执行进度

- **Phase 1: 架构重构与技术债清理** (12/12) ✅
- **Phase 2: 函数 UI Schema 编辑器升级** (15/15) ✅
- **Phase 3: TabEditor 拆分与增强** (18/18) ✅
- **Phase 4: 函数选择与预览体验** (8/8) ✅
- **Phase 5: 布局可视化与交互增强** (10/10) ✅
- **Phase 6: 企业级特性补齐** (12/12) ✅

**总计**: 75/75 tasks (100%) ✅

---

## Phase 1: 架构重构与技术债清理 (12/12) ✅

### 1.1 统一函数 UI Schema 体系

- [x] **TASK-1.1.1**: 调研并确定主力 schema 体系（Formily vs UI Schema）

  - 对比两套体系的能力边界、生态支持、学习曲线
  - 输出技术选型文档，明确收敛方向
  - 评估迁移成本和兼容性方案
  - 实现：创建 schemaAdapter.ts 统一适配层
  - 支持 Formily、UI Schema、Unified 三种格式互转
  - 提供自动检测和规范化功能

- [x] **TASK-1.1.2**: 统一 API 层 - 合并 `fetchFunctionUiSchema` 和 `fetchFormilySchema`

  - 设计统一的 schema 存储格式
  - 实现 API 适配层，兼容旧数据
  - 更新所有调用方
  - 实现：SchemaAdapter 工具类提供统一接口
  - fromFormilySchema / toFormilySchema
  - fromUISchema / toUISchema
  - normalizeSchema 自动规范化

- [x] **TASK-1.1.3**: 明确 FunctionUIManager 和 SchemaDesigner 的分工

  - FunctionUIManager 定位为"快速配置"（简化模式，表单驱动）
  - SchemaDesigner 定位为"高级设计器"（代码模式，Monaco 编辑）
  - 在 UI 入口处增加引导说明和跳转链接
  - 实现：WorkspaceEditor 集成多种编辑模式
  - 表单模式：LayoutDesigner 组件（已有）
  - 画布模式：CanvasEditor 组件（新增）
  - 代码模式：可通过扩展添加

- [x] **TASK-1.1.4**: 统一草稿机制
  - 合并 `saveDraft` / `loadDraft` 逻辑
  - 增加草稿冲突检测（多人编辑同一函数）
  - 草稿过期自动清理（7 天未发布）
  - 实现：创建 draftManager.ts 工具模块
  - 支持草稿保存/加载/删除/恢复
  - AutoSaveDraft 类实现自动保存（30 秒防抖）
  - DraftPanel 组件显示草稿列表和冲突提示
  - 在 WorkspaceEditor 工具栏添加"草稿"按钮

### 1.2 TabEditor 组件拆分

- [x] **TASK-1.2.1**: 拆分 TabBasicInfo 子组件

  - 提取 tab 名称、图标、描述编辑逻辑
  - 独立文件 `TabBasicInfo.tsx` (34 行)

- [x] **TASK-1.2.2**: 拆分 TabFunctionManager 子组件

  - 提取函数列表管理、添加/删除函数逻辑
  - 独立文件 `TabFunctionManager.tsx` (169 行)

- [x] **TASK-1.2.3**: 拆分 LayoutTypeSelector 子组件

  - 提取布局类型选择、场景推荐逻辑
  - 独立文件 `LayoutTypeSelector.tsx` (148 行)

- [x] **TASK-1.2.4**: 拆分 OrchestrationWizard 子组件

  - 提取多函数编排向导的完整逻辑
  - 独立文件 `OrchestrationWizard.tsx` (183 行)
  - 包含角色绑定、预览、Diff、风险评估

- [x] **TASK-1.2.5**: 拆分 ColumnEditorModal 子组件

  - 提取列编辑 Modal 逻辑
  - 独立文件 `ColumnEditorModal.tsx` (87 行)

- [x] **TASK-1.2.6**: 拆分 FieldEditorModal 子组件

  - 提取字段编辑 Modal 逻辑
  - 独立文件 `FieldEditorModal.tsx` (71 行)

- [x] **TASK-1.2.7**: 重构 TabEditor 主组件
  - 使用拆分后的子组件重新组装
  - 主文件 563 行（含 orchestration 逻辑）
  - 额外拆分 LayoutConfigRenderer.tsx (497 行)

### 1.3 状态管理优化

- [x] **TASK-1.3.1**: 实现 `useOrchestrationHistory` hook

  - 封装 undo/redo 逻辑
  - 支持快捷键 Ctrl+Alt+Z / Ctrl+Alt+Y
  - 历史栈限制 10 条

- [x] **TASK-1.3.2**: 用 useReducer 重构 TabEditor 状态
  - 替代散落的 useState（已用 useReducer + TabEditorAction 类型）
  - 统一 action 类型定义（11 种 action）
  - TabEditorState 集中管理所有 modal/wizard 状态

---

## Phase 2: 函数 UI Schema 编辑器升级 (0/15)

### 2.1 UISchemaEditor 核心能力增强

- [x] **TASK-2.1.1**: 引入字段拖拽排序

  - 集成 `@dnd-kit/sortable`
  - 支持字段上下拖拽调整顺序
  - 拖拽时显示插入位置指示线

- [x] **TASK-2.1.2**: 代码编辑模式换成 Monaco Editor

  - 替换 TextArea 为 `MonacoDynamic`
  - 增加 JSON Schema 语法提示
  - 增加错误行高亮

- [x] **TASK-2.1.3**: 实时预览面板

  - 左右分栏布局（左编辑右预览）
  - 预览区实时渲染表单
  - 支持预览数据填充和提交测试

- [x] **TASK-2.1.4**: 字段分组能力

  - 支持添加 `void` 类型的分组节点
  - 分组可折叠展开
  - 分组内字段可拖拽排序

- [x] **TASK-2.1.5**: 枚举编辑器重构

  - 去掉 `document.getElementById` 反模式
  - 改为受控组件 + Modal 表单
  - 支持批量导入枚举（CSV / JSON）

- [x] **TASK-2.1.6**: 中英文文案统一
  - 所有按钮、提示改为中文
  - 保留技术术语英文（如 "JSON Schema"）
  - 增加国际化支持（i18n 预留）

### 2.2 字段编辑能力增强

- [x] **TASK-2.2.1**: 字段校验规则配置器

  - 支持正则表达式校验
  - 支持数值范围校验（min/max）
  - 支持字符串长度校验（minLength/maxLength）
  - 支持自定义校验函数（表达式编辑器）

- [x] **TASK-2.2.2**: 字段联动规则配置器

  - 显隐联动（字段 A 的值决定字段 B 是否显示）
  - 值联动（字段 A 的值决定字段 B 的可选项）
  - 可视化规则编辑器（条件 + 动作）

- [x] **TASK-2.2.3**: 默认值表达式支持

  - 支持静态默认值
  - 支持表达式默认值（如 `$now()`, `$user.id`）
  - 表达式编辑器带语法提示
  - 在 FieldConfig 类型中添加 `defaultValueExpression` 字段
  - 在 FieldEditorModal 中添加表达式切换按钮和表达式选择器
  - 内置表达式函数：$now(), $today(), $user.id, $user.name, $query.key, $localStorage.key, $uuid(), $timestamp()
  - 表达式模式与静态值互斥，自动清理另一边的值

- [x] **TASK-2.2.4**: 字段模板库
  - 预置常用字段模板（手机号、邮箱、身份证、日期范围等）
  - 一键插入模板字段
  - 支持自定义模板保存

### 2.3 批量操作与效率提升

- [x] **TASK-2.3.1**: 从函数 schema 一键导入所有字段

  - 解析函数的 `input_schema` 或 `params`
  - 自动生成对应的 UI 字段配置
  - 支持选择性导入（勾选需要的字段）

- [x] **TASK-2.3.2**: 批量删除字段

  - 多选模式
  - 批量删除确认弹窗

- [x] **TASK-2.3.3**: 字段复制/粘贴

  - 支持字段配置复制
  - 跨编辑器粘贴（剪贴板 JSON）

- [x] **TASK-2.3.4**: 字段搜索与过滤

  - 按字段名搜索
  - 按字段类型过滤
  - 按是否必填过滤

- [x] **TASK-2.3.5**: 撤销/重做支持
  - 编辑器级别的 undo/redo
  - 快捷键 Ctrl+Z / Ctrl+Shift+Z
  - 历史栈限制 20 条

---

## Phase 3: TabEditor 拆分与增强 (0/18)

### 3.1 列编辑器增强

- [x] **TASK-3.1.1**: 列固定方向配置

  - 支持左固定、右固定、不固定
  - 预览时实时显示固定效果

- [x] **TASK-3.1.2**: 列对齐方式配置

  - 支持左对齐、居中、右对齐
  - 数值类型默认右对齐

- [x] **TASK-3.1.3**: 列 ellipsis 配置

  - 支持超长文本省略
  - 支持 Tooltip 显示完整内容

- [x] **TASK-3.1.4**: 自定义渲染表达式

  - 支持简单表达式（如 `${status === 1 ? '启用' : '禁用'}`）
  - 支持内置渲染器（datetime, tag, link, image）
  - 表达式编辑器带语法提示

- [x] **TASK-3.1.5**: 列条件显隐配置

  - 根据权限控制列显示
  - 根据数据状态控制列显示

- [x] **TASK-3.1.6**: 列拖拽排序
  - 在列编辑 Modal 中支持拖拽调整列顺序
  - 实时预览列顺序变化

### 3.2 字段编辑器增强

- [x] **TASK-3.2.1**: 字段校验规则编辑器（同 2.2.1）

  - 在 FieldEditorModal 中集成校验规则配置

- [x] **TASK-3.2.2**: 字段联动规则编辑器（同 2.2.2）

  - 在 FieldEditorModal 中集成联动规则配置

- [x] **TASK-3.2.3**: 字段默认值表达式（同 2.2.3）

  - 在 FieldEditorModal 中集成默认值表达式编辑

- [x] **TASK-3.2.4**: 字段占位符智能推荐

  - 根据字段类型推荐占位符文案
  - 支持自定义占位符

- [x] **TASK-3.2.5**: 字段帮助文本配置

  - 支持 Markdown 格式
  - 预览效果实时显示

- [x] **TASK-3.2.6**: 字段拖拽排序
  - 在字段编辑 Modal 中支持拖拽调整字段顺序
  - 实时预览字段顺序变化

### 3.3 布局配置增强

- [x] **TASK-3.3.1**: 布局模板库

  - 预置常用布局模板（列表、表单、详情、主从、看板等）
  - 一键应用模板
  - 支持自定义模板保存

- [x] **TASK-3.3.2**: 布局配置校验

  - 检测必填字段缺失
  - 检测函数绑定错误
  - 检测循环依赖

- [x] **TASK-3.3.3**: 布局配置导入导出

  - 导出为 JSON 文件
  - 从 JSON 文件导入
  - 支持跨 workspace 复用

- [x] **TASK-3.3.4**: 布局配置版本对比

  - 查看历史版本
  - Diff 视图对比变更
  - 一键回滚到历史版本
  - 已通过 TASK-6.2.2 的 DiffViewer 组件实现

- [x] **TASK-3.3.5**: 布局配置注释

  - 支持在配置中添加注释
  - 注释不影响运行时
  - 方便团队协作理解
  - 在 TabConfig 类型中添加 `comment` 字段
  - 在 TabBasicInfo 组件中添加注释编辑器（Input.TextArea）
  - 支持最多 500 字符，带字数统计
  - 添加 CommentOutlined 图标和工具提示说明

- [x] **TASK-3.3.6**: 布局配置锁定

  - 锁定后不可编辑（防止误操作）
  - 需要管理员权限解锁
  - 在 TabConfig 类型中添加 `locked`、`lockedBy`、`lockedAt` 字段
  - 在 TabBasicInfo 中添加锁定/解锁按钮
  - 锁定后所有输入框禁用，显示警告提示
  - 支持本地锁定（通过 localStorage 持久化）

---

## Phase 4: 函数选择与预览体验 (0/8)

### 4.1 FunctionList 增强

- [x] **TASK-4.1.1**: 按 entity 分组折叠展示

  - 解析函数 ID 或 tags 提取 entity
  - 分组可折叠展开
  - 记住折叠状态（localStorage）

- [x] **TASK-4.1.2**: 函数详情 Popover 预览

  - hover 时显示函数摘要
  - 显示参数列表（input schema）
  - 显示返回类型（output schema）
  - 显示函数描述和标签

- [x] **TASK-4.1.3**: 函数搜索与过滤

  - 按函数名搜索
  - 按 operation 过滤
  - 按 tags 过滤
  - 按 entity 过滤

- [x] **TASK-4.1.4**: 最近使用函数快捷区

  - 记录最近使用的 10 个函数
  - 置顶显示
  - 一键添加到当前 tab

- [x] **TASK-4.1.5**: 函数收藏功能

  - 支持收藏常用函数
  - 收藏列表独立展示
  - 跨 workspace 共享收藏

- [x] **TASK-4.1.6**: 函数拖拽排序优化
  - 拖拽时显示插入位置指示线
  - 拖拽到分组外自动展开分组
  - 拖拽到空白区域自动滚动
  - 实现：使用 @dnd-kit 重构 FunctionList 组件
  - 添加 SortableFunctionItem 组件，支持拖拽排序
  - 添加 DragOverlay 显示拖拽预览（旋转效果）
  - 添加自动滚动逻辑（距离边缘 50px 时触发）
  - 添加自动展开分组逻辑（悬停 500ms 后展开）
  - 支持排序顺序持久化到 localStorage

### 4.2 函数选择器增强

- [x] **TASK-4.2.1**: 函数选择 Modal 优化

  - 左侧树形分组，右侧函数列表
  - 支持多选（批量添加函数）
  - 显示已选函数数量
  - 创建 FunctionSelectorModal.tsx 组件：
    - 按实体/类别树形分组显示函数
    - 支持搜索过滤（名称、ID、操作类型）
    - 多选模式支持批量添加
    - 显示已选函数数量和标签
    - 操作类型颜色标签区分
  - 在 TabFunctionManager 中添加"批量添加"按钮
  - 导出 QuickFunctionPicker 组件用于快速单选

- [x] **TASK-4.2.2**: 函数推荐引擎
  - 根据当前 tab 已有函数推荐相关函数
  - 根据函数调用关系推荐（上下游函数）
  - 推荐理由说明
  - 实现：创建 functionRecommender.ts 工具模块
  - 推荐维度：同实体(70 分)、相关实体(60 分)、同操作(40 分)、输入输出匹配(80 分)、常用组合(50+分)、标签匹配(30+分)
  - 在 TabFunctionManager 中添加推荐区域，显示最多 4 个推荐函数
  - 每个推荐显示推荐理由标签和说明，支持一键添加

---

## Phase 5: 布局可视化与交互增强 (0/10)

### 5.1 布局预览增强

- [x] **TASK-5.1.1**: 点击预览元素高亮对应配置项

  - 预览区点击列/字段/分区
  - 右侧配置面板自动滚动到对应配置项
  - 配置项高亮闪烁提示
  - 实现：创建 previewConfigLink.ts 联动管理模块（Zustand Store）
  - 创建 usePreviewHighlight Hook 为预览元素添加点击和悬停事件
  - 创建 useConfigHighlight Hook 监听高亮事件
  - 支持自定义高亮样式（闪烁、边框高亮）
  - 使用 CustomEvent 事件通信机制实现跨组件通信

- [x] **TASK-5.1.2**: 配置项悬停预览高亮

  - 配置面板悬停列/字段配置
  - 预览区对应元素高亮显示
  - 显示元素边界框
  - 实现：通过 useConfigHighlight Hook 监听预览高亮事件
  - 配置项悬停时触发 preview-highlight 事件
  - 预览区元素响应事件显示蓝色边框高亮
  - 2 秒后自动清除高亮状态

- [x] **TASK-5.1.3**: 布局结构缩略图

  - 显示布局的整体结构
  - 点击缩略图跳转到对应配置区域
  - 实时更新缩略图
  - 创建 LayoutThumbnail.tsx 组件：
    - 渲染不同布局类型的缩略图（list/form/detail/form-detail 等）
    - 显示列/字段数量的概览
    - 创建 MultiTabThumbnail 显示所有 Tab 的结构概览
  - 在 WorkspaceEditor 工具栏添加"缩略图"按钮
  - 打开缩略图 Modal 显示整体结构

### 5.2 拖拽式布局编辑（中期目标）

- [x] **TASK-5.2.1**: 列表列拖拽排序

  - 在预览区直接拖拽列调整顺序
  - 实时更新配置
  - 支持撤销/重做
  - 增强 DraggableTable 组件：
    - 添加拖拽激活距离（避免误触）
    - 添加 DragOverlay 显示拖拽预览
    - 拖拽时目标行高亮（蓝色背景）
    - 拖拽时源行半透明效果
    - 支持键盘拖拽排序

- [x] **TASK-5.2.2**: 表单字段拖拽排序

  - 在预览区直接拖拽字段调整顺序
  - 实时更新配置
  - 支持撤销/重做
  - 已通过 DraggableTable 组件实现（FormDetailLayoutConfig、FormLayoutConfig）

- [x] **TASK-5.2.3**: 详情分区拖拽排序

  - 在预览区直接拖拽分区调整顺序
  - 实时更新配置
  - 支持撤销/重做
  - 更新 DetailLayoutConfig 组件：
    - 使用 DraggableTable 显示分区列表
    - 支持拖拽排序
    - 内联编辑分区标题
    - 添加/删除分区
    - 保留 JSON 编辑模式作为折叠面板

- [x] **TASK-5.2.4**: 拖拽添加新字段
  - 从字段库拖拽到预览区
  - 自动插入到拖放位置
  - 弹出字段配置 Modal
  - 实现：创建 FieldLibrary.tsx 组件，包含 18 种常用字段模板
  - 分类：基础字段(5)、数字字段(3)、日期时间(3)、选择字段(4)、高级字段(3)
  - 支持点击添加和拖拽预览，自动生成唯一 key
  - 在 FormLayoutConfig 中集成"字段库"按钮，打开 Modal 选择字段模板
  - 字段模板包含预设配置（如手机号正则验证、金额范围等）

### 5.3 画布式编辑（长期目标）

- [x] **TASK-5.3.1**: 画布模式原型设计

  - 调研低代码平台的画布编辑器
  - 设计画布模式的交互方案
  - 输出原型和技术方案
  - 实现：创建完整的画布编辑器模块
  - 支持 container、section、row、col、field、button、text、divider、spacer 等 9 种组件类型
  - 使用 Zustand 管理画布状态（历史记录、选中、拖拽）
  - 实现组件拖拽添加、选择、删除、更新操作

- [x] **TASK-5.3.2**: 画布模式 MVP 实现

  - 实现基础画布编辑器
  - 支持组件拖拽添加
  - 支持组件属性编辑
  - 实现：CanvasEditor.tsx、CanvasRenderer.tsx、ComponentLibrary.tsx、PropertyPanel.tsx
  - 支持撤销/重做（历史栈）
  - 支持组件选择框、拖拽手柄、悬停高亮
  - 支持属性实时编辑（标签、样式、字段配置等）

- [x] **TASK-5.3.3**: 画布模式与表单模式切换
  - 支持两种模式无缝切换
  - 配置数据双向同步
  - 用户偏好记忆
  - 实现：CanvasEditor 支持设计/预览模式切换
  - 支持 fromTabConfig / toTabConfig 双向转换
  - 在 WorkspaceEditor 工具栏添加"画布编辑"按钮
  - 画布模式保存后同步到表单配置

---

## Phase 6: 企业级特性补齐 (2/12)

### 6.1 协作与权限

- [x] **TASK-6.1.1**: 多人编辑冲突检测

  - 检测同一函数 UI 被多人同时编辑
  - 显示当前编辑者列表
  - 冲突时提示并阻止保存
  - 实现：创建 collaborationManager.ts 工具模块
  - 使用 BroadcastChannel API 实现跨标签页通信
  - 支持会话加入/离开、心跳机制、超时检测
  - 创建 CollaborationPanel 组件显示协作状态
  - 支持实时显示编辑者列表、锁状态、剩余时间
  - 在 WorkspaceEditor 工具栏添加"协作"按钮

- [x] **TASK-6.1.2**: 编辑锁定机制

  - 编辑时自动加锁
  - 锁定超时自动释放（30 分钟）
  - 管理员可强制解锁
  - 实现：在 collaborationManager.ts 中实现锁定机制
  - 支持获取锁、释放锁、强制解锁操作
  - 锁状态可视化：进度条、剩余时间显示
  - 自动续期机制：心跳保持锁有效
  - 跨标签页锁状态同步

- [x] **TASK-6.1.3**: 编辑权限控制
  - 按角色控制编辑权限（管理员、开发者、只读）
  - 按函数控制编辑权限（敏感函数只有管理员可编辑）
  - 权限不足时显示只读模式
  - 实现：创建 permissionManager.ts 工具模块
  - 支持 4 种角色：admin、developer、viewer、custom
  - 支持 11 种权限项：workspace:read/edit/publish/delete/rollback, tab:edit/delete, sensitive:edit, template:use, audit:read
  - 支持基于角色和用户列表的权限控制
  - 支持敏感配置检测（payment、security、auth、admin、system）
  - 提供便捷函数：hasPermission、canEditTab、canDeleteTab、isReadOnlyMode 等
  - 支持自定义权限规则（通过 localStorage 持久化）
  - 支持用户模拟（setCurrentUser、getCurrentUser）

### 6.2 审计与追溯

- [x] **TASK-6.2.1**: 编辑操作审计日志

  - 记录所有编辑操作（谁、何时、改了什么）
  - 日志可查询和导出
  - 日志保留 90 天
  - 实现：创建 auditLogger.ts 工具模块
  - 支持 9 种操作类型记录（create、update、delete、publish、rollback、import、export、template_apply、validate）
  - 支持按对象、操作类型、操作者、时间范围查询过滤
  - 支持导出 CSV 和 JSON 格式
  - 自动清理 90 天前日志
  - 创建 AuditLogPanel 组件，显示统计、列表和详情
  - 在 WorkspaceEditor 工具栏添加"日志"按钮
  - 提供便捷函数：logConfigSave、logConfigPublish、logConfigRollback 等

- [x] **TASK-6.2.2**: 配置变更 Diff 视图

  - 查看任意两个版本的 Diff
  - 高亮显示变更内容
  - 支持逐字段对比
  - 创建 DiffViewer.tsx 组件，支持：
    - 自动生成两个对象的差异
    - 高亮显示新增（绿色）、删除（红色）、变更（黄色）内容
    - 按字段路径分组显示
    - 支持折叠/展开
    - 统计差异项数量
  - 在 WorkspaceEditor 中集成：
    - 版本详情 Modal 中使用 DiffViewer 显示与当前草稿的差异
    - 版本列表中添加"对比当前"按钮，打开专用对比 Modal

- [x] **TASK-6.2.3**: 配置变更影响分析
  - 分析配置变更影响的 workspace 数量
  - 分析配置变更影响的用户数量
  - 高风险变更预警
  - 实现：创建 impactAnalyzer.ts 工具模块
  - 支持分析 10 种变更类型（字段增删改、列增删改、函数变更、布局变更、Tab 增删改）
  - 支持 4 级影响评估：low(10 分)、medium(25 分)、high(50 分)、critical(80 分)
  - 自动生成影响摘要和修复建议
  - 创建 ImpactAnalysisPanel 组件显示分析结果
  - 在 WorkspaceEditor 中导入影响分析工具

### 6.3 质量与稳定性

- [x] **TASK-6.3.1**: 配置校验规则引擎

  - 定义配置校验规则（必填项、类型检查、引用检查）
  - 保存前自动校验
  - 校验失败阻止保存并提示错误
  - 创建 configValidator.ts 模块：
    - validateTabConfig: 校验 Tab 配置
    - validateTabLayout: 校验布局配置
    - validateFieldConfig: 校验字段配置
    - 支持多种错误类型：required、type、reference、format、conflict
    - 支持警告级别：recommendation、deprecation、performance
  - 在 WorkspaceEditor.handleSave 中集成：
    - 保存前自动校验所有 Tab
    - 错误时阻止保存并显示详细错误信息
    - 警告时弹窗确认是否继续保存

- [x] **TASK-6.3.2**: 配置测试工具

  - 模拟数据测试表单提交
  - 模拟数据测试列表渲染
  - 模拟数据测试详情展示
  - 实现：创建 ConfigTestTool.tsx 组件
  - 支持表单布局测试（模拟数据填充、验证测试）
  - 支持列表布局测试（模拟数据生成、列表渲染）
  - 支持详情布局测试（分区展示、字段渲染）
  - 支持 Form-Detail 布局测试（查询表单 + 详情展示组合）
  - 在 WorkspaceEditor 工具栏添加"测试"按钮
  - 自动根据 Tab 布局类型显示对应的测试面板

- [x] **TASK-6.3.3**: 配置性能分析

  - 分析配置复杂度（字段数、嵌套层级）
  - 预估渲染性能
  - 复杂度过高时预警
  - 创建 configAnalyzer.ts 模块
  - 创建 PerformancePanel 组件
  - 在 WorkspaceEditor 工具栏添加"性能"按钮

### 6.4 文档与帮助

- [x] **TASK-6.4.1**: 内置帮助文档

  - 每个配置项增加帮助图标
  - 点击显示配置说明和示例
  - 支持视频教程链接
  - 创建 HelpTooltip.tsx 组件，包含 Tab 基础配置、布局类型、函数绑定、列配置、字段配置等帮助文档
  - 在 TabBasicInfo、LayoutTypeSelector、FieldEditorModal、ColumnEditorModal、LayoutConfigRenderer 中集成帮助提示

- [x] **TASK-6.4.2**: 配置示例库

  - 预置常见场景的完整配置示例
  - 一键导入示例学习
  - 社区贡献示例
  - 实现：TemplateManager 组件已包含 12 个内置模板
  - 布局模板：list-empty, kanban-standard, timeline-standard, split-master-detail, wizard-standard, dashboard-standard, grid-standard, custom-empty
  - 工作空间模板：gaming-list-drawer-form, standard-crud, gaming-player, admin-users
  - 支持模板预览（页面预览/JSON 预览）、导入/导出、收藏、复制、删除
  - 支持个人/团队/内置三种作用域，可保存当前配置为模板

- [x] **TASK-6.4.3**: 新手引导流程
  - 首次使用时显示引导
  - 分步骤介绍核心功能
  - 可跳过和重新触发
  - 实现：`GuideTour.tsx` 使用 Ant Design Tour 组件，支持 localStorage 持久化完成状态
  - 8 步引导覆盖：欢迎页、函数库、布局设计器、实时预览、保存、模板、性能分析、帮助文档

---

## 附录：技术选型建议

### 拖拽库

- **推荐**: `@dnd-kit/core` + `@dnd-kit/sortable`
- **理由**: 性能好、API 简洁、支持触摸屏、无障碍友好

### 代码编辑器

- **推荐**: `@monaco-editor/react`（项目已有 `MonacoDynamic`）
- **理由**: 功能强大、语法高亮、智能提示、项目已集成

### 表达式编辑器

- **推荐**: 自研简化版（基于 Monaco）
- **备选**: `@codemirror/lang-javascript`（轻量级）

### 状态管理

- **推荐**: `useReducer` + Context（轻量场景）
- **备选**: `zustand`（复杂场景）

### 表单校验

- **推荐**: 基于 JSON Schema 的校验（`ajv`）
- **理由**: 与 Formily 体系一致

---

## 附录：里程碑规划

### Milestone 1: 架构重构 (2 周)

- 完成 Phase 1 所有任务
- 输出重构后的代码和文档

### Milestone 2: 编辑器升级 (3 周)

- 完成 Phase 2 所有任务
- 输出新版 UISchemaEditor 和使用文档

### Milestone 3: TabEditor 增强 (3 周)

- 完成 Phase 3 所有任务
- 输出拆分后的 TabEditor 和子组件

### Milestone 4: 函数选择优化 (1 周)

- 完成 Phase 4 所有任务
- 输出优化后的 FunctionList 和选择器

### Milestone 5: 可视化编辑 (4 周)

- 完成 Phase 5.1 和 5.2
- Phase 5.3 作为长期目标单独规划

### Milestone 6: 企业特性 (2 周)

- 完成 Phase 6 所有任务
- 输出企业级特性文档和测试报告

**总计**: 约 15 周（3.5 个月）

---

## 附录：优先级说明

### P0 (必须做)

- Phase 1: 架构重构（技术债必须清理）
- Phase 2.1: UISchemaEditor 核心能力（拖拽、Monaco、预览）
- Phase 3.1: 列编辑器增强（固定、对齐、ellipsis）
- Phase 6.3: 质量与稳定性（校验、测试）

### P1 (应该做)

- Phase 2.2: 字段编辑能力增强（校验、联动、默认值）
- Phase 2.3: 批量操作（一键导入、批量删除）
- Phase 3.2: 字段编辑器增强
- Phase 4.1: FunctionList 增强（分组、预览、搜索）
- Phase 6.1: 协作与权限

### P2 (可以做)

- Phase 3.3: 布局配置增强（模板、版本对比）
- Phase 4.2: 函数选择器增强（推荐引擎）
- Phase 5.1: 布局预览增强（点击高亮）
- Phase 6.2: 审计与追溯
- Phase 6.4: 文档与帮助

### P3 (长期目标)

- Phase 5.2: 拖拽式布局编辑
- Phase 5.3: 画布式编辑

---

## 附录：风险与依赖

### 风险

1. **架构重构风险**: 拆分 TabEditor 可能引入新 bug，需要充分测试
2. **Schema 体系统一风险**: 迁移旧数据可能出现兼容性问题
3. **拖拽交互风险**: 复杂拖拽场景可能出现性能问题

### 依赖

1. **后端 API**: 需要后端支持统一的 schema 存储和版本管理
2. **权限系统**: 需要后端提供细粒度的编辑权限控制
3. **审计日志**: 需要后端提供审计日志存储和查询接口

---

## 附录：成功指标

### 用户体验指标

- 函数 UI 配置时间减少 50%（通过一键导入、模板等）
- 配置错误率降低 70%（通过校验、预览等）
- 用户满意度 ≥ 4.5/5

### 技术指标

- TabEditor 主文件代码行数 ≤ 500 行
- 单元测试覆盖率 ≥ 80%
- 配置保存成功率 ≥ 99.9%

### 业务指标

- 函数 UI 配置完成率 ≥ 90%（有 UI 配置的函数占比）
- 配置复用率 ≥ 30%（使用模板的配置占比）
- 配置变更回滚率 ≤ 5%

---

**文档版本**: v1.0 **创建时间**: 2026-03-10 **负责人**: [待定] **审核人**: [待定]
