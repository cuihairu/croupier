# Protobuf 自定义 Options 使用指南

## 概述

protoc-gen-croupier 插件支持通过 protobuf 自定义 options 来控制生成后的函数描述符（descriptors）和 UI 配置。这些 options 允许您在不修改生成代码的情况下，自定义函数的行为、权限、展示方式等。

## 支持的 Options

### 1. 函数级 Options (`croupier.options.v1.FunctionOptions`)

通过在 RPC 方法上添加 `(croupier.options.v1.function)` 选项来配置函数：

```protobuf
import "croupier/options/v1/function.proto";
import "croupier/common/v1/ui.proto";

service PlayerService {
  rpc GetPlayer(GetPlayerRequest) returns (GetPlayerResponse) {
    option (croupier.options.v1.function) = {
      function_id: "player.get_info";      // 全局唯一函数ID
      version: "1.2.0";                    // 版本号
      category: "player";                   // 分类
      risk: "low";                          // 风险级别：low/medium/high
      mode: "query";                        // 调用模式：query/command
      timeout: "10s";                       // 超时时间
      route: "lb";                          // 路由策略：lb/broadcast/targeted/hash
      placement: "agent";                   // 部署位置：core/agent
      idempotency_key: true;               // 是否支持幂等键
      two_person_rule: true;               // 是否需要双人审核

      // 展示相关
      display_name: {
        zh: "获取玩家信息",
        en: "Get Player Info"
      };
      summary: {
        zh: "根据玩家ID获取玩家信息",
        en: "Retrieve player information by ID"
      };
      tags: ["player", "info"];            // 标签列表

      // 权限配置
      permissions: {
        verbs: ["read"];                   // 所需操作权限
        scopes: ["game", "env", "function_id"]; // 支持的 ABAC scopes（示例）
        defaults: [                        // 默认角色 → verbs
          { role: "admin", verbs: ["read"] },
          { role: "operator", verbs: ["read"] }
        ];
      };
    };
  }
}
```

### 2. 字段级 UI Options (`croupier.options.v1.ui`)

通过在消息字段上添加 `(croupier.options.v1.ui)` 选项来配置 UI 展示：

```protobuf
message GetPlayerRequest {
  uint64 player_id = 1 [(croupier.options.v1.ui) = {
    label: "玩家ID",
    widget: "input",
    placeholder: "请输入玩家ID"
  }];

  string status = 2 [(croupier.options.v1.ui) = {
    label: "状态",
    widget: "select",
    enum_map: { key: "ACTIVE", value: "正常" }
    enum_map: { key: "BANNED", value: "封禁" }
  }];

  string reason = 3 [(croupier.options.v1.ui) = {
    label: "原因",
    widget: "textarea",
    placeholder: "请输入详细原因",
    sensitive: true
  }];
}
```

### 3. 支持的 UI 组件类型

- **input**: 文本输入框
- **number**: 数字输入框
- **textarea**: 多行文本
- **select**: 下拉选择（单选）
- **multiselect**: 下拉选择（多选）
- **checkbox**: 复选框
- **radio**: 单选按钮组
- **date**: 日期选择
- **datetime**: 日期时间选择
- **time**: 时间选择
- **upload**: 文件上传
- **switch**: 开关
- **slider**: 滑块
- **rate**: 评分
- **color**: 颜色选择
- **password**: 密码输入
- **email**: 邮箱输入
- **url**: URL输入
- **phone**: 电话号码输入

### 4. 高级配置

#### 路由选项

```protobuf
option (croupier.options.v1.function) = {
  route: "hash";                         // 基于哈希的路由
  routing: {                              // 路由配置
    hash_key: "player_id",              // 哈希键字段
    jsonpath: "$.request.player_id"     // JSONPath 表达式
  };
};
```

#### 速率限制（TODO：待实现）

```protobuf
option (croupier.options.v1.function) = {
  // 速率限制配置（需在 schema 和生成器中完善支持）
  rate_limit: {
    value: 100,                          // 请求数
    window: "1m"                        // 时间窗口
  };
  concurrency: 10;                       // 最大并发数
};
```

## 生成的输出

当运行 protoc-gen-croupier 时，这些 options 会被转换为：

1. **manifest.json**: 包含函数的元数据
2. **descriptors/*.json**: 函数描述符，包含 transport、auth、semantics 等信息
3. **ui/*.schema.json**: JSON Schema 定义
4. **ui/*.uischema.json**: UI 配置，用于表单渲染

## 最佳实践

1. **命名规范**
   - `function_id`: 使用全小写，用点分隔命名空间，如 `player.get_info`
   - `category`: 使用短名称，如 `player`, `inventory`, `payment`

2. **安全配置**
   - 对敏感操作使用 `two_person_rule: true`
   - 根据风险级别设置 `risk` 字段
   - 精细配置 `permissions`

3. **用户体验**
   - 使用 `display_name` 提供多语言支持
   - 通过 `summary` 清晰描述功能
   - 使用 `tags` 便于分类和搜索

4. **UI 配置**
   - 为所有输入字段提供清晰的 `label`
   - 使用适当的 `widget` 类型
   - 设置合理的 `required` 和验证规则

## 示例项目

完整示例请参考：`docs/proto-options-example.proto`

## 注意事项

1. 确保 import 相关的 proto 文件
2. options 的值需要是字符串类型（即使在 proto 中定义为其他类型）
3. UI options 中的 `default` 值需要是字符串格式
4. 生成的 pack 可以使用 `./bin/croupier packs validate` 验证
