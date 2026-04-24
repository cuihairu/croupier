# Workspace API 文档

## 配置管理 API

### 获取配置

获取指定对象的 Workspace 配置。

**请求**：

```http
GET /api/v1/workspaces/:objectKey/config
```

**参数**：

- `objectKey` (path): 对象标识，如 `player`、`order`

**响应**：

```json
{
  "objectKey": "player",
  "title": "玩家管理",
  "description": "玩家信息管理和操作",
  "icon": "UserOutlined",
  "layout": {
    "type": "tabs",
    "tabs": [
      {
        "key": "info",
        "title": "玩家信息",
        "icon": "InfoCircleOutlined",
        "functions": ["player.getInfo", "player.updateLevel"],
        "layout": {
          "type": "form-detail",
          "queryFunction": "player.getInfo",
          "queryFields": [
            {
              "key": "playerId",
              "label": "玩家ID",
              "type": "input",
              "required": true
            }
          ],
          "detailSections": [
            {
              "title": "基本信息",
              "fields": [
                { "key": "playerId", "label": "玩家ID" },
                { "key": "nickname", "label": "昵称" },
                { "key": "level", "label": "等级" }
              ]
            }
          ],
          "actions": [
            {
              "key": "updateLevel",
              "label": "更新等级",
              "function": "player.updateLevel",
              "type": "modal"
            }
          ]
        }
      }
    ]
  },
  "permissions": ["player.view"],
  "meta": {
    "createdAt": "2026-03-06T10:00:00Z",
    "updatedAt": "2026-03-06T12:00:00Z",
    "createdBy": "admin",
    "version": 1
  }
}
```

**错误响应**：

```json
{
  "error": "NOT_FOUND",
  "message": "配置不存在"
}
```

---

### 保存配置

保存或更新 Workspace 配置。

**请求**：

```http
PUT /api/v1/workspaces/:objectKey/config
Content-Type: application/json
```

**参数**：

- `objectKey` (path): 对象标识

**请求体**：

```json
{
  "objectKey": "player",
  "title": "玩家管理",
  "layout": {
    "type": "tabs",
    "tabs": [...]
  }
}
```

**响应**：

```json
{
  "objectKey": "player",
  "title": "玩家管理",
  "layout": {...},
  "meta": {
    "createdAt": "2026-03-06T10:00:00Z",
    "updatedAt": "2026-03-06T14:00:00Z",
    "updatedBy": "admin",
    "version": 2
  }
}
```

**错误响应**：

```json
{
  "error": "VALIDATION_ERROR",
  "message": "配置验证失败",
  "details": ["objectKey 不能为空", "title 不能为空"]
}
```

---

### 获取配置列表

获取所有 Workspace 配置列表。

**请求**：

```http
GET /api/v1/workspaces/configs
```

**查询参数**：

- `page` (query, optional): 页码，默认 1
- `pageSize` (query, optional): 每页数量，默认 20
- `search` (query, optional): 搜索关键词

**响应**：

```json
{
  "data": [
    {
      "objectKey": "player",
      "title": "玩家管理",
      "description": "玩家信息管理和操作",
      "icon": "UserOutlined",
      "meta": {
        "updatedAt": "2026-03-06T14:00:00Z"
      }
    },
    {
      "objectKey": "order",
      "title": "订单管理",
      "description": "订单查询和处理",
      "icon": "ShoppingOutlined",
      "meta": {
        "updatedAt": "2026-03-05T10:00:00Z"
      }
    }
  ],
  "total": 10,
  "page": 1,
  "pageSize": 20
}
```

---

### 删除配置

删除指定的 Workspace 配置。

**请求**：

```http
DELETE /api/v1/workspaces/:objectKey/config
```

**参数**：

- `objectKey` (path): 对象标识

**响应**：

```json
{
  "success": true,
  "message": "配置已删除"
}
```

**错误响应**：

```json
{
  "error": "NOT_FOUND",
  "message": "配置不存在"
}
```

---

### 克隆配置

克隆现有配置到新的对象。

**请求**：

```http
POST /api/v1/workspaces/:objectKey/clone
Content-Type: application/json
```

**参数**：

- `objectKey` (path): 源对象标识

**请求体**：

```json
{
  "targetKey": "player_test",
  "targetTitle": "玩家管理（测试）"
}
```

**响应**：

```json
{
  "objectKey": "player_test",
  "title": "玩家管理（测试）",
  "layout": {...},
  "meta": {
    "createdAt": "2026-03-06T15:00:00Z",
    "createdBy": "admin",
    "version": 1
  }
}
```

---

### 导出配置

导出配置为 JSON 文件。

**请求**：

```http
GET /api/v1/workspaces/:objectKey/export
```

**参数**：

- `objectKey` (path): 对象标识

**响应**：

```json
{
  "objectKey": "player",
  "title": "玩家管理",
  "layout": {...},
  "exportedAt": "2026-03-06T15:00:00Z",
  "exportedBy": "admin"
}
```

---

### 导入配置

从 JSON 文件导入配置。

**请求**：

```http
POST /api/v1/workspaces/import
Content-Type: application/json
```

**请求体**：

```json
{
  "config": {
    "objectKey": "player",
    "title": "玩家管理",
    "layout": {...}
  },
  "overwrite": false
}
```

**参数**：

- `config`: 配置 JSON
- `overwrite`: 是否覆盖已存在的配置

**响应**：

```json
{
  "success": true,
  "objectKey": "player",
  "message": "配置导入成功"
}
```

---

## 函数描述符 API

### 获取函数描述符列表

获取所有已注册的函数描述符。

**请求**：

```http
GET /api/v1/functions/descriptors
```

**查询参数**：

- `entity` (query, optional): 过滤实体，如 `player`
- `operation` (query, optional): 过滤操作类型，如 `list`、`query`

**响应**：

```json
{
  "data": [
    {
      "id": "player.getInfo",
      "entity": "player",
      "operation": "query",
      "display_name": {
        "zh": "获取玩家信息",
        "en": "Get Player Info"
      },
      "description": {
        "zh": "根据玩家ID获取玩家详细信息",
        "en": "Get player details by player ID"
      },
      "input_schema": {
        "type": "object",
        "properties": {
          "playerId": {
            "type": "string",
            "description": "玩家ID"
          }
        },
        "required": ["playerId"]
      },
      "output_schema": {
        "type": "object",
        "properties": {
          "playerId": { "type": "string" },
          "nickname": { "type": "string" },
          "level": { "type": "integer" },
          "gold": { "type": "integer" }
        }
      }
    },
    {
      "id": "player.list",
      "entity": "player",
      "operation": "list",
      "display_name": {
        "zh": "玩家列表",
        "en": "Player List"
      },
      "input_schema": {
        "type": "object",
        "properties": {
          "page": { "type": "integer", "default": 1 },
          "pageSize": { "type": "integer", "default": 10 }
        }
      },
      "output_schema": {
        "type": "object",
        "properties": {
          "data": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "playerId": { "type": "string" },
                "nickname": { "type": "string" },
                "level": { "type": "integer" }
              }
            }
          },
          "total": { "type": "integer" }
        }
      }
    }
  ],
  "total": 50
}
```

---

### 调用函数

调用指定的函数。

**请求**：

```http
POST /api/v1/functions/:functionId/invoke
Content-Type: application/json
```

**参数**：

- `functionId` (path): 函数 ID，如 `player.getInfo`

**请求体**：

```json
{
  "playerId": "123456"
}
```

**响应**：

```json
{
  "playerId": "123456",
  "nickname": "张三",
  "level": 50,
  "gold": 10000,
  "vipLevel": 3,
  "createdAt": "2026-01-01T00:00:00Z"
}
```

**错误响应**：

```json
{
  "error": "FUNCTION_ERROR",
  "message": "玩家不存在",
  "code": "PLAYER_NOT_FOUND"
}
```

---

## 前端 API

### loadWorkspaceConfig

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

**示例**：

```typescript
import { loadWorkspaceConfig } from '@/services/workspaceConfig';

// 使用缓存
const config = await loadWorkspaceConfig('player');

// 强制刷新
const freshConfig = await loadWorkspaceConfig('player', { forceRefresh: true });

// 不使用缓存
const noCache = await loadWorkspaceConfig('player', { useCache: false });
```

---

### saveWorkspaceConfig

保存 Workspace 配置。

```typescript
function saveWorkspaceConfig(config: WorkspaceConfig): Promise<WorkspaceConfig>;
```

**示例**：

```typescript
import { saveWorkspaceConfig } from '@/services/workspaceConfig';

const config: WorkspaceConfig = {
  objectKey: 'player',
  title: '玩家管理',
  layout: {
    type: 'tabs',
    tabs: [...]
  }
};

const savedConfig = await saveWorkspaceConfig(config);
```

---

### listWorkspaceConfigs

获取配置列表。

```typescript
function listWorkspaceConfigs(): Promise<WorkspaceConfig[]>;
```

**示例**：

```typescript
import { listWorkspaceConfigs } from '@/services/workspaceConfig';

const configs = await listWorkspaceConfigs();
console.log('配置数量:', configs.length);
```

---

### deleteWorkspaceConfig

删除配置。

```typescript
function deleteWorkspaceConfig(objectKey: string): Promise<void>;
```

**示例**：

```typescript
import { deleteWorkspaceConfig } from '@/services/workspaceConfig';

await deleteWorkspaceConfig('player');
```

---

### validateWorkspaceConfig

验证配置。

```typescript
function validateWorkspaceConfig(config: WorkspaceConfig): {
  valid: boolean;
  errors: string[];
};
```

**示例**：

```typescript
import { validateWorkspaceConfig } from '@/services/workspaceConfig';

const result = validateWorkspaceConfig(config);
if (!result.valid) {
  console.error('验证失败:', result.errors);
}
```

---

### cloneWorkspaceConfig

克隆配置。

```typescript
function cloneWorkspaceConfig(
  sourceKey: string,
  targetKey: string,
  targetTitle: string,
): Promise<WorkspaceConfig>;
```

**示例**：

```typescript
import { cloneWorkspaceConfig } from '@/services/workspaceConfig';

const cloned = await cloneWorkspaceConfig('player', 'player_test', '玩家管理（测试）');
```

---

### exportWorkspaceConfig

导出配置。

```typescript
function exportWorkspaceConfig(objectKey: string): Promise<string>;
```

**示例**：

```typescript
import { exportWorkspaceConfig } from '@/services/workspaceConfig';

const json = await exportWorkspaceConfig('player');
// 保存到文件
const blob = new Blob([json], { type: 'application/json' });
const url = URL.createObjectURL(blob);
const a = document.createElement('a');
a.href = url;
a.download = 'player-config.json';
a.click();
```

---

### importWorkspaceConfig

导入配置。

```typescript
function importWorkspaceConfig(configJson: string): Promise<WorkspaceConfig>;
```

**示例**：

```typescript
import { importWorkspaceConfig } from '@/services/workspaceConfig';

// 从文件读取
const file = event.target.files[0];
const text = await file.text();
const imported = await importWorkspaceConfig(text);
```

---

### WorkspaceRenderer

Workspace 渲染器组件。

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
import WorkspaceRenderer from '@/components/WorkspaceRenderer';

<WorkspaceRenderer config={config} loading={loading} error={error} context={{ userId: '123' }} />;
```

---

### useWorkspaceConfig

使用配置的 Hook。

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
import { useWorkspaceConfig } from '@/components/WorkspaceRenderer';

function MyComponent() {
  const { config, loading, error, reload } = useWorkspaceConfig('player');

  if (loading) return <Spin />;
  if (error) return <Alert message={error} type="error" />;

  return <WorkspaceRenderer config={config} />;
}
```

---

## 错误代码

| 错误代码            | 说明         |
| ------------------- | ------------ |
| `NOT_FOUND`         | 配置不存在   |
| `VALIDATION_ERROR`  | 配置验证失败 |
| `FUNCTION_ERROR`    | 函数调用失败 |
| `PERMISSION_DENIED` | 权限不足     |
| `INTERNAL_ERROR`    | 内部错误     |

---

## 数据类型

### WorkspaceConfig

```typescript
interface WorkspaceConfig {
  objectKey: string;
  title: string;
  description?: string;
  icon?: string;
  layout: WorkspaceLayout;
  permissions?: string[];
  meta?: WorkspaceMeta;
}
```

### WorkspaceLayout

```typescript
interface WorkspaceLayout {
  type: 'tabs' | 'sections' | 'wizard' | 'dashboard';
  tabs?: TabConfig[];
  sections?: SectionConfig[];
}
```

### TabConfig

```typescript
interface TabConfig {
  key: string;
  title: string;
  icon?: string;
  functions: string[];
  layout: TabLayout;
  defaultActive?: boolean;
  permissions?: string[];
}
```

### TabLayout

```typescript
type TabLayout = FormDetailLayout | ListLayout | FormLayout | DetailLayout | CustomLayout;
```

完整类型定义请参考 [src/types/workspace.ts](../src/types/workspace.ts)。

---

## 版本历史

### v1.0.0 (2026-03-06)

- 初始版本
- 支持配置管理 API
- 支持函数调用 API
- 支持前端配置服务

---

## 相关文档

- [用户指南](./WORKSPACE_USER_GUIDE.md)
- [开发指南](./WORKSPACE_DEV_GUIDE.md)
- [架构设计](../ARCHITECTURE_DESIGN.md)
