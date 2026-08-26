---
title: 平台配置分层设计——配置文件与数据库的唯一来源问题
---

# 平台配置分层设计：配置文件 vs 数据库的唯一来源

## 状态

Accepted（分层模型定稿）。原则一句话：**逐层覆盖，L3 最高；bootstrap 类配置永远只存在于 L2**。

## 1. 问题

网站配置中心（site-settings-design.md）引出通用问题：一个配置项同时存在于配置文件和数据库时，谁是唯一来源？

采用业界标准的**分层覆盖模型**（Kubernetes/Spring Cloud 同款思路）：

```
L1 代码内置默认      编译期，兜底
L2 配置文件 / env    部署期，server.yaml + CROUPIER_* 环境变量
L3 数据库配置        运行时，管理台修改，即时生效（最高优先级）
```

读取顺序：L1 ← L2 ← L3，**逐层覆盖，L3 最高**。全系统唯一规则，不允许任何配置反向。

## 2. 为什么这个方向是对的

1. **新部署零配置可用**：装好就能跑（L1 全兜底），逐步按需定制；
2. **基础设施即代码**：部署相关的选择留在 server.yaml 进 Git review；
3. **运营改动免重启**：品牌/页脚/开关这类高频变更走 L3，改完即生效；
4. **升级安全**：新版本新增字段自动获得 L1 默认值，存量部署不被破坏。

## 3. 四个必须正视的坑与对策

### 坑 1：默认值漂移（最经典的困惑）

场景：yaml 写 `logo: a.png`，管理员在 UI 改成 `b.png`（L3 生效）。之后运维改 yaml 为 `c.png` 并重启——**页面仍是 b.png**，运维以为改错了。

对策（三条一起做）：

- UI 显示来源徽标：「数据库覆盖」/「跟随配置文件」；
- 提供一键「恢复跟随配置文件」= 删除 L3 记录（不是把 yaml 值拷进 DB）；
- 文档显著位置写明本节。

### 坑 2：Bootstrap 鸡生蛋

读 L3 必须先连数据库；而数据库连接信息（DSN/host/port/JWT secret）不可能存在数据库里。

**铁律：bootstrap 类配置只存在于 L2，永不进 L3 白名单。**

### 坑 3：优先级不一致

绝不允许出现“A 配置 file 优先、B 配置 DB 优先”。唯一规则见 §1，代码里用统一的 `PlatformSettings.Get(key)` 入口，禁止散落的 if。

### 坑 4：什么都能进 L3 会失控

L3 白名单制（§4 表格标注）。准入标准：**纯运行时行为/展示、无安全边界、改错可立即改回**。

## 4. 全量配置归类表

| 配置                                        | 层                         | 说明                                                                                        |
| ------------------------------------------- | -------------------------- | ------------------------------------------------------------------------------------------- |
| database DSN / multiGame                    | **L2 only**                | bootstrap                                                                                   |
| server host/port/TLS                        | **L2 only**                | bootstrap                                                                                   |
| auth JWT secret                             | **L2 only**                | 安全 + bootstrap                                                                            |
| storage driver/bucket                       | **L2 only**                | bootstrap                                                                                   |
| telemetry/collector                         | **L2 only**                | 启动期接线                                                                                  |
| featureFlags 五域开关                       | **L2 + L3 覆盖（已落地）** | L2=物理裁剪（路由注册）；L3=运行时软开关（middleware 403），合成 L2∧L3，key 为 `features.*` |
| 观测集成 URL（alertmanager/grafana/jaeger） | **L3 主战场（已落地）**    | key 为 `obs.*`，自 OpsStateStore 内存态迁入，重启不丢；env var 为 L2 兜底                   |
| 站点品牌/logo/页脚/登录页                   | **L3 主战场**              | site-settings-design.md                                                                     |
| 默认语言                                    | L2 缺省 + L3 覆盖          |                                                                                             |
| 告警阈值默认（dbmon 等）                    | L1 内置 + 未来 L3          |                                                                                             |
| SMTP/通知渠道（P2 待迁）                    | 未来 L3                    | 审批/告警通知的配置来源                                                                     |
| 游戏业务数值/活动/IAP                       | 不属于平台配置             | 走 ConfigVersion（game-scoped，另有一套）                                                   |

## 5. 实现要点

```
internal/platform/settings
├── Store        接口：Get(key) / Set(key,value) / Clear(key)=恢复L2
├── dbStore      platform_settings 表（key unique, value json, updated_by）
└── Layered      组合 L1 defaults map + L2 (已解析的 config.Config) + L3 dbStore

GET /api/v1/public/site       → Layered 快照（公开字段）
PUT /api/v1/site/:key         → 写 L3 + 审计
DELETE /api/v1/site/:key      → 清 L3 = 「恢复跟随配置文件」
响应体带 source 字段: "default"|"config"|"database"
```

- 启动时 L2 已由 config 包解析完成；L3 在 NewServiceContext 后异步加载，加载失败 fail-open 到 L2（同 feature flags 哲学）；
- 变更通知：前端保存后 setInitialState 即可；服务端消费方（如未来 SMTP）注册 OnChange 回调。

## 6. 分阶段

| 阶段             | 内容                                                                                                                             |
| ---------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| **P1（已落地）** | platform_settings 表 + Layered 读取 + 站点品牌/页脚/登录页三类 key + 来源徽标 + 恢复按钮                                         |
| **P2（已落地）** | features.* 五域运行时软开关（L2∧L3 合成 + middleware 拦截 + 设置中心 UI）；obs.* 观测 URL 迁入 L3（旧 OpsStateStore 值兼容读取） |
| P3               | SMTP/通知渠道迁入 L3（审批/告警通知配置来源）；按 key 权限细分；变更历史对比视图                                                 |

## 7. Review Checklist

- 新增 L3 白名单项必须在 §4 表格登记归属层，否则 review failure；
- 任何配置读取不得绕过 Layered 入口直连 config 或直查表；
- bootstrap 类出现在 L3 白名单 = 一票否决；
- Clear（恢复 L2）与 Set 必须都有审计记录。
