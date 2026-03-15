# Dashboard Extension UI Draft

更新时间：2026-03-15
状态：草案

本文件定义 Dashboard 中扩展商店与安装管理的最小 UI 结构，用于支撑第一阶段扩展系统落地。

---

## 1. 设计目标

Dashboard 第一阶段不追求完整的动态页面生态，先完成“扩展可管理”。

必须支持：

- 浏览商店目录
- 查看扩展详情
- 安装扩展
- 查看安装实例
- 启停扩展
- 修改配置
- 查看健康和事件

第一阶段不要求：

- 完整低代码页面系统
- 所有扩展页面都自动生成
- 复杂拖拽式扩展编排

---

## 2. 页面结构

建议新增一级导航：

- `扩展`

下设页面：

- `商店`
- `已安装`

详情页和安装页走二级页面。

---

## 3. 页面清单

### 3.1 商店列表页

路径建议：

- `/extensions/store`

作用：

- 展示 catalog 列表
- 支持搜索、筛选、查看详情

最小信息结构：

| 字段 | 说明 |
|---|---|
| `id` | 扩展 ID |
| `displayName` | 展示名 |
| `vendor` | 发布方 |
| `summary` | 摘要 |
| `latestVersion` | 最新版本 |
| `kind` | official/community/private |
| `status` | active/deprecated/hidden |
| `installed` | 是否已安装 |
| `defaultInstall` | 是否推荐安装 |
| `tags` | 标签 |

筛选项：

- 关键字
- 官方 / 社区 / 私有
- 已安装 / 未安装
- 稳定 / beta / experimental

### 3.2 扩展详情页

路径建议：

- `/extensions/store/:extensionId`

作用：

- 展示扩展说明与版本
- 触发安装

最小信息结构：

| 字段 | 说明 |
|---|---|
| `metadata` | 基本信息 |
| `latestRelease` | 最新 release |
| `releaseList` | 可安装版本 |
| `targets` | server/agent/hybrid |
| `scopeTypes` | 可安装范围 |
| `permissions` | 所需权限 |
| `capabilities` | 暴露能力 |
| `dependencies` | 依赖关系 |
| `docs` | 文档内容或链接 |

动作：

- 安装
- 查看版本变更

### 3.3 安装向导页

路径建议：

- `/extensions/install/:extensionId`

流程建议：

1. 选择版本
2. 选择 scope
3. 选择 target
4. 填写配置
5. 绑定密钥
6. 确认安装

最小请求结构：

```ts
type InstallExtensionPayload = {
  extensionId: string;
  releaseVersion: string;
  scopeType: string;
  scopeId: string;
  targetType: string;
  targetId?: string;
  config: Record<string, unknown>;
  secretRefs: Record<string, string>;
};
```

### 3.4 已安装列表页

路径建议：

- `/extensions/installations`

作用：

- 管理所有安装实例
- 快速查看状态与执行常用动作

最小信息结构：

| 字段 | 说明 |
|---|---|
| `installationId` | 安装实例 ID |
| `extensionId` | 扩展 ID |
| `displayName` | 展示名 |
| `releaseVersion` | 当前版本 |
| `scopeLabel` | 范围 |
| `targetLabel` | 目标 |
| `status` | 生命周期状态 |
| `healthStatus` | 健康状态 |
| `enabled` | 启用状态 |
| `updatedAt` | 更新时间 |

列表动作：

- 启用
- 停用
- 升级
- 配置
- 事件
- 卸载

### 3.5 安装实例详情页

路径建议：

- `/extensions/installations/:installationId`

标签页建议：

- 概览
- 配置
- 能力
- 绑定
- 健康
- 事件

#### 概览

展示：

- 基本信息
- 当前状态
- 目标节点
- 最近错误
- 最近同步时间

#### 配置

展示：

- 配置表单
- 密钥绑定
- 测试连接

#### 能力

展示：

- capability 列表
- operation 列表
- 是否可见 / 启用

#### 绑定

展示：

- function bindings
- provider bindings
- page bindings
- workflow bindings

#### 健康

展示：

- 当前健康状态
- 最近检查记录

#### 事件

展示：

- 安装事件
- 启停事件
- reconcile 事件
- 错误事件

---

## 4. 前端数据模型草案

### 4.1 CatalogItem

```ts
type ExtensionCatalogItem = {
  id: string;
  name: string;
  displayName: string;
  vendor: string;
  kind: 'official' | 'community' | 'private';
  summary: string;
  iconUrl?: string;
  latestVersion: string;
  status: 'active' | 'hidden' | 'deprecated';
  installed: boolean;
  defaultInstall: boolean;
  tags: string[];
};
```

### 4.2 InstallationItem

```ts
type ExtensionInstallationItem = {
  installationId: number;
  extensionId: string;
  displayName: string;
  releaseVersion: string;
  scopeType: string;
  scopeId: string;
  targetType: string;
  targetId?: string;
  status: string;
  healthStatus: string;
  enabled: boolean;
  lastError?: string;
  updatedAt: string;
};
```

### 4.3 InstallationDetail

```ts
type ExtensionInstallationDetail = {
  installation: ExtensionInstallationItem;
  configSchema?: Record<string, unknown>;
  config: Record<string, unknown>;
  secretRefs: Record<string, string>;
  capabilities: Array<{
    capabilityKey: string;
    operationKey: string;
    displayName: string;
    enabled: boolean;
    visible: boolean;
  }>;
  bindings: Array<{
    bindingType: string;
    bindingKey: string;
    targetRef?: string;
    status: string;
    lastError?: string;
  }>;
  health: {
    status: string;
    message?: string;
    checkedAt?: string;
  };
  events: Array<{
    eventType: string;
    level: string;
    message: string;
    createdAt: string;
    createdBy?: string;
  }>;
};
```

---

## 5. 第一阶段交互要求

### 5.1 必须具备

- 安装流程可走通
- 启用 / 停用 / 卸载可直接在列表或详情执行
- 可查看错误与事件
- 可修改配置并触发重建

### 5.2 可延后

- 拖拽式扩展页面装配
- 完整 schema-based page builder
- 复杂版本对比
- 多节点差异视图

---

## 6. 推荐前端实施顺序

1. 先做 `/extensions/installations`
2. 再做 `/extensions/store`
3. 再做安装向导
4. 再做详情页标签页
5. 最后补 schema renderer 最小版

原因：

- 已安装列表最直接体现系统价值
- 商店页依赖 catalog，但交互更简单
- 安装向导要等接口模型稳定

---

## 7. 与现有 Dashboard 的关系

建议新增独立页面域，不要一开始强塞到现有功能页面里。

第一阶段只要求：

- 新增扩展域的路由和服务层
- 不要求立即改造所有旧业务页面

第二阶段再逐步把 analytics、platform 等页面迁入扩展域。

---

## 8. 下一步

下一步需要补：

- 具体 API DTO 对齐
- Dashboard route/menu 草案
- 安装向导字段渲染规则
- schema renderer 最小控件集
