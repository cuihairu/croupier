# Workspace 开发指南

## 1. 架构设计

### 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                     Workspace 架构                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────────┐          ┌──────────────────┐        │
│  │  WorkspaceConfig │          │  Layout Engine   │        │
│  │  (配置层)         │  ────►   │  (渲染层)         │        │
│  │                  │          │                  │        │
│  │  - objectKey     │          │  - Renderer      │        │
│  │  - title         │          │  - TabsLayout    │        │
│  │  - layout        │          │  - FormDetail    │        │
│  │                  │          │  - List          │        │
│  └──────────────────┘          │  - Form          │        │
│         │                      │  - Detail        │        │
│         │                      └──────────────────┘        │
│         ▼                              │                    │
│  ┌──────────────────┐                 ▼                    │
│  │  Config Service  │          ┌──────────────────┐        │
│  │  (服务层)         │          │  Components      │        │
│  │                  │          │  (组件层)         │        │
│  │  - load          │          │                  │        │
│  │  - save          │          │  - WorkspaceRenderer     │
│  │  - validate      │          │  - TabContentRenderer    │
│  │  - cache         │          │  - FieldRenderer │        │
│  └──────────────────┘          │  - ColumnRenderer│        │
│                                └──────────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

### 核心模块

#### 1. 类型定义 (src/types/)

- **workspace.ts**: Workspace 配置类型
- **layout.ts**: Layout Engine 相关类型

#### 2. 配置服务 (src/services/)

- **workspaceConfig.ts**: 配置加载、保存、验证、缓存

#### 3. 渲染器 (src/components/WorkspaceRenderer/)

- **index.tsx**: 主渲染器
- **TabsLayout.tsx**: Tabs 布局
- **TabContentRenderer.tsx**: Tab 内容渲染器
- **renderers/**: 各类布局渲染器
  - FormDetailRenderer.tsx
  - ListRenderer.tsx
  - FormRenderer.tsx
  - DetailRenderer.tsx

#### 4. 编排器 (src/pages/WorkspaceEditor/)

- **index.tsx**: 编排器主页面
- **components/**: 编排器组件
  - FunctionList.tsx
  - LayoutDesigner.tsx
  - TabEditor.tsx
  - ConfigPreview.tsx

### 数据流

```
用户操作
  │
  ▼
WorkspaceEditor (编排器)
  │
  ├─► FunctionList (拖拽函数)
  │
  ├─► LayoutDesigner (设计布局)
  │     │
  │     └─► TabEditor (编辑 Tab)
  │
  ├─► ConfigPreview (实时预览)
  │     │
  │     └─► WorkspaceRenderer (渲染)
  │
  └─► saveWorkspaceConfig (保存)
        │
        ▼
      API Server
        │
        ▼
      Database
```

## 2. 核心概念

### WorkspaceConfig

描述 Workspace 的完整配置，包括标题、布局、权限等。

```typescript
interface WorkspaceConfig {
  objectKey: string; // 对象标识
  title: string; // 显示标题
  description?: string; // 描述
  icon?: string; // 图标
  layout: WorkspaceLayout; // 布局配置
  permissions?: string[]; // 权限列表
  meta?: WorkspaceMeta; // 元数据
}
```

### Layout Engine

根据配置动态渲染界面的核心引擎。

**工作流程**：

1. 加载 WorkspaceConfig
2. 根据 layout.type 选择布局组件
3. 渲染布局组件
4. 布局组件根据配置渲染具体内容

### 渲染器

每种布局类型对应一个渲染器：

- **FormDetailRenderer**: 表单-详情布局
- **ListRenderer**: 列表布局
- **FormRenderer**: 表单布局
- **DetailRenderer**: 详情布局

## 3. 扩展开发

### 添加新的布局类型

#### 步骤 1: 定义类型

在 `src/types/workspace.ts` 中添加新的布局类型：

```typescript
// 添加到 TabLayout 类型
export type TabLayout =
  | FormDetailLayout
  | ListLayout
  | FormLayout
  | DetailLayout
  | CustomLayout
  | NewLayout; // 新增

// 定义新布局类型
export interface NewLayout {
  type: 'new';
  // 添加布局特有的配置字段
  newField: string;
}
```

#### 步骤 2: 创建渲染器

创建 `src/components/WorkspaceRenderer/renderers/NewRenderer.tsx`：

```typescript
import React from 'react';
import type { NewLayout } from '@/types/workspace';

export interface NewRendererProps {
  layout: NewLayout;
  objectKey: string;
  context?: Record<string, any>;
}

export default function NewRenderer({ layout, objectKey, context }: NewRendererProps) {
  // 实现渲染逻辑
  return <div>{/* 渲染内容 */}</div>;
}
```

#### 步骤 3: 注册渲染器

在 `src/components/WorkspaceRenderer/TabContentRenderer.tsx` 中注册：

```typescript
import NewRenderer from './renderers/NewRenderer';

export default function TabContentRenderer({ tab, objectKey, context }: TabContentRendererProps) {
  const { layout } = tab;

  switch (layout.type) {
    case 'new':
      return <NewRenderer layout={layout} objectKey={objectKey} context={context} />;

    // ... 其他类型
  }
}
```

#### 步骤 4: 添加编排器支持

在 `src/pages/WorkspaceEditor/components/TabEditor.tsx` 中添加配置界面：

```typescript
function renderLayoutConfig(layout: any, onChange: (layout: any) => void): React.ReactNode {
  switch (layout.type) {
    case 'new':
      return renderNewConfig(layout, onChange);

    // ... 其他类型
  }
}

function renderNewConfig(layout: any, onChange: (layout: any) => void): React.ReactNode {
  return (
    <Form layout="vertical">
      <Form.Item label="新字段">
        <Input
          value={layout.newField}
          onChange={(e) => onChange({ ...layout, newField: e.target.value })}
        />
      </Form.Item>
    </Form>
  );
}
```

### 添加新的渲染方式

#### 步骤 1: 定义渲染类型

在类型定义中添加新的渲染类型：

```typescript
export interface ColumnConfig {
  key: string;
  title: string;
  render?: 'text' | 'status' | 'datetime' | 'custom' | 'newRender'; // 新增
  // ...
}
```

#### 步骤 2: 实现渲染函数

在渲染器中添加渲染逻辑：

```typescript
function renderColumn(col: any, text: any, record: any): React.ReactNode {
  switch (col.render) {
    case 'newRender':
      return renderNewRender(text, col.renderOptions);

    // ... 其他类型
  }
}

function renderNewRender(value: any, options: any): React.ReactNode {
  // 实现新的渲染逻辑
  return <span>{value}</span>;
}
```

### 自定义组件

#### 创建自定义组件

```typescript
// src/components/CustomWorkspace/MyCustomComponent.tsx
import React from 'react';

export interface MyCustomComponentProps {
  config: any;
  context?: Record<string, any>;
}

export default function MyCustomComponent({ config, context }: MyCustomComponentProps) {
  return <div>{/* 自定义内容 */}</div>;
}
```

#### 使用自定义组件

在配置中指定自定义组件：

```json
{
  "type": "custom",
  "component": "MyCustomComponent",
  "props": {
    "customProp": "value"
  }
}
```

## 4. API 文档

### 配置服务 API

#### loadWorkspaceConfig

加载 Workspace 配置。

```typescript
function loadWorkspaceConfig(
  objectKey: string,
  options?: {
    forceRefresh?: boolean;
    useCache?: boolean;
  },
): Promise<WorkspaceConfig | null>;
```

**参数**：

- `objectKey`: 对象标识
- `options.forceRefresh`: 是否强制刷新（跳过缓存）
- `options.useCache`: 是否使用缓存

**返回**：

- 成功：WorkspaceConfig
- 不存在：null

**示例**：

```typescript
const config = await loadWorkspaceConfig('player');
const freshConfig = await loadWorkspaceConfig('player', { forceRefresh: true });
```

#### saveWorkspaceConfig

保存 Workspace 配置。

```typescript
function saveWorkspaceConfig(config: WorkspaceConfig): Promise<WorkspaceConfig>;
```

**参数**：

- `config`: Workspace 配置

**返回**：

- 保存后的配置

**示例**：

```typescript
const savedConfig = await saveWorkspaceConfig({
  objectKey: 'player',
  title: '玩家管理',
  layout: { ... }
});
```

#### validateWorkspaceConfig

验证 Workspace 配置。

```typescript
function validateWorkspaceConfig(config: WorkspaceConfig): {
  valid: boolean;
  errors: string[];
};
```

**参数**：

- `config`: Workspace 配置

**返回**：

- `valid`: 是否有效
- `errors`: 错误信息列表

**示例**：

```typescript
const result = validateWorkspaceConfig(config);
if (!result.valid) {
  console.error('配置验证失败:', result.errors);
}
```

### 渲染器 API

#### WorkspaceRenderer

主渲染器组件。

```typescript
interface WorkspaceRendererProps {
  config: WorkspaceConfig | null;
  loading?: boolean;
  error?: string;
  context?: Record<string, any>;
}
```

**示例**：

```typescript
<WorkspaceRenderer config={config} loading={loading} error={error} context={{ userId: '123' }} />
```

#### useWorkspaceConfig

使用 Workspace 配置的 Hook。

```typescript
function useWorkspaceConfig(objectKey: string): {
  config: WorkspaceConfig | null;
  loading: boolean;
  error?: string;
  reload: () => void;
};
```

**示例**：

```typescript
const { config, loading, error, reload } = useWorkspaceConfig('player');
```

### 工具函数

#### getIcon

根据图标名称获取图标组件。

```typescript
function getIcon(iconName?: string): React.ReactNode;
```

#### renderField

渲染表单字段。

```typescript
function renderField(field: FieldConfig): React.ReactNode;
```

#### renderColumn

渲染列内容。

```typescript
function renderColumn(col: ColumnConfig, text: any, record: any): React.ReactNode;
```

## 5. 测试

### 单元测试

使用 Jest 和 React Testing Library 进行单元测试。

**示例**：

```typescript
import { render } from '@testing-library/react';
import WorkspaceRenderer from '@/components/WorkspaceRenderer';

describe('WorkspaceRenderer', () => {
  it('should render tabs layout', () => {
    const config = {
      objectKey: 'test',
      title: '测试',
      layout: {
        type: 'tabs',
        tabs: [
          {
            key: 'tab1',
            title: 'Tab 1',
            functions: [],
            layout: { type: 'list' },
          },
        ],
      },
    };

    const { getByText } = render(<WorkspaceRenderer config={config} />);
    expect(getByText('Tab 1')).toBeInTheDocument();
  });
});
```

### 集成测试

测试完整的工作流程。

**示例**：

```typescript
describe('Workspace Integration', () => {
  it('should create and render workspace', async () => {
    // 1. 创建配置
    const config = await saveWorkspaceConfig({ ... });

    // 2. 加载配置
    const loaded = await loadWorkspaceConfig(config.objectKey);

    // 3. 渲染
    const { getByText } = render(<WorkspaceRenderer config={loaded} />);

    // 4. 验证
    expect(getByText(config.title)).toBeInTheDocument();
  });
});
```

### E2E 测试

使用 Playwright 或 Cypress 进行端到端测试。

**示例**：

```typescript
test('should edit workspace config', async ({ page }) => {
  // 1. 访问编排器
  await page.goto('/workspace-editor/player');

  // 2. 添加 Tab
  await page.click('text=添加 Tab');
  await page.fill('input[name="title"]', '新 Tab');
  await page.click('text=确定');

  // 3. 保存配置
  await page.click('text=保存配置');

  // 4. 验证
  await expect(page.locator('text=保存成功')).toBeVisible();
});
```

## 6. 部署

### 构建

```bash
# 构建生产版本
npm run build

# 输出到 dist/ 目录
```

### 环境变量

```bash
# .env.production
CROUPIER_API_URL=https://api.croupier.io
CROUPIER_WS_URL=wss://api.croupier.io
```

### Docker 部署

```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/nginx.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

## 7. 性能优化

### 配置缓存

配置会自动缓存 5 分钟，减少 API 调用。

```typescript
// 使用缓存
const config = await loadWorkspaceConfig('player');

// 强制刷新
const freshConfig = await loadWorkspaceConfig('player', { forceRefresh: true });
```

### 懒加载

渲染器组件使用懒加载：

```typescript
const FormDetailRenderer = lazy(() => import('./renderers/FormDetailRenderer'));
```

### 虚拟滚动

大列表使用虚拟滚动：

```typescript
import { FixedSizeList } from 'react-window';
```

### React.memo

缓存组件，避免不必要的重渲染：

```typescript
export default React.memo(WorkspaceRenderer);
```

## 8. 调试技巧

### 查看配置

在编排器中切换到"代码"视图，查看配置 JSON。

### 使用 React DevTools

安装 React DevTools 浏览器扩展，查看组件树和 props。

### 网络请求

使用浏览器开发者工具的 Network 面板，查看 API 请求和响应。

### 控制台日志

在关键位置添加日志：

```typescript
console.log('Config loaded:', config);
console.log('Rendering layout:', layout.type);
```

## 9. 常见问题

### 配置不生效

1. 检查配置是否保存成功
2. 清除缓存重新加载
3. 检查配置格式是否正确

### 渲染器报错

1. 检查布局类型是否正确
2. 检查必填字段是否缺失
3. 查看控制台错误信息

### 性能问题

1. 减少列数
2. 启用分页
3. 使用虚拟滚动
4. 优化渲染逻辑

## 10. 贡献指南

### 代码规范

- 使用 TypeScript
- 遵循 ESLint 规则
- 使用 Prettier 格式化
- 编写单元测试

### 提交规范

```
feat(workspace): 添加新功能
fix(workspace): 修复 Bug
docs(workspace): 更新文档
refactor(workspace): 重构代码
test(workspace): 添加测试
```

### Pull Request

1. Fork 仓库
2. 创建特性分支
3. 提交更改
4. 创建 Pull Request
5. 等待审查

## 11. 参考资源

- [React 文档](https://react.dev/)
- [Ant Design 文档](https://ant.design/)
- [Pro Components 文档](https://procomponents.ant.design/)
- [TypeScript 文档](https://www.typescriptlang.org/)
