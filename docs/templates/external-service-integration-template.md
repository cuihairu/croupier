---
title: 外部服务接入模板——供应商自助 checklist（附录 A 全文）
---

# 外部服务接入模板（供应商自助 checklist）

## 状态

- 状态: **Draft（M4 落地稿，2026-10-10）**——走查对象 herald（webhook-out）/ Alertmanager+generic（webhook-in）已按本模板逐项复核（见文末走查记录）；模板正文待用户过目定稿。
- 依据: [plugin-mechanism §4](../design/plugin-mechanism.md)（标准接入面五要素 + 三种协议形态）、[provider-plugin-design §3.2/§3.3/§7](../design/provider-plugin-design.md)（manifest provider 块 / binding 契约 / 权限面）。
- 用途: 外部服务（短信网关/风控 API/数据平台/工单系统/告警系统…）接入 croupier 时，供应方或集成者按 10 步填空即完成接入评估与实施。

## 接入路径判据（先选路径，再走 10 步）

| 路径              | 适用判据                                                                 | 载体                                                               | 涉及步骤                                |
| ----------------- | ------------------------------------------------------------------------ | ------------------------------------------------------------------ | --------------------------------------- |
| **A. 扩展安装面** | 需要安装生命周期/多实例/多 scope/面板可见（§6.1 判据：要安装面的走五表） | release manifest（provider 块）→ 五表 binding                      | 全部 10 步                              |
| **B. 平台接口位** | 单实例语义、无需安装面、平台级口子（告警出口/入口、身份源等）            | 编译期 L1 实现 + server 配置段（`herald:` / `alertInbound:` 同款） | 1/2/5/7/8/9（3/4/6 不适用，见各步标注） |

## 步骤 1/10 能力清单

填写供应商提供的能力清单（每项一行）：

- 操作名（`<方法名>`，key 经 SanitizeKey 归一，禁止空格/大小写混排）
- 每操作：HTTP 方法 + 路径 + 请求/响应 JSON 形态（http-rest 形态必填）
- 幂等性声明：哪些操作可安全重试，幂等键由谁携带（调用方）

## 步骤 2/10 协议形态（三选一）

| 形态          | 判据                               | 供应商需提供                                                                             |
| ------------- | ---------------------------------- | ---------------------------------------------------------------------------------------- |
| `http-rest`   | 供应商有 REST API                  | 「操作清单」即可，**不需要写任何代码**（走 openapi driver）                              |
| `webhook-out` | croupier→供应商推送（告警出口类）  | 接收端 URL + HMAC 密钥托管方式；供应商侧按统一信封字段路由（信封=plugin-mechanism §5.2） |
| `webhook-in`  | 供应商→croupier 推送（告警入口类） | 按 §5.3 契约映射：HMAC-SHA256 签名头 + 时间戳防重放 + 幂等 eventId                       |

自定义协议 = 写 L1 driver（平台发版节奏），**不给配置面**。

## 步骤 3/10 manifest provider 块 JSON（路径 A 专属）

release `manifest_json` 的 provider 块（字段闭集 + 守卫测试 + binding 同构，三件套约束）：

```json
{
  "provider": {
    "type": "openapi",
    "operations": ["day_report", "user_live", "order_list"],
    "permissions": { "operate": "external-platform.operate" }
  }
}
```

- `type`：闭集起步 `openapi | webhook`（`webhook` 为 P2 预留）；缺省 `openapi`。
- `operations`：方法闭集；调用面 function ID = `external.<provider>.<method>`。
- `permissions`：可选覆盖；缺省继承统一模式三层权限。

## 步骤 4/10 binding config（路径 A 专属）

安装时从 manifest 派生默认绑定（与 provider 块字段集同构：`provider/type/operations/enabled/config`）；运营在 installation 上覆写 `config`：

- `baseURL`：http/https，**secguard 出站守卫白名单内**；
- 每服务一个 base，操作路径由 driver 协议推导。

## 步骤 5/10 secrets 登记

凭据**一律引用不落明文**（config/secrets 拆分）：

- 路径 A（扩展安装态）：installation `secret_refs_json`（`map[string]string` 引用；**现状边界：存储已落，解析器未建**——运行时读取先走 env 引用，见下）；
- 路径 B（部署态）：配置段 `secretEnv` / `tokenEnv` 指向环境变量名（herald `tokenEnv: HERALD_TRIGGER_TOKEN`、入站 `secretEnv: ALERTMANAGER_INBOUND_SECRET` 先例），布线时 `os.Getenv` 解析；
- 轮换 = 改 secret 不动配置。

## 步骤 6/10 capability 声明（路径 A 专属）

- **管理面**（安装/启停/配置）：统一模式三层 `<domain>.read/operate/admin`（manifest 权限声明）；
- **调用面**：`external.<provider>` + operation（RBAC 按 capability/operation 裁决）；
- 事件前缀 `<domain>_*`（如 `external_platform_call_failed`）；审计走平台既有 audit 链。
- 路径 B 无 capability 面：门控=共享密钥（如入站 webhook 的 HMAC）或配置开关（如 `herald.enabled`）——无 JWT 端点上 RBAC capability 不可执行（M3 已裁决，见 todo.md 边界①）。

## 步骤 7/10 超时与重试

- driver 统一默认：连接 5s / 收发 30s，可按 provider 覆盖；
- 出站推送（webhook-out）：出口管理器有限重试 + 拒绝短路（Outlet Manager：3 次间隔 2s；herald 适配器 IsTransport 分类先例）；
- 幂等键由调用方携带；失败语义标准化（§6.6）。

## 步骤 8/10 拨测（test-connection）

- 路径 A：`POST /api/v1/installations/:id/test-connection`（`internal/api/extension/service.go` TestConnection）；
- 路径 B：本地冒烟用例先例——herald 出口 `heraldd_smoke_test.go`（env 门控，CI 不依赖外网）、入站 webhook 单测（httptest 全链）；
- 验收标准：拨测绿 + 一次真实操作端到端（如面板发起 → 审批 → 留痕 → 资产动作完成，§8 M5 门）。

## 步骤 9/10 面板可见性验收

- 路径 A：安装后页面声明（manifest ui.pages）经发布链可见；binding 在面板可见；
- 路径 B：落既有页面（如入站告警落 `/ops/alerts` 列表——severity/service/summary 三列有值即验收；出口失败计数进 server 日志/指标）；
- 已知边界：ops 告警页不展示 details/metadata 列（要辨识来源需扩 DTO+页面列，另批）。

## 步骤 10/10 升级与下线约定

- 升级 = 版本切换 + config 对新 schema 校验 + reconcile 重建绑定（既有机制）；`min_core_version` semver 校验 + 依赖图（`missing_dependency`/`dependency_cycle`/`version_mismatch`）；
- 回滚 = upgrade 到旧版本；
- 下线路径 A = 卸载（binding/绑定派生物随 installation 撤销）；路径 B = 配置关（`enabled: false` 即回到现状行为，M2 herald 缺省关先例）；
- 凭据轮换 = 改 secret 不动配置（步骤 5）。

## 走查记录（首个真实供应商）

### 走查对象 1：herald（webhook-out，M2 交付 2026-10-10）

- 路径判据：B（平台接口位——单实例告警出口，无安装面）→ 步骤 3/4/6 不适用；
- ①能力清单：两类品类（agent-alerts/availability）× severity 三档；③④⑥跳过；⑤secrets：`herald.tokenEnv` 环境变量引用（trigger token）；⑦超时重试：Manager 3 次×2s + Permanent 短路；⑧拨测：heraldd 冒烟（品类注册→dispatch 受理→dedup 折叠）；⑨面板：告警页不受影响 + 失败计数；⑩下线：`herald.enabled: false` 缺省关。
- **herald 侧播种四件**（部署口径，courier 式契约）：app token、groups 受众、`delivery.category_urgency`、`dedup.enabled`。

### 走查对象 2：Alertmanager / generic（webhook-in，M3 交付 2026-10-10）

- 路径判据：B（平台接口位——告警入口）；③④⑥不适用（门控=来源级 HMAC 共享密钥）；
- ①能力清单：两类来源闭集；②协议形态：入站 HMAC-SHA256 + ±5 分钟防重放 + eventId 幂等；⑤secrets：`alertInbound.sources.<source>.secretEnv`；⑦重试：调用方重试由 eventId 幂等兜底；⑧拨测：12 个 inbound 单测（签名/重放/映射/幂等全链）；⑨面板：落 `/ops/alerts` 与 capture/dbmon 同桶（排除面外）；⑩下线：来源不配置=403。

### 走查修订项（模板据此修订）

1. **路径判据前置**：原 10 步默认扩展安装面；走查发现两个真实供应商都走平台接口位（路径 B）——模板顶部加路径判据表，3/4/6 步标注路径 A 专属。
2. **secrets 现状双轨**：`secret_refs_json`（安装态）存储已落但运行时解析器未建——模板步骤 5 如实双轨：路径 B 用 env 引用（现网唯一跑通形态），路径 A 安装态存储先落。
3. **webhook-in 幂等键派生**：Alertmanager payload 无显式 eventId——从 labels+status 派生确定性指纹（`am:<sha256 前 16 hex>`）；模板步骤 2 补「无显式幂等键的入站协议需约定派生规则」。
4. **herald 侧播种清单**：出口类接入不是 croupier 单侧完成——供应商侧配置（品类紧急度矩阵/去重开关/受众）是 courier 式契约的另一半，模板步骤 2 补播种清单要求。

### http-rest 形态：尚无真实走查（边界）

模板 http-rest 分支（步骤 3/4/6 全量演练）依赖首个 REST 供应商实接（#66 P1 external-platform 迁移，**待后审授权，不在本批队列**）——该批交付时回填本节。

## 已知边界（诚实清单）

- 模板正文**待用户过目**定稿（M4 验收门「模板过目」——本批交付 Draft 版+走查记录，过目后修订）。
- `webhook` driver type / generic webhook 出口（钉钉/飞书模板化）/ secret_refs 运行时解析器 / nonce 严格防重放：均留位未做（触发条件见 plugin-mechanism §8 远期行）。
