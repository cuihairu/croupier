# 第三方运营平台接入架构设计

## 概述

本文档描述 Croupier Server 中可扩展的第三方运营平台接入架构，支持动态配置和启用多个运营平台。

### 已支持的 Provider

| Provider | 类型 | 说明 | 状态 |
|----------|------|------|------|
| **QuickSDK** | 专用 | 游戏运营数据平台，20+ API | ✅ 已实现 |
| **OpenAPI** | 通用 | 任意 HTTP API，配置驱动 | ✅ 已实现 |

### 快速选择

- **有 SDK 的第三方平台** → 编写专用 Provider（如 QuickSDK）
- **只有 HTTP API 的服务器** → 使用 OpenAPI Provider，无需编码

## 架构设计

### 1. 目录结构

```
server/
├── internal/
│   └── platform/
│       ├── provider/           # Provider 接口定义
│       │   ├── provider.go     # Provider 接口
│       │   ├── registry.go     # Provider 注册表
│       ├── quicksdk/           # QuickSDK 实现
│       │   ├── client.go       # HTTP 客户端
│       │   ├── sign.go         # 签名算法
│       │   ├── api.go          # 20个 API 实现
│       │   └── provider.go     # QuickSDK Provider 实现
│       ├── openapi/            # OpenAPI 通用 Provider
│       │   └── provider.go     # OpenAPI Provider 实现
│       ├── ratelimit/          # 速率限制工具
│       │   └── tokenbucket.go  # 令牌桶实现
│       ├── loader.go           # 配置加载器
│       └── server.go           # Platform gRPC 服务
├── configs/
│   └── platforms.yaml          # 平台配置文件
└── pkg/pb/platform/v1/         # gRPC 生成代码
```

### 2. 核心 API 设计

#### 2.1 Provider 接口

```go
package provider

// Provider 定义第三方运营平台接口
type Provider interface {
    // Name 返回平台名称
    Name() string

    // Init 初始化 Provider
    Init(ctx context.Context, config ProviderConfig) error

    // IsEnabled 检查是否启用
    IsEnabled() bool

    // SupportedMethods 返回支持的方法列表
    SupportedMethods() []string

    // Call 调用平台 API
    Call(ctx context.Context, method string, request []byte) ([]byte, error)

    // Close 关闭 Provider
    Close() error
}

// ProviderConfig 平台配置
type ProviderConfig struct {
    Enabled   bool                   `yaml:"enabled" json:"enabled"`
    Type      string                 `yaml:"type" json:"type"`       // "quicksdk", "thinkingdata", etc.
    Config    map[string]interface{} `yaml:"config" json:"config"`   // 平台特定配置
    RateLimit *RateLimitConfig       `yaml:"rate_limit" json:"rate_limit"`
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
    RequestsPerMinute int `yaml:"requests_per_minute" json:"requests_per_minute"`
    BurstSize         int `yaml:"burst_size" json:"burst_size"`
}
```

#### 2.2 Registry 注册表

```go
// Registry 管理所有 Provider
type Registry struct {
    mu        sync.RWMutex
    providers map[string]Provider  // key: platform name
}

// Register 注册新的 Provider
func (r *Registry) Register(p Provider) error

// Unregister 注销 Provider
func (r *Registry) Unregister(name string) error

// Get 获取 Provider
func (r *Registry) Get(name string) (Provider, bool)

// List 列出所有 Provider
func (r *Registry) List() []Provider

// Call 调用指定平台的方法
func (r *Registry) Call(ctx context.Context, platform, method string, request, response interface{}) error
```

### 3. QuickSDK Provider 设计

#### 3.1 配置

```yaml
platforms:
  quicksdk:
    enabled: true
    type: quicksdk
    config:
      open_id: "${QUICKSDK_OPEN_ID}"
      open_key: "${QUICKSDK_OPEN_KEY}"
      api_base_url: "https://www.quicksdk.com"
      timeout: 30s
      retry_count: 3
      enable_cache: true
      cache_duration: 300s
    rate_limit:
      requests_per_minute: 1000
      burst_size: 100
```

#### 3.2 支持的方法

| 方法 | QuickSDK 接口 | 描述 |
|------|--------------|------|
| `channel_list` | open/channelList | 获取渠道列表 |
| `server_list` | open/serverList | 获取区服列表 |
| `product_list` | open/productList | 获取产品列表 |
| `role_info` | open/roleInfo | 获取角色信息 |
| `order_list` | open/orderList | 获取订单列表 |
| `day_report` | open/dayReport | 单日报表 |
| `day_hour_report` | open/dayHourReport | 每小时报表 |
| `user_live` | open/userLive | 玩家留存 |
| `channel_days_report` | open/channelDaysReport | 渠道报表 |
| `channel_report` | open/channelReport | 渠道日报 |
| `ad_report` | open/adReport | 广告效果报表 |
| `media_app_list` | open/getMediaApp | 广告主列表 |
| `ad_plan_group_list` | open/getAdPlanGroup | 广告分组列表 |
| `package_version_list` | open/getPackageVersion | 分包列表 |
| `ad_pages_list` | open/getAdPages | 落地页列表 |
| `create_ad_plan` | open/createAdPlan | 创建广告计划 |
| `update_ad_plan` | open/updateAdPlan | 更新广告计划 |
| `ad_plan_list` | open/getAdPlan | 广告计划列表 |
| `user_lost_list` | open/uwlLost | 流失预警 |
| `push_message` | open/pushMessage | 消息推送 |

### 4. gRPC API 定义

```protobuf
syntax = "proto3";

package croupier.platform.v1;

option go_package = "github.com/cuihairu/croupier/gen/go/croupier/platform/v1";

// Platform 服务
service PlatformService {
    // 调用第三方平台 API
    rpc CallPlatform(CallPlatformRequest) returns (CallPlatformResponse);

    // 获取支持的平台列表
    rpc ListPlatforms(ListPlatformsRequest) returns (ListPlatformsResponse);

    // 获取平台支持的方法列表
    rpc ListPlatformMethods(ListPlatformMethodsRequest) returns (ListPlatformMethodsResponse);
}

message CallPlatformRequest {
    string platform = 1;    // 平台名称: "quicksdk"
    string method = 2;      // 方法名: "day_report"
    bytes request = 3;      // 请求参数 (JSON)
}

message CallPlatformResponse {
    bytes response = 1;     // 响应数据 (JSON)
    string error = 2;       // 错误信息
}

message ListPlatformsRequest {}

message PlatformInfo {
    string name = 1;
    bool enabled = 2;
    repeated string methods = 3;
}

message ListPlatformsResponse {
    repeated PlatformInfo platforms = 1;
}

message ListPlatformMethodsRequest {
    string platform = 1;
}

message ListPlatformMethodsResponse {
    repeated string methods = 1;
}
```

### 5. 使用示例

#### 5.1 代码调用

```go
// 通过 Registry 调用
registry := platform.NewRegistry()
err := registry.Call(ctx, "quicksdk", "day_report", request, &response)

// 直接通过 Service 调用
client := platformv1.NewPlatformServiceClient(conn)
resp, err := client.CallPlatform(ctx, &platformv1.CallPlatformRequest{
    Platform: "quicksdk",
    Method:   "day_report",
    Request:  jsonRequest,
})
```

#### 5.2 HTTP API

```bash
POST /api/v1/platform/call
{
    "platform": "quicksdk",
    "method": "day_report",
    "request": {
        "productCode": "xxx",
        "bTime": 1704067200,
        "eTime": 1704153600
    }
}
```

## 实现计划

### Phase 1: 基础框架 (2-3 人日)
- [x] Provider 接口定义
- [x] Registry 实现
- [x] 配置加载
- [x] gRPC Proto 定义

### Phase 2: QuickSDK 实现 (4-5 人日)
- [x] HTTP 客户端 + 签名
- [x] 20 个 API 实现
- [x] 缓存支持
- [x] 速率限制

### Phase 3: 集成 (2-3 人日)
- [x] 集成到 Server
- [x] HTTP API 端点
- [x] 前端 UI 支持

### Phase 4: OpenAPI 通用 Provider (1-2 人日)
- [x] OpenAPI Provider 实现
- [x] 配置示例
- [x] 设计文档更新
- [ ] OpenAPI 规范自动发现完善

### Phase 5: 测试与文档 (1-2 人日)
- [ ] 单元测试
- [ ] 集成测试
- [ ] 使用文档

**已完成: Phase 1, Phase 2, Phase 3, Phase 4 (部分)**

## 已实现的 Provider

### 1. QuickSDK Provider

专用 Provider，对接 QuickSDK 游戏运营数据平台。

**配置：**
```yaml
platforms:
  quicksdk:
    enabled: true
    type: quicksdk
    config:
      open_id: "${QUICKSDK_OPEN_ID}"
      open_key: "${QUICKSDK_OPEN_KEY}"
      api_base_url: "https://www.quicksdk.com"
      timeout: 30s
      retry_count: 3
      enable_cache: true
      cache_duration: 300s
    rate_limit:
      requests_per_minute: 1000
      burst_size: 100
```

**支持的方法（20+）：**
| 分类 | 方法 |
|------|------|
| 基础数据 | `channel_list`, `server_list`, `product_list`, `role_info`, `order_list` |
| 运营报表 | `day_report`, `day_hour_report`, `user_live`, `channel_days_report`, `channel_report` |
| 广告管理 | `ad_report`, `media_app_list`, `ad_plan_group_list`, `create_ad_plan`, `update_ad_plan` |
| 其他 | `user_lost_list`, `push_message` |

### 2. OpenAPI Provider

通用 Provider，支持任意 HTTP API，无需编写代码。

**特点：**
- ✅ 配置驱动，无需编码
- ✅ 多种认证方式
- ✅ 灵活参数映射
- ✅ 自动发现 OpenAPI/Swagger 接口
- ✅ 请求/响应转换
- ✅ 内置重试和限流

详见下文「OpenAPI 通用 Provider」章节。

## 未来扩展

### 添加新的专用 Provider

需要编写代码的场景：

1. 实现 `Provider` 接口
2. 在 `loader.go` 中注册类型
3. 在配置文件中添加配置

```go
// 示例：添加 ThinkingData
type ThinkingDataProvider struct { ... }

func (t *ThinkingDataProvider) Name() string { return "thinkingdata" }
// ... 实现其他接口方法
```

```yaml
platforms:
  thinkingdata:
    enabled: true
    type: thinkingdata
    config:
      app_id: "xxx"
      app_key: "xxx"
      server_url: "https://xxx.thinkingdata.cn"
```

### OpenAPI 通用 Provider（推荐）

对于有 HTTP API 但无 SDK 的服务器，使用 OpenAPI Provider 无需编写代码。

---

## OpenAPI 通用 Provider 详解

### 概述

OpenAPI Provider 是一个通用 HTTP API 客户端，通过 YAML 配置即可接入任意 HTTP 服务。

**适用场景：**
- 游戏服务器管理 API
- 内部管理后台接口
- 第三方 OpenAPI/Swagger 服务
- 快速原型验证

**核心优势：**
- ✅ **零代码** - 纯配置驱动
- ✅ **灵活认证** - 支持 Bearer、Basic、API Key、自定义 Header
- ✅ **参数映射** - Path/Query/Header 参数灵活配置
- ✅ **请求转换** - 字段映射或 Go 模板
- ✅ **响应转换** - 提取指定字段、包装响应
- ✅ **自动发现** - 从 OpenAPI/Swagger 文档自动发现接口

### 快速开始

#### 1. 最简配置

```yaml
platforms:
  my_game_server:
    enabled: true
    type: openapi
    config:
      base_url: "http://my-game-server:8081"
      auth:
        type: bearer
        token: "my-secret-token"
      methods:
        - name: get_player
          path: "/api/player/get"
          method: POST
```

#### 2. 调用示例

```bash
# HTTP API 调用
curl -X POST http://croupier-server:8080/api/v1/platform/call \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "my_game_server",
    "method": "get_player",
    "request": {"player_id": "12345"}
  }'
```

### 配置参考

#### 完整配置结构

```yaml
platforms:
  <platform_name>:
    enabled: true
    type: openapi
    config:
      # 基础配置
      base_url: "http://api.example.com"          # 必填：API 基础 URL
      timeout: 30s                                 # 可选：请求超时
      retry_count: 3                               # 可选：重试次数

      # OpenAPI 规范（可选，用于自动发现）
      openapi_spec: "http://api.example.com/openapi.json"

      # 认证配置
      auth:
        type: bearer                               # none, bearer, basic, api_key, custom
        token: "${API_TOKEN}"                      # bearer 类型
        # username: "user"                         # basic 类型
        # password: "${PASSWORD}"
        # api_key:                                 # api_key 类型
        #   name: "X-API-Key"
        #   value: "${KEY}"
        #   in: "header"                           # header 或 query
        # custom_headers:                          # custom 类型
        #   X-Custom-Header: "value"

      # 默认请求头
      headers:
        User-Agent: "MyApp/1.0"

      # 响应转换（全局）
      transform:
        success_field: "code"                      # 成功字段
        success_value: 0                           # 成功值
        data_field: "data"                         # 数据字段
        error_field: "message"                     # 错误字段

      # 方法定义
      methods:
        - name: <method_name>                      # 方法名
          path: "/api/endpoint"                    # API 路径
          method: POST                             # HTTP 方法
          description: "Method description"        # 描述

          # 参数映射
          parameters:
            - name: param_name                     # API 参数名
              in: path                             # 位置: path, query, header
              from: input_field                    # 输入字段名（默认同 name）
              required: true                       # 是否必填
              default: "default_value"             # 默认值

          # 请求体配置
          request_body:
            type: json                             # 类型: json, form, text
            # 方式1：字段映射
            fields:
              dest_field: src_field
            # 方式2：Go 模板
            template: '{"field": "{{ .input }}"}'

          # 响应转换
          response_mapping:
            extract_path: "data.items"             # JSON 路径提取
            wrap: true                             # 包装响应

    # 速率限制
    rate_limit:
      requests_per_minute: 60
      burst_size: 10
```

#### 认证方式详解

| 类型 | 说明 | 配置 |
|------|------|------|
| `none` | 无认证 | `type: none` |
| `bearer` | Bearer Token | `type: bearer; token: "xxx"` |
| `basic` | HTTP Basic Auth | `type: basic; username: "xxx"; password: "xxx"` |
| `api_key` | API Key | `type: api_key; api_key: {name: "X-Key", value: "xxx", in: "header"}` |
| `custom` | 自定义 Header | `type: custom; custom_headers: {X-Token: "xxx"}` |

#### 参数位置详解

| 位置 | 说明 | 示例 |
|------|------|------|
| `path` | URL 路径参数 | `/api/users/{id}` 中的 `id` |
| `query` | URL 查询参数 | `/api/users?page=1` 中的 `page` |
| `header` | 请求头 | `X-Request-ID: xxx` |

#### 请求体配置详解

**字段映射方式：**
```yaml
request_body:
  type: json
  fields:
    apiField: inputField    # API 字段名: 输入字段名
    user_id: player_id
    server: server_id
```

**Go 模板方式：**
```yaml
request_body:
  type: json
  template: '{"user_id": "{{ .player_id }}", "action": "{{ .action }}"}'
```

#### 响应转换详解

```yaml
response_mapping:
  # 提取嵌套字段
  extract_path: "data.items"     # 从 {"data": {"items": [...]}} 提取 items

  # 包装响应
  wrap: true                     # 将响应包装为 {"data": <原始响应>}

  # 成功判断（配合全局 transform）
  success_field: "code"          # 检查 code 字段
  success_value: 0               # code == 0 表示成功
```

### 实际案例

#### 案例 1：游戏服玩家查询

```yaml
platforms:
  game_server:
    enabled: true
    type: openapi
    config:
      base_url: "http://game-server:8081"
      auth:
        type: bearer
        token: "${GAME_ADMIN_TOKEN}"
      methods:
        - name: get_player
          description: "获取玩家信息"
          path: "/api/player/info"
          method: POST
          request_body:
            type: json
            fields:
              player_id: player_id
              server_id: server_id
```

**调用：**
```json
{
  "platform": "game_server",
  "method": "get_player",
  "request": {
    "player_id": "12345",
    "server_id": "s1"
  }
}
```

#### 案例 2：游戏服封禁玩家

```yaml
platforms:
  game_server:
    config:
      methods:
        - name: ban_player
          description: "封禁玩家"
          path: "/api/player/ban"
          method: POST
          request_body:
            type: json
            template: '{"player_id": "{{ .player_id }}", "reason": "{{ .reason }}", "duration": {{ .duration }}}'
          response_mapping:
            extract_path: "data"
```

**调用：**
```json
{
  "platform": "game_server",
  "method": "ban_player",
  "request": {
    "player_id": "12345",
    "reason": "作弊",
    "duration": 86400
  }
}
```

#### 案例 3：带路径参数的 API

```yaml
platforms:
  game_server:
    config:
      methods:
        - name: get_guild
          path: "/api/guild/{guild_id}/members"
          method: GET
          parameters:
            - name: guild_id
              in: path
              from: guild_id
              required: true
            - name: page
              in: query
              from: page
              default: "1"
```

**调用：**
```json
{
  "platform": "game_server",
  "method": "get_guild",
  "request": {
    "guild_id": "guild_123",
    "page": "2"
  }
}
```

#### 案例 4：自动发现 OpenAPI 接口

如果服务提供 OpenAPI/Swagger 文档，可以自动发现接口：

```yaml
platforms:
  game_server:
    enabled: true
    type: openapi
    config:
      base_url: "http://game-server:8081"
      openapi_spec: "http://game-server:8081/openapi.json"
      auth:
        type: api_key
        api_key:
          name: "X-API-Key"
          value: "${API_KEY}"
          in: "header"
      # methods 留空，自动从 openapi_spec 发现
```

### 目录结构

```
internal/platform/openapi/
└── provider.go       # OpenAPI Provider 核心实现

internal/platform/ratelimit/
└── tokenbucket.go    # 令牌桶速率限制器
```

### 限制与注意事项

1. **安全性**：敏感信息（如 Token）应使用环境变量 `${VAR_NAME}`
2. **超时**：默认 30 秒，可根据需要调整
3. **重试**：仅对 5xx 错误自动重试
4. **速率限制**：建议根据服务端限流配置合适的 `requests_per_minute`

