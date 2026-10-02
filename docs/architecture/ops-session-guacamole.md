# 运维会话（Ops Session）——Guacamole 网关形态设计

- 状态：**设计稿，待用户过目后实施**（分期 todo 见 §8）
- 日期：2026-10-03
- 参照实现（用户拍板「照已验证实现移植，不依赖 wingman 服务，协议/票据口径保持一致」）：
  - wingman `docs/remote-gateway-guacamole-design.md`（DG-1~DG-10 全套决策，三协议 e2e 已验证）
  - cockpit `internal/server/api_guacamole.go` / `api_vault.go` / `cov_guac_relay_test.go`（目标转发中继 0767b8b 原型）
- 本文编号：**OS-x**（Ops Session 决策），与仓库既有架构文档并列互引
- 一句话：运维/开发经浏览器登录公共服务器/开发机（SSH/RDP/VNC），全程审计 + 录屏留档，凭据不落明文、不出服务端。

---

## 0. 一页总结

| 面                           | 载荷                      | 协议                                     | 归属                       |
| ---------------------------- | ------------------------- | ---------------------------------------- | -------------------------- |
| 既有控制面（函数调用/任务）  | 结构化 JSON               | HTTP + 自建 TCP transport                | 不动                       |
| **运维会话像素面（本设计）** | 连续图形指令流 + 输入事件 | Guacamole 协议（guacd 翻译 SSH/RDP/VNC） | server 新增 ops-session 域 |

核心拓扑（与 wingman §4.5 中继形态一致，guacd 不直拨目标）：

```
dashboard (React, guacamole-common-js)
   │ WS /api/v1/ops-session/ws?ticket=…（Guacamole 指令流）
   ▼
croupier server
   ├─ 票据 REST POST /api/v1/ops-session/tickets（ops:view 校验 + 审计链）
   ├─ 网关桥：WS ↔ guacd TCP 透传 + select/connect 注入（凭据只在此段内存）
   ├─ 凭据保险箱（个人箱主口令派生 + 共享箱 server 密钥，§5）
   └─ per-session 回环中继 OpsRelay（127.0.0.1:0）
   │ TCP 4822（guacd 仅 localhost）
   ▼
guacd 1.5.5（锁定版本，§7.2）
   │ connect 的 hostname/port 指向回环中继
   ▼
server 内中继 ── 新增 proxy.* 帧族 ──► 既有 agent TCP 通道
   ▼
croupier agent（目标网络侧，只拨号不监听）
   │ 从自身网络位拨真实 target
   ▼
服务器 / 开发机（SSH 22 / RDP 3389 / VNC 5900）
```

---

## 1. 背景与目标

运维/开发需要登录公共服务器、开发机处理故障与发布——今天走个人 SSH 客户端 + 口口相传的口令，四个痛点：

1. **凭据管理失控**：root 口令在聊天记录里流传，无共享凭据的权限收口；
2. **无审计**：谁在什么时候登录了哪台机器、做了多久，无从回答；出了事只有登录机器翻 last/wtmp；
3. **无留档**：高危操作过程不可回放，排障与定责都缺证据；
4. **无准入**：任何拿到网络可达性 + 口令的人都能登，不过 RBAC、不过审批。

目标：浏览器内完成 SSH/RDP/VNC 登录，**申请即审计、连接即录屏、凭据不出保险箱**，治理面全部挂 croupier 现成能力（RBAC/ABAC、审计哈希链、审批流、game/env 隔离）。

## 2. 为什么是 Guacamole 网关形态（用户已拍板，记录理由）

- **不自研像素流**：RDP/RFB 是打磨了近三十年的领域，guacd（Apache 顶级项目）把「翻译成浏览器 canvas 指令流」产品化；自研 = 重做编码自适应 + 协议状态机 + 输入语义映射三件事。SSH 是 guacd 服务端终端仿真，前端与桌面会话同构。
- **不部署完整 guacamole-client（Java webapp）**：那套自带用户体系/连接管理 GUI/存储，与 croupier 已有 RBAC、审计、注册表全面重复。只取三件：guacd（无状态翻译器）+ server 内的桥 + 前端渲染库 `guacamole-common-js`。
- **不依赖 wingman 服务**：wingman/cockpit 是实现参照，移植形态；运行期零依赖，协议与票据口径与它们保持一致（OS-3）。

否决备选（照 wingman §3，防反复）：自研 MJPEG 轮询流、WebRTC（内网部署形态下基建代价不成比例，公网弱网需求出现时另立项）、浏览器直连 VNC（浏览器无 RFB/RDP 支持，且绕过审计）、商业远控（闭源、链路不可审计）。

## 3. 协议与票据口径（与 wingman 一致的部分，逐条锁定）

以下口径**保持一致**（前端公共组件心智、已知坑防御都建立在其上）：

1. **票据**：一次性短时效 token（TTL 5 分钟），REST 申请 → WS 升级时校验消费。连接参数（协议/host/port/凭据/是否只读/是否录制）随票据携带；凭据只存在于 server→guacd 的 `connect` 指令，**永不下发浏览器、不进 URL、不进日志**。WS 握手是票据唯一消费点，断线重连须重新申请票据（无自动重连）。
2. **WS 通道**：Guacamole 指令流双向透传；票据双通道——URL query `?ticket=`（首选，guacamole-common-js WebSocketTunnel 语义）+ `Sec-WebSocket-Protocol[0]` 兼容位；subprotocol 回显不匹配即握手失败断开。
3. **guacd 握手**：`select`（协议 `ssh|rdp|vnc`）→ guacd 回 `args`（首段版本名如 `VERSION_1_5_0`）→ `connect` 按名单**逐位**填值（首参必须回版本串，未提供参数空串占位；个数不等 guacd 静默断连——wingman `TestGuacApplyVersionArg` 护栏先例）。
4. **内部指令拦截**：空 opcode 指令（ping）由网关拦截回显，绝不转发 guacd（common-js 15s receiveTimeout 语义）。
5. **guacd 版本锁定 1.5.5**：1.6.0 镜像 RDP 链路两个空指针崩溃（`guac_audio_assign_encoder` / `guac_user_supports_webp`），无参数可规避；网关对 1.5.5 显式 `disable-audio` 规避同类路径；镜像钉 digest。`GUACD_LOG_LEVEL` 生产保持 info（debug 会打印 connect 参数含口令）。
6. **只读语义**：监看（read_only=true）拦输入注入；发送类入口（剪贴板上行/文件上传）仅接管模式渲染，网关不解析指令内容（数据面纯管道原则）。
7. **中继实现纪律**（cockpit 实测踩坑，全部保留）：读缓冲 `buf[:n]` 入队前必须拷贝；`proxy.data` 可能先于拨号完成到达（guacd connect 一成功就发首包），写入先进 per-conn 队列、拨通后按序冲刷；代理流不可缓冲重放，链路断开即拆全部中继连接、会话作废重开。

## 4. server 侧模块落点

```
internal/opssession/            # 域包（对齐 internal/api 分域惯例）
  gateway.go                    # 网关桥：WS↔guacd 透传 + select/connect 注入 + ping 拦截
  ticket.go                     # 一次性票据管理（零依赖，照 wingman remoteticket 形态）
  relay.go                      # per-session 回环中继 OpsRelay（127.0.0.1:0）
  recording.go                  # 录制参数注入 + 录像检索
  vault.go                      # 保险箱（个人箱 + 共享箱）
internal/api/opssession/        # HTTP/WS handler（挂 internal/api/routes）
internal/agent/proxytunnel.go   # agent 侧拨号端（只拨号、零新增监听）
web/src/pages/OpsSessions/      # 前端：目标列表 / 会话面板 / 录像回放
web/src/components/OpsSession/  # 前端公共件（连接 hook / 票据客户端 / 播放器）
```

REST 面（v1，croupier API Response Contract：成功裸 payload、错误 `{error,message,details?}`，**不带 wingman 的 success envelope**）：

```
POST   /api/v1/ops-session/tickets            # 签发票据          ops:view（接管另需 ops:control）
GET    /api/v1/ops-session/ws?ticket=…        # WS 升级（gin + gorilla/websocket）
GET    /api/v1/ops-session/targets            # 目标条目列表      ops:view
POST   /api/v1/ops-session/targets            # 建目标条目        ops:control
GET    /api/v1/ops-session/vault/status       # 保险箱状态        ops:view
POST   /api/v1/ops-session/vault/setup|unlock|lock|change-password
GET    /api/v1/ops-session/vault/credentials  # 元数据列表（密文永不序列化）
PUT    /api/v1/ops-session/vault/credentials/:id
DELETE /api/v1/ops-session/vault/credentials/:id
GET    /api/v1/ops-session/recordings         # 录像列表          ops:view
GET    /api/v1/ops-session/recordings/:name/download   # 下载（审计）  ops:view
DELETE /api/v1/ops-session/recordings/:name   # 删除              ops:control
GET    /api/v1/ops-session/sessions           # 会话审计报表      ops:view
```

## 5. 凭据保险箱（OS-4，双形态混合）

wingman 落地的是「一箱一用户」主口令模型，cockpit 落地的是「server 级 env 密钥」模型。croupier 的共享诉求（公共服务器凭据按权限共享给运维角色）决定了**按条目类型混合**：

### 5.1 个人条目（wingman 形态）

```
主口令 ──PBKDF2-HMAC-SHA256(salt, 600k 轮)──▶ KEK
随机 32B DEK ──AES-256-GCM(KEK)──▶ WrappedKey（落库）
凭据 password/privateKey ──AES-256-GCM(DEK)──▶ 密文（落库）
```

- `vault_masters` 表：每用户至多一行，KDFSalt/KDFIterations/WrappedKey；不存可校验主口令的明文摘要（GCM 认证失败即口令错误，AEAD 天然防错口令/防篡改）；
- DEK 解锁期间驻留服务端内存，空闲超时自动锁回；主口令与 DEK 永不落库；**忘口令 = 不可恢复**（设计属性）；
- 条目唯一键 `(userId, targetId)`，连接时按目标元组自动取用（快捷调用——不用每次手输）。

### 5.2 共享条目（cockpit 形态，公共服务器凭据）

- 密钥：`CROUPIER_VAULT_SHARED_KEY`（server env，32B hex）；条目密文 AES-256-GCM 落库，明文仅在 connect 注入时内存解密；
- **权限即共享**：条目带 `gameId/env` 维度（复用 X-Game-ID/X-Env 隔离）+ 运维角色授权（ops:view 可见元数据与取用连接，明文凭据任何 API 都不返回）；
- 快捷调用与个人条目同路径：票据申请时 `credentialRef` 指向共享条目，服务端解密注入 connect，浏览器全程不见明文。

### 5.3 事件审计

setup/unlock/lock/change-password/upsert/delete 各落审计链一条（不含口令与密文）；共享条目变更是高危动作，挂 §6 审批策略。

## 6. 治理面映射（用户令 ①，全部挂现成能力）

| 治理诉求      | croupier 现成能力                                                                                             | 映射                                                                                                                                                     |
| ------------- | ------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 权限点        | RBAC/ABAC（`internal/security/rbac`、`internal/policy`）                                                      | 新增 `ops:view`（监看/票据/录像/报表）与 `ops:control`（接管/建目标/删录像/改共享凭据）两个权限码；接管比监看多一档（wingman desktop:view/control 先例） |
| 审计          | 审计哈希链（`internal/audit`，ChainInfo prev/hash）                                                           | 四事件入链：票据申请 / 连接建立 / 断开（含时长）/ 失败；另记录像下载、删除、vault 事件；记录含操作者、目标、协议、game 维度，**不含凭据与画面内容**      |
| 高危审批      | 审批流（`internal/api/approval`）+ 策略引擎（`internal/policy/manager.go` 的 `ApprovalWorkflow: two_person`） | 高危目标（danger 级，如生产 root 接管）票据申请先过审批两人复核，批准后票据方可签发；复用 rejectSelfApproval 语义                                        |
| game/env 隔离 | `svc.GameDBMiddleware` + `dbctx.Resolve`（X-Game-ID/X-Env）                                                   | 目标条目、凭据条目、录像、会话审计行全部带 gameId/env 维度；单库模式行级隔离、multiGame 模式物理分库，均沿用既有机制                                     |
| 会话报表      | 新表 `ops_session_audits`（照 wingman §17：专表而非 JSON meta 聚合）                                          | 会话结束一次性写终态行（closed/failed）；会话 ID 拨号前生成，失败也有唯一标识；多维过滤 + groupBy + 时间分桶                                             |

## 7. 关键决策与已知坑（OS-5~OS-8）

### OS-5 agent 侧隧道帧族（中继数据面）

wingman 用 `proxy.new/proxy.data(base64)/proxy.close/proxy.error` Notify 帧走其 Agent TCP。croupier 对应落点：`pkg/protocol` 新增帧族 `MsgProxyNew / MsgProxyData / MsgProxyClose / MsgProxyError`（编号顺延既有分域段位），载荷 protobuf（对齐 sdk wire 协议文档同步义务）：

- `ProxyNew{connId, targetHost, targetPort}`：server→agent，令 agent 拨号；
- `ProxyData{connId, payload}`：双向，payload base64（与 wingman 口径一致）；
- `ProxyClose{connId}` / `ProxyError{connId, reason}`。

agent 侧 `proxytunnel.go` 只拨号、零新增监听（复用既有 outbound 连接，不碰「agent 本地 19091 注册监听」语义）；server 侧中继实现纪律照 §3.7。SDK feature matrix 同步追加（Go SDK 帧族是 server↔agent 内部协议，不动各语言 SDK 的 L1 面，但 wire 协议文档必须更新）。

### OS-6 录屏留档（用户令 ③）

- connect 参数注入：`recording-path`（guacd 容器视角）/ `recording-name = {agentID}-{sessionID}.mjs`（与审计同源唯一）/ `create-recording-path=true`；
- **安全默认恒定**：`recording-include-keys=false`（按键内容永不进录像——录像里出现明文口令是审计资产变泄漏源）；鼠标轨迹保留；
- 双路径配置分离（guacd 视角 / server 视角挂载点，两容器卷挂载可不同）；
- 录制未配置时票据申请即拒（record=true 且路径未配置 → 400 带 details），不留到握手期；
- name 做 basename 校验（拒路径分隔符与 `..`）；
- 回放：浏览器内 `Guacamole.SessionRecording`（guacamole-common-js@1.5.0 自带；其 Blob 直连分支在 1.5.0 是坏的，照 wingman 方案用 BlobRecordingTunnel 绕行 + 防退化用例锁契约）；guacenc 离线转码下载路径保留。

### OS-7 HTTP/契约对齐

- wingman 是 `success` envelope 风格——移植时**全部改造成 croupier API Response Contract**（成功裸 payload，错误 `{error, message, details}`），票据响应体 `{ticket, expiresAt, sessionId}`；
- 所有对外字段 lowerCamelCase；本地化字段走 `LocalizedText` 契约（目标条目名称/备注）；
- SSE/WS 是显式例外（WS 传 Guacamole 指令流，非 JSON）。

### OS-8 数据库迁移

新表 `vault_masters` / `opssession_credentials` / `opssession_targets` / `ops_session_audits` / `opssession_recordings_meta`（若元数据不直接扫目录）——**编号迁移**（`internal/svc/migrations.go`：HasTable 检查后 CreateTable，0019 先例）+ `MinimumRequiredVersion` bump + `migrate_test.go` 合成清单同步，三处同 PR；线上 postgres 单库模式直接适用。模型侧 game 维度列 + `dbctx.Resolve` 读写。

### OS-9 部署形态

- guacd 与 server 同 compose（回环中继可见性硬前提）；guacd 仅 localhost 监听、钉 1.5.5 digest；
- deploy 栈 / quickstart 栈 / CI compose 追加 guacd 服务（profile 分层，默认可关）；
- 录制卷双挂（guacd 写入路径 = server 读取路径）。

## 8. 实施分期（todo，待用户过目）

| 期                         | 内容                                                                                                                                                                                                                                                                 | 验收口径                                                          |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| **P0 网关骨干**            | guacd compose（1.5.5 钉 digest）+ server 网关桥（select/connect 逐位对齐/ping 拦截/版本首参）+ 票据 REST + WS + `ops:view`/`ops:control` 权限点 + 审计四事件 + `MsgProxy*` 帧族 + agent proxytunnel + 回环中继 + 前端会话面板（guacamole-common-js）+ SSH 真链路 e2e | 浏览器 SSH 登录目标机（手输凭据），审计链出票据/连接/断开三条记录 |
| **P1 凭据保险箱**          | vault 个人箱（主口令 PBKDF2/GCM/解锁驻留/空闲锁回）+ 共享箱（env 密钥 + game/env 维度 + 角色共享）+ 快捷调用（credentialRef 自动注入）+ vault 事件审计 + 前端保险箱管理页                                                                                            | 建共享凭据条目 → 运维角色免输口令登录 → 明文任何 API 不可见       |
| **P2 录屏留档**            | 录制参数注入 + 录像目录双挂 + 检索/下载/删除 API + 浏览器内回放（BlobRecordingTunnel）+ 审计关联（报表「有录像」可跳转）                                                                                                                                             | 登录会话产生 .mjs → 浏览器内回放可见操作轨迹、不可见按键内容      |
| **P3 治理深化 + 真实走查** | 高危目标审批流（two_person 挂钩票据申请）+ 会话审计报表专表（多维过滤/聚合/分桶）+ 前端报表页 + **用户令 ④ 完整走查**：建公共 agent 目标条目 → 运维角色登录 SSH → 审计出记录 → 录屏可回放                                                                            | ④ 全链路演示过一遍 + DoD 全项（guard/迁移/文档三层/CI）           |

每期交付按仓库 DoD 收口：`go build/test`、`tsc`/jest、guard、发布链闭环（涉及 PageSpec 的前端页面走提案→accept-and-publish）、`docs/architecture/ops-session-guacamole.md`（本文）+ `docs/sdks/` wire 文档 + `docs/dashboard/` 使用文档三层同步、部署链 Docker+deploy+CI。

## 9. 不做清单（防反复）

- 不部署 Java guacamole-client webapp；不自研像素流；不做 WebRTC 第二通道（公网弱网需求出现时另立项）；
- guacd 不暴露公网、浏览器不直连 guacd/endpoint；
- agent 不做像素采集/编码/协议翻译（纯 TCP 字节转发）；不新增任何 agent 监听端口；
- 不做 iOS（平台不可能，wingman DG-5 同判）；Android 像素面（droidVNC-NG 桥）列为远期触发式，不在本设计范围；
- 录像不做自动清理/TTL（运维策略，等真实规模）；报表不做跨实例聚合（部署形态确定后再设计）；
- 不复刻 wingman 的文件传输/剪贴板 UI（§14/§15）进第一期——croupier 运维场景以 SSH 终端为主，需要时按 wingman 同构方案追加，不阻塞本期。

---

## 附：与 wingman/cockpit 的口径对照速查

| 项              | wingman/cockpit                            | croupier 本设计                  | 一致性                       |
| --------------- | ------------------------------------------ | -------------------------------- | ---------------------------- |
| 票据 TTL/一次性 | 5min / 消费即作废                          | 同                               | 一致                         |
| WS 票据通道     | query ?ticket= + subprotocol 兼容          | 同                               | 一致                         |
| connect 参数    | 按 args 逐位、首参版本串                   | 同                               | 一致                         |
| guacd 版本      | 1.5.5 锁定 + disable-audio                 | 同                               | 一致                         |
| 中继            | 回环 listener + proxy.* 帧                 | 同（帧族换 croupier protobuf）   | 语义一致                     |
| 录制            | include-keys=false / {agent}-{session}.mjs | 同                               | 一致                         |
| 回放            | SessionRecording + BlobRecordingTunnel     | 同                               | 一致                         |
| HTTP 契约       | success envelope                           | croupier 裸 payload + error 对象 | **有意改造**（仓库契约强制） |
| 权限码          | desktop:view/control                       | ops:view/control                 | 语义同构、码面归 croupier 域 |
| vault           | 一箱一用户（wingman）/ env 密钥（cockpit） | 按条目类型双形态混合             | 形态移植 + 共享扩展          |
| agent 拨号端    | C++ ProxyTunnel（wingman runtime）         | Go proxytunnel（croupier agent） | 同拓扑                       |
