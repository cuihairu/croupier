# 完整游戏团队角色权限体系设计

## 角色层级架构

```
超级管理员 (super_admin)
├── 管理层
│   ├── 系统管理员 (admin)
│   ├── 项目经理 (project_manager)
│   └── 制作人 (producer)
├── 技术团队
│   ├── 技术负责人 (tech_lead)
│   ├── 高级开发工程师 (senior_developer)
│   ├── 开发工程师 (developer)
│   ├── 测试工程师 (tester)
│   └── 运维工程师 (ops)
├── 设计团队
│   ├── 游戏策划/设计师 (game_designer)
│   ├── 关卡策划 (level_designer)
│   ├── 系统策划 (system_designer)
│   ├── 数值策划 (numerical_designer)
│   └── UI设计师 (ui_designer)
├── 运营团队
│   ├── 游戏运营 (operator)
│   ├── 市场营销 (marketing)
│   ├── 社区管理 (community)
│   └── 内容管理员 (content_manager)
├── 数据分析团队
│   ├── 数据分析师 (analyst)
│   ├── 商业智能分析师 (bi_analyst)
│   └── 用户研究员 (user_researcher)
├── 客服团队
│   ├── 客服主管 (support_manager)
│   ├── 高级客服 (senior_support)
│   └── 客服人员 (support)
└── 特殊角色
    ├── 游戏管理员 (gm)
    ├── 托/机器人操作员 (bot_operator)
    ├── 安全专员 (security)
    └── 审计员 (auditor)
```

## 权限域说明

| 权限域 | 说明 | 示例权限 |
|--------|------|----------|
| `system:*` | 系统级操作 | `system:config`, `system:restart`, `system:backup` |
| `user:*` | 用户管理 | `user:create`, `user:update`, `user:delete`, `user:view` |
| `game:*` | 游戏配置管理 | `game:config`, `game:deploy`, `game:restart` |
| `player:*` | 玩家管理 | `player:query`, `player:update`, `player:ban` |
| `function:*` | 函数管理 | `function:register`, `function:deploy`, `function:test` |
| `job:*` | 任务管理 | `job:create`, `job:view`, `job:cancel`, `job:retry` |
| `audit:*` | 审计查看 | `audit:view`, `audit:export` |
| `monitor:*` | 监控数据 | `monitor:view`, `monitor:alert` |
| `data:*` | 数据分析 | `data:query`, `data:export`, `data:report` |
| `design:*` | 游戏设计/策划 | `design:level`, `design:system`, `design:feature` |
| `numerical:*` | 数值配置 | `numerical:balance`, `numerical:economy` |
| `level:*` | 关卡管理 | `level:create`, `level:edit`, `level:publish` |
| `content:*` | 内容管理 | `content:create`, `content:edit`, `content:publish` |
| `marketing:*` | 市场营销 | `marketing:campaign`, `marketing:analytics` |
| `community:*` | 社区管理 | `community:moderate`, `community:event` |
| `event:*` | 活动管理 | `event:create`, `event:config`, `event:start` |
| `announcement:*` | 公告系统 | `announcement:create`, `announcement:publish` |
| `mail:*` | 邮件系统 | `mail:send`, `mail:template` |
| `ban:*` | 封禁管理 | `ban:player`, `ban:temporary`, `ban:permanent` |
| `reward:*` | 奖励发放 | `reward:send`, `reward:config`, `reward:compensation` |
| `gm:*` | GM工具 | `gm:teleport`, `gm:spawn`, `gm:modify` |
| `bot:*` | 机器人/托管理 | `bot:create`, `bot:config`, `bot:control` |
| `security:*` | 安全管理 | `security:monitor`, `security:investigate` |
| `economy:*` | 游戏经济系统 | `economy:balance`, `economy:report` |
| `support:*` | 客服功能 | `support:ticket`, `support:chat` |

## 游戏行业特色角色详解

### **🎯 策划类角色 (Design Team)**

#### **游戏策划/设计师 (game_designer)**
**核心职责**：游戏整体设计，玩法创新，用户体验优化
**关键权限**：
- `design:*` - 全面设计权限
- `level:*` - 关卡设计权限
- `event:design` - 活动设计权限
- `data:query` - 数据查询支持设计决策

**典型工作场景**：
- 设计新的游戏玩法机制
- 制定游戏整体设计方案
- 优化用户体验和游戏流程
- 基于数据分析调整游戏设计

#### **数值策划 (numerical_designer)**
**核心职责**：游戏数值平衡，经济系统设计，奖励机制设计
**关键权限**：
- `numerical:*` - 数值配置全权限
- `economy:*` - 经济系统管理
- `reward:config` - 奖励配置权限
- `data:economy` - 经济数据分析

**典型工作场景**：
- 设计角色成长曲线和数值平衡
- 配置游戏内经济系统参数
- 调整奖励机制和掉落概率
- 分析经济数据并优化平衡性

#### **关卡策划 (level_designer)**
**核心职责**：关卡设计，难度平衡，关卡内容制作
**关键权限**：
- `level:*` - 关卡管理全权限
- `design:level` - 关卡设计权限
- `content:level` - 关卡内容管理

### **📊 运营类角色 (Operations Team)**

#### **游戏运营 (operator)**
**核心职责**：活动策划，用户运营，数据运营，内容运营
**关键权限**：
- `event:*` - 活动管理全权限
- `announcement:*` - 公告发布权限
- `reward:send` - 奖励发放权限
- `data:operation` - 运营数据分析

**典型工作场景**：
- 策划并执行游戏内活动
- 发布游戏公告和通知
- 给玩家发放活动奖励
- 分析运营数据优化策略

#### **社区管理 (community)**
**核心职责**：社区维护，用户互动，内容审核，社区活动
**关键权限**：
- `community:*` - 社区管理全权限
- `player:communicate` - 玩家沟通权限
- `content:community` - 社区内容管理
- `event:community` - 社区活动管理

### **🤖 特殊角色 (Special Roles)**

#### **托/机器人操作员 (bot_operator)**
**核心职责**：游戏托管理，机器人配置，自动化操作
**关键权限**：
- `bot:*` - 机器人管理全权限
- `player:bot` - 机器人玩家管理
- `data:bot` - 机器人数据分析
- `monitor:bot` - 机器人监控

**典型工作场景**：
- 配置和管理游戏内机器人
- 设置自动化游戏行为
- 监控机器人运行状态
- 调整机器人策略和参数

#### **游戏管理员 (gm)**
**核心职责**：游戏内管理，玩家争议处理，游戏秩序维护
**关键权限**：
- `gm:*` - GM工具全权限
- `player:*` - 玩家管理全权限
- `ban:*` - 封禁管理权限
- `reward:*` - 奖励发放权限

**典型工作场景**：
- 处理玩家之间的争议和冲突
- 对违规玩家进行封禁处理
- 给受损玩家发放补偿奖励
- 维护游戏内秩序和公平性

## 角色权限配置统计

### **权限分布统计**
- **管理层角色** (4个)：拥有最高级别的系统和业务权限
- **技术团队** (5个)：专注于系统开发、测试、运维权限
- **���计团队** (5个)：专注于游戏内容、数值、关卡设计权限
- **运营团队** (4个)：专注于用户运营、活动、社区管理权限
- **数据团队** (3个)：专注于数据分析、商业智能权限
- **客服团队** (3个)：专注于玩家支持、问题处理权限
- **特殊角色** (4个)：专注于GM、安全、审计、机器人管理权限

### **权限域覆盖分析**
- **系统权限**：主要分配给管理层和技术团队
- **业务权限**：主要分配给运营、设计、客服团队
- **数据权限**：主要分配给数据分析团队和管理层
- **特殊权限**：分配给相应的专业角色(GM、安全、审计等)

## 游戏行业权限管理最佳实践

### **1. 分阶段权限管理**
- **开发阶段**：开发、测试角色拥有较高权限
- **测试阶段**：QA和测试团队权限增加
- **运营阶段**：运营、客服、GM角色权限激活
- **维护阶段**：运维和安全角色权限重要性提升

### **2. 权限轮换机制**
- **定期轮换**：高权限角色定期轮换避免权力集中
- **临时授权**：紧急情况下临时提升权限
- **权限回收**：离职或角色变更时及时回收权限

### **3. 游戏特色安全措施**
- **经济安全**：数值策划权限需要双人确认
- **玩家数据保护**：严格控制玩家隐私数据访问
- **游戏公平性**：GM操作全程记录审计
- **反作弊机制**：安全专员监控异常行为

### **4. 跨团队协作权限**
- **设计-开发协作**：设计师可查看开发进度
- **运营-数据协作**：运营可获取数据分析支持
- **客服-GM协作**：客服可申请GM介入处理

## 配置文件说明

### **完整配置文件**
- `configs/rbac.game-roles.json` - 23个角色的权限配置
- `configs/users.game-roles.json` - 对应的用户账号配置
- `scripts/setup-game-roles.sh` - 一键部署脚本

### **快速部署**
```bash
# 查看配置
./scripts/setup-game-roles.sh
# 选择选项2预览配置

# 应用配置
./scripts/setup-game-roles.sh
# 选择选项1应用配置

# 重启服务
make dev
```

这个完整的游戏团队角色权限体系覆盖了游戏开发和运营的全生命周期，确保每个专业角色都有合适的权限来完成工作，同时保持系统的安全性和游戏的公平性。