# 第三方运营平台接入架构设计

## 概述

本文档描述 Croupier Server 中可扩展的第三方运营平台接入架构，支持动态配置和启用多个运营平台（如 QuickSDK、未来的其他平台）。

## 架构设计

### 1. 目录结构

```
server/
├── internal/
│   └── platform/
│       ├── provider/           # Provider 接口定义
│       │   ├── provider.go     # Provider 接口
│       │   ├── config.go       # Provider 配置
│       │   └── registry.go     # Provider 注册表
│       ├── quicksdk/           # QuickSDK 实现
│       │   ├── client.go       # HTTP 客户端
│       │   ├── sign.go         # 签名算法
│       │   ├── api.go          # 20个 API 实现
│       │   └── provider.go     # QuickSDK Provider 实现
│       └── service/            # Platform 服务
│           └── platform.go     # 统一接入服务
├── configs/
│   └── platforms.example.yaml  # 平台配置示例
└── proto/
    └── platform/
        └── v1/
            └── platform.proto  # Platform gRPC 定义
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
    Init(config ProviderConfig) error

    // IsEnabled 检查是否启用
    IsEnabled() bool

    // Call 调用平台 API
    Call(ctx context.Context, method string, request, response interface{}) error

    // GetSupportedMethods 返回支持的方法列表
    GetSupportedMethods() []string

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

## 未来扩展

### 专用 Provider

添加新的专用平台 Provider（如 QuickSDK）需要：

1. 实现 `Provider` 接口
2. 在配置文件中添加配置
3. Registry 自动发现和加载

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

### OpenAPI 通用 Provider

对于不提供 SDK 但有 HTTP API 的服务器，可以使用 **OpenAPI Provider** 快速接入，无需编写代码。

#### 使用场景

- 游戏服管理 API
- 内部管理后台
- 第三方 OpenAPI/Swagger 服务
- 快速原型验证

#### 配置示例

```yaml
platforms:
  # 方式1：手动定义方法
  game_server:
    enabled: true
    type: openapi
    config:
      base_url: "http://game-server.example.com:8081"
      auth:
        type: bearer
        token: "${GAME_SERVER_TOKEN}"
      timeout: 30s
      retry_count: 3
      methods:
        - name: get_role
          path: "/api/role/get"
          method: POST
          request_body:
            type: json
            fields:
              user_id: user_id
        - name: ban_user
          path: "/api/user/ban"
          method: POST
    rate_limit:
      requests_per_minute: 60
      burst_size: 10

  # 方式2：自动发现（从 OpenAPI/Swagger 文档）
  game_server_auto:
    enabled: true
    type: openapi
    config:
      base_url: "http://game-server.example.com:8081"
      openapi_spec: "http://game-server.example.com:8081/openapi.json"
      auth:
        type: api_key
        api_key:
          name: "X-API-Key"
          value: "${API_KEY}"
          in: "header"
```

#### 支持的认证方式

| 类型 | 说明 | 配置示例 |
|------|------|----------|
| `none` | 无认证 | `type: none` |
| `bearer` | Bearer Token | `type: bearer; token: "xxx"` |
| `basic` | HTTP Basic | `type: basic; username: "xxx"; password: "xxx"` |
| `api_key` | API Key | `type: api_key; api_key: {name: "X-Key", value: "xxx", in: "header"}` |
| `custom` | 自定义 Header | `type: custom; custom_headers: {X-Token: "xxx"}` |

#### 方法映射配置

```yaml
methods:
  - name: send_mail
    description: "发送游戏内邮件"
    path: "/api/mail/send"
    method: POST
    # 路径参数
    parameters:
      - name: user_id
        in: path    # path, query, header
        from: user_id
        required: true
      - name: server_id
        in: query
        from: server_id
    # 请求体映射
    request_body:
      type: json              # json, form, text
      template: '{"to": "{{ .user_id }}", "title": "{{ .title }}"}'
      # 或使用 fields 映射
      fields:
        to: user_id
        title: title
        content: content
    # 响应转换
    response_mapping:
      extract_path: "data.items"
      wrap: true
```

#### 调用示例

```bash
# 调用游戏服 API
curl -X POST http://croupier-server/api/v1/platform/call \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "game_server",
    "method": "get_role",
    "request": {
      "user_id": "12345"
    }
  }'
```

#### OpenAPI Provider 目录结构

```
internal/platform/openapi/
├── provider.go       # OpenAPI Provider 实现
├── parser.go         # OpenAPI/Swagger 规范解析器
└── transformer.go    # 请求/响应转换器
```
