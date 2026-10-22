# croupier 手机端原型（移动优先可视稿）

静态 HTML 原型：375 宽视口（iPhone 基准），深色单主题。供产品评审用，不含业务逻辑。

## 产品定位

GM 运营平台的移动侧——**随时接住告警与审批，随手完成低风险操作**。
重型配置与批量操作仍归桌面端；手机端解决「人不在工位时的事故响应与审批时效」。

## 页面清单与截图

| 页面 | 文件 | 截图 |
| --- | --- | --- |
| P1 登录 | `login.html` | `screenshots/p1-login.png` |
| P2 首页看板 | `dashboard.html` | `screenshots/p2-dashboard.png` |
| P3 服务器列表与状态 | `servers.html` | `screenshots/p3-servers.png` |
| P4 告警处理 | `alerts.html` | `screenshots/p4-alerts.png` |
| P5 远程操作入口 | `ops.html` | `screenshots/p5-ops.png` |
| P6 我的设置 | `settings.html` | `screenshots/p6-settings.png` |

索引页：`index.html`（浏览器打开可逐页跳转；每页手机稿下方附「设计说明」：
布局 / 关键交互 / 信息层级 三节）。

截图由 Playwright 对 `.screen` 元素截图产出（375×812，deviceScaleFactor 2）。

## 设计基线

- **基调**：Linear 风格暗色管理台（画布 `#08090a`、面板 `#0f1011`、半透明描边），
  叠加 GM 运营状态色体系。
- **状态色**（全站贯通，与桌面端运维台同语义）：
  - 在线/成功 `#4cb782` · 警告 `#f5a623` · 紧急 `#eb5757` · 信息 `#5e9eff`
  - 环境 chip：`PROD` 红 / `DEV` 蓝 / `TEST` 紫
- **强调色** `#5e6ad2`：**中性占位色，非最终品牌色**。
- **品牌位**：灰色字母 tile（"C"）为中性占位，**不伪造任何 logo**；定稿时由
  品牌方提供资产替换 `app.css` 中 `.brand-tile` / `.wordmark`。
- **字体**：Inter（回退 PingFang SC / Noto Sans SC）；机器标识（函数名、节点名、
  规则 ID、版本号）一律 JetBrains Mono。
- **导航**：底部四 Tab（首页 / 服务器 / 告警 / 我的）+ 中央凸起「执行」入口——
  远程操作是这个工具的核心动作，给一级空间。

## 跨页一致性约定

1. **审批语义三处强化**：P2 指标卡（琥珀计数）→ P5 风险提示条 → P5 执行记录
   「审批中」态，颜色与文案同一套。
2. **scope 联动**：game/env 切换在 P2/P5 顶栏以 pill 呈现，切 PROD 二次确认（P6）。
3. **Schema 驱动**：P5 参数表单由函数契约 JSON Schema 生成，与桌面端同源，
   不做移动端专属字段定义。
4. **颜色只给需要行动的项**：健康指标中性白，异常数值才上色。

## 已知边界

- 静态可视稿：无路由/请求/状态逻辑；交互以设计说明文字表达。
- 数据均为演示占位，非真实游戏数据。
- 深色单主题；浅色主题与平板断点未纳入本稿。
- 品牌资产（logo/品牌色）待用户提供后替换。

## 复现截图

```bash
# 仓库根目录
node web/node_modules/@playwright/test -> 需要 playwright chromium
# 截图脚本见本目录 screenshots/ 生成方式（Playwright element screenshot）
```
