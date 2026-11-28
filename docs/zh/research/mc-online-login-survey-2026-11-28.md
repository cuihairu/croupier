# Minecraft online-mode 登录：探测适配边界调查

日期：2026-11-28。PROBE-003（资料调查，MC 半边）交付物。收集 Java 版
online-mode 登录协议链与其账号/许可条件，并逐级说明每种探测能证明什么、
不能证明什么。PROBE-003 的自家游戏半边明确不在本文范围——等待游戏方自己的
协议/鉴权文档，保持 user-gated。

本仓 grounding：

- **SLP 适配器已完整实现**（`core/healthprobe/games/minecraft/slp.go`）：
  层 3 游戏协议 Checker，按公开 Server List Ping 规范自写，仅标准库，以游戏
  协议名 `minecraft` 注册（`internal/app/agent/probe_protocols.go` blank import）。
  其头部注释即本文确认的边界：SLP 不触碰登录/加密链，因此可**零账号**验证
  任意 Java 服务器——也因此**只是状态证据、绝不是登录证据**（PROBE-003 的
  约束条款）。
- 出网实测证据（2026-11-28，本机）：`sessionserver.mojang.com/session/minecraft/hasJoined`
  → **204**（活；204 是文档化的「未加入」响应，端点行为符合规范）、
  `api.mojang.com` → 根路径 **404**（主机活）、legacy `authserver.mojang.com`
  → **不可达**（legacy Yggdrasil 鉴权随 Microsoft 账号迁移已退役）。

## 1. online-mode 登录链（公开规范，wiki.vg）

```
C→S  Handshake（intent=login、协议版本）
C→S  Login Start（玩家名；新版本另带客户端 UUID）
S→C  Encryption Request（服务器公钥 DER、随机 verify token、should-authenticate 标志）
     ── 客户端账号鉴权（在下一步之前或穿插） ──
C→Microsoft  device-code OAuth → XBL user token（user.auth.xboxlive.com）
             → XSTS token（xsts.auth.xboxlive.com）
             → minecraftservices.com authentication/login_with_xbox → access_token
C→Mojang     session join（sessionserver.mojang.com/session/minecraft/join）：
             accessToken + selectedProfile + serverId hash
             （serverId+sharedSecret+服务器公钥 的 weird-hex sha1）
C→S  Encryption Response（shared secret + verify token，用服务器公钥 RSA 加密）
S→Mojang  hasJoined?username&serverId → 200 带 profile JSON = 鉴权通过；204 = 拒绝
     ── 双方切换 AES-CFB8 流加密；随后压缩阈值生效 ──
S→C/C→S  （1.20.2+）Login Acknowledged → configuration 状态 → play 状态
```

与探测相关的事实：

- **offline-mode** 服务器完全跳过加密/鉴权；UUID 由
  `nameUUIDFromBytes("OfflinePlayer:" + name)`（MD5 v3）推导。对 offline
  服务器做登录路径探测完全不需要账号。
- **代理转发**（平台经运维面代理探测时相关）：BungeeCord IP forwarding 信任
  handshake 数据（必须防火墙隔离）；Velocity modern 用共享 secret 经插件
  通道对转发玩家信息签名。
- 服务器**每次 online-mode 登录**出一次 `hasJoined` 外呼——登录级探测消耗
  Mojang 侧速率预算（社区实测 IP 级限流为每 10 分钟数百次量级）。

## 2. 账号与许可条件

| 侧                        | 条件                                                                                                                                                      |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 玩家客户端                | Microsoft 账号**且具 Java 版权益**（购买或 Game Pass）。token 链需要到 MSA 的 OAuth 流（headless 友好的是 device code）与 XBL→XSTS→minecraftservices 链。 |
| 游戏服务器（online-mode） | **自身无需账号。** 需出网可达 `sessionserver.mojang.com`（每次登录 `hasJoined`）与到 Microsoft/Mojang 主机的 DNS/TLS。                                    |
| 探测执行方                | 见下分级——只有 C 级需要真实持权账号，即用户保留的许可决策（许可证变更另行授权）。                                                                         |

## 3. 探测分级：每级能证明什么

| 级别                                   | 需要                                          | 能证明                                                               | 不能证明                                                                                       |
| -------------------------------------- | --------------------------------------------- | -------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| **A. SLP**（已实现，`minecraft` 协议） | 无                                            | 服务器活；版本；玩家数；motd/icon                                    | 登录/鉴权的一切——约束条款：**SLP 不是登录证据**                                                |
| **B. 登录路径可达性**（无账号）        | 无                                            | handshake→Login Start→Encryption Request 路径；服务端 TLS/密钥交换活 | *成功*登录：online-mode 服务器会在 `hasJoined`（未知 profile）丢弃合成客户端、到不了 play 状态 |
| **C. 真实登录探测**                    | **持权账号+用户授权** + MSA token secret 存储 | 端到端玩家登录、与玩家完全一致                                       | ——（这是全证据级；门=许可决策）                                                                |
| **D. 鉴权依赖健康**                    | 无                                            | 服务器主机到 `sessionserver`/`api.mojang.com` 的可达性/延迟          | 游戏侧登录行为                                                                                 |

B 级细节：对 **offline-mode** 目标，合成登录可零账号走通到 play 状态——完整
登录路径证据，但只证明无鉴权路径。对 **online-mode** 目标则停在加密边界
——这本身就是「服务器登录机制活着」的有效信号。

## 4. 结论与剩余闸门

- PROBE-003 收集类交付的 MC 半边 = 本文 + 已实现的 SLP 适配器；B/D 级探测
  零账号、排期即可实施（不属 PROBE-003 本体——PROBE-003 是调查）。
- PROBE-003 完全收口的剩余闸门，均为用户决策：
  1. **自家游戏登录协议/鉴权文档**——外部无法调查，须游戏方提供。
  2. **持权账号授权**（C 级）——保留中的许可决策；没有它，online-mode
     目标的登录证据止步于 B 级加密边界。
- 对未来任何实施重申约束条款：SLP 状态检查绝不得记为玩家登录证据。
