---
title: 网站配置中心设计（Site Settings）
---

# 网站配置中心设计（Site Settings）

## 状态

Proposed → P1 已落地。对标 new-api 的「系统设置」页：管理员在 UI 上修改站点品牌/页脚/登录页文案，**保存即生效、无需重新构建或重启**。

## 1. 问题

站点外观与文案当前是**构建期硬编码**，改个名字都要重新构建前端：

| 项                   | 现状                                                | 位置   |
| -------------------- | --------------------------------------------------- | ------ |
| 产品名称/副标题/logo | `web/src/config/branding.ts` + `defaultSettings.ts` | 构建期 |
| 登录页文案           | 同上（BRAND）                                       | 构建期 |
| 页脚链接/版权        | `components/Footer` 组件内写死 Ant Design Pro 链接  | 组件内 |
| favicon              | `config.ts favicons` + `/public`                    | 构建期 |
| 默认语言             | locale 配置 + 用户偏好                              | 混合   |

## 2. 与相邻系统的边界

| 系统                     | 管什么                                            | 不管什么                   |
| ------------------------ | ------------------------------------------------- | -------------------------- |
| **SiteSettings（本文）** | 全站单例的「门面」配置：品牌/页脚/登录页/默认语言 | 业务数据、按游戏区分的东西 |
| featureFlags             | 域级开关（有没有）                                | 外观与文案                 |
| ConfigVersion            | 按 game/env 的业务运行时配置（数值表/活动/IAP）   | 全站单例的门面项           |

判定规则：**一个部署一份、与 game 无关、纯展示** → SiteSettings；其余归各自系统。

## 3. 数据模型

```
SiteSetting（单行结构化，namespace=site 的特化存储）
├── ID (固定=1，启动 upsert)
├── Branding
│   ├── siteName        string  顶部标题/登录页主标题
│   ├── logoUrl         string  头部 logo（/public 路径或外链）
│   ├── faviconUrl      string
│   └── description     string  登录页副标题
├── Footer
│   ├── copyright       string
│   ├── icp             string  备案号（可选，渲染为 beian.miit 链接）
│   └── links           JSON [{key,title,url}]（可空 = 用默认 GitHub 链接）
├── Preference
│   └── defaultLocale   zh-CN | en-US
└── UpdatedBy / UpdatedAt
```

- 存储用 `SiteSetting` 独立表（迁移 0013），不做 KV：字段演进可控、类型安全；
- 所有字符串走既有 LocalizedText 规范的地方（如 notes 类）不在此列——站点文案默认单语言由管理员自填。

## 4. API

| 端点                      | 认证       | 说明                                        |
| ------------------------- | ---------- | ------------------------------------------- |
| `GET /api/v1/public/site` | 公开       | 登录页/未登录场景也要取；返回全部展示类字段 |
| `PUT /api/v1/site`        | admin 权限 | 全量保存（表单式 UI），审计记录操作者       |

- 公开端点进认证白名单（同 public/support 模式）；
- 前端 `getInitialState` 启动拉一次，失败降级到构建期默认值（fail-open，与 feature flags 同语义）；
- 保存后前端通过 `setInitialState` 即时刷新（无感生效，不用刷新页面）；登录页因 layout=false 在下次进入时生效。

## 5. 前端消费点改造

| 消费点                       | 改造                                                          |
| ---------------------------- | ------------------------------------------------------------- |
| `defaultSettings.title/logo` | 变为兜底默认；layout 从 `initialState.siteConfig` 覆盖        |
| `branding.BRAND`（登录页）   | 同上兜底                                                      |
| `components/Footer`          | 读 siteConfig：copyright/icp/links；空则维持现状              |
| favicon                      | P2：动态注入 link 标签（P1 保持构建期，favicon 动态换收益低） |

## 6. 分阶段

| 阶段             | 内容                                                                                   |
| ---------------- | -------------------------------------------------------------------------------------- |
| **P1（已落地）** | 模型+迁移+公开 GET/admin PUT+系统管理「网站配置」页+Footer/Login/layout 三处消费点接入 |
| P2               | favicon/favicon 动态注入；主题色 colorPrimary 可配；多语言站点文案（LocalizedText 化） |
| P3               | 登录页 banner 图上传（复用 objstore）；邮件 SMTP 配置等运维向设置迁入同一页面          |

## 7. Review Checklist

- `GET /public/site` 返回的内容必须全部可公开（不得混入密钥/内部地址）；
- 新增站点配置项需同步：模型字段 + DTO + 前端表单 + 消费点，四处缺一即 review failure；
- 未初始化时所有 getter 必须回退构建期默认值（新装体验不受影响）；
- 保存操作写审计（admin.operation）。
