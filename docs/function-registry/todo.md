# 函数注册系统重构 TODO

更新时间：2026-05-01

## 设计原则

1. **职责分离**
   - SDK 只负责注册函数元数据
   - Dashboard 根据元数据生成 UI
   - 两者通过清晰的 API 边界交互

2. **存储格式统一**
   - 核心使用 Protobuf 定义元数据
   - JSON Schema 用于参数校验
   - OpenAPI 作为可选导入/导出格式

## Phase 1: 新的 Protobuf 定义和基础结构

### 1.1 创建 FunctionMetadata Proto 定义
- [x] 创建 `proto/croupier/function/v1/metadata.proto`
- [x] 定义 `FunctionMetadata` 消息
- [x] 定义 `FunctionBehavior` 消息
- [x] 定义 `FunctionSecurity` 消息
- [x] 添加 buf.yaml 配置
- [x] 运行 `make proto` 生成代码

### 1.2 实现核心 Registry Store
- [x] 创建 `internal/function/registry/store.go`
- [x] 实现元数据存储接口
- [x] 实现索引功能（按 category、tags、id 查询）
- [x] 实现生命周期管理

### 1.3 实现 Registry 核心
- [x] 创建 `internal/function/registry/registry.go`
- [x] 实现 Register 函数
- [x] 实现 Get 函数
- [x] 实现 List 函数
- [x] 实现 Unregister 函数

## Phase 2: OpenAPI 兼容层

### 2.1 实现 Converter
- [x] 创建 `internal/function/openapi/converter.go`
- [x] 实现 `ImportFromSpec` (OpenAPI → FunctionMetadata)
- [x] 实现 `ExportToSpec` (FunctionMetadata → OpenAPI)

### 2.2 实现 Schema Mapper
- [x] 创建 `internal/function/openapi/schema_mapper.go`
- [x] 实现 OpenAPI Schema → JSON Schema 转换
- [x] 实现 JSON Schema → OpenAPI Schema 转换

## Phase 3: SDK 注册 API

### 3.1 Go SDK
- [x] 创建 `sdks/go/function/registry.go`
- [x] 实现 `registerFunction` API
- [x] 实现 `registerFromOpenAPI` API
- [x] 创建 Builder 模式 `sdks/go/function/builder.go`

### 3.2 其他语言 SDK
- [ ] Java SDK 实现
- [ ] C++ SDK 实现
- [ ] JS/TS SDK 实现
- [ ] Python SDK 实现
- [ ] C# SDK 实现

## Phase 4: Dashboard UI 生成器

### 4.1 UI Generator 实现
- [x] 创建 `web/src/utils/function-ui-generator.ts`
- [x] 创建 `web/src/utils/types.ts`
- [x] 实现菜单路径推导
- [x] 实现图标映射
- [x] 实现操作类型推导
- [x] 实现 Formily Schema 转换

### 4.2 REST API
- [x] 创建 `internal/function/api/handler.go`
- [x] 创建 `internal/function/api/dto.go`
- [x] 创建 `internal/function/api/service.go`
- [x] 实现 GET /api/v1/metadata/functions - 列出所有函数（支持过滤）
- [x] 实现 GET /api/v1/metadata/functions/:id - 获取单个函数
- [x] 实现 POST /api/v1/metadata/functions - 注册函数
- [x] 实现 PUT /api/v1/metadata/functions/:id - 更新函数
- [x] 实现 DELETE /api/v1/metadata/functions/:id - 删除函数
- [x] 实现 POST /api/v1/metadata/functions/import/openapi - 从 OpenAPI 导入
- [x] 实现 GET /api/v1/metadata/functions/categories - 获取分类列表
- [x] 实现 GET /api/v1/metadata/functions/tags - 获取标签列表
- [x] 集成到主路由 `internal/handler/routes.go`

## Phase 5: 迁移现有函数

### 5.1 兼容层
- [x] 创建 `internal/function/migration/converter.go`
- [x] `LocalFunctionDescriptor` → `FunctionMetadata` 转换
- [x] `FunctionDescriptor` → `FunctionMetadata` 转换
- [x] 双向转换函数 (Metadata → Legacy)
- [x] Agent 同时支持两种注册格式 (通过转换层)

### 5.2 逐步迁移
- [x] 迁移现有函数使用新格式
- [x] 更新文档
- [x] 更新示例

## 兼容性保证

- [x] 保留现有 `LocalFunctionDescriptor` 和 `FunctionDescriptor`
- [x] 提供双向转换函数
- [x] 确保 Agent 同时支持两种格式
