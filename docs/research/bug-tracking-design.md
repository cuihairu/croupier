---
title: 缺陷追踪（Bug Tracking）选型调研与游戏行业设计
---

# 缺陷追踪选型调研与游戏行业设计

## 状态

Accepted（调研结论 + P1 设计与落地）。缺陷从客服工单模型中独立，具备游戏上下文、复现信息与外部链接（GitHub 等）。

## 1. 问题

当前「客服系统 → 缺陷列表」复用 Ticket 模型（`category: 'bug'` 过滤视图）。这确实不合理：

| 维度     | 工单（Ticket）                                         | 缺陷（Bug）                                                 |
| -------- | ------------------------------------------------------ | ----------------------------------------------------------- |
| 消费者   | 客服（GM）                                             | 研发/测试/策划                                              |
| 生命周期 | open → in_progress → resolved → closed（服务请求闭环） | triage → confirmed → fixing → verify → released（版本闭环） |
| 关键字段 | 玩家联系方式、情绪安抚                                 | 严重度、复现步骤、影响版本、修复版本、外部链接              |
| 终态     | 玩家满意即关闭                                         | **随版本发布关闭**（fix 后还要等 hotfix 上线）              |
| 归属     | 客服系统                                               | 研发协作域                                                  |

工单是"对人的服务"，缺陷是"对产品的修复追踪"——两者只存在**单向转化**关系（玩家反馈升级为缺陷），不应共用一张表。

## 2. 市面产品调研

### 2.1 现代 bug 追踪标杆

| 产品                                     | 设计亮点                                                                                                                         | 游戏适配 | 许可               |
| ---------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | -------- | ------------------ |
| **Linear**                               | 业界公认的现代化标杆：Triage 状态（先分流再认领）、Cycle 迭代、severity 与 priority 分离、sub-issues、键盘优先、外部链接关联     | ★★★★     | SaaS               |
| **GitHub Issues**                        | 标签/milestone/assignee、issue 模板（bug report 内置复现步骤/环境）、与 PR/commit 双向自动关联、closing keywords（`fixes #123`） | ★★★      | 免费开源生态       |
| **Jira**                                 | 企业标准：epic/story/**bug** 层级、affectsVersion/**fixVersion** 双版本字段、workflow 引擎、组件（按系统模块分派）               | ★★★★     | 商业               |
| **Shortcut（原 Clubhouse）**             | 游戏公司偏爱（Scopely 等）：story/bug 分离、Iteration、文件级关联                                                                | ★★★★     | SaaS               |
| **YouTrack**                             | JetBrains：敏捷查询语言、自定义字段极强、按规则自动分派                                                                          | ★★★      | 商业（小团队免费） |
| **Azure DevOps Boards**                  | bug 可配置为 requirement 或 task，企业级查询                                                                                     | ★★       | 商业               |
| **Plane / Redmine / Bugzilla / Backlog** | 开源系：Plane 是现代开源 Linear-like；Redmine/Bugzilla 传统；Backlog 日系（游戏公司 Nulab 出品，评论+协作为核心）                | ★★       | 开源               |

### 2.2 值得借鉴的具体设计

1. **Linear 的 Triage 流**：新缺陷先进 `triage`（未确认），确认后进 `confirmed`——避免未复现的误报污染研发队列；
2. **Jira 的双版本**：`affectsVersion`（哪些版本受影响）+ `fixVersion`（随哪个版本修复）——游戏热修复节奏下尤其重要；
3. **GitHub 的模板与关联**：创建时引导填复现步骤/环境；正文与评论支持外部链接自动识别；状态变更自动通知；
4. **severity ⊥ priority**：严重度（对玩家影响）与优先级（排期顺序）正交——"轻微但挡板需求"可以 low severity + high priority；
5. **Shortcut 的 story/bug 分离**：同一系统内两种工作项类型，不混表。

### 2.3 结论：自建轻量缺陷模型，而非集成外部 tracker

理由（与客服调研一致，见 game-support-systems.md §2.4）：

1. **游戏上下文不可外流**：缺陷需带 playerId/区服/设备（可查玩家档），外部 SaaS 无法与 GM 函数调用联动；
2. **croupier 的独特闭环**：玩家工单 → 一键升级缺陷（自动带玩家上下文）→ 修复 → 关联 GitHub PR/issue 跟踪发布——工单在系统内才能完成转化；
3. **外部链接集成即可**：团队已用 GitHub/GitLab 管代码，缺陷本体在 croupier、**通过外部链接跳转**关联 issue/PR/wiki/监控面板，比全量同步（双向 webhook）成本低两个数量级且不会漂移。

## 3. 游戏行业缺陷设计（P1 已落地）

### 3.1 数据模型

```
Bug
├── 标题/描述（Markdown）
├── Status      triage → confirmed → fixing → verify → released → wontfix/rejected
├── Severity    blocker(阻断) critical(严重) major(一般) minor(轻微)
├── Priority    urgent | high | normal | low（与 severity 正交）
├── Assignee    负责人
├── 游戏上下文   gameId/env + serverId + platform(iOS/Android/PC/WebGL/Editor) + deviceModel + osVersion
├── 复现信息     steps(复现步骤) + reproducibility(always/often/sometimes/once)
├── 版本        affectsVersion（客户端/后端版本） + fixVersion（修复版本）
├── 来源        source: player(GM录入) | internal(测试) | ticket(工单转化)
├── SourceTicketID  转化来源工单（反向可查"这个缺陷来自哪些玩家投诉"）
├── Links       []BugLink{url, kind(github_issue|github_pr|gitlab|jira|wiki|monitor|other), title}
└── Extra       自由 JSON（崩溃上报 ID、监控面板参数等）
```

### 3.2 与工单的转化链路

```
玩家工单(Ticket, category=bug)  ──转化──▶  Bug(source=ticket, sourceTicketID=N)
                                              自动携带: 玩家上下文/设备/区服/复现描述
工单列表保留 category=bug 兼容视图(只读过渡);新缺陷一律走 Bug 模型
```

### 3.3 外部链接

- 每条缺陷可挂多条链接，`kind` 枚举驱动图标与跳转行为（GitHub issue/PR 直接跳转，wiki/monitor 打开新窗）；
- 输入 GitHub URL 时自动识别 `owner/repo#number` 作为展示标题；
- 这是 P1 的"跳转集成"；双向同步（GitHub webhook 回写状态）列为 P2 可选。

### 3.4 命名说明：Bug 而非 Issues

GitHub 用 Issues 是因为其**泛议题定位**（bug/需求/讨论混装，靠标签区分），而本模型是纯缺陷语义（severity/复现步骤/双版本字段，无需求字段），叫 Issues 会产生"可提需求"的错误预期。若未来演化为研发协作中心（缺陷+需求+讨论），再以 `type` 字段（bug|feature|question）扩展并更名，避免现在做空泛承诺。

### 3.5 后续阶段

| 阶段 | 内容                                                                                                                                                                             |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| P2   | 工单列表「升级为缺陷」按钮（一行完成转化）；崩溃聚合（同 stack 聚合为一条缺陷+计数）                                                                                             |
| P3   | GitHub App/webhook 双向同步（issue 关闭 → 缺陷自动 verify）；版本发布看板（某 fixVersion 下缺陷清单导出 changelog）；可选演进为 Issues（type=bug\|feature\|question 多类型议题） |

## 4. 菜单归属

缺陷从「客服系统」移出，与函数/页面同属研发协作域：「函数与页面 → 缺陷追踪」（`/functions/bugs`）。客服侧仅保留工单/FAQ/反馈——服务与产品问题分流。

## 5. Review Checklist

- Bug 模型字段变更走编号迁移；
- 新增外部链接 kind 必须是受控枚举（前端图标映射依赖），不可自由文本；
- 工单→缺陷转化必须携带玩家上下文，禁止只拷标题；
- status/severity/priority 的取值集合变更需同步前端色板与标签映射。
