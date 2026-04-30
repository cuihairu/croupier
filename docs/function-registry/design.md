# 函数注册系统重构设计

## 概述

本文档描述 Croupier 函数注册系统的重构设计。核心目标是**严格分离函数注册（SDK）和 UI 生成（Dashboard）**，同时提供 OpenAPI 兼容层用于快速集成第三方 API。

## 设计原则

1. **职责分离**
   - SDK 只负责注册函数元数据
   - Dashboard 根据元数据生成 UI
   - 两者通过清晰的 API 边界交互

2. **存储格式统一**
   - 核心使用 Protobuf 定义元数据
   - JSON Schema 用于参数校验
   - OpenAPI 作为可选导入/导出格式

3. **向后兼容**
   - 保留现有 `LocalFunctionDescriptor` 和 `FunctionDescriptor`
   - 新系统与旧系统并存，逐步迁移

## 架构分层

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Dashboard (Web UI)                                │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  UI Generator Layer (独立模块)                                   │    │
│  │  - 读取 FunctionMetadata                                         │    │
│  │  - 生成 Formily Schema                                           │    │
│  │  - 生成菜单、路由、权限配置                                       │    │
│  └─────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────┘
                                    ↑ HTTP REST
┌─────────────────────────────────────────────────────────────────────────┐
│                          Server / Agent                                 │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  Function Registry (核心)                                         │    │
│  │  - 存储 FunctionMetadata                                          │    │
│  │  - 索引、路由、生命周期                                            │    │
│  │  - 提供 REST API                                                   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  OpenAPI Compatibility Layer                                      │    │
│  │  - OpenAPI Spec → FunctionMetadata                                │    │
│  │  - FunctionMetadata → OpenAPI Spec                                │    │
│  └─────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────┘
                                    ↑ TCP Session
┌─────────────────────────────────────────────────────────────────────────┐
│                              SDK                                        │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  Function Registration API                                         │    │
│  │  - registerFunction(metadata)                                     │    │
│  │  - registerFromOpenAPI(spec)                                      │    │
│  │  - Builder 模式构建元数据                                           │    │
│  └─────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────┘
```

## 核心数据模型

### FunctionMetadata (新)

```protobuf
message FunctionMetadata {
  // 标识字段
  string id = 1;              // 唯一标识，格式: <domain>.<entity>.<action>
  string version = 2;         // 语义化版本

  // 分类字段
  string category = 3;        // 分类
  repeated string tags = 4;   // 标签

  // 文档字段
  string name = 5;            // 简短名称
  string description = 6;     // 详细描述

  // 参数定义 (JSON Schema)
  string input_schema = 7;
  string output_schema = 8;

  // 行为定义
  FunctionBehavior behavior = 9;

  // 安全定义
  FunctionSecurity security = 10;

  // 扩展字段
  map<string, string> extensions = 11;
}
```

**关键设计点：**
- **不包含 UI 相关字段**（如 menu、display_name、i18n 等）
- **不包含权限细节**（只保留 risk_level 和 permission）
- UI 配置完全由 Dashboard 根据元数据推导

### 与旧模型对比

| 字段 | LocalFunctionDescriptor | FunctionDescriptor | FunctionMetadata (新) |
|------|------------------------|-------------------|----------------------|
| id | ✓ | ✓ | ✓ |
| input_schema | ✓ | ✓ | ✓ |
| output_schema | ✓ | ✓ | ✓ |
| category (x-category) | ✓ | ✓ | ✓ |
| risk (x-risk) | ✓ | ✓ | ✓ |
| entity (x-entity) | ✓ | ✓ | - (推导) |
| operation (x-operation) | ✓ | ✓ | - (推导) |
| display_name | ✗ | ✓ (I18nText) | - (由 UI 生成) |
| menu | ✗ | ✓ | - (由 UI 生成) |
| permissions | ✗ | ✓ | 简化版 |
| summary | ✓ | ✓ (I18nText) | 简化版 |

## OpenAPI 兼容层

### 转换映射

| OpenAPI 字段 | FunctionMetadata 字段 |
|-------------|----------------------|
| operationId | id |
| summary | name |
| description | description |
| tags[0] | category |
| tags | tags |
| requestBody.content | input_schema |
| responses["200"] | output_schema |
| x-risk | security.risk_level |
| x-category | category |
| x-permission | security.permission |

### 使用场景

1. **导入第三方 API**
   ```go
   functions, _ := converter.ImportFromSpec(openAPIJSON, options)
   for _, fn := range functions {
       sdk.Register(ctx, fn)
   }
   ```

2. **导出 OpenAPI Spec**
   ```go
   spec := registry.ExportToOpenAPISpec()
   ```

## UI 生成层 (Dashboard)

### 推导规则

| UI 配置 | 推导规则 |
|---------|---------|
| 菜单路径 | `<category>/<inferred-action>/<id>` |
| 图标 | 根据 category 映射 |
| 操作类型 | 根据 id 后缀推导 (get/list→read, create→create, etc.) |
| 表单 Schema | 直接使用 input_schema 转换为 Formily |
| 权限 | `function.<id>` |

### 示例

```
函数 ID: player.ban
├─ 菜单路径: /player/management/ban
├─ 图标: UserOutlined
├─ 操作: manage, approve (if high risk)
└─ 权限: function.player.ban
```

## 文件结构

```
proto/
└── croupier/
    └── function/
        └── v1/
            └── metadata.proto          # 新的函数元数据定义

internal/
└── function/
    ├── openapi/                        # OpenAPI 兼容层
    │   ├── converter.go
    │   └── schema_mapper.go
    ├── registry/                       # 函数注册中心
    │   ├── registry.go
    │   ├── store.go
    │   └── indexer.go
    └── api/                            # REST API
        └── handler.go

sdks/go/
└── function/
    ├── registry.go                     # SDK 注册接口
    └── builder.go                      # Builder 模式

web/src/utils/
└── function-ui-generator.ts            # UI 生成器
```

## 迁移策略

1. **Phase 1**: 实现新的 Protobuf 定义和基础结构
2. **Phase 2**: 实现 OpenAPI 兼容层
3. **Phase 3**: 实现 SDK 注册 API
4. **Phase 4**: 实现 Dashboard UI 生成器
5. **Phase 5**: 逐步迁移现有函数

## 兼容性保证

- 保留 `LocalFunctionDescriptor` 和 `FunctionDescriptor`
- 提供转换函数在旧模型和新模型之间转换
- Agent 同时支持两种注册格式
