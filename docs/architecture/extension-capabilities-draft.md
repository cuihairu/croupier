# Extension Capabilities Draft

更新时间：2026-03-15
状态：草案

本文件定义扩展系统的 `capabilities.yaml` 草案，用于描述扩展对外暴露的能力和操作。

---

## 1. 设计目标

`capabilities.yaml` 负责表达：

- 扩展提供了哪些能力
- 每个能力有哪些操作
- 每个操作如何绑定到底层运行时
- 哪些能力显示在 UI
- 哪些能力需要权限控制

要求：

- UI 不直接依赖底层 method 名称
- 权限不直接绑到底层 driver
- capability / operation 必须稳定，可长期演进

---

## 2. 核心概念

### 2.1 Capability

代表一类能力，例如：

- `platform.management`
- `analytics.query`
- `alert.rule`

### 2.2 Operation

代表 capability 下的具体动作，例如：

- `list`
- `detail`
- `create`
- `update`
- `delete`
- `invoke`
- `run`

### 2.3 Binding

operation 最终如何落地到 runtime：

- function
- provider_method
- workflow
- page_action
- job

---

## 3. 顶层结构

建议结构：

```yaml
api_version: croupier.io/v1alpha1
kind: CapabilitySet
metadata:
  extension_id: official.external-platform
  version: 1.0.0
spec:
  capabilities:
    - key: platform.management
      display_name: Platform Management
      description: Manage third-party platforms
      visible: true
      operations: []
```

---

## 4. 字段说明

### 4.1 metadata

| 字段 | 说明 |
|---|---|
| `extension_id` | 所属扩展 |
| `version` | 对应 release 版本 |

### 4.2 capability 字段

| 字段 | 必填 | 说明 |
|---|---|---|
| `key` | 是 | 全局能力键 |
| `display_name` | 是 | 展示名 |
| `description` | 否 | 说明 |
| `category` | 否 | UI 分类 |
| `visible` | 否 | 是否显示在前台 |
| `enabled_by_default` | 否 | 默认启用 |
| `permissions` | 否 | 关联权限 |
| `operations` | 是 | 操作列表 |

### 4.3 operation 字段

| 字段 | 必填 | 说明 |
|---|---|---|
| `key` | 是 | 操作键 |
| `display_name` | 是 | 展示名 |
| `description` | 否 | 说明 |
| `risk` | 否 | `low/medium/high/critical` |
| `permissions` | 否 | 额外权限点 |
| `input_schema` | 否 | 输入 schema 引用 |
| `output_schema` | 否 | 输出 schema 引用 |
| `binding` | 是 | 底层绑定 |
| `ui` | 否 | 页面呈现建议 |

### 4.4 binding 字段

| 字段 | 必填 | 说明 |
|---|---|---|
| `type` | 是 | `function/provider_method/workflow/job/page_action` |
| `target` | 是 | 目标引用 |
| `driver` | 否 | 指定 driver |
| `timeout` | 否 | 调用超时 |
| `retry` | 否 | 重试次数 |

### 4.5 ui 字段

| 字段 | 说明 |
|---|---|
| `surface` | `list/detail/dashboard/modal/action-bar` |
| `form` | 输入表单 schema 引用 |
| `result` | 结果渲染器引用 |
| `hidden` | 是否隐藏 |

---

## 5. 示例

```yaml
api_version: croupier.io/v1alpha1
kind: CapabilitySet
metadata:
  extension_id: official.external-platform
  version: 1.0.0
spec:
  capabilities:
    - key: platform.management
      display_name: Platform Management
      description: Manage third-party platform integrations
      category: platform
      visible: true
      enabled_by_default: true
      permissions:
        - platform.read
      operations:
        - key: list
          display_name: List Platforms
          description: List installed platforms
          risk: low
          permissions:
            - platform.read
          output_schema: schemas/platform-list-output.schema.json
          binding:
            type: function
            target: extension.platform.list
            timeout: 5s
            retry: 0
          ui:
            surface: list
            result: table
        - key: invoke
          display_name: Invoke Method
          description: Invoke a platform method
          risk: high
          permissions:
            - platform.invoke
          input_schema: schemas/platform-invoke-input.schema.json
          output_schema: schemas/platform-invoke-output.schema.json
          binding:
            type: provider_method
            target: platform.call
            driver: openapi-driver
            timeout: 15s
            retry: 1
          ui:
            surface: detail
            form: forms/platform-invoke.form.yaml
            result: json
```

---

## 6. 命名规则

### 6.1 capability key

格式：

- `<domain>.<name>`

例如：

- `platform.management`
- `analytics.query`
- `alert.rule`

### 6.2 operation key

格式：

- 单动作短词

例如：

- `list`
- `detail`
- `run`
- `invoke`
- `create`
- `disable`

不要把 capability 和 operation 混成一层，例如：

- 错误：`platform_invoke`
- 正确：`platform.management` + `invoke`

---

## 7. 权限映射建议

权限建议分两层：

- capability 级权限
- operation 级权限

例如：

- capability：`platform.read`
- operation：`platform.invoke`

这样可以控制：

- 可以看，但不能操作
- 可以列举，但不能执行高风险动作

---

## 8. 与 runtime binding 的关系

`capabilities.yaml` 是产品视角。

`bindings/*.yaml` 是运行时视角。

关系如下：

- capability / operation 定义“对外是什么”
- binding 定义“底层怎么执行”

不要把所有细节都塞进 capability。

---

## 9. 第一阶段最小要求

第一阶段只要求做到：

- capability / operation 可入库
- operation 可绑定 function 或 provider_method
- UI 能展示 capability 列表
- 权限可按 operation 控制

第一阶段可暂缓：

- 多级 capability 树
- 动态 capability discover 合并
- 复杂 UI 编排

---

## 10. 下一步

下一步需要补：

- `bindings/*.yaml` 草案
- capability 对应数据库模型细化
- capability 到前端菜单映射规则
