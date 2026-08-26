---
title: 内部工具集成（CI/CD 等）设计——工具箱 Tool Registry
---

# 内部工具集成设计：工具箱（Tool Registry）

## 状态

Proposed → P1 已落地（链接注册表 + 研发域工具箱页面）。P2/P3 见 §4。

## 1. 问题

团队内部有大量配套工具：Jenkins/GitLab CI/GitHub Actions（构建发布）、Grafana（监控）、Wiki（文档）、制品库、内部平台……散落在各处书签里。用户诉求：**集中控制、统一入口**，并且能与 croupier 的工作流（缺陷/任务/发布）互相关联。

## 2. 方案对比

| 方案              | 描述                                           | 评价                                                                 |
| ----------------- | ---------------------------------------------- | -------------------------------------------------------------------- |
| A. 单向链接注册表 | 管理员登记工具 URL+分类，门户集中展示跳转      | 零凭证、零同步风险；不显示工具内部状态                               |
| B. 深度 API 集成  | Jenkins API 拉构建状态、GitLab API 拉 pipeline | 需管理各工具的 API token（密钥管理成本）；每接一个工具定制一份适配器 |
| C. 全量替代       | 在 croupier 内实现 CI 编排                     | 明确不做——Jenkins/GitLab 已是成熟系统，重复造轮子                    |

**选择 A 起步，B 按需渐进**：与缺陷追踪外部链接（bug-tracking-design.md §2.3）同一哲学——**链接跳转式集成优先**，深度集成只在明确价值处做（如 Jenkins 构建状态显示在发布看板）。

## 3. P1 设计（已落地）

### 3.1 数据模型

```
ToolLink
├── Name/URL/Description
├── Category   ci | repo | monitor | docs | artifact | other（受控枚举，驱动图标/分组）
├── Icon       可选自定义图标 URL（缺省按 category 显示内置图标）
├── GameID/Env 可选作用域（空 = 全局工具；指定 = 仅该游戏环境可见）
├── Enabled    软开关（下线工具保留配置）
├── Sort       排序
└── CreatedBy
```

- 工具登记是**管理动作**（`tools:manage`），查看是研发域读权限（`dev:read`）；
- 按 category 分组渲染（CI/CD、代码仓库、监控、文档、制品、其他），外部新窗打开；
- 受 `featureFlags.dev` 控制（工具箱挂在研发域下）。

### 3.2 API

- `GET /api/v1/tools`（列表，按 scope 过滤：全局 + 当前 game/env）
- `POST/PUT/DELETE /api/v1/tools/:id`（管理）

### 3.3 前端

研发域新增「工具箱」页（`/dev/tools`）：分类卡片网格 + 管理员内联增删改；空态引导登记第一个工具（Jenkins/GitLab/Grafana 常见示例提示）。

## 4. 后续阶段

| 阶段 | 内容                                                                                                                     | 触发条件                     |
| ---- | ------------------------------------------------------------------------------------------------------------------------ | ---------------------------- |
| P2   | Jenkins/GitLab 状态拉取：登记时可选填 API token（加密存储），工具卡片显示最近构建状态；缺陷 links 关联 CI 时展示状态徽标 | 用户明确要"看状态而不用跳走" |
| P3   | 事件回流：Jenkins webhook（构建失败 → 告警中心/关联缺陷评论）；发布看板聚合（fixVersion → 构建链路）                     | 发布流程数字化时             |

## 5. Review Checklist

- category 是受控枚举（前端图标映射依赖），新增值需前后端同步；
- URL 必须校验 http(s)，防 `javascript:` 注入（链接直接 target=_blank 打开）；
- 登记/编辑必须 `tools:manage` 权限 + 审计（admin.operation）；
- P2 引入 token 时必须走加密存储与掩码展示（参考 certificates 的 secret 处理）。
