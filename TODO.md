# Croupier 开发 TODO List

## 🎯 当前优先级 (P0 - 核心功能)

### 1. 基于 X-Render 的动态界面系统 (X-Render UI System)
**目标**: 基于阿里巴巴 X-Render 框架创建动态表单和界面生成系统
**技术选型**: Form-Render + Ant Design 5.x + JSON Schema 驱动

#### 1.1 X-Render 集成基础架构 ✨
- [ ] 前端依赖安装和配置
  - [ ] 安装 `form-render` 核心包
  - [ ] 安装 `@ant-design/icons` 图标库
  - [ ] 确保 Ant Design 5.x 兼容性
  - [ ] 配置 TypeScript 支持
- [ ] 创建函数调用 Hook 系统
  - [ ] `useFunctionInvoke` - 核心函数调用 Hook
  - [ ] `useFunctionSchema` - 函数 Schema 获取 Hook
  - [ ] `useEntityCRUD` - 实体 CRUD 操作 Hook
- [ ] 通用组件封装
  - [ ] `FunctionForm` - 基于 Schema 的动态表单组件
  - [ ] `EntityManager` - 实体管理器组件
  - [ ] `SchemaPreview` - Schema 预览组件

#### 1.2 后端 API 实现 (已完成 ✅)
- [x] `GET /api/entities` - 获取所有实体列表 (已实现)
- [x] `POST /api/entities` - 创建新实体定义 (已实现)
- [x] `GET /api/entities/:id` - 获取单个实体详情 (已实现)
- [x] `PUT /api/entities/:id` - 更新实体定义 (已实现)
- [x] `DELETE /api/entities/:id` - 删除实体定义 (已实现)
- [x] `POST /api/entities/validate` - 验证实体定义 (已实现)
- [x] `POST /api/entities/:id/preview` - 预览实体 UI (已实现)
- [x] `POST /api/invoke` - 函数调用接口 (已实现)

#### 1.3 前端动态界面实现
- [ ] 实体管理页面
  - [ ] 基于 X-Render 的实体创建表单
  - [ ] 实体列表展示 (ProTable 风格)
  - [ ] 实体编辑器 (JSON Schema 可视化编辑)
  - [ ] 实体预览功能
- [ ] 函数调用界面
  - [ ] 动态生成函数调用表单 (基于函数描述符 JSON Schema)
  - [ ] 实时表单验证
  - [ ] 函数执行结果展示
  - [ ] 异步任务状态监控
- [ ] 组件管理界面
  - [ ] 组件安装/卸载界面
  - [ ] 组件配置管理
  - [ ] 组件依赖关系可视化

### 2. 完善现有组件功能

#### 2.1 物品管理组件补全
- [ ] 实现缺失的函数描述符
  - [ ] `item.update.json` - 更新物品模板
  - [ ] `item.delete.json` - 删除物品模板
- [ ] 创建库存管理函数
  - [ ] `inventory.add.json` - 向玩家库存添加物品
  - [ ] `inventory.remove.json` - 从玩家库存移除物品
  - [ ] `inventory.get.json` - 获取玩家库存
  - [ ] `inventory.transfer.json` - 玩家间物品转移

#### 2.2 经济系统组件补全
- [ ] 完善货币管理函数
  - [ ] `currency.create.json` - 创建新货币类型
  - [ ] `currency.get.json` - 获取货币详情
  - [ ] `currency.list.json` - 货币列表
  - [ ] `currency.update.json` - 更新货币设置
  - [ ] `currency.delete.json` - 删除货币类型
- [ ] 实现钱包管理函数
  - [ ] `wallet.add.json` - 增加玩家货币
  - [ ] `wallet.deduct.json` - 扣除玩家货币
  - [ ] `wallet.get.json` - 获取玩家钱包余额
  - [ ] `wallet.transfer.json` - 玩家间转账
  - [ ] `transaction.list.json` - 交易记录查询

### 3. 邮件系统组件
- [ ] 创建邮件系统组件结构
  - [ ] `components/mail-system/manifest.json`
  - [ ] `components/mail-system/descriptors/mail.entity.json`
  - [ ] `components/mail-system/descriptors/mail.resource.json`
- [ ] 实现邮件 CRUD 函数
  - [ ] `mail.send.json` - 发送邮件
  - [ ] `mail.get.json` - 获取邮件详情
  - [ ] `mail.list.json` - 邮件列表 (收件箱/发件箱)
  - [ ] `mail.read.json` - 标记邮件为已读
  - [ ] `mail.delete.json` - 删除邮件
  - [ ] `mail.claim.json` - 领取邮件附件

### 4. 公会系统组件
- [ ] 创建公会系统组件结构
  - [ ] `components/guild-system/manifest.json`
  - [ ] `components/guild-system/descriptors/guild.entity.json`
  - [ ] `components/guild-system/descriptors/guild.resource.json`
- [ ] 实现公会管理函数
  - [ ] `guild.create.json` - 创建公会
  - [ ] `guild.get.json` - 获取公会详情
  - [ ] `guild.list.json` - 公会列表
  - [ ] `guild.update.json` - 更新公会信息
  - [ ] `guild.disband.json` - 解散公会
- [ ] 实现公会成员管理
  - [ ] `guild.member.invite.json` - 邀请成员
  - [ ] `guild.member.kick.json` - 踢出成员
  - [ ] `guild.member.promote.json` - 提升职位
  - [ ] `guild.member.list.json` - 成员列表

### 5. 活动系统组件
- [ ] 创建活动系统组件结构
  - [ ] `components/activity-system/manifest.json`
  - [ ] `components/activity-system/descriptors/activity.entity.json`
  - [ ] `components/activity-system/descriptors/activity.resource.json`
- [ ] 实现活动管理函数
  - [ ] `activity.create.json` - 创建活动
  - [ ] `activity.get.json` - 获取活动详情
  - [ ] `activity.list.json` - 活动列表
  - [ ] `activity.update.json` - 更新活动
  - [ ] `activity.delete.json` - 删除活动
  - [ ] `activity.start.json` - 启动活动
  - [ ] `activity.stop.json` - 停止活动

## 🔧 功能增强 (P1 - 重要功能)

### 6. 函数注册增强

#### 6.1 注册表对象索引
- [ ] 扩展 `AgentSession` 结构
  ```go
  type FunctionMeta struct {
      Entity    string // player, item, currency
      Operation string // create, read, update, delete
      Enabled   bool
  }
  Functions map[string]FunctionMeta // 替换现有的 map[string]bool
  ```

#### 6.2 对象级别函数查询
- [ ] 在 `registry.go` 中添加方法
  - [ ] `GetFunctionsForEntity(entity string) []*AgentSession`
  - [ ] `GetEntitiesWithOperation(operation string) map[string][]*AgentSession`
  - [ ] `GetFunctionByEntityOp(entity, operation string) []*AgentSession`

#### 6.3 函数自动发现
- [ ] 创建 `internal/discovery/` 包
  - [ ] 基于实体定义自动发现可用操作
  - [ ] 检查函数完整性 (是否缺少 CRUD 操作)
  - [ ] 生成函数覆盖率报告

### 7. 基于 X-Render 的 UI 自动化系统 ✨
**替代原有 UI Schema 增强计划**

#### 7.1 X-Render 表格系统
- [ ] 基于 Form-Render 创建动态表格组件
  - [ ] 自动根据实体定义生成 ProTable 配置
  - [ ] 支持动态搜索、筛选、排序
  - [ ] 集成函数调用的 CRUD 操作
  - [ ] 支持自定义渲染器 (badge, datetime, number)

#### 7.2 X-Render 表单系统
- [ ] 基于 JSON Schema 的动态表单生成
  - [ ] 自动根据函数 params Schema 生成表单
  - [ ] 支持复杂表单控件 (DatePicker, Select, Upload)
  - [ ] 内置表单验证规则
  - [ ] 集成函数调用提交逻辑

#### 7.3 X-Render 组件库扩展
- [ ] 创建基于 X-Render 的通用组件
  - [ ] `XResourceTable` - X-Render 驱动的资源表格
  - [ ] `XEntityForm` - X-Render 驱动的实体表单
  - [ ] `XSchemaBuilder` - 可视化 Schema 构建器
  - [ ] `XFunctionInvoker` - 函数调用器组件

### 8. 权限系统增强

#### 8.1 对象级权限
- [ ] 扩展 RBAC 支持对象级权限
  - [ ] `player:read`, `player:write`, `player:delete`
  - [ ] `item:read`, `item:write`, `item:delete`
  - [ ] 支持通配符权限 `player:*`

#### 8.2 条件权限增强
- [ ] 扩展 `allow_if` 表达式引擎
  - [ ] 支持更多函数 (`has_permission`, `is_owner`)
  - [ ] 支持数值比较 (`level > 10`)
  - [ ] 支持时间条件 (`time_in_range`)

### 9. 开发工具

#### 9.1 组件打包工具
- [ ] 创建 `cmd/pack-builder/` 命令行工具
  - [ ] 验证组件定义完整性
  - [ ] 生成组件压缩包
  - [ ] 依赖关系检查

#### 9.2 Schema 验证工具
- [ ] 创建 `cmd/schema-validator/` 工具
  - [ ] JSON Schema 语法验证
  - [ ] 实体定义验证
  - [ ] 函数描述符验证

#### 9.3 开发辅助工具
- [ ] 创建 `scripts/generate-entity.sh`
  - [ ] 基于模板快速生成实体
  - [ ] 自动生成 CRUD 函数描述符
- [ ] 创建 `scripts/test-functions.sh`
  - [ ] 自动测试所有函数描述符
  - [ ] 生成测试报告

## 🚀 高级功能 (P2 - 未来功能)

### 10. 基于 X-Render 的可视化设计器 ✨
**采用 X-Render 生态替代自研方案**

#### 10.1 X-Render 实体设计器
- [ ] 集成 X-Render Form Builder
  - [ ] 拖拽式字段设计器 (基于 X-Render)
  - [ ] 可视化 JSON Schema 生成
  - [ ] 实时预览功能
  - [ ] 字段属性配置器

#### 10.2 X-Render 表单设计器
- [ ] 基于 Form-Render 的表单构建器
  - [ ] 可视化表单布局设计
  - [ ] 自定义验证规则配置
  - [ ] 组件属性可视化编辑
  - [ ] 表单主题和样式配置

#### 10.3 集成工作流设计器
- [ ] X-Render + 工作流引擎集成
  - [ ] 可视化业务流程设计
  - [ ] 条件分支支持 (基于函数调用)
  - [ ] 多步骤表单向导
  - [ ] 工作流状态管理

### 11. 数据源集成

#### 11.1 数据库连接器
- [ ] MySQL/PostgreSQL 直连
- [ ] Redis 缓存集成
- [ ] MongoDB 文档存储

#### 11.2 外部 API 集成
- [ ] RESTful API 连接器
- [ ] GraphQL 查询支持
- [ ] 第三方服务集成

### 12. 多租户支持

#### 12.1 租户隔离
- [ ] 数据库级别隔离
- [ ] 组件级别隔离
- [ ] 权限级别隔离

#### 12.2 租户管理
- [ ] 租户注册和配置
- [ ] 资源配额管理
- [ ] 计费和统计

### 13. 插件生态

#### 13.1 插件市场
- [ ] 组件发布平台
- [ ] 版本管理
- [ ] 评价和下载

#### 13.2 第三方集成
- [ ] Discord Bot 集成
- [ ] 微信小程序集成
- [ ] 钉钉/企业微信集成

## 📋 测试和文档 (P3 - 质量保证)

### 14. 单元测试
- [ ] `internal/pack/manager_test.go` 补全
- [ ] `internal/validation/` 测试覆盖
- [ ] HTTP API 集成测试

### 15. 文档完善
- [ ] API 文档生成 (Swagger)
- [ ] 组件开发指南
- [ ] 最佳实践文档
- [ ] 部署运维指南

### 16. 示例和教程
- [ ] 完整的示例游戏项目
- [ ] 组件开发教程
- [ ] 视频教程制作

---

## 📊 完成度追踪

### 已完成 ✅
- [x] 组件管理系统基础架构
- [x] 玩家管理组件 (90%)
- [x] 物品管理组件 (60%)
- [x] 经济系统组件 (30%)
- [x] 架构文档编写
- [x] 实体管理后端 API (完整实现)
- [x] 函数调用接口 (`/api/invoke`)
- [x] JSON Schema 验证系统

### 当前进行中 🔄
- [ ] X-Render 前端集成 (0% - 新启动)
- [ ] 基于 X-Render 的动态界面系统 (0% - 替代原方案)

### 技术选型确定 ✨
- **前端框架**: React + TypeScript
- **UI 库**: Ant Design 5.x
- **动态表单**: X-Render Form-Render
- **后端**: Go + Gin + GORM
- **数据库**: PostgreSQL/SQLite
- **协议**: JSON Schema 驱动

### 预计完成时间 (基于 X-Render 集成)
- **P0 功能** (X-Render 集成): 1-2 周 ⚡ (大幅缩短)
- **P1 功能**: 3-4 周 ⚡ (优化时间)
- **P2 功能**: 1-2 个月 ⚡ (采用成熟方案)

### 里程碑 (更新)
1. **Alpha 版本** - 完成 X-Render 集成 + 基础动态表单 (1-2 周)
2. **Beta 版本** - 完成 P1 功能 + 可视化设计器 (1 个月)
3. **正式版本** - 完成 P2 功能 + 完整生态 (2 个月)

### X-Render 集成优势 🚀
- **开发效率提升 80%**: 无需自研 UI 组件生成器
- **维护成本降低**: 使用阿里巴巴维护的成熟方案
- **功能完整性**: 开箱即用的丰富功能
- **社区支持**: 7.8k+ stars 的活跃社区
- **技术债减少**: 基于行业标准的 JSON Schema

---

**重要说明**:
通过采用 X-Render，我们将原计划的自研 UI Schema 生成系统替换为成熟的开源方案，预计可以节省 60% 的开发时间，同时获得更强大和稳定的功能。

---

这个 TODO List 将指导整个系统的开发进程，确保按优先级有序推进。