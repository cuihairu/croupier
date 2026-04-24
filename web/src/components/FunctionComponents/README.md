# Function Components 使用指南

本文档介绍如何使用 Croupier 函数管理系统中的通用组件。

## 快速开始

### 安装和导入

```tsx
// 导入所有组件
import {
  FunctionListTable,
  FunctionDetailPanel,
  FunctionFormRenderer,
  FunctionCallHistory,
  RegistryViewer,
} from '@/components/FunctionComponents';

// 或者单独导入
import FunctionListTable from './FunctionListTable';
import { FunctionItem } from './FunctionListTable';
```

## 组件详解

### 1. FunctionListTable - 函数列表表格

用于展示函数列表，支持搜索、过滤、排序和多选。

#### 基本用法

```tsx
import { FunctionListTable, FunctionItem } from '@/components/FunctionComponents';

const MyFunctionList = () => {
  const [functions, setFunctions] = useState<FunctionItem[]>([]);
  const [loading, setLoading] = useState(false);

  const handleRefresh = async () => {
    setLoading(true);
    try {
      const data = await fetchFunctions();
      setFunctions(data);
    } finally {
      setLoading(false);
    }
  };

  const handleInvoke = (func: FunctionItem) => {
    // 处理函数调用
    console.log('Invoke function:', func.id);
  };

  const handleViewDetail = (func: FunctionItem) => {
    // 查看函数详情
    console.log('View detail:', func.id);
  };

  return (
    <FunctionListTable
      data={functions}
      loading={loading}
      onRefresh={handleRefresh}
      onInvoke={handleInvoke}
      onViewDetail={handleViewDetail}
      showActions={{ view: true, invoke: true }}
      pagination={{ pageSize: 10 }}
      searchable={true}
      filters={true}
    />
  );
};
```

#### 高级配置

```tsx
<FunctionListTable
  data={functions}
  loading={loading}
  // 分页配置
  pagination={{
    current: 1,
    pageSize: 20,
    total: 100,
    showSizeChanger: true,
    showQuickJumper: true,
  }}
  // 操作按钮配置
  showActions={{
    view: true,
    invoke: true,
    edit: true,
    delete: false,
    toggle: true,
  }}
  // 多选配置
  selectable={true}
  onSelectionChange={(selectedRows) => {
    console.log('Selected:', selectedRows);
  }}
  // 紧凑模式
  compact={false}
  // 事件处理
  onEdit={(func) => editFunction(func)}
  onDelete={(func) => deleteFunction(func)}
  onToggleStatus={(func) => toggleFunctionStatus(func)}
/>
```

### 2. FunctionDetailPanel - 函数详情面板

用于展示函数的详细信息，包括基本信息、参数定义、调用历史等。

#### 基本用法

```tsx
import { FunctionDetailPanel } from '@/components/FunctionComponents';

const MyFunctionDetail = ({ functionId }: { functionId: string }) => {
  const [functionDetail, setFunctionDetail] = useState(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchFunctionDetail(functionId).then(setFunctionDetail);
  }, [functionId]);

  return (
    <FunctionDetailPanel
      function={functionDetail}
      loading={loading}
      showActions={true}
      onInvoke={() => handleInvoke()}
      onEdit={() => handleEdit()}
      onToggleStatus={() => handleToggle()}
      compact={false}
    />
  );
};
```

#### 紧凑模式

```tsx
<FunctionDetailPanel
  function={functionDetail}
  compact={true} // 紧凑模式，减少占用空间
  showActions={false} // 隐藏操作按钮
/>
```

### 3. FunctionFormRenderer - 动态表单渲染器

基于 JSON Schema 自动渲染表单，支持复杂的验证和条件渲染。

#### 基本用法

```tsx
import { FunctionFormRenderer, JSONSchema } from '@/components/FunctionComponents';

const MyFunctionForm = ({ schema }: { schema: JSONSchema }) => {
  const handleSubmit = (values: any) => {
    console.log('Form submitted:', values);
    // 执行函数调用
  };

  const handleChange = (changedFields: any, allValues: any) => {
    console.log('Form changed:', allValues);
  };

  return (
    <FunctionFormRenderer
      schema={schema}
      onSubmit={handleSubmit}
      onChange={handleChange}
      loading={false}
      disabled={false}
      showValidationErrors={true}
      validateTrigger="onChange"
      submitText="执行函数"
      resetText="重置表单"
      showReset={true}
    />
  );
};
```

#### 使用 UI Schema 自定义表单

```tsx
const uiSchema = {
  fields: {
    username: {
      widget: 'input',
      placeholder: '请输入用户名',
      description: '用户名必须是唯一的',
    },
    age: {
      widget: 'number',
      min: 18,
      max: 100,
    },
    preferences: {
      'ui:layout': {
        type: 'grid',
        cols: 2,
      },
    },
  },
  'ui:groups': [
    {
      title: '基本信息',
      fields: ['username', 'email', 'age'],
    },
    {
      title: '偏好设置',
      fields: ['theme', 'language', 'notifications'],
    },
  ],
};

<FunctionFormRenderer schema={schema} uiSchema={uiSchema} onSubmit={handleSubmit} />;
```

### 4. FunctionCallHistory - 调用历史

以时间线形式展示函数调用历史，支持查看详情和重新运行。

#### 基本用法

```tsx
import { FunctionCallHistory } from '@/components/FunctionComponents';

const MyCallHistory = ({ functionId }: { functionId: string }) => {
  const handleRefresh = () => {
    console.log('Refreshing call history');
  };

  const handleViewDetail = (call) => {
    console.log('View call detail:', call);
  };

  const handleRerun = (call) => {
    console.log('Rerun call:', call);
  };

  return (
    <FunctionCallHistory
      functionId={functionId}
      limit={20}
      showRefresh={true}
      compact={false}
      onRefresh={handleRefresh}
      onViewDetail={handleViewDetail}
      onRerun={handleRerun}
    />
  );
};
```

#### 自动刷新模式

```tsx
<FunctionCallHistory
  functionId={functionId}
  autoRefresh={true}
  refreshInterval={30000} // 30秒刷新一次
/>
```

### 5. RegistryViewer - 注册表查看器

监控服务注册表，展示服务健康状态和函数覆盖率。

#### 基本用法

```tsx
import { RegistryViewer } from '@/components/FunctionComponents';

const MyRegistryView = () => {
  const handleServiceClick = (service) => {
    console.log('Service clicked:', service);
  };

  const handleRefresh = (services) => {
    console.log('Registry refreshed:', services);
  };

  return (
    <RegistryViewer
      gameId="game-123"
      showStats={true}
      showHealthCheck={true}
      autoRefresh={true}
      refreshInterval={60000}
      compact={false}
      onServiceClick={handleServiceClick}
      onRefresh={handleRefresh}
    />
  );
};
```

## 工具函数

### 格式化工具

```tsx
import {
  formatDuration,
  formatTimestamp,
  formatRelativeTime,
  formatPercentage,
} from '@/components/FunctionComponents/utils/formatters';

// 格式化持续时间
const duration = formatDuration(5432); // "5.43s"

// 格式化时间戳
const timestamp = formatTimestamp('2023-12-01T10:30:00Z'); // "2023/12/1 18:30:00"

// 格式化相对时间
const relative = formatRelativeTime('2023-12-01T10:30:00Z'); // "2小时前"

// 格式化百分比
const percentage = formatPercentage(85.678); // "85.7%"
```

### 验证工具

```tsx
import {
  validateFunctionParams,
  validateJSONSchema,
  validateFunctionId,
} from '@/components/FunctionComponents/utils/validators';

// 验证函数参数
const paramsValidation = validateFunctionParams(params, schema);
if (!paramsValidation.valid) {
  console.error('Validation errors:', paramsValidation.errors);
}

// 验证JSON Schema
const schemaValidation = validateJSONSchema(schema);
if (!schemaValidation.valid) {
  console.error('Schema errors:', schemaValidation.errors);
}

// 验证函数ID
const isValidFunctionId = validateFunctionId('my-function'); // true
```

### 常量

```tsx
import {
  FUNCTION_EXECUTION_STATUS,
  COLOR_THEME,
  DEFAULT_CONFIG,
} from '@/components/FunctionComponents/utils/constants';

// 使用状态常量
const status = FUNCTION_EXECUTION_STATUS.SUCCESS;

// 使用颜色主题
const successColor = COLOR_THEME.STATUS.SUCCESS;

// 使用默认配置
const pageSize = DEFAULT_CONFIG.PAGINATION.DEFAULT_PAGE_SIZE;
```

## 最佳实践

### 1. 数据获取和缓存

```tsx
// 使用自定义Hook处理数据获取
const useFunctions = (gameId?: string) => {
  const [functions, setFunctions] = useState<FunctionItem[]>([]);
  const [loading, setLoading] = useState(false);

  const fetchFunctions = useCallback(async () => {
    setLoading(true);
    try {
      const data = await getFunctionSummary({ game_id: gameId });
      setFunctions(data);
    } finally {
      setLoading(false);
    }
  }, [gameId]);

  useEffect(() => {
    fetchFunctions();
  }, [fetchFunctions]);

  return { functions, loading, refresh: fetchFunctions };
};
```

### 2. 错误处理

```tsx
const MyComponent = () => {
  const [error, setError] = useState<string>('');

  const handleError = (err: any) => {
    const message = formatError(err);
    setError(message);
    message.error(message);
  };

  const handleAction = async () => {
    try {
      await someAsyncAction();
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div>
      {error && <Alert message={error} type="error" closable />}
      {/* 组件内容 */}
    </div>
  );
};
```

### 3. 响应式设计

```tsx
import { useBreakpoint } from 'antd';

const MyResponsiveTable = () => {
  const screens = useBreakpoint();

  const getColumns = () => {
    const baseColumns = [
      /* 基础列配置 */
    ];

    if (screens.md) {
      return [...baseColumns /* 桌面端专用列 */];
    }

    return baseColumns;
  };

  return (
    <FunctionListTable
      columns={getColumns()}
      compact={!screens.md}
      // 其他配置
    />
  );
};
```

### 4. 性能优化

```tsx
import { useMemo, useCallback } from 'react';

const OptimizedComponent = ({ functions }: { functions: FunctionItem[] }) => {
  // 使用useMemo缓存计算结果
  const processedFunctions = useMemo(() => {
    return functions.map((func) => ({
      ...func,
      displayName: func.displayName?.zh || func.id,
      categoryName: func.category || '未分类',
    }));
  }, [functions]);

  // 使用useCallback缓存事件处理函数
  const handleInvoke = useCallback((func: FunctionItem) => {
    invokeFunction(func.id);
  }, []);

  return (
    <FunctionListTable
      data={processedFunctions}
      onInvoke={handleInvoke}
      // 其他配置
    />
  );
};
```

## 类型定义

所有组件都提供了完整的 TypeScript 类型定义：

```tsx
import type {
  FunctionItem,
  FunctionDetail,
  FunctionCall,
  RegistryService,
  JSONSchema,
  FormUISchema,
} from '@/components/FunctionComponents';
```

## 故障排除

### 常见问题

1. **组件不显示**：检查导入路径是否正确
2. **类型错误**：确保传入的数据符合接口定义
3. **性能问题**：使用 React.memo 和 useMemo 优化渲染
4. **API 错误**：检查网络请求和权限设置

### 调试技巧

```tsx
// 开发环境下启用调试
if (process.env.NODE_ENV === 'development') {
  console.log('Component props:', props);
  console.log('Form values:', form.getFieldsValue());
}
```

## 更新日志

### v1.0.0 (当前版本)

- ✅ 5 个核心组件完成
- ✅ 完整的 TypeScript 支持
- ✅ 工具函数和常量
- ✅ 详细的使用文档

### 计划中的功能

- 🔄 WebSocket 实时更新
- 🔄 更多图表组件
- 🔄 无障碍访问支持
- 🔄 国际化扩展

## 贡献指南

欢迎提交 Issue 和 Pull Request 来改进这些组件。

### 开发环境设置

```bash
# 安装依赖
npm install

# 启动开发服务器
npm run dev

# 运行测试
npm test

# 构建生产版本
npm run build
```

### 代码规范

- 使用 TypeScript 编写类型安全的代码
- 遵循 ESLint 和 Prettier 配置
- 编写单元测试覆盖新功能
- 更新文档说明 API 变更
