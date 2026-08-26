---
title: 版本发布系统设计——资源管理 / CDN / 灰度放量
---

# 版本发布系统设计（Release Management）

## 状态

Proposed → P1 已落地（版本模型 + OSS 资源托管 + 灰度状态机 + 客户端检查更新 API + 发布台前端）。

## 1. 问题

游戏版本发布是一条独特流水线：**客户端资源包（热更/整包）→ 对象存储 → CDN 分发 → 按渠道/区服/百分比灰度放量 → 全量 → 出问题秒级回滚**。croupier 已有 objstore（file/S3/OSS/COS 四驱动）但缺少版本化语义：哪个包属于哪个版本、放了多少、谁在用哪个版本。

## 2. 市面方案调研

| 产品                                          | 定位                           | 游戏适配 | 评价                                    |
| --------------------------------------------- | ------------------------------ | -------- | --------------------------------------- |
| **Firebase App Distribution + Remote Config** | 包分发 + 灰度配置              | ★★★      | Google 生态，国内不可达；无游戏区服语义 |
| **阿里云 EMAS**                               | 移动发布/热修复                | ★★★★     | 国内主流，但 SaaS 定制弱、数据外流      |
| **蒲公英 / fir.im**                           | 内测包分发（上传→二维码→下载） | ★★       | 只有分发无灰度；面向移动测试            |
| **Bugly 升级 SDK**（腾讯）                    | 全量/灰度升级                  | ★★★★     | 灰度按百分比/分组，但强绑腾讯系         |
| **LaunchDarkly**                              | feature flag 灰度              | ★★       | 配置级灰度（非包分发），按 MAU 收费     |
| **Unity Cloud Build / Addressables**          | 构建 + 资源寻址                | ★★★      | 生成侧，分发/灰度仍需自建               |
| **CodePush（已退役）/ Capgo**                 | RN/Capacitor 热更              | ★        | 非 Unity/原生游戏场景                   |

**结论：自建。** 与客服/缺陷同理——灰度规则需要游戏语义（渠道/区服/设备白名单 + GM 联动：出问题直接关联缺陷/回滚），且资源包属公司资产应在自有 OSS；市面方案都只覆盖一段。分发生义上直接复用 objstore 四驱动 + 云厂商 CDN（刷新 API P2 接入），**不自建下载源站**。

## 3. P1 设计（已落地）

### 3.1 数据模型

```
GameRelease
├── GameID/Env           作用域
├── Channel              渠道（official/ios/android/tapdoc/taptap…，自由字符串）
├── Version              语义化版本（如 1.5.0）
├── Platform             ios|android|pc|webgl
├── Type                 hotfix（热更资源包）| full（整包）| forced（强更配置）
├── Status               状态机（3.2）
├── 资源
│   ├── ObjectKey        objstore 对象键（包文件）
│   ├── Size             字节
│   ├── Checksum         SHA-256（完整性校验）
│   └── Manifest         JSON 资源清单（文件→hash 映射，客户端差量下载用）
├── Notes                更新说明（LocalizedText 契约）
├── 灰度
│   ├── GrayPercent      当前放量百分比（0-100）
│   ├── Whitelist        白名单（设备/区服/玩家 ID 列表，JSON）
│   └── GrayBuckets      灰度桶种子（设备 hash 分桶，保证同设备稳定命中）
└── CreatedBy / DueAt
```

### 3.2 发布状态机

```
draft（草稿，可改） → uploading（传包校验中） → testing（内测：仅白名单可取）
   → gray（灰度：白名单 + 设备 hash 百分比分桶命中）
   → full（全量：所有设备可取）
任意非 full 状态 → archived（废弃）
full → rolled_back（回滚：客户端取不到该版，回退上一 full 版本）
```

不变量：同 `(game, env, channel, platform)` 同时最多一个 `full`；`gray` 的 Percent 只增不减（减 = 回滚语义，走 rolled_back）；状态只能向前推进或归档/回滚（不可从 full 退回 gray 再推进，避免客户端缓存混乱）。

### 3.3 灰度命中算法（服务端，客户端零逻辑）

```
check-update(deviceId, channel, platform):
  1. 找该 (game,env,channel,platform) 的最高优先级版本：
     testing → 设备在 Whitelist ? 命中 : skip
     gray    → 设备在 Whitelist 或 bucket(deviceId, seed) < GrayPercent ? 命中 : skip
     full    → 命中
     rolled_back/archived → skip
  2. 命中版本与客户端当前版本比较（semver），高则返回 {version, url, size, checksum, manifest, forced}
  3. url 为 objstore 下载 URL（P1 直接下载；P2 换 CDN 域名 + 预热）
```

分桶：`sha256(deviceId + GrayBuckets)[:8] % 10000 / 100 < percent`——同设备结果稳定，放量只影响新进入桶的设备。

### 3.4 API

管理面（`releases:manage`，dev flag 域）：

- `POST /api/v1/releases` 创建（draft）
- `POST /api/v1/releases/:id/artifact` 上传资源包（multipart → objstore → checksum/size 回填 → testing 可推进）
- `POST /api/v1/releases/:id/transition` 状态推进（action: testing|gray|full|archive|rollback，gray 附 percent）
- `GET /api/v1/releases` 列表/筛选

客户端（公开，游戏内 SDK 调用）：

- `POST /api/v1/releases/check` 检查更新（deviceId/channel/platform/currentVersion → 命中版本或 latest）

### 3.5 前端

研发域「版本发布」页（`/dev/releases`）：版本列表（状态/渠道/灰度百分比）、创建对话框、状态推进（含灰度百分比滑杆）、回滚二次确认；资源包上传走管理面 artifact 端点。

## 4. 后续阶段

| 阶段    | 内容                                                                                                                                                      |
| ------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| P2 CDN  | 云厂商刷新/预热 API（阿里云 RefreshObjectCaches/PushObjectCache，腾讯 Purge/Push）；版本 full 时自动预热；下载 URL 域名可配（CDN 域名替换 objstore 直链） |
| P2 增量 | manifest 差量下发（客户端已有版本 → 只返回变更文件列表）                                                                                                  |
| P3 数据 | 灰度监控：检查量/下载量/到达率按版本统计（复用 analytics 管道），异常自动暂停灰度                                                                         |
| P3 强更 | forced 类型版本携带最低版本策略（低于 X 强更弹窗/踢线），与服务端版本联动                                                                                 |

## 5. Review Checklist

- 状态机迁移必须校验合法路径（service 层 switch），非法迁移 400 而非静默落库；
- `full` 唯一性在事务内保证（同 scope 渠道平台）；
- artifact 上传必须校验 checksum 后才允许离开 draft；
- check-update 是公开端点：不得泄露未发布版本的 notes；deviceId 仅用于分桶不做存储（P3 统计需匿名化）；
- 新增状态/类型值需同步前端映射与文档状态机图。
