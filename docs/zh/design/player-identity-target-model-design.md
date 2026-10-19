---
title: 玩家身份与业务目标模型
---

# 玩家身份与业务目标模型

## 状态与范围

- 状态：设计待评审；本文不把现有 `internal/api/player` 演示 CRUD 扩张为玩家权威库。
- 范围：账号、角色、区服、大区、渠道、平台、外部账号、合服映射和统一目标引用。
- 权威边界：玩家资产、封禁、订单、邮件、活动仍以[游戏数据权威归属](./game-authority-ownership-design.md)为准；本模型只定义识别与引用，不复制业务真值。
- 现状：`internal/api/player/dto.go` 的 `Player` 只有平台自增 `id`、`gameId`、昵称和展示属性，没有 `env`、账号 ID、角色 ID、区服 ID、渠道、平台或旧 ID 映射；不能作为安全执行目标。审批记录已有 `GameID/Env/FunctionID/TaskID/IdempotencyKey/Payload/Metadata`，但目标引用尚未形成统一闭包。

## 设计原则

1. `gameId/env` 是隔离边界；区服、角色、账号、渠道和平台是业务目标，不塞入 env，也不用物理主机代替区服。
2. 展示名、昵称、邮箱、手机号和平台昵称只做检索辅助；发起、审批、执行和审计必须使用稳定 ID。
3. 目标引用是完整闭包，不接受只有角色 ID 或只有账号 ID 的模糊请求。
4. 游戏能力不足时字段可选，但缺字段必须表现为不可执行或降级为检索结果，不得默认填充 `0`、`default` 或当前区服。
5. 所有引用在执行前解析为当前快照；审批后目标变化应按高风险操作契约的重审规则处理。

## 身份对象

| 对象    | 建议字段                                      | 说明与边界                                                                  |
| ------- | --------------------------------------------- | --------------------------------------------------------------------------- |
| Account | `accountId`, `platform`, `externalAccountId`  | 玩家登录主体，可拥有多个角色；平台/外部 ID 只描述来源，不代表本仓用户账号。 |
| Role    | `roleId`, `serverId`, `accountId?`            | 游戏内受控对象；账号体系缺失的单角色游戏可声明角色即账号，但不能伪造账号。  |
| Server  | `serverId`, `regionId?`, `clusterId?`         | 逻辑区服；合服后可保留多个历史区服并指向当前逻辑服。                        |
| Region  | `regionId`                                    | 运营大区/集群，只用于分组、路由和统计，不替代 `serverId`。                  |
| Channel | `channelId`, `platform?`                      | 登录/投放渠道，用于归因和展示；不影响目标唯一性。                           |
| Mapping | `fromRef`, `toRef`, `mappingType`, `mappedAt` | 合服、账号迁移和外部系统映射；历史引用永久可追溯。                          |

`platform` 使用闭集（例如 `android/ios/steam/ps/xbox/switch/web/third_party`），新增值需要游戏数据源声明；`channelId` 是游戏侧字符串，不假定全局唯一。

## 统一目标引用

所有函数、任务、审批、工单、审计和探测保存以下规范形状：

```json
{
  "targetType": "player_role",
  "scope": { "gameId": "game-a", "env": "prod" },
  "target": {
    "accountId": "account-001",
    "roleId": "role-001",
    "serverId": "s1001",
    "regionId": "cn-east",
    "channelId": "channel-x",
    "platform": "ios"
  },
  "display": { "nickname": "展示名", "accountName": "账号名" },
  "resolvedAt": "2026-10-19T10:00:00Z",
  "identityFingerprint": "sha256:..."
}
```

- `targetType` 初始闭集：`player_role`、`player_account`、`server`、`player_cohort`。玩家个人资产操作必须解析到 `player_role`；账号级封禁、通知或查询可使用 `player_account`。
- `identityFingerprint` 对 scope、targetType 和稳定目标字段做规范化哈希；`display`、检索条件和扩展字段不参与。
- `player_cohort` 必须携带生成规则、快照版本和数量/额度摘要，并复用批量操作设计，不允许用动态查询隐式扩大审批范围。
- 目标引用进入审批 payload、任务参数、函数调用、审计和工单时保持同一 JSON 形状，不做各层私有缩写。

## 合服与旧 ID

1. 合服导入生成 `server_merge` 映射：`fromRef=(旧 serverId, roleId)` → `toRef=(当前 serverId, roleId)`；角色 ID 冲突时以游戏权威侧新生成或 canonical 角色键为准。
2. 历史工单与审计不得被重写。旧引用按只读方式解析为当前 `toRef`，并同时在执行上下文保存 `originalRef` 和 `resolvedRef`。
3. 新操作默认执行当前引用；需要针对旧 ID 重放或补偿时，必须通过映射解析并进入新的 `operationId`/审批链。
4. 映射冲突、多条当前角色或映射缺失时返回 `ambiguous_target`，人工处理，不得按排序取第一条。
5. 合服映射本身是高危配置变更，必须版本化、审计、可回滚查询，并声明导入窗口；不支持把映射当作游戏侧已合服成功的证明。

## 一账号多角色

- 查询账号可返回多个角色；执行操作必须让操作者显式选择一个 `player_role`。
- 账号级操作要声明作用面（通知、封禁登录、查询）并写明是否逐角色生效；禁止把账号操作静默广播到所有角色产生业务副作用。
- 审批摘要展示账号与目标角色，批量给账号下多个角色发奖时按批量目标列表逐项生成引用和额度校验。

## 审批、执行与审计联动

- 审批 payload 至少包含 `operationId`、函数契约指纹、统一目标引用、请求摘要和快照时间。
- `IdempotencyKey` 沿用高风险操作契约，等于 `operationId`；目标引用是请求指纹的一部分，同 ID 改目标必须冲突。
- 任务和执行日志保存 `identityFingerprint`；审计 additionally 保存 `originalRef/resolvedRef`、审批 ID、任务 ID、traceId 和游戏执行凭据。
- 页面展示昵称可作为辅助，但请求体和确认文案必须展示稳定 ID；日志详情能从目标引用跳转到原始审批与任务。
- 探测目标若代表玩家或区服，只引用 `server` 或专用探测点，不把探测目标伪装成玩家业务目标。

## 校验规则

1. scope 缺 `gameId/env`、目标缺必填稳定 ID、`targetType` 未知、platform 未声明、账号/角色形状不匹配：拒绝发布或执行。
2. 同一 `roleId` 在不同区服可存在；唯一性至少由 `(scope, serverId, roleId)` 保证，账号存在时增加 `accountId` 归属校验。
3. 映射解析必须返回 0 或 1 个当前目标；多条结果为冲突。
4. 旧引用只能读取映射，不得根据昵称或模糊搜索自动改写。
5. 检索接口返回候选列表，不隐式选择候选。

## 验收场景

1. `s1001/role-1` 与 `s1002/role-1` 同时存在，页面选择、函数入参、审批摘要、任务、审计和回放均保留完整引用，不发生串目标。
2. `account-001` 拥有 `s1001/role-a` 与 `s1002/role-b`；未选择角色时账号级操作只允许账号级能力，选择角色后才可发角色资产。
3. 合服前审批引用旧 `s1001/role-1`，合服后查询仍显示原引用，同时给出当前 `s2000/role-1` 解析结果；新补偿操作使用新 operationId 并记录双引用。
4. 合并后出现两个同角色候选且映射冲突，系统返回 `ambiguous_target`，不执行、不默认选择。
5. 只有昵称或邮箱时仅返回候选检索结果，不能发起受控写。
6. 审计关闭 OTel 后仍能按 `operationId + identityFingerprint + approvalId/taskId` 串联完整链路。

## 落地批次

1. 定义 Go/JSON 目标引用与校验器，先覆盖上述拒绝和指纹规则；不改现有 Player CRUD。
2. PageSpec 表单与函数契约接入统一引用，展示层保留只读昵称。
3. 审批、任务、执行日志和审计补齐字段与迁移；存量记录保留原 payload，不回填伪造身份。
4. 建立合服映射导入/查询/冲突处理用例，映射能力由游戏数据源声明。
5. 与默认游戏页面逐项接入玩家工作台、邮件补偿、封禁、订单和活动目标选择器。
