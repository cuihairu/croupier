# Official External Platform Migration Draft

更新时间：2026-03-15
状态：草案

本文件定义 `official.external-platform` 作为第一批官方扩展的迁移方案。

> 说明（2026-03-15）：当前实现已经进入 extension-first 路径。本文中提到的 YAML 路径仅用于迁移兼容，不是推荐默认入口。

---

## 1. 目标

把历史“第三方平台集成”从：

- Server 侧 `configs/platforms.yaml`
- Agent 侧 `providers.yaml`
- 写死 provider type 分支
- 直接依赖 `PlatformLoader` / `ProviderManager`

重构为：

- 官方扩展 `official.external-platform`
- 基于 installation 的实例管理
- 基于 driver 的运行时绑定
- 兼容现有 `external.v1` 调用协议

---

## 2. 当前结构总结

### 2.1 当前 Server 侧

当前链路：

- `internal/api/platform/service.go`
- `internal/platform/loader.go`
- `internal/platform/provider/*`
- `internal/platform/quicksdk`
- `internal/platform/openapi`

特点：

- provider 实例通过 YAML 加载
- provider type 通过 `switch` 写死
- registry 负责 runtime call
- API 层只做通用转发

### 2.2 当前 Agent 侧

当前链路：

- `internal/app/agent/provider.go`
- `internal/nng/agent_server.go`

特点：

- Agent 读取本地 provider 配置（迁移期）
- 只支持 `openapi`
- 自动把 provider methods 注册成 functions
- 调用时按 `provider.method` 转回 provider runtime

### 2.3 当前问题

- 正式配置主数据源仍是 YAML
- 安装实例不可审计、不可升级、不可回滚
- provider 配置与产品安装模型未打通
- Dashboard 无法真正管理“安装”
- Server 与 Agent 的配置模型分裂

---

## 3. 迁移后的目标形态

### 3.1 扩展身份

新增官方扩展：

- `official.external-platform`

扩展职责：

- 管理第三方平台安装实例
- 提供平台能力列表与调用入口
- 通过 `openapi-driver` / 过渡期 `quicksdk adapter` 暴露 capability

### 3.2 核心分工

核心负责：

- catalog / release / installation
- 权限、审计、Agent sync
- runtime binding

扩展负责：

- 平台 capability 定义
- 平台页面
- 平台 provider 配置模板
- 平台调用编排

driver 负责：

- 真正执行 openapi / provider_method

---

## 4. 迁移边界

### 4.1 保留的东西

第一阶段保留：

- `croupier.external.v1` 协议
- `provider.Provider` 接口
- `provider.Registry`
- `openapi` provider 的运行时实现

理由：

- 这些已经具备良好的 runtime 边界
- 没必要第一阶段全部打散

### 4.2 需要替换的东西

优先替换：

- `configs/platforms.yaml` 主配置地位
- Agent 侧本地 provider 配置主配置地位
- `internal/platform/loader.go` 的 YAML 主入口
- `internal/app/agent/provider.go` 的 YAML 主入口

### 4.3 需要过渡的东西

- `quicksdk` 暂时保留为 `external-platform` 内部适配器
- `internal/api/platform` 暂时保留对旧前端和协议的兼容入口

---

## 5. 目标安装模型

每个“第三方平台实例”都变成一条 `extension_installation`。

示例：

- extension: `official.external-platform`
- scope: `game=g1, env=prod`
- target: `agent=agent-01`
- config:
  - provider_type: `openapi`
  - provider_name: `game_server`
  - base_url: `http://127.0.0.1:8080`
  - openapi_spec: `...`

或者：

- provider_type: `quicksdk`
- provider_name: `quicksdk`
- open_id: `...`
- open_key_ref: `secret://...`

---

## 6. manifest 设计建议

`official.external-platform` 的 manifest 建议：

- `driver`: `internal-ui-driver`
- `targets`: `server`, `agent`
- `install_mode`: `scoped`
- `capabilities`:
  - `platform.management`
  - `platform.discovery`
  - `platform.invoke`

bindings 建议：

- Server 页面绑定
- Agent provider/function 绑定

---

## 7. 迁移步骤

### Step 1：扩展目录与 release 建立

目标：

- 把 `official.external-platform` 作为 catalog/release 写入系统

任务：

- 定义 manifest
- 定义 capabilities
- 准备 release 元数据

### Step 2：抽象安装配置结构

目标：

- 把原 YAML 的 provider entry 转成 installation config

建议配置结构：

```json
{
  "provider_type": "openapi",
  "provider_name": "game_server",
  "game_id": "g1",
  "env": "prod",
  "provider_config": {
    "base_url": "http://127.0.0.1:8080",
    "spec_url": "http://127.0.0.1:8080/openapi.json"
  }
}
```

### Step 3：Server runtime 改为 installation 驱动

目标：

- 不再从 `configs/platforms.yaml` 直接建 provider

动作：

- 新增 `ExternalPlatformRuntimeAdapter`
- 从 installation repo 读取平台实例
- 转成 provider registry entries

### Step 4：Agent runtime 改为 sync payload 驱动

目标：

- Agent 不再以 `providers.yaml` 为主

动作：

- Agent `ProviderManager` 接受 `AgentInstallationPayload`
- 按 payload 初始化 provider
- 注册 functions

### Step 5：保留旧协议兼容层

目标：

- 旧调用入口仍能工作

动作：

- `internal/api/platform/service.go` 改为读取新的 runtime registry
- `external.v1` 协议保持不变

### Step 6：Dashboard 迁移

目标：

- 第三方平台从“写死页面 + 写死配置”转为安装实例管理

动作：

- 通过扩展详情页和安装页安装平台
- 通过实例详情页查看 methods、bindings、健康状态

---

## 8. 代码重构建议

### 8.1 第一阶段不立即删除

- `internal/platform/provider`
- `internal/platform/openapi`
- `internal/platform/quicksdk`
- `internal/api/platform`
- `internal/app/agent/provider.go`

### 8.2 第一阶段新增

建议新增：

```text
internal/extensions/official/externalplatform/
  manifest/
  service/
  runtime/
  adapter/
```

建议职责：

- `manifest/`
  - 扩展元数据与示例配置
- `service/`
  - 安装配置校验、平台实例视图转换
- `runtime/`
  - installation -> provider runtime plan
- `adapter/`
  - 兼容旧 `PlatformLoader` / `ProviderManager`

### 8.3 第二阶段可删除

当 installation 路径跑通后，再考虑逐步删除：

- `configs/platforms.yaml` 的正式入口地位
- Agent 本地 provider 配置的正式入口地位
- `internal/platform/loader.go` 中 YAML 驱动主逻辑

---

## 9. 兼容策略

### 9.1 短期兼容

短期允许：

- 若显式开启兼容开关且无 installation 数据，则 fallback 到 YAML
- 若存在 installation 数据，则优先 installation

### 9.2 中期目标

- YAML 只用于本地开发和迁移导入

### 9.3 长期目标

- 删除 YAML 作为正式主数据源

---

## 10. 风险与注意事项

### 10.1 风险

- `quicksdk` 并非通用 driver，需要过渡适配
- Agent function 注册与 installation binding 之间需要稳定映射
- 旧平台 API 可能假设平台名唯一，需要和 installation scope 对齐

### 10.2 注意事项

- 不要在第一阶段强行把所有 provider 改造成全新接口
- 不要先动 `external.v1` 协议
- 不要先删旧 YAML 路径

---

## 11. 第一阶段验收标准

- 可以在 catalog 中看到 `official.external-platform`
- 可以创建平台 installation
- 可以把 installation 同步到 Agent
- Agent 可以注册平台 functions
- 旧 `platform` API 仍可调用
- YAML 不再是唯一主配置来源

---

## 12. 下一步

下一步需要补：

- `official.external-platform` manifest 示例
- 平台 installation config schema
- 旧 YAML 到 installation 的导入脚本设计
