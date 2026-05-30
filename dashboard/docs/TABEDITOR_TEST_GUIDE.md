# TabEditor 重构测试指南

## 快速测试步骤

### 1. 编译检查

```bash
cd C:/Users/cui/Workspaces/croupier/croupier-dashboard
npm run build
```

**预期结果**: 编译成功，无错误

### 2. 启动开发服务器

```bash
npm run dev
```

**预期结果**: 服务器启动成功，访问 http://localhost:8000

### 3. 功能测试清单

#### 3.1 基本信息编辑 ✓

- [ ] 修改 Tab 标题
- [ ] 选择 Tab 图标
- [ ] 切换"设为默认页"开关

#### 3.2 函数管理 ✓

- [ ] 从左侧拖拽函数到函数列表
- [ ] 点击"查看 JSON"按钮，查看函数详情
- [ ] 点击"界面向导"按钮，快速生成布局
- [ ] 删除函数

#### 3.3 布局类型选择 ✓

- [ ] 切换不同的布局类型（list、form、detail 等）
- [ ] 点击"应用推荐"按钮（如果有推荐场景）
- [ ] 点击"自动推导"按钮
- [ ] 点击"一键补全"按钮
- [ ] 点击"编排向导"按钮

#### 3.4 编排向导 ✓

- [ ] 打开编排向导 Modal
- [ ] 切换编排模式（标准列表、查询详情、主从分栏、仪表盘）
- [ ] 切换应用模式（覆盖当前布局、仅补空字段）
- [ ] 手动调整角色绑定
- [ ] 点击"重置为推荐"按钮
- [ ] 查看变更预览和风险提示
- [ ] 应用编排方案

#### 3.5 撤销/重做 ✓

- [ ] 应用编排后，点击"撤销编排"按钮
- [ ] 使用快捷键 Ctrl+Alt+Z 撤销
- [ ] 点击"重做编排"按钮
- [ ] 使用快捷键 Ctrl+Alt+Y 重做

#### 3.6 列表布局配置 ✓

- [ ] 选择列表函数
- [ ] 点击"自动填充"按钮
- [ ] 点击"添加列"按钮
- [ ] 编辑列（字段名、标题、渲染方式、列宽、可排序）
- [ ] 删除列

#### 3.7 表单布局配置 ✓

- [ ] 选择提交函数
- [ ] 点击"自动填充"按钮
- [ ] 点击"添加字段"按钮
- [ ] 编辑字段（字段名、标签、类型、必填、占位符）
- [ ] 删除字段

#### 3.8 查询详情布局配置 ✓

- [ ] 选择查询函数
- [ ] 配置查询字段
- [ ] 配置详情分区

#### 3.9 其他布局类型 ✓

- [ ] 详情布局（detail）
- [ ] 看板布局（kanban）
- [ ] 时间线布局（timeline）
- [ ] 主从分栏布局（split）
- [ ] 向导流程布局（wizard）
- [ ] 仪表盘布局（dashboard）
- [ ] 网格布局（grid）
- [ ] 自定义布局（custom）

---

## 常见问题排查

### 问题 1: 编译错误

**症状**: `npm run build` 失败

**可能原因**:

- Import 路径错误
- 类型定义不匹配
- 缺少依赖

**排查步骤**:

1. 检查错误信息中的文件路径
2. 检查 import 语句是否正确
3. 检查类型定义是否匹配

### 问题 2: 组件不显示

**症状**: 页面空白或组件不渲染

**可能原因**:

- 组件导出/导入错误
- Props 传递错误
- 条件渲染逻辑错误

**排查步骤**:

1. 打开浏览器开发者工具，查看 Console 错误
2. 检查 React DevTools，查看组件树
3. 检查 Props 是否正确传递

### 问题 3: 功能异常

**症状**: 某个功能不工作

**可能原因**:

- 事件处理函数未正确绑定
- 状态更新逻辑错误
- 回调函数参数错误

**排查步骤**:

1. 在相关函数中添加 `console.log` 调试
2. 检查事件处理函数是否被调用
3. 检查状态是否正确更新

### 问题 4: 编排向导异常

**症状**: 编排向导不工作或结果不正确

**可能原因**:

- 函数描述符数据不完整
- 角色绑定逻辑错误
- 布局生成逻辑错误

**排查步骤**:

1. 检查 `descriptors` 数组是否正确传递
2. 检查 `orchestrationUtils.ts` 中的逻辑
3. 检查 `buildOrchestrationLayout` 函数的返回值

---

## 性能测试

### 1. 渲染性能

**测试方法**:

1. 打开 React DevTools Profiler
2. 执行各种操作（切换布局类型、添加函数等）
3. 查看渲染时间和次数

**预期结果**:

- 单次操作渲染时间 < 100ms
- 无不必要的重复渲染

### 2. 内存占用

**测试方法**:

1. 打开浏览器开发者工具 Memory 面板
2. 执行各种操作
3. 查看内存占用变化

**预期结果**:

- 无明显内存泄漏
- 内存占用稳定

---

## 回归测试

### 1. 对比测试

**方法**: 使用旧版本和新版本对比测试

**步骤**:

1. 在旧版本中执行所有功能
2. 记录结果
3. 在新版本中执行相同操作
4. 对比结果是否一致

### 2. 边界测试

**测试场景**:

- [ ] 没有函数时的行为
- [ ] 只有一个函数时的行为
- [ ] 有大量函数时的行为（10+ 个）
- [ ] 切换布局类型时的数据保留
- [ ] 删除正在使用的函数时的行为

### 3. 异常测试

**测试场景**:

- [ ] 函数描述符数据缺失
- [ ] 布局配置数据损坏
- [ ] 网络请求失败
- [ ] 并发操作（快速点击）

---

## 自动化测试建议

### 1. 单元测试

**推荐工具**: Jest + React Testing Library

**测试文件结构**:

```
TabEditor/
├── __tests__/
│   ├── TabBasicInfo.test.tsx
│   ├── TabFunctionManager.test.tsx
│   ├── LayoutTypeSelector.test.tsx
│   ├── OrchestrationWizard.test.tsx
│   ├── ColumnEditorModal.test.tsx
│   ├── FieldEditorModal.test.tsx
│   ├── LayoutConfigRenderer.test.tsx
│   ├── orchestrationUtils.test.ts
│   ├── scenarioUtils.test.ts
│   ├── healLayoutUtils.test.ts
│   └── useOrchestrationHistory.test.ts
```

**测试覆盖目标**: ≥ 80%

### 2. 集成测试

**推荐工具**: Cypress 或 Playwright

**测试场景**:

- 完整的工作流测试（从添加函数到配置布局）
- 编排向导的完整流程
- 撤销/重做功能

### 3. E2E 测试

**推荐工具**: Cypress 或 Playwright

**测试场景**:

- 用户完整操作流程
- 跨页面交互
- 数据持久化

---

## 测试报告模板

### 测试信息

- **测试人员**: [姓名]
- **测试时间**: [日期]
- **测试环境**: [浏览器版本、操作系统]
- **测试版本**: [Git commit hash]

### 测试结果

| 功能模块     | 测试项   | 结果    | 备注 |
| ------------ | -------- | ------- | ---- |
| 基本信息编辑 | 修改标题 | ✅ / ❌ |      |
| 基本信息编辑 | 选择图标 | ✅ / ❌ |      |
| ...          | ...      | ...     | ...  |

### 发现的问题

| 问题编号 | 严重程度 | 问题描述 | 复现步骤 | 状态          |
| -------- | -------- | -------- | -------- | ------------- |
| BUG-001  | 高/中/低 | [描述]   | [步骤]   | 待修复/已修复 |

### 总结

- **通过率**: X%
- **严重问题数**: X
- **中等问题数**: X
- **轻微问题数**: X
- **建议**: [建议内容]

---

## 快速验证脚本

```bash
#!/bin/bash

echo "=== TabEditor 重构快速验证 ==="

echo "1. 检查文件是否存在..."
files=(
  "src/pages/WorkspaceEditor/components/TabEditor.tsx"
  "src/pages/WorkspaceEditor/components/TabEditor/TabBasicInfo.tsx"
  "src/pages/WorkspaceEditor/components/TabEditor/TabFunctionManager.tsx"
  "src/pages/WorkspaceEditor/components/TabEditor/LayoutTypeSelector.tsx"
  "src/pages/WorkspaceEditor/components/TabEditor/OrchestrationWizard.tsx"
  "src/pages/WorkspaceEditor/components/TabEditor/ColumnEditorModal.tsx"
  "src/pages/WorkspaceEditor/components/TabEditor/FieldEditorModal.tsx"
  "src/pages/WorkspaceEditor/components/TabEditor/LayoutConfigRenderer.tsx"
  "src/pages/WorkspaceEditor/components/TabEditor/orchestrationUtils.ts"
  "src/pages/WorkspaceEditor/components/TabEditor/scenarioUtils.ts"
  "src/pages/WorkspaceEditor/components/TabEditor/healLayoutUtils.ts"
  "src/pages/WorkspaceEditor/components/TabEditor/useOrchestrationHistory.ts"
  "src/pages/WorkspaceEditor/components/TabEditor/index.ts"
)

for file in "${files[@]}"; do
  if [ -f "$file" ]; then
    echo "✅ $file"
  else
    echo "❌ $file (缺失)"
  fi
done

echo ""
echo "2. 检查代码行数..."
old_lines=$(wc -l < "src/pages/WorkspaceEditor/components/TabEditor.old.tsx")
new_lines=$(wc -l < "src/pages/WorkspaceEditor/components/TabEditor.tsx")
reduction=$((100 - (new_lines * 100 / old_lines)))
echo "旧版本: $old_lines 行"
echo "新版本: $new_lines 行"
echo "减少: $reduction%"

echo ""
echo "3. 运行编译检查..."
npm run build

echo ""
echo "=== 验证完成 ==="
```

---

**文档版本**: v1.0 **创建时间**: 2026-03-10 **更新时间**: 2026-03-10
