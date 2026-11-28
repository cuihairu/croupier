---
title: 玩家工单身份方案评审
---

# 玩家工单身份方案评审

日期：2026-11-28。状态：**提案待 owner 决策**。本页是 AUD-041 方案决策交付物：
评审公开玩家工单的身份缺口（`internal/api/support/player.go`），对比两个候选
身份来源（SDK 签名 vs 玩家会话 proof），并给出 create/list 两端的强制面提案。
本页不改任何代码——实施以 owner 对契约的拍板为门。

## 1. 现状（代码事实，2026-11-28 核实）

- `CreateTicket`（匿名）：`playerId` 直接采信请求 body，**无任何证明**——
  任何人可以冒任何玩家名义提单。
- `ListMyTickets`（匿名）：**仅按提交的 `playerId`** 查询参数过滤——扫描
  playerId 即可枚举任意玩家的工单标题/状态。该面无速率限制。
- 两端点的 `gameId`/`env` 均取自 query 参数。
- 平台**没有玩家鉴权**：玩家身份按架构归游戏方所有（单公司 scope 模型）。
  `internal/security/jwtutil` 只为 admin token 提供 HS256 `Sign`/`Parse`；
  无 JWKS/RSA 机制、无玩家 token 面。游戏注册表（`internal/model/game.go`）
  没有按游戏 secret 字段。

## 2. 契约拍板：身份来源

| 选项 | 形态                                                                                                                         | 优点                                                                                      | 缺点                                                                  |
| ---- | ---------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- | --------------------------------------------------------------------- |
| (a)  | **SDK 签名**——游戏服务端对每次请求联签（按游戏 secret 对 playerId+nonce+timestamp 做 HMAC）                                  | 无需共享 token 语义；逐请求证明                                                           | 游戏客户端每次工单调用都要经自己服务端；无过期语义；list 每页也要签名 |
| (b)  | **玩家会话 proof**——游戏服务端在自己的玩家登录后签发短时效 support token（HS256 JWT：playerId+gameId+env+exp+aud），平台校验 | 一个 token 覆盖 create+list+后续；过期有界；复用 `jwtutil` HS256 解析；登录权威留在游戏方 | 需按游戏 secret 注册+轮换；token 签发是游戏侧新工作                   |

**推荐 (b)**——`player-support-token` profile，游戏服务端在其玩家登录后签发，
平台校验：

- Claims：`playerId`、`gameId`、`env`、`exp`（≤15 分钟）、`aud: "support"`。
  平台身份**只取自 claims**——create/list 提交的 `playerId` 被忽略（与 claims
  冲突时拒绝）。
- 校验复用既有 HS256 路径（`jwtutil`），以**按游戏 secret** 为密钥。不引入新
  JWT 库、不做 JWKS。
- 选项 (a) 保留为按游戏后备 profile（游戏无法签发 token 时）——同一 secret
  注册面，不同校验形态（对规范化请求做 HMAC）。v1 不实现。

本方案**不引入平台玩家鉴权**：token 只证明「游戏方最近为该玩家背书」。登录、
账号合并、身份所有权仍在游戏方。

## 3. 强制面

| 端点                           | 现状                       | 提案                                                                                                      |
| ------------------------------ | -------------------------- | --------------------------------------------------------------------------------------------------------- |
| `POST /public/support/tickets` | 匿名，playerId 取自 body   | 要求有效 player-support token；`playerId`/`gameId`/`env` 取自 claims；与 claims 冲突的 body 值拒绝（400） |
| `GET /public/support/tickets`  | 匿名，按提交 playerId 过滤 | 要求有效 token；按 token 的 `playerId` 过滤——**提交 playerId 枚举路径消除**                               |
| 游戏作用域                     | query 参数，未校验         | 取自 token claims；有 token 时忽略 `X-Game-ID`/`X-Env` 式 query 值                                        |

两端点仍留在 `/public` 路由族（无 admin 会话）但加 token 闸；失败响应走统一
错误契约（缺失/过期 token → `401 unauthorized`，跨游戏 claims → `403 forbidden`）。

## 4. Secret 注册与轮换

- 按游戏 support secret 注册在游戏注册表，**只写字段**（经 admin API 设置，
  任何读路径不返回；列表响应按既有敏感字段助手打码）。
- 存储：games 表加 secret 列 → **编号迁移**（schema 迁移契约：0015/0021/0025/0027
  补列模式；模型变更与迁移同 PR）。
- 轮换：每游戏至多两个活跃版本（当前+上一版）；token 携带 `kid` 式版本号；
  重叠窗内两版均可验。轮换是 admin 动作、进审计。
- 启用 Vault（`VaultConfig`）时 secret 可改为引用——引用路径沿 extension
  `SecretRefsJSON` 先例。Vault 可选；注册表列是基线。

## 5. 验收映射（设计 → 验证场景）

| #   | 场景                               | 期望                                                               |
| --- | ---------------------------------- | ------------------------------------------------------------------ |
| A1  | 有效 token create                  | 工单创建，`playerId`/`gameId`/`env` 取自 claims；body 冲突拒绝 400 |
| A2  | 无 token / 过期 token create       | 401 `unauthorized`                                                 |
| A3  | 提交 playerId 但无 token 的 list   | 401——无 token 即无法枚举                                           |
| A4  | 有效 token list                    | 仅该玩家自己的工单、仅公开状态（无 assignee/内部评论）             |
| A5  | 游戏 A 的 token 用于游戏 B 端点    | 403 `forbidden`（claims/游戏绑定）                                 |
| A6  | 重叠窗内上一版 secret 签发的 token | 接受（轮换窗）；窗外 401                                           |
| A7  | admin 读路径                       | 任何游戏 list/detail 响应都不出现 secret（打码）                   |

## 6. 待 owner 决策

- [ ] **D1** 身份来源——批准选项 (b)：游戏签发 HS256 `player-support-token`（≤15 分钟，`aud: support`），身份只取 claims
- [ ] **D2** 强制面——批准 create 与 list 双端 token 闸；提交 playerId 忽略
- [ ] **D3** secret 注册面——批准只写按游戏 secret 列 + 编号迁移 + 两版本轮换
- [ ] **D4** 匿名 create——确认 v1 **无**匿名提单路径（按游戏匿名提单 opt-in 待有游戏提出再议）
- [ ] 验收映射 §5（A1–A7）——确认其为实施的验证基准

## 7. 已知边界

- 游戏方必须集成 token 签发（服务端、在其玩家登录后）。在游戏上线该能力前，
  其公开客服面对玩家实质停用——运营侧工单处理不受影响。
- 该 proof 不向平台证明玩家除工单外任何身份；不得复用为通用玩家会话。
- 公开面的速率限制不在本决策内（先身份；限流策略另立条目）。
