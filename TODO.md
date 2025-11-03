# Croupier 开发 TODO List

## 🎯 当前优先级 (P0 - 核心功能)

### 1. 实体管理界面 (Entity Management UI)
**目标**: 创建可视化的实体定义和编辑界面

#### 1.1 后端 API 实现
- [ ] 在 `server.go` 中添加实体管理路由
  - [ ] `GET /api/entities` - 获取所有实体列表
  - [ ] `POST /api/entities` - 创建新实体定义
  - [ ] `GET /api/entities/:id` - 获取单个实体详情
  - [ ] `PUT /api/entities/:id` - 更新实体定义
  - [ ] `DELETE /api/entities/:id` - 删除实体定义
  - [ ] `POST /api/entities/validate` - 验证实体定义
  - [ ] `POST /api/entities/:id/preview` - 预览实体 UI

#### 1.2 实体存储管理
- [ ] 扩展 `ComponentManager` 支持实体管理
  - [ ] 添加 `LoadEntities()` 方法
  - [ ] 添加 `SaveEntity()` 方法
  - [ ] 添加 `DeleteEntity()` 方法
  - [ ] 添加实体验证逻辑

#### 1.3 JSON Schema 验证增强
- [ ] 创建 `internal/validation/entity.go`
  - [ ] 实体定义格式验证
  - [ ] Schema 语法验证
  - [ ] UI 配置验证
  - [ ] 操作映射验证

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

### 7. UI Schema 增强

#### 7.1 ProTable 配置生成器
- [ ] 创建 `internal/ui/protable.go`
  - [ ] 基于实体定义自动生成列配置
  - [ ] 支持搜索、筛选、排序配置
  - [ ] 支持自定义渲染器 (badge, datetime, number)

#### 7.2 ProForm 配置生成器
- [ ] 创建 `internal/ui/proform.go`
  - [ ] 基于函数 params Schema 生成表单
  - [ ] 支持复杂表单控件 (DatePicker, Select, Upload)
  - [ ] 支持表单验证规则

#### 7.3 UI 组件库扩展
- [ ] 创建前端 UI 组件模板
  - [ ] `ResourceTable.tsx` - 通用资源表格组件
  - [ ] `EntityForm.tsx` - 通用实体表单组件
  - [ ] `SchemaRenderer.tsx` - JSON Schema 渲染器
  - [ ] `FieldPicker.tsx` - 字段选择器

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

### 10. 可视化设计器

#### 10.1 实体设计器
- [ ] 拖拽式字段设计器
- [ ] 可视化关系定义
- [ ] 实时预览功能

#### 10.2 表单设计器
- [ ] 可视化表单布局
- [ ] 自定义验证规则
- [ ] 组件属性编辑器

#### 10.3 工作流设计器
- [ ] 可视化业务流程设计
- [ ] 条件分支支持
- [ ] 多步骤表单向导

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

### 当前进行中 🔄
- [ ] 实体管理界面 (20%)

### 预计完成时间
- **P0 功能**: 2-3 周
- **P1 功能**: 4-6 周
- **P2 功能**: 2-3 个月

### 里程碑
1. **Alpha 版本** - 完成 P0 所有功能，基础可用
2. **Beta 版本** - 完成 P1 主要功能，功能完善
3. **正式版本** - 完成核心 P2 功能，生产就绪

---

这个 TODO List 将指导整个系统的开发进程，确保按优先级有序推进。