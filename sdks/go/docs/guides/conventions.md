# Croupier Go SDK 约定规范

本文档详细说明使用 Croupier Go SDK 时需要遵守的约定和规范。

## 目录

- [命名约定](#命名约定)
- [函数注册约定](#函数注册约定)
- [虚拟对象设计约定](#虚拟对象设计约定)
- [错误处理约定](#错误处理约定)
- [版本管理约定](#版本管理约定)
- [安全约定](#安全约定)
- [避让规则](#避让规则)

---

## 命名约定

### 函数 ID 命名

函数 ID 采用 `entity.operation` 格式，描述函数所属实体和执行的操作。

#### 格式规则

```
[namespace.]entity.operation
```

| 部分 | 说明 | 示例 |
|------|------|------|
| `namespace` (可选) | 命名空间，用于模块分组 | `game`, `inventory`, `chat` |
| `entity` | 实体名称，小写 | `player`, `item`, `guild` |
| `operation` | 操作名称，小写动词 | `get`, `create`, `update`, `delete`, `ban` |

#### 命名示例

```go
// ✅ 正确的函数 ID
"player.get"              // 获取玩家信息
"player.ban"              // 封禁玩家
"player.update_profile"   // 更新玩家资料
"item.create"             // 创建道具
"item.delete"             // 删除道具
"wallet.transfer"         // 钱包转账
"game.player.ban"         // 带命名空间
"inventory.item.add"      // 带命名空间

// ❌ 错误的函数 ID
"PlayerGet"               // 不要使用驼峰命名
"player-get"              // 不要使用连字符
"player_get"              // 不要使用下划线
"get_player"              // 实体应该在前
"player"                  // 缺少操作
""                        // 不能为空
```

### 实体命名

实体代表业务领域的对象，使用**小写单数名词**。

```go
// ✅ 推荐的实体名称
"player"      // 玩家
"item"        // 道具
"guild"       // 公会
"wallet"      // 钱包
"match"       // 比赛
"chat"        // 聊天

// ❌ 避免使用
"players"     // 不要使用复数
"Player"      // 不要使用大写
"player_data" // 不要使用下划线
```

### 操作命名

操作使用**小写动词**，表示对实体执行的动作。

#### CRUD 标准操作

| 操作 | 说明 | 适用场景 |
|------|------|----------|
| `create` | 创建新实体 | 新建玩家、创建道具 |
| `get` | 获取实体信息 | 查询玩家、查询道具 |
| `update` | 更新实体信息 | 修改玩家属性、更新道具状态 |
| `delete` | 删除实体 | 删除玩家、删除道具 |
| `list` | 列出实体集合 | 查询玩家列表、查询道具列表 |

#### 业务操作

| 操作 | 说明 | 示例 |
|------|------|------|
| `ban` | 封禁/禁用 | `player.ban` |
| `unban` | 解封/启用 | `player.unban` |
| `transfer` | 转移 | `wallet.transfer`, `item.transfer` |
| `add` | 添加 | `inventory.item.add`, `guild.member.add` |
| `remove` | 移除 | `inventory.item.remove`, `guild.member.remove` |
| `start` | 启动 | `match.start` |
| `end` | 结束 | `match.end` |
| `join` | 加入 | `match.join` |
| `leave` | 离开 | `match.leave` |

---

## 函数注册约定

### 注册时机

所有函数必须在调用 `Connect()` **之前**完成注册。

```go
client := croupier.NewClient(config)

// ✅ 正确：先注册，后连接
client.RegisterFunction(descriptor1, handler1)
client.RegisterFunction(descriptor2, handler2)
client.Connect()  // 连接时会将所有注册的函数上传到 Agent

// ❌ 错误：连接后不能再注册
client.Connect()
client.RegisterFunction(descriptor3, handler3)  // 无效，会失败
```

### 函数唯一性

同一个服务实例内，函数 ID 必须唯一。

```go
// ❌ 错误：重复注册相同 ID
desc1 := &croupier.FunctionDescriptor{
    ID:      "player.get",
    Version: "1.0.0",
}

desc2 := &croupier.FunctionDescriptor{
    ID:      "player.get",  // 与 desc1 相同
    Version: "1.0.0",
}

client.RegisterFunction(desc1, handler1)
client.RegisterFunction(desc2, handler2)  // 会覆盖或报错
```

### FunctionDescriptor 必填字段

```go
descriptor := &croupier.FunctionDescriptor{
    // ✅ 必填字段
    ID:      "player.get",  // 函数 ID，不能为空
    Version: "1.0.0",       // 版本号，不能为空

    // ⭐ 推荐字段
    Category:  "player",  // 业务分类
    Risk:      "low",     // 风险等级: low, medium, high
    Entity:    "player",  // 关联实体
    Operation: "read",    // 操作类型: create, read, update, delete
    Enabled:   true,      // 是否启用
}
```

### 风险等级 (Risk)

风险等级用于标识函数操作的影响范围和重要性。

| 等级 | 说明 | 示例 |
|------|------|------|
| `low` | 只读操作，无副作用 | `player.get`, `item.list` |
| `medium` | 有副作用但可逆 | `player.update`, `item.add` |
| `high` | 有重大影响或不可逆 | `player.delete`, `player.ban`, `wallet.transfer` |

```go
descriptor := &croupier.FunctionDescriptor{
    ID:      "player.ban",
    Version: "1.0.0",
    Risk:    "high",  // 封禁是高风险操作
}
```

---

## 虚拟对象设计约定

### 虚拟对象结构

虚拟对象将相关的 CRUD 操作组合在一起，形成完整的实体管理单元。

```go
player := &croupier.VirtualObjectDescriptor{
    // 基础信息
    ID:          "player",                    // 对象 ID
    Version:     "1.0.0",                     // 版本号
    Name:        "游戏玩家",                  // 显示名称
    Description: "管理玩家信息",              // 描述

    // Schema 定义（可选）
    Schema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "id":    map[string]string{"type": "string"},
            "name":  map[string]string{"type": "string"},
            "level": map[string]string{"type": "integer"},
        },
    },

    // 操作映射
    Operations: map[string]string{
        "create": "player.create",
        "read":   "player.get",
        "update": "player.update",
        "delete": "player.delete",
        "ban":    "player.ban",
        "unban":  "player.unban",
    },
}
```

### 标准操作映射

虚拟对象应尽可能支持标准 CRUD 操作：

| 操作 | 函数 ID | 说明 |
|------|---------|------|
| `create` | `{entity}.create` | 创建新实体 |
| `read` | `{entity}.get` | 读取实体信息 |
| `update` | `{entity}.update` | 更新实体信息 |
| `delete` | `{entity}.delete` | 删除实体 |

---

## 错误处理约定

### 函数处理器错误处理

函数处理器应该捕获所有异常并返回标准错误响应。

```go
func SafeHandler(ctx context.Context, payload []byte) string {
    // 1. 解析输入
    var data map[string]interface{}
    if err := json.Unmarshal(payload, &data); err != nil {
        return ErrorResponse("INVALID_JSON", err.Error())
    }

    // 2. 验证参数
    playerID, ok := data["player_id"].(string)
    if !ok || playerID == "" {
        return ErrorResponse("INVALID_PARAM", "player_id is required")
    }

    // 3. 执行业务逻辑
    result, err := processPlayer(playerID)
    if err != nil {
        return ErrorResponse("INTERNAL_ERROR", err.Error())
    }

    // 4. 返回成功响应
    return SuccessResponse(result)
}

func ErrorResponse(code, message string) string {
    resp := map[string]interface{}{
        "status":  "error",
        "code":    code,
        "message": message,
    }
    data, _ := json.Marshal(resp)
    return string(data)
}

func SuccessResponse(data interface{}) string {
    resp := map[string]interface{}{
        "status": "success",
        "data":   data,
    }
    bytes, _ := json.Marshal(resp)
    return string(bytes)
}
```

### 标准错误响应格式

所有错误响应应遵循统一格式：

```json
{
  "status": "error",
  "code": "ERROR_CODE",
  "message": "Human readable error message",
  "details": {}
}
```

### 常见错误码

| 错误码 | 说明 | HTTP 等价 |
|--------|------|----------|
| `INVALID_PARAM` | 参数错误 | 400 |
| `INVALID_JSON` | JSON 格式错误 | 400 |
| `UNAUTHORIZED` | 未授权 | 401 |
| `FORBIDDEN` | 无权限 | 403 |
| `NOT_FOUND` | 资源不存在 | 404 |
| `ALREADY_EXISTS` | 资源已存在 | 409 |
| `INTERNAL_ERROR` | 内部错误 | 500 |
| `SERVICE_UNAVAILABLE` | 服务不可用 | 503 |

---

## 版本管理约定

### 语义化版本

SDK 遵循语义化版本规范 (Semver)：`MAJOR.MINOR.PATCH`

| 部分 | 说明 | 示例 |
|------|------|------|
| `MAJOR` | 主版本号，不兼容的 API 变更 | 1.0.0 → 2.0.0 |
| `MINOR` | 次版本号，向后兼容的功能新增 | 1.0.0 → 1.1.0 |
| `PATCH` | 修订号，向后兼容的问题修正 | 1.0.0 → 1.0.1 |

### 版本兼容性

```go
// ✅ 兼容变更：增加 PATCH
// 1.0.0 → 1.0.1
descriptor.Version = "1.0.1"  // 修复 bug

// ✅ 兼容变更：增加 MINOR
// 1.0.0 → 1.1.0
descriptor.Version = "1.1.0"  // 新增可选参数

// ❌ 不兼容变更：增加 MAJOR
// 1.0.0 → 2.0.0
descriptor.Version = "2.0.0"  // 修改参数结构
```

---

## 安全约定

### 输入验证

所有函数处理器必须验证输入参数：

```go
func SecureHandler(ctx context.Context, payload []byte) string {
    // 1. 检查 payload 是否为空
    if len(payload) == 0 {
        return ErrorResponse("EMPTY_PAYLOAD", "payload is empty")
    }

    // 2. 验证 JSON 格式
    var data map[string]interface{}
    if err := json.Unmarshal(payload, &data); err != nil {
        return ErrorResponse("INVALID_JSON", err.Error())
    }

    // 3. 验证必需字段
    playerID, exists := data["player_id"]
    if !exists {
        return ErrorResponse("MISSING_PLAYER_ID", "player_id is required")
    }

    // 4. 类型检查
    playerIDStr, ok := playerID.(string)
    if !ok {
        return ErrorResponse("INVALID_PLAYER_ID_TYPE", "player_id must be string")
    }

    // 5. 值范围检查
    if len(playerIDStr) == 0 || len(playerIDStr) > 64 {
        return ErrorResponse("INVALID_PLAYER_ID_VALUE", "player_id length must be 1-64")
    }

    // 6. 业务逻辑
    return ProcessRequest(data)
}
```

### TLS 配置

生产环境必须启用 TLS：

```go
config := &croupier.ClientConfig{
    AgentAddr:  "agent.croupier.io:443",
    Insecure:   false,  // 生产环境必须为 false
    CertFile:   "/etc/tls/client.crt",
    KeyFile:    "/etc/tls/client.key",
    CAFile:     "/etc/tls/ca.crt",
    ServerName: "agent.croupier.io",
}
```

---

## 避让规则

### 函数注册避让

当多个服务实例注册相同函数时，Agent 会根据以下规则路由请求：

1. **版本优先级**：优先路由到高版本的函数
   ```
   player.get v2.0.0 > player.get v1.1.0 > player.get v1.0.0
   ```

2. **负载均衡**：同版本函数之间进行负载分配

3. **健康检查**：不健康的实例不会被路由

```go
// 设置合适的版本号
descriptor.Version = "2.0.0"  // 新版本会优先处理请求
```

### 服务优先级

通过 `Risk` 字段影响路由优先级：

```go
// 高风险函数会被更谨慎地路由
highRiskDesc := &croupier.FunctionDescriptor{
    ID:      "player.ban",
    Risk:    "high",
}

// 低风险函数可以更自由地负载均衡
lowRiskDesc := &croupier.FunctionDescriptor{
    ID:      "player.get",
    Risk:    "low",
}
```

### 优雅降级

当服务不可用时，返回降级响应而非错误：

```go
func GracefulDegradeHandler(ctx context.Context, payload []byte) string {
    result, err := ProcessRequest(payload)
    if err != nil {
        // 返回降级响应
        return `{"status":"degraded","message":"Service temporarily unavailable","cached":true}`
    }
    return result
}
```

---

## 完整示例

### 符合约定的完整实现

```go
package main

import (
    "context"
    "encoding/json"

    "github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

// PlayerHandlers 玩家管理处理器 - 符合所有约定
type PlayerHandlers struct{}

// Get 获取玩家信息 (low risk, read operation)
func (h *PlayerHandlers) Get(ctx context.Context, payload []byte) string {
    var data map[string]interface{}
    if err := json.Unmarshal(payload, &data); err != nil {
        return Error("INVALID_JSON", err.Error())
    }

    // 参数验证
    playerID, ok := data["player_id"].(string)
    if !ok || playerID == "" {
        return Error("MISSING_PARAM", "player_id is required")
    }

    // 业务逻辑
    result := map[string]interface{}{
        "id":     playerID,
        "name":   "Player One",
        "level":  50,
    }

    return Success(result)
}

// Update 更新玩家信息 (medium risk, update operation)
func (h *PlayerHandlers) Update(ctx context.Context, payload []byte) string {
    var data map[string]interface{}
    if err := json.Unmarshal(payload, &data); err != nil {
        return Error("INVALID_JSON", err.Error())
    }

    if _, ok := data["player_id"]; !ok {
        return Error("MISSING_PARAM", "player_id is required")
    }

    // 业务逻辑
    return Success(map[string]interface{}{
        "updated_fields": []string{"level", "exp"},
    })
}

// Ban 封禁玩家 (high risk, sensitive operation)
func (h *PlayerHandlers) Ban(ctx context.Context, payload []byte) string {
    var data map[string]interface{}
    if err := json.Unmarshal(payload, &data); err != nil {
        return Error("INVALID_JSON", err.Error())
    }

    // 高风险操作需要额外验证
    // 从 context 获取操作者信息并验证权限

    return Success(map[string]interface{}{
        "action":    "ban",
        "player_id": data["player_id"],
    })
}

func Error(code, message string) string {
    resp := map[string]interface{}{
        "status":  "error",
        "code":    code,
        "message": message,
    }
    data, _ := json.Marshal(resp)
    return string(data)
}

func Success(data interface{}) string {
    resp := map[string]interface{}{
        "status": "success",
        "data":   data,
    }
    bytes, _ := json.Marshal(resp)
    return string(bytes)
}

func main() {
    // 配置
    config := &croupier.ClientConfig{
        GameID:    "my-game",
        Env:       "production",
        ServiceID: "player-service",
        AgentAddr: "agent.croupier.io:443",
        Insecure:  false,  // 生产环境启用 TLS
    }

    client := croupier.NewClient(config)
    handlers := &PlayerHandlers{}

    // 注册函数 - 遵循命名约定
    getDesc := &croupier.FunctionDescriptor{
        ID:        "player.get",
        Version:   "1.0.0",
        Category:  "player",
        Risk:      "low",
        Entity:    "player",
        Operation: "read",
    }

    updateDesc := &croupier.FunctionDescriptor{
        ID:        "player.update",
        Version:   "1.0.0",
        Category:  "player",
        Risk:      "medium",
        Entity:    "player",
        Operation: "update",
    }

    banDesc := &croupier.FunctionDescriptor{
        ID:        "player.ban",
        Version:   "1.0.0",
        Category:  "player",
        Risk:      "high",
        Entity:    "player",
        Operation: "update",
    }

    // 在连接前注册所有函数
    client.RegisterFunction(getDesc, handlers.Get)
    client.RegisterFunction(updateDesc, handlers.Update)
    client.RegisterFunction(banDesc, handlers.Ban)

    // 连接并启动服务
    if err := client.Connect(); err != nil {
        panic(err)
    }

    client.Serve()
}
```
