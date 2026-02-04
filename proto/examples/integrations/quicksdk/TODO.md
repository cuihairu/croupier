# QuickSDK 集成任务拆分

## 项目概述

接入 QuickSDK 开放平台的所有 20 个运营数据接口到 Croupier 系统。

**QuickSDK API 文档**: https://www.quicksdk.com/doc-1133.html

**参考**: Prometheus 集成示例 `examples/integrations/prom/v1/prom.proto`

---

## 阶段一：Proto 定义 (3-4 人日)

### 1.1 创建目录结构
- [ ] 创建 `examples/integrations/quicksdk/v1/` 目录
- [ ] 创建 `quicksdk.proto` 文件

### 1.2 定义基础消息类型
- [ ] 定义 `QuicksdkConfig` - API 配置 (openId, openKey, baseUrl)
- [ ] 定义 `QuicksdkRequest` - 通用请求基类 (time, sign)
- [ ] 定义 `QuicksdkResponse` - 通用响应基类 (status, message, data)

### 1.3 定义基础数据接口消息 (5个)
- [ ] `GetChannelListRequest/Response` - 渠道列表
- [ ] `GetServerListRequest/Response` - 区服列表
- [ ] `GetProductListRequest/Response` - 产品列表
- [ ] `GetRoleInfoRequest/Response` - 角色信息
- [ ] `GetOrderListRequest/Response` - 订单列表

### 1.4 定义运营报表接口消息 (5个)
- [ ] `GetDayReportRequest/Response` - 单日报表
- [ ] `GetDayHourReportRequest/Response` - 每小时报表
- [ ] `GetUserLiveRequest/Response` - 玩家留存
- [ ] `GetChannelDaysReportRequest/Response` - 渠道报表（多日）
- [ ] `GetChannelReportRequest/Response` - 渠道日报表

### 1.5 定义广告管理接口消息 (8个)
- [ ] `GetAdReportRequest/Response` - 广告效果报表
- [ ] `GetMediaAppListRequest/Response` - 广告主列表
- [ ] `GetAdPlanGroupListRequest/Response` - 广告分组列表
- [ ] `GetPackageVersionListRequest/Response` - 分包列表
- [ ] `GetAdPagesListRequest/Response` - 落地页列表
- [ ] `CreateAdPlanRequest/Response` - 创建广告计划
- [ ] `UpdateAdPlanRequest/Response` - 更新广告计划
- [ ] `GetAdPlanListRequest/Response` - 广告计划列表

### 1.6 定义其他功能接口消息 (2个)
- [ ] `GetUserLostListRequest/Response` - 流失预警
- [ ] `PushMessageRequest/Response` - 消息推送

### 1.7 定义服务
- [ ] 定义 `QuicksdkDataService` 服务
- [ ] 为每个 RPC 添加 function 注解
- [ ] 为请求参数添加 UI 注解（如需要）

---

## 阶段二：Go SDK 实现 (8-12 人日)

### 2.1 核心 HTTP 客户端 (2-3 人日)
- [ ] 创建 `pkg/integrations/quicksdk/` 目录
- [ ] 实现 `QuicksdkConfig` 结构体
- [ ] 实现 `QuicksdkClient` 核心客户端
  - [ ] HTTP POST 请求封装
  - [ ] 签名算法实现 (MD5)
  - [ ] 重试机制
  - [ ] 超时控制
  - [ ] 错误处理

### 2.2 基础数据接口实现 (2-3 人日)
- [ ] 实现 `GetChannelList()` - `open/channelList`
- [ ] 实现 `GetServerList()` - `open/serverList`
- [ ] 实现 `GetProductList()` - `open/productList`
- [ ] 实现 `GetRoleInfo()` - `open/roleInfo`
- [ ] 实现 `GetOrderList()` - `open/orderList`

### 2.3 运营报表接口实现 (2-3 人日)
- [ ] 实现 `GetDayReport()` - `open/dayReport`
- [ ] 实现 `GetDayHourReport()` - `open/dayHourReport`
- [ ] 实现 `GetUserLive()` - `open/userLive`
- [ ] 实现 `GetChannelDaysReport()` - `open/channelDaysReport`
- [ ] 实现 `GetChannelReport()` - `open/channelReport`

### 2.4 广告管理接口实现 (4-5 人日)
- [ ] 实现 `GetAdReport()` - `open/adReport`
- [ ] 实现 `GetMediaAppList()` - `open/getMediaApp`
- [ ] 实现 `GetAdPlanGroupList()` - `open/getAdPlanGroup`
- [ ] 实现 `GetPackageVersionList()` - `open/getPackageVersion`
- [ ] 实现 `GetAdPagesList()` - `open/getAdPages`
- [ ] 实现 `CreateAdPlan()` - `open/createAdPlan`
- [ ] 实现 `UpdateAdPlan()` - `open/updateAdPlan`
- [ ] 实现 `GetAdPlanList()` - `open/getAdPlan`

### 2.5 其他功能接口实现 (1-2 人日)
- [ ] 实现 `GetUserLostList()` - `open/uwlLost`
- [ ] 实现 `PushMessage()` - `open/pushMessage`

### 2.6 Croupier SDK 集成 (2-3 人日)
- [ ] 创建 Quicksdk 函数处理器
- [ ] 实现 20 个函数的 Croupier 注册
- [ ] 添加函数描述符 (FunctionDescriptor)
- [ ] 集成到 SDK 示例代码

---

## 阶段三：Python SDK 实现 (6-8 人日)

### 3.1 核心 HTTP 客户端
- [ ] 创建 `croupier_sdk_quicksdk/` 包
- [ ] 实现 `QuicksdkConfig` 类
- [ ] 实现 `QuicksdkClient` 类
  - [ ] HTTP POST 请求封装
  - [ ] 签名算法实现
  - [ ] 重试机制
  - [ ] 错误处理

### 3.2 接口实现
- [ ] 实现 20 个 API 接口方法
- [ ] 实现 Proto 消息转换
- [ ] 实现 Croupier 函数注册

### 3.3 测试与文档
- [ ] 单元测试
- [ ] 集成测试
- [ ] 使用文档

---

## 阶段四：其他 SDK 实现 (可选)

### 4.1 Java SDK
- [ ] 核心 HTTP 客户端
- [ ] 20 个接口实现
- [ ] Croupier 集成

### 4.2 C++ SDK
- [ ] 核心 HTTP 客户端
- [ ] 20 个接口实现
- [ ] Croupier 集成

### 4.3 JavaScript/TypeScript SDK
- [ ] 核心 HTTP 客户端
- [ ] 20 个接口实现
- [ ] Croupier 集成

---

## 阶段五：测试与文档 (4-6 人日)

### 5.1 单元测试
- [ ] Go SDK 单元测试 (覆盖率 > 80%)
- [ ] Python SDK 单元测试
- [ ] Mock QuickSDK API 响应

### 5.2 集成测试
- [ ] 端到端测试
- [ ] 与真实 QuickSDK API 测试（如果有测试环境）

### 5.3 文档
- [ ] API 使用文档
- [ ] 配置说明文档
- [ ] 示例代码
- [ ] README 更新

---

## QuickSDK 接口速查表

| # | 接口名称 | 接口地址 | 请求方式 | 优先级 |
|---|---------|---------|---------|-------|
| 1 | 获取游戏接入的所有渠道 | `open/channelList` | POST | P0 |
| 2 | 获取游戏配置的所有区服 | `open/serverList` | POST | P0 |
| 3 | 获取产品列表 | `open/productList` | POST | P0 |
| 4 | 获取游戏内角色信息 | `open/roleInfo` | POST | P1 |
| 5 | 获取订单列表 | `open/orderList` | POST | P1 |
| 6 | 获取单日报表数据 | `open/dayReport` | POST | P0 |
| 7 | 获取每小时报表数据 | `open/dayHourReport` | POST | P1 |
| 8 | 获取玩家存留列表 | `open/userLive` | POST | P0 |
| 9 | 获取渠道报表数据 | `open/channelDaysReport` | POST | P1 |
| 10 | 获取渠道日报表数据 | `open/channelReport` | POST | P1 |
| 11 | 获取广告计划效果报表 | `open/adReport` | POST | P1 |
| 12 | 获取广告主列表 | `open/getMediaApp` | POST | P2 |
| 13 | 获取广告计划分组列表 | `open/getAdPlanGroup` | POST | P2 |
| 14 | 获取游戏分包列表 | `open/getPackageVersion` | POST | P2 |
| 15 | 获取广告落地页列表 | `open/getAdPages` | POST | P2 |
| 16 | 创建广告计划 | `open/createAdPlan` | POST | P2 |
| 17 | 更新广告计划 | `open/updateAdPlan` | POST | P2 |
| 18 | 获取广告计划列表 | `open/getAdPlan` | POST | P2 |
| 19 | 获取流失风险预警列表 | `open/uwlLost` | POST | P1 |
| 20 | 消息推送 | `open/pushMessage` | POST | P2 |

**优先级说明**:
- **P0**: 核心功能，必须优先实现
- **P1**: 重要功能，第二批实现
- **P2**: 辅助功能，最后实现

---

## 签名算法

QuickSDK 签名规则：

1. 将去掉 `sign` 的所有参数按键名首字母自然排序排列
2. 将所有参数按如下形式拼接 `k1=v1&k2=v2&`
3. 在第二步的字符尾部拼接 `openKey` 的值
4. 将前三步所得字符串 `k1=v1&k2=v2&openKey` 计算 MD5
5. 所得 MD5 即为签名值

---

## API 限制

- **频率限制**: 1000 次/分钟
- **每小时限制**: 不超过 10000 次
- **请求方式**: HTTP POST
- **返回格式**: JSON
- **编码格式**: UTF-8

---

## 配置示例

```yaml
quicksdk:
  enabled: true
  open_id: "your_open_id"
  open_key: "your_open_key"
  api_base_url: "https://www.quicksdk.com"
  timeout: 30
  retry_count: 3
  enable_cache: true
  cache_duration: 300
  features:
    # 基础数据
    channel_list: true
    server_list: true
    product_list: true
    role_info: true
    order_list: true
    # 运营报表
    day_report: true
    day_hour_report: true
    user_live: true
    channel_days_report: true
    channel_report: true
    # 广告管理
    ad_report: true
    media_app: true
    ad_plan_group: true
    package_version: true
    ad_pages: true
    create_ad_plan: true
    update_ad_plan: true
    get_ad_plan: true
    # 其他功能
    user_lost: true
    push_message: true
```

---

## 预计工作量

| 阶段 | 人日 | 负责人 | 状态 |
|------|------|--------|------|
| 阶段一：Proto 定义 | 3-4 | | 待开始 |
| 阶段二：Go SDK 实现 | 8-12 | | 待开始 |
| 阶段三：Python SDK 实现 | 6-8 | | 待开始 |
| 阶段四：其他 SDK 实现 | 可选 | | 待开始 |
| 阶段五：测试与文档 | 4-6 | | 待开始 |
| **总计** | **21-36 人日** | | |

---

## 参考资料

- [QuickSDK 开放接口授权参数](https://www.quicksdk.com/doc-1133.html)
- [QuickSDK 运营数据平台使用手册](https://www.quicksdk.com/doc-0.html?cid=27)
- [Prometheus 集成示例](../prom/v1/prom.proto)
- [Croupier Function Options](../../croupier/options/v1/function.proto)
- [Croupier UI Options](../../croupier/options/v1/ui.proto)
