# Croupier SDK 缺失功能清单

本文档汇总了各个 SDK 相对于规范和最佳实践的缺失功能，以及需要改进的地方。

**最后更新:** 2025-01-06 - 重试机制已完成

## 目录

- [功能对比矩阵](#功能对比矩阵)
- [Go SDK](#go-sdk)
- [JavaScript/TypeScript SDK](#javascripttypescript-sdk)
- [Python SDK](#python-sdk)
- [Java SDK](#java-sdk)
- [C++ SDK](#c-sdk)
- [跨 SDK 统一改进项](#跨-sdk-统一改进项)

---

## 功能对比矩阵

| 功能类别 | Go SDK | JS SDK | Python SDK | Java SDK | C++ SDK |
|---------|:------:|:------:|:----------:|:--------:|:-------:|
| **核心接口** ||||||
| Client 接口 | ✅ | ✅ | ✅ | ✅ | ✅ |
| Invoker 接口 | ✅ | ✅ | ✅ | ✅ | ✅ |
| **函数注册** ||||||
| 函数描述符 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 版本管理 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 元数据支持 | ✅ | ✅ | ✅ | ✅ | ✅ |
| JSON Schema 验证 | ✅ | ✅ | ✅ | ✅ | ✅ |
| **连接管理** ||||||
| 连接建立 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 自动重连 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 指数退避 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 会话管理 | ✅ | ✅ | ✅ | ✅ | ✅ |
| **生命周期** ||||||
| Start/Stop | ✅ | ✅ | ✅ | ✅ | ✅ |
| 优雅关闭 | ✅ | ✅ | ✅ | ✅ | ✅ |
| **配置选项** ||||||
| ClientConfig | ✅ | ✅ | ✅ | ✅ | ✅ |
| InvokerConfig | ✅ | ✅ | ✅ | ✅ | ✅ |
| TLS/mTLS | ✅ | ✅ | ✅ | ✅ | ✅ |
| **错误处理** ||||||
| 错误类型 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 重试机制 | ✅ | ✅ | ✅ | ✅ | ✅ |
| **特殊功能** ||||||
| 心跳机制 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 幂等性支持 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 作业管理 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 流式事件 | ✅ | ✅ | ✅ | ✅ | ✅ |
| **高级功能** ||||||
| 文件传输 | ❌ | ❌ | ❌ | ❌ | ❌ |
| 虚拟对象系统 | ❌ | ❌ | ❌ | ❌ | ✅ |
| 组件系统 | ❌ | ❌ | ❌ | ❌ | ✅ |
| **代码质量** ||||||
| 单元测试 | ✅ | ⚠️ | ✅ | ✅ | ✅ |
| 集成测试 | ⚠️ | ❌ | ⚠️ | ⚠️ | ✅ |
| 文档完整性 | ✅ | ✅ | ✅ | ✅ | ✅ |

---

## Go SDK

**位置:** `sdks/go/`

> ✅ **更新:** 自动重连已于 2025-01-06 完成实现

### 已实现功能

| 功能 | 状态 | 说明 |
|------|:----:|------|
| **Invoker 接口** | ✅ | 完整实现 |
| **幂等性支持** | ✅ | `InvokeOptions` 包含 `IdempotencyKey` |
| **流式事件** | ✅ | 使用 channel 流式获取作业事件 |
| **作业取消** | ✅ | 支持 `CancelJob()` 方法 |
| **Schema 设置** | ✅ | 支持 `SetSchema()` 方法 |
| **JSON Schema 验证** | ✅ | 使用 `gojsonschema` 完整实现 |
| **自动重连** | ✅ | 指数退避 + 抖动重连策略 |

### 缺失功能

| 功能 | 优先级 | 说明 |
|------|--------|------|
| **文件传输功能** | 低 | `uploadFile` 接口未实现 |

### 重试机制说明

> ✅ **更新:** 重试机制已于 2025-01-06 完成实现

Go SDK 的重试机制包含以下特性：

| 特性 | 说明 |
|------|------|
| **指数退避** | 每次重试延迟按倍数增长 |
| **抖动** | 添加随机性防止惊群效应 |
| **可重试状态码** | UNAVAILABLE, INTERNAL, UNKNOWN, ABORTED, DEADLINE_EXCEEDED |
| **可配置** | 通过 `RetryConfig` 配置所有参数 |
| **调用级覆盖** | `InvokeOptions.retry` 可覆盖默认配置 |

### 代码示例 - 重试机制

```go
import "github.com/cuihairu/croupier/sdks/go/pkg/croupier"

// 创建 Invoker（带重试配置）
config := &croupier.InvokerConfig{
    Address: "127.0.0.1:8080",
    Retry: &croupier.RetryConfig{
        Enabled:           true,
        MaxAttempts:       3,              // 最多重试 3 次
        InitialDelayMs:    100,            // 初始延迟 100ms
        MaxDelayMs:        5000,           // 最大延迟 5 秒
        BackoffMultiplier: 2.0,            // 指数退避倍数
        JitterFactor:      0.1,            // 抖动因子
    },
}

invoker := croupier.NewInvoker(config)
err := invoker.Connect(ctx)

// Invoke 会在失败时自动重试
result, err := invoker.Invoke(ctx, "player.ban", payload)
```

### 需要改进

| 项目 | 当前状态 | 建议改进 |
|------|----------|----------|
| 集成测试 | 较少 | 增加端到端测试 |

### 代码示例 - 自动重连配置

```go
// 自动重连已实现
import "github.com/cuihairu/croupier/sdks/go/pkg/croupier"

// 创建 Invoker（带自动重连配置）
config := &croupier.InvokerConfig{
    Address:        "127.0.0.1:8080",
    TimeoutSeconds: 30,
    Insecure:       true,
    Reconnect: &croupier.ReconnectConfig{
        Enabled:           true,
        MaxAttempts:       0,        // 0 = 无限重试
        InitialDelayMs:    1000,     // 初始延迟 1 秒
        MaxDelayMs:        30000,    // 最大延迟 30 秒
        BackoffMultiplier: 2.0,      // 指数退避倍数
        JitterFactor:      0.2,      // 抖动因子
    },
}

invoker := croupier.NewInvoker(config)
err := invoker.Connect(ctx)
// 连接断开会自动重连

// 或使用默认配置
invoker := croupier.NewInvoker(nil)  // 使用默认配置（包含自动重连）
```

---

## JavaScript/TypeScript SDK

**位置:** `sdks/js/`

> ✅ **更新:** 自动重连和重试机制已于 2025-01-06 完成实现

### 已实现功能

| 功能 | 状态 | 说明 |
|------|:----:|------|
| **Invoker 接口** | ✅ | 完整实现 |
| **幂等性支持** | ✅ | `InvokeOptions` 包含 `idempotencyKey` |
| **流式事件** | ✅ | 使用 AsyncGenerator 流式获取作业事件 |
| **作业取消** | ✅ | 支持 `cancelJob()` 方法 |
| **Schema 设置** | ✅ | 支持 `setSchema()` 方法 |
| **Schema 验证** | ✅ | 完整的 JSON Schema 验证 |
| **自动重连** | ✅ | 指数退避 + 抖动重连策略 |
| **重试机制** | ✅ | `executeWithRetry` + 指数退避 + 抖动 |

### 缺失功能

| 功能 | 优先级 | 说明 |
|------|--------|------|
| **文件传输功能** | 低 | 未实现 |

### 代码示例 - 重试机制

```typescript
// 重试机制已实现
import { Invoker, InvokerConfig, RetryConfig } from '@croupier/sdk';

// 配置重试策略
const invoker = new Invoker({
  address: '127.0.0.1:8080',
  retry: {
    enabled: true,
    maxAttempts: 3,           // 最多重试 3 次
    initialDelayMs: 100,      // 初始延迟 100ms
    maxDelayMs: 5000,         // 最大延迟 5 秒
    backoffMultiplier: 2,     // 指数退避倍数
    jitterFactor: 0.1,        // 抖动因子
  },
});

await invoker.connect();

// Invoke 会在失败时自动重试
await invoker.invoke('player.ban', payload);

// 单次调用可覆盖重试配置
await invoker.invoke('player.ban', payload, {
  retry: { enabled: false },  // 本次调用不重试
});
```

### 需要改进

| 项目 | 当前状态 | 建议改进 |
|------|----------|----------|
| 测试覆盖 | 基础测试 | 需要增加更多单元测试 |
| 错误处理 | 基础 | 需要更详细的错误分类 |
| 文档示例 | 较少 | 需要更多使用场景示例 |

### 代码示例 - Schema 验证

```typescript
// Schema 验证已实现
import { Invoker } from '@croupier/sdk';

const invoker = new Invoker({ address: '127.0.0.1:8080' });

// 设置 Schema
await invoker.setSchema('player.ban', {
  type: 'object',
  properties: {
    player_id: { type: 'string' },
    reason: { type: 'string' }
  },
  required: ['player_id']
});

// Invoke 会自动验证 payload
await invoker.invoke('player.ban', '{"player_id": "123"}');
// 如果 payload 缺少 player_id，会抛出验证错误
```

### 代码示例 - 自动重连配置

```typescript
// 自动重连已实现
import { Invoker, InvokerConfig } from '@croupier/sdk';

// 配置重连策略
const invoker = new Invoker({
  address: '127.0.0.1:8080',
  reconnect: {
    enabled: true,
    maxAttempts: 0,        // 0 = 无限重试
    initialDelayMs: 1000,  // 初始延迟 1 秒
    maxDelayMs: 30000,     // 最大延迟 30 秒
    backoffMultiplier: 2,  // 指数退避倍数
    jitterFactor: 0.2,     // 抖动因子
  },
});

await invoker.connect();
// 连接断开时会自动重连
```

---

## Python SDK

**位置:** `sdks/python/`

> ✅ **更新:** Schema 验证和重试机制已于 2025-01-06 完成实现

### 已实现功能

| 功能 | 状态 | 说明 |
|------|:----:|------|
| **Invoker 接口** | ✅ | 完整实现，支持同步和异步调用 |
| **幂等性支持** | ✅ | `InvokeOptions` 包含 `idempotency_key` |
| **流式事件** | ✅ | 使用 `async for` 流式获取作业事件 |
| **作业取消** | ✅ | 支持 `cancel_job()` 方法 |
| **Schema 设置** | ✅ | 支持 `set_schema()` 方法 |
| **Schema 验证** | ✅ | 完整的 JSON Schema 验证 |
| **自动重连** | ✅ | 指数退避 + 抖动重连策略 |
| **重试机制** | ✅ | `_execute_with_retry` + 指数退避 + 抖动 |

### 缺失功能

| 功能 | 优先级 | 说明 |
|------|--------|------|
| **文件传输功能** | 低 | 未实现 |

### 代码示例 - 重试机制

```python
# 重试机制已实现
from croupier import Invoker, InvokerConfig, RetryConfig

# 创建 Invoker（带重试配置）
invoker = Invoker(InvokerConfig(
    address="127.0.0.1:8080",
    retry=RetryConfig(
        enabled=True,
        max_attempts=3,          # 最多重试 3 次
        initial_delay_ms=100,    # 初始延迟 100ms
        max_delay_ms=5000,       # 最大延迟 5 秒
        backoff_multiplier=2.0,  # 指数退避倍数
        jitter_factor=0.1,       # 抖动因子
    ),
))

await invoker.connect()

# Invoke 会在失败时自动重试
await invoker.invoke("player.ban", payload)
```

---

## Java SDK

**位置:** `sdks/java/`

> ✅ **更新:** Schema 验证和重试机制已于 2025-01-06 完成实现

### 已实现功能

| 功能 | 状态 | 说明 |
|------|:----:|------|
| **Invoker 接口** | ✅ | 完整实现，支持同步调用 |
| **幂等性支持** | ✅ | `InvokeOptions` 包含 `idempotencyKey` |
| **流式作业事件** | ✅ | 使用 Reactive Streams Publisher |
| **作业取消** | ✅ | 支持 `cancelJob()` 方法 |
| **Schema 设置** | ✅ | 支持 `setSchema()` 方法 |
| **Schema 验证** | ✅ | 完整的 JSON Schema 验证 |
| **异常处理** | ✅ | 完整的 `InvokerException` 和 `ErrorCode` |
| **自动重连** | ✅ | 指数退避 + 抖动重连策略 |
| **重试机制** | ✅ | `executeWithRetry` + 指数退避 + 抖动 |

### 缺失功能

| 功能 | 优先级 | 说明 |
|------|--------|------|
| **文件传输功能** | 低 | 未实现 |

### 代码示例 - 重试机制

```java
// 重试机制已实现
import io.github.cuihairu.croupier.sdk.*;
import io.github.cuihairu.croupier.sdk.invoker.*;

// 创建 Invoker（带重试配置）
Invoker invoker = CroupierSDK.createInvoker(
    InvokerConfig.builder()
        .address("127.0.0.1:8080")
        .insecure(true)
        .retry(RetryConfig.builder()
            .enabled(true)
            .maxAttempts(3)          // 最多重试 3 次
            .initialDelayMs(100)      // 初始延迟 100ms
            .maxDelayMs(5000)         // 最大延迟 5 秒
            .backoffMultiplier(2.0)   // 指数退避倍数
            .jitterFactor(0.1)        // 抖动因子
            .build())
        .build()
);

invoker.connect();

// Invoke 会在失败时自动重试
String result = invoker.invoke("player.ban", payload);
```

---

## C++ SDK

**位置:** `sdks/cpp/`

> ✅ **更新:** 重试机制已于 2025-01-06 完成实现

### 已实现功能

| 功能 | 状态 | 说明 |
|------|:----:|------|
| **Invoker 接口** | ✅ | 完整实现 |
| **幂等性支持** | ✅ | `InvokeOptions` 包含 `idempotency_key` |
| **流式作业事件** | ✅ | 使用 `std::future` 和 `std::vector<JobEvent>` |
| **作业取消** | ✅ | 支持 `CancelJob()` 方法 |
| **Schema 设置** | ✅ | 支持 `SetSchema()` 方法 |
| **自动重连** | ✅ | 指数退避 + 抖动重连策略 |
| **重试机制** | ✅ | `RetryConfig` + `SetRetryConfig()` + 指数退避 |
| **可配置日志** | ✅ | `disable_logging`/`debug_logging` 配置 |
| **Token 脱敏** | ✅ | `MaskSensitive()` + `LogMasked()` |

### 缺失功能

| 功能 | 优先级 | 说明 |
|------|--------|------|
| **文件传输功能** | 低 | 虽有基础设施，但未实现（所有 SDK 都未实现） |

### 需要改进

| 项目 | 当前状态 | 建议改进 |
|------|----------|----------|
| 文档 | 详细 | 可读性强，保持 |
| 构建系统 | CMake | 已完善 |
| 示例代码 | 丰富 | 已完善 |

### 代码示例 - 重试机制

```cpp
// 重试机制已实现
#include "croupier/sdk/croupier_client.h"

using namespace croupier::sdk;

// 创建 Invoker
CroupierInvoker invoker(config);

// 配置重试策略
RetryConfig retry_config;
retry_config.enabled = true;
retry_config.max_attempts = 3;           // 最多重试 3 次
retry_config.initial_delay_ms = 100;     // 初始延迟 100ms
retry_config.max_delay_ms = 5000;        // 最大延迟 5 秒
retry_config.backoff_multiplier = 2.0;   // 指数退避
retry_config.jitter_factor = 0.1;        // 抖动因子

invoker.SetRetryConfig(retry_config);

// Invoke 会在失败时自动重试
std::string result = invoker.Invoke("player.ban", payload);
```

### 代码示例 - 可配置日志

```cpp
// 可配置日志已实现
#include "croupier/sdk/croupier_client.h"

using namespace croupier::sdk;

ClientConfig config;
config.agent_addr = "127.0.0.1:19090";

// 禁用日志
config.disable_logging = true;

// 或启用调试日志
config.debug_logging = true;

CroupierClient client(config);
```

### 优势功能

C++ SDK 独有的高级功能（其他 SDK 可参考）：

| 功能 | 说明 |
|------|------|
| **虚拟对象系统** | `VirtualObjectDescriptor` 支持对象关系 |
| **组件系统** | `ComponentDescriptor` 支持组件依赖管理 |
| **模板生成** | 支持从模板生成描述符 |
| **工具函数库** | `utils` 命名空间提供丰富的辅助函数 |

---

## 跨 SDK 统一改进项

### 1. 高优先级（必须实现）

| 功能 | 说明 | 影响 SDK |
|------|------|----------|
| ~~**Invoker 接口**~~ | ~~Python 和 Java SDK 必须实现~~ | ✅ 已完成 |
| ~~**自动重连**~~ | ~~连接断开后自动重连（指数退避）~~ | ✅ 所有 SDK 已完成 |

### 2. 中优先级（建议实现）

| 功能 | 说明 | 影响 SDK |
|------|------|----------|
| ~~**JSON Schema 验证**~~ | ~~Invoker 端验证请求 payload~~ | ✅ 所有 SDK 已完成 |
| ~~**重试机制**~~ | ~~可配置的重试策略~~ | ✅ 所有 SDK 已完成 |
| **集成测试** | 端到端测试覆盖 | JS, Python, Java |

### 3. 低优先级（可选实现）

| 功能 | 说明 | 影响 SDK |
|------|------|----------|
| **文件传输** | 支持上传文件到服务器 | 所有 SDK |
| **高级重试** | 断路器、舱壁模式 | 所有 SDK |
| **指标收集** | 内置 Prometheus 指标 | 所有 SDK |

---

## 实现优先级路线图

### Phase 1: 核心功能补全（必须）✅ 已完成

1. ~~**Python SDK** - 实现 Invoker 接口~~ ✅ 已完成
2. ~~**Java SDK** - 实现 Invoker 接口~~ ✅ 已完成
3. ~~**所有 SDK** - 添加幂等性支持~~ ✅ 已完成
4. ~~**JS SDK** - 实现自动重连（指数退避）~~ ✅ 已完成
5. ~~**Go SDK** - JSON Schema 验证~~ ✅ 已确认实现
6. ~~**Python SDK** - 实现自动重连~~ ✅ 已完成
7. ~~**Java SDK** - 实现自动重连~~ ✅ 已完成
8. ~~**Go SDK** - 实现自动重连~~ ✅ 已完成
9. ~~**C++ SDK** - 实现自动重连~~ ✅ 已完成
10. ~~**所有 SDK** - JSON Schema 验证~~ ✅ 已完成

### Phase 2: 功能增强（建议）

1. ~~**Go SDK** - JSON Schema 验证~~ ✅ 已完成
2. ~~**所有 SDK** - JSON Schema 验证~~ ✅ 已完成
3. ~~**所有 SDK** - 重试机制（调用失败重试）~~ ✅ 已完成
4. ~~**Python/Java SDK** - 实现自动重连~~ ✅ 已完成
5. ~~**Go SDK** - 实现自动重连~~ ✅ 已完成
6. ~~**C++ SDK** - 实现自动重连~~ ✅ 已完成
7. **JS SDK** - 增加测试覆盖率
8. **集成测试** - 端到端测试覆盖

### Phase 3: 高级特性（可选）

1. **所有 SDK** - 文件传输功能
2. **所有 SDK** - 高级重试策略
3. **C++ 以外的 SDK** - 虚拟对象系统
4. **所有 SDK** - 内置指标收集

---

## 功能完成度评分

| SDK | 核心功能 | 高级功能 | 代码质量 | 总体评分 |
|-----|:--------:|:--------:|:--------:|:--------:|
| **C++** | 100% | 98% | 95% | **98%** |
| **Go** | 100% | 95% | 90% | **95%** |
| **Java** | 98% | 92% | 85% | **92%** ⬆️ |
| **Python** | 95% | 85% | 80% | **87%** ⬆️ |
| **JS** | 98% | 85% | 75% | **86%** ⬆️ |

> ⬆️ 表示评分因功能完成而提升（Java, Python, JS: 重试机制）

### 评分标准

- **核心功能**: Client/Invoker 接口、函数注册、连接管理
- **高级功能**: 作业管理、幂等性、重试、TLS、自动重连
- **代码质量**: 测试覆盖、文档完整性、示例代码

---

## 相关文档

- [SDK 行为规范](./specification.md) - 所有 SDK 必须遵守的行为规范
- [SDK 行为规范](./specification.md) - 所有 SDK 的共同行为约束
- [Go SDK 示例](../../sdks/go/examples/) - Go SDK 使用示例
- [JS SDK 示例](../../sdks/js/examples/) - JS SDK 使用示例
