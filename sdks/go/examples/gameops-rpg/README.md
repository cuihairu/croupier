# gameops-rpg — 复杂游戏模拟 demo

与 gameops-demo（纯 CRUD 镜像浏览器种子）不同，本 demo 模拟一个有内在状态的 RPG 世界，
让平台运维链（调用 / 审批 / 异步任务）跑在**活数据**上：玩家有等级/金币/钻石/背包，
邮件带可领取附件（补偿流核心），订单有幂等发货，封禁与玩家状态联动。

## 世界模型

| 实体 | 字段 | 备注 |
|---|---|---|
| player | id / nickname / level / xp / gold / diamonds / inventory / status(online·offline·banned) / vip | 升级消耗 `level*100` XP，可连跳 |
| item | id / name / type(resource·equipment·consumable) / tradable | 8 种种子物品 |
| quest 模板 | id / goalType(kill·spend·recharge·login·collect·level) / goalCount / 奖励 / daily | 每日任务按 UTC 日刷新 |
| quest 进度 | progress / status(in_progress·claimable·completed) / day | 领取一次性入账 |
| mail | id / 收件人 / 标题 / 附件(itemId+count) / expireAt / read / claimed | **附件领取 = 补偿流核心** |
| order | orderNo / player / product / amount / status(pending·paid·shipped·refunded) / shipKey | 发货入钻石 |
| ban | player / reason / start·end / operator / active | 与 player.status 联动 |

## 一致性规则（验收锚点）

1. **发货幂等**：同一订单 `order.ship` 二次调用返回 `already_shipped`，钻石只入账一次。
2. **封禁拒领取**：封禁玩家 `mail.claim` 返回 `player_banned`，背包不变。
3. **余额 fail-closed**：超支消费（`insufficient_gold`）与退款钻石不足（`insufficient_diamonds`）
   都拒绝且状态不动；退款已发货订单扣回钻石。
4. **邮件过期**：过期邮件领取返回 `mail_expired`；领取恰一次（重复 `mail_already_claimed`）。

## 玩法循环

`-tick-seconds`（默认 5，0 关闭）驱动 tick：过期封禁自动解封 → 每日任务跨日刷新 →
在线玩家随机行为（35% 任务推进 / 25% 消耗药水 / 25% 充值产生 paid 订单 / 15% 掉线）→
离线玩家小概率上线。世界快照（`-state-file`）JSON 持久化，重启恢复（原子写：temp+rename）。

## 函数面（17 个，走 SDK→Agent 注册）

- `player.*`：list / get / inventory（背包查询）/ ban / unban
- `mail.*`：send（带附件）/ list / claim（附件领取）
- `task.*`：list（任务模板+进度）/ claim（领奖励）/ refresh（每日刷新）
- `order.*`：list / ship（幂等发货）/ refund（扣回钻石）
- `economy.*`：snapshot（全服经济汇总）/ audit（操作留痕）
- `report.generate`：唯一 task 模式函数，**Risk=high** → 平台调用即建审批单
  （`POST /api/v1/approvals/:id/approve` 通过后续跑执行），runSeq 持久化防重启重发 reportId

## 运行

```bash
# 先起 server（127.0.0.1:19090 控制面）与 agent（127.0.0.1:19091 SDK 面）
go run ./examples/gameops-rpg -agent 127.0.0.1:19091 \
  -game-id default -env dev \
  -state-file /tmp/gameops-rpg-state.json -tick-seconds 5
```

平台侧验证（login → invoke → 审批）：

```bash
TOKEN=$(curl -s -X POST localhost:18780/api/v1/auth/login \
  -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | jq -r .token)
curl -s -X POST localhost:18780/api/v1/functions/invoke -H "Authorization: Bearer $TOKEN" \
  -H 'X-Game-ID: default' -H 'X-Env: dev' -H 'Content-Type: application/json' \
  -d '{"id":"mail.send","params":{"playerId":"10001","title":"补偿","attachments":[{"itemId":"item_potion","count":5}]}}'
```

## 已知边界

- 内存态模拟：非真实游戏服务端，不能替代 GAME-002~010 的真实接入方验收。
- tick 行为由本地 RNG 驱动，无跨进程时钟同步。
- 邮件领取是"玩家侧"语义（actor=player），平台审批门由函数 Risk 等级控制。
