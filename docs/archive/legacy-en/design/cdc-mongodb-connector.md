---
title: MongoDB Connector——内置源收口最后一项（已实施）
---

# MongoDB Connector（已实施）

## 状态

- 状态: **Implemented（2026-10-16 落地，M1–M4 单轮收口）**。
- 关联: [CDC Connector 架构](cdc-connector-architecture.md) §P1 梯队（MongoDB 与 SQL Server/Oracle 同级）；todo 条目 4310。
- 收口意义: 内置源已闭环三源（MySQL/MariaDB/Postgres），MongoDB 是内置源收口的**最后一项**——之后新源一律由使用方按 `SourceConnector` 契约自行实现（不并入内置源）。

### 决策记录（2026-10-16，PATROL1016 巡检推进单代决）

推进单条款授权「可自主决策的直接拍板」，§9 三点代决如下：

1. **立项与排期**：已代决立项，本轮即排（非不可逆、非超范围——简档已备、
   todo 在列、依赖已在图）。内置源自此停止扩张。
2. **`beforeImage` v1=false**：按简档建议保守声明。updateLookup 给的是
   **后像**不是前像（简档 §3 原文此处口径有误，见下修正③）；真前像要
   `changeStreamPreAndPostImages`（另开集合），v1 不要求。
3. **Snapshot v1 不做**：`ErrUnsupportedSnapshot` 显式拒绝，回补需求出现
   再上 collection scan + 事件去重。

实施期对简档的三处技术口径修正（以代码与活栈实测为准）：

- **① `ColumnsMapped=true`**（简档原文误写 false）：契约语义是「像键是否
  列名（false=位置下标）」。mongo 行像键=bson 字段名，属列名形态 → true。
  binlog ROW 裸下标才是 false（provider_bridge 既有口径）。
- **② 非行级事件流级排除**（简档原文「runner 计数 Skipped」）：change
  stream pipeline `$match operationType $in [insert,update,replace,delete]`
  在服务端排除 drop/rename/invalidate——不进客户端、不进 runner、无计数
  （mapChangeEvent 的拒绝分支是第二道防线，非主路径）。
- **③ `Before` 恒 nil**（简档原文 update/delete 行把 updateLookup 当前像）：
  updateLookup 只挂 `fullDocument` 后像；delete 事件只有 documentKey 无像。
  与 beforeImage=false 声明一致，不伪造前像。

## 1. 范围

**做**：MongoDB change streams 接入，`sourceType="mongodb"`。支持 MongoDB 6.0+（go driver v2 线）、replica set / sharded（经 mongos）/ Atlas。

**不做**：SQL Server / Oracle / TiDB / CockroachDB / 国产库——明确不做（口径不变）。

**硬前置**：change streams **必须 replica set**（standalone mongod 无 change streams）。`Open` 期校验 `hello().setName`，standalone 直接返回错误并给可执行提示（`--replSet rs0`），不做静默降级。

## 2. 契约映射（`SourceConnector` 六方法）

| 方法         | MongoDB 落地                                                                                                                                                          |
| ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Open`       | mongo driver v2 客户端；连接串只从 `SourceConfig.DSNEnv` 指向的环境变量解析（凭据红线不变）；replica set 校验                                                         |
| `Discover`   | `listCollections` → `TableMeta{Name:"db.collection", PrimaryKey:["_id"]}`；schema-less 无列元数据，`ColumnsMapped=true`（像键=bson 字段名）；同时产出 `Capabilities`  |
| `Snapshot`   | **v1 不支持**：返回 `ErrUnsupportedSnapshot`（`capabilities.snapshot=false`，绝不静默回空流）                                                                         |
| `Poll`       | `Watch(pipeline, {ResumeAfter: tok, FullDocument:"updateLookup"})` 永久消费；`From` 空=从当前时间点起，日志显式记录起点；`From.Kind` 只认 `token`，其他 kind 显式拒绝 |
| `Checkpoint` | resume token 序列化 → `Position{Kind:"token", Value:<hex>}`                                                                                                           |
| `Close`      | close-join（置位 closed → cancel 流 ctx → 等 pump 退出 → Disconnect），幂等                                                                                           |

**实际改动的代码触点**（简档预案两个，落地四个半）：

- `PositionKindOf`（`agents/capture/source/provider_bridge.go`）：加 `"mongodb": "token"` 分支（不加会把 mongo 位点静默错标成 `gtid`）。`Position.Kind` 闭集已有 `"token"`，**零枚举扩展**。已落地。
- `sourceType` 闭集 `"mysql"|"postgres"` → 扩 `"mongodb"`：`config.SourceTypes` + `config.Validate` 错误消息。已落地。
- `runner.newConnector` dispatch：加 `case "mongodb"` 分支（`runner.go`）。已落地。
- runner 能力标签分支：`captureConnectorCapabilities` 对 mongodb 取 `MongoCapabilities()`。已落地。
- runner `probeTarget`：加 `mongo.AddressFromDSN` 分支（mongodb URI → host:port，`mongodb+srv` 走 SRV 解析；Readiness 探测用，非凭据面）。已落地（简档未列）。

## 3. 事件映射

| change stream `operationType`    | `ChangeEvent.Operation` | 像                                                                  |
| -------------------------------- | ----------------------- | ------------------------------------------------------------------- |
| `insert`                         | `insert`                | `After=fullDocument`                                                |
| `update`                         | `update`                | `After=fullDocument`（updateLookup 后像）；`Before` 恒 nil（修正③） |
| `replace`                        | `update`                | `After=fullDocument`（replace 必带完整后像）；`Before` 恒 nil       |
| `delete`                         | `delete`                | 无像（documentKey only，单像缺失合法）                              |
| `drop` / `rename` / `invalidate` | ——                      | 非行级事件：流级 `$match` 排除，不进 `ChangeEvent`（修正②）         |

update 查不回 `fullDocument`（行已被删/权限不可见）时跳过不伪造（双像全空的
update 会被契约 `Validate` 拒绝）。行像转 JSON 走 ext-JSON relaxed 形态：
数值保数值、`ObjectId`→`{"$oid"}`、日期→`{"$date"}`。

## 4. 能力声明（诚实边界，server 发布校验消费）

| 能力                  | 声明                     | 理由                                                                                                                                                                         |
| --------------------- | ------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `afterImage`          | ✅                       | insert/replace 全量后像；update 依赖 `FullDocument=updateLookup`（`_id` 必须存在）                                                                                           |
| `beforeImage`         | ❌ **v1 保守声明 false** | updateLookup 给后像不给前像（真前像需 `changeStreamPreAndPostImages` 另开集合）；drop/rename 无像。声明 false 后 gate lint 的依赖+能力感知守卫会拒绝依赖前像的规则——如实生效 |
| `transactionMetadata` | ✅                       | `txnNumber`/`lsid`/`txnStartClusterTime`/`txnCommitTime` 原生字段                                                                                                            |
| `snapshot`            | ❌                       | 见 §2                                                                                                                                                                        |
| `resumableCheckpoint` | ✅                       | resume token 稳定、可持久化、跨重启续传（等价 LSN）                                                                                                                          |
| `pushdown`            | ❌                       | change stream pipeline 是 `$match`/`$addFields` 变换，不是订阅级过滤                                                                                                         |

## 5. delta 与 gates 兼容

- burst 规则 `deltaField` 语义与现有源一致：insert=后像值、update=后像-前像、delete=前像值。
- remove 语义：`delete` 事件无前像 → `skippedNoDelta` 降级跳过（与 PG 非键列前像同族，属能力声明的诚实边界，非缺陷）。
- 表名形态 `db.collection` 两段，与 `captureTables` 白名单同形态——**gate lint 依赖校验零改动**。

## 6. CI / 活栈前置（最易踩的坑）

已落地（`.github/workflows/ci.yml` `capture-matrix` job）：

- change streams 必须 replica set：CI 用 `mongo:7 mongod --replSet rs0 --bind_ip_all` + 就绪等待 + `mongosh --eval 'try { rs.initiate(...) } catch(e){ print(e.codeName) }'` + 等 `hello().isWritablePrimary`。`capture-matrix` 走 `docker run` setup（service 容器传不了守护进程启动参数，同 MySQL 注释先例）。
- 宿主侧 DSN 需 `directConnection=true`：rs 配置在容器内通告 `localhost:27017`，宿主端口映射直连不走路由发现。
- 活栈用例 `TestMongoConnectorChangeStreams`（`tags=integration`，env `TEST_MONGO_CAPTURE_DSN`）：建集合 → 空位点 Poll → INSERT/UPDATE/DELETE 三操作 → token 位点续传（断流期写入不丢）→ 白名单外集合不进流 → 非行级事件（drop）不进流。本地 `mongo:7` 容器实测 3.1s 通过（2026-10-16）。
- 一个实现期实踩的坑：change stream 的 `wallTime` 是 BSON UTC datetime，
  解码成 `int64` 报 `cannot decode UTC datetime into an integer type`——须用
  `bson.DateTime`（回退 `clusterTime` 秒值 ×1000）。

## 7. 依赖与门禁

- `go.mongodb.org/mongo-driver/v2` **已在依赖图**（go.mod indirect，v2.5.0）→ 落地仅需 indirect→direct 提升（版本零新解析，实测版本未变）。提升连带带出 driver 的证书鉴权间接依赖 4 项（`xdg-go/scram`、`xdg-go/pbkdf2`、`xdg-go/stringprep`、`youmark/pkcs8`），go.sum 增 2 行——非新直接依赖。
- 契约层单测零外部依赖（事件映射 / 能力声明 / 位点序列化）；活栈测试打 `integration` 标签。
- go.mod 属依赖清单变更 → 已按仓库规则跑全套门禁。

## 8. 分期（全部完成，2026-10-16）

- **M1 契约接入** ✅：`sourceType="mongodb"` 闭集 + `PositionKindOf` 的 `token` 分支 + 能力声明 + gate lint 兼容（能力自报经 labels 上行，`CapAfterImage` 消费面零改动）。
- **M2 Connector 实现** ✅：事件映射 / 位点序列化 / updateLookup 策略 / 错误面（契约单测全绿：能力闭集、契约守卫、缺 env、非行级拒绝、四操作映射、ext-JSON 形态、token 往返、pipeline 构建、AddressFromDSN）。
- **M3 活栈集成** ✅：rs 容器 + 三操作 + token 位点续传 + 白名单/非行级负向窗口（CI matrix 已接入；本地 `mongo:7` 实测通过）。
- **M4 文档三层同步** ✅：本简档状态与决策记录 / `configs/capture-agent.example.yaml` sourceType 面 / 活栈验证记录。

## 9. 原待拍板三点 → 已代决

见「决策记录」小节：立项排期=本轮（已代决）；beforeImage v1=false；snapshot 不做。

## 已知边界

- standalone mongod 不支持 change streams（`Open` 硬校验报错并给可执行提示，不降级）。
- schema-less 无列元数据：`ColumnsMapped=true` 但仅指像键=bson 字段名；`SchemaVersion` 不可得——DDL 漂移检测在 mongo 面退化。
- 行像是 **ext-JSON relaxed 形态**（数值保数值、`ObjectId`→`{"$oid"}`、日期→`{"$date"}`）——schema-less 源无标量保证，消费方按 ext-JSON 形态理解。
- `Before` 恒 nil（能力声明 beforeImage=false；真前像需 `changeStreamPreAndPostImages`，v1 不做）。
- update 查不回 `fullDocument`（行已被删/权限不可见）时跳过不伪造。
- 含点集合名（`db.a.b`）与两段白名单形态不兼容，pipeline 白名单按整段 coll 匹配会漏——配置白名单时避开含点集合名。
- `Account` 血缘 v1 未填（driver 无统一会话用户读取面，简档「尽力而为」落到「不填」）。
- `FullDocument=updateLookup` 每次 update 多一次点查，热表存在读放大。
- 非行级事件（drop/rename/invalidate）流级排除，无事件亦无计数面。
