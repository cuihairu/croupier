---
title: MongoDB Connector 立项简档——内置源收口最后一项（待批）
---

# MongoDB Connector（立项待批）

## 状态

- 状态: **Proposed（立项待批，未动码）**。
- 关联: [CDC Connector 架构](cdc-connector-architecture.md) §P1 梯队（MongoDB 与 SQL Server/Oracle 同级）；todo 条目 4310。
- 收口意义: 内置源已闭环三源（MySQL/MariaDB/Postgres），MongoDB 是内置源收口的**最后一项**——之后新源一律由使用方按 `SourceConnector` 契约自行实现（不并入内置源）。

## 1. 范围

**做**：MongoDB change streams 接入，`sourceType="mongodb"`。支持 MongoDB 6.0+（go driver v2 线）、replica set / sharded（经 mongos）/ Atlas。

**不做**：SQL Server / Oracle / TiDB / CockroachDB / 国产库——明确不做（口径不变）。

**硬前置**：change streams **必须 replica set**（standalone mongod 无 change streams）。`Open` 期校验 `hello().setName`，standalone 直接返回错误并给可执行提示（`--replSet rs0`），不做静默降级。

## 2. 契约映射（`SourceConnector` 六方法）

| 方法         | MongoDB 落地                                                                                                                                      |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Open`       | mongo driver v2 客户端；连接串只从 `SourceConfig.DSNEnv` 指向的环境变量解析（凭据红线不变）；replica set 校验                                     |
| `Discover`   | `listCollections` → `TableMeta{Name:"db.collection", PrimaryKey:["_id"]}`；schema-less 无列元数据，`ColumnsMapped=false`；同时产出 `Capabilities` |
| `Snapshot`   | **v1 不支持**：返回 `ErrUnsupportedSnapshot`（`capabilities.snapshot=false`，绝不静默回空流）                                                     |
| `Poll`       | `Watch(pipeline, {ResumeAfter: tok, FullDocument:"updateLookup"})` 永久消费；`From` 空=从当前时间点起，日志显式记录起点                           |
| `Checkpoint` | resume token 序列化 → `Position{Kind:"token", Value:<hex>}`                                                                                       |
| `Close`      | Cancel watch + Disconnect（幂等，Close 后调用返回错误）                                                                                           |

**两个必须改的代码触点**（实现批次先动手处）：

- `PositionKindOf`（`agents/capture/source/provider_bridge.go`）现状 `default: "gtid"`——不加 `"mongodb": "token"` 分支，mongo 位点会被标成 `gtid` 落 checkpoint（静默错标）。`Position.Kind` 闭集已有 `"token"`，**无需扩枚举**。
- `sourceType` 闭集 `"mysql"|"postgres"` → 扩 `"mongodb"`（`source/event.go` 注释、白名单形态 `db.collection`）。

## 3. 事件映射

| change stream `operationType`    | `ChangeEvent.Operation` | 像                                                                              |
| -------------------------------- | ----------------------- | ------------------------------------------------------------------------------- |
| `insert`                         | `insert`                | `After=fullDocument`                                                            |
| `update`                         | `update`                | `After=fullDocument`（whenAvailable）；`Before`=updateLookup 读回的前像         |
| `replace`                        | `update`                | `After=fullDocument`（replace 必带完整后像）；`Before`=updateLookup（无则 nil） |
| `delete`                         | `delete`                | `Before`=updateLookup（无则 nil）                                               |
| `drop` / `rename` / `invalidate` | ——                      | 非行级事件：不进 `ChangeEvent`，runner 计数 `Skipped`                           |

## 4. 能力声明（诚实边界，server 发布校验消费）

| 能力                  | 声明                     | 理由                                                                                                                             |
| --------------------- | ------------------------ | -------------------------------------------------------------------------------------------------------------------------------- |
| `afterImage`          | ✅                       | insert/replace 全量后像；update 依赖 `FullDocument=updateLookup`（`_id` 必须存在）                                               |
| `beforeImage`         | ❌ **v1 保守声明 false** | updateLookup 每次 update 多一次点查；drop/rename 无像。声明 false 后 gate lint 的依赖+能力感知守卫会拒绝依赖前像的规则——如实生效 |
| `transactionMetadata` | ✅                       | `txnNumber`/`lsid`/`txnStartClusterTime`/`txnCommitTime` 原生字段                                                                |
| `snapshot`            | ❌                       | 见 §2                                                                                                                            |
| `resumableCheckpoint` | ✅                       | resume token 稳定、可持久化、跨重启续传（等价 LSN）                                                                              |
| `pushdown`            | ❌                       | change stream pipeline 是 `$match`/`$addFields` 变换，不是订阅级过滤                                                             |

## 5. delta 与 gates 兼容

- burst 规则 `deltaField` 语义与现有源一致：insert=后像值、update=后像-前像、delete=前像值。
- remove 语义：`delete` 事件无前像 → `skippedNoDelta` 降级跳过（与 PG 非键列前像同族，属能力声明的诚实边界，非缺陷）。
- 表名形态 `db.collection` 两段，与 `captureTables` 白名单同形态——**gate lint 依赖校验零改动**。

## 6. CI / 活栈前置（最易踩的坑）

- change streams 必须 replica set：CI 用 `mongo:7 --replSet rs0` + `mongosh --eval 'rs.initiate()'` + 等 primary。当前 `capture-matrix` job 走 `docker run` setup（service 容器传不了守护进程启动参数，同 MySQL 注释先例），加一条即可。
- 本地活栈验证同样需要 rs0；standalone mongod 会报 `Change streams are only available on replica set members`。
- 活栈用例增 `TestMongoDBConnectorChangeStreams`（`tags=integration`，三操作 + 位点续读 + 断线重连）。

## 7. 依赖与门禁

- `go.mongodb.org/mongo-driver/v2` **已在依赖图**（go.mod indirect，v2.5.0）→ 落地时仅需 indirect→direct 提升（版本零新解析）；go.mod 仍属依赖清单变更，按仓库规则跑全套测试门禁（不止受影响包）。
- 契约层单测零外部依赖（事件映射 / 能力声明 / 位点序列化）；活栈测试打 `integration` 标签。

## 8. 分期

- **M1 契约接入**：`sourceType="mongodb"` 闭集 + `PositionKindOf` 的 `token` 分支 + 能力声明 + gate lint 兼容自检。
- **M2 Connector 实现**：事件映射 / 位点序列化 / updateLookup 策略 / 错误面（契约单测全绿）。
- **M3 活栈集成**：rs 容器 + 三操作 + 位点续读 + 断线重连（CI matrix 接入）。
- **M4 文档三层同步**：connector 简档 / capture-agent 配置面 options 说明 / 活栈验证记录。

## 9. 待用户拍板的三点

1. **立项与排期**——它是内置源收口项，之后内置源停止扩张。
2. **`beforeImage` 是否 v1 就声明 true**：要承担 updateLookup 双倍读源的读放大 + drop/rename 无像的降级。建议 v1 保守 false。
3. **Snapshot 是否纳入本轮**：建议 v1 不做（`ErrUnsupportedSnapshot`），回补需求出现再上 collection scan + 事件去重（按 `_id` + `clusterTime` 水位）。

## 已知边界

- standalone mongod 不支持 change streams（`Open` 硬校验报错并给可执行提示，不降级）。
- schema-less 无列元数据：`ColumnsMapped=false`、`SchemaVersion` 不可得——DDL 漂移检测在 mongo 面退化。
- `Account` 血缘尽力而为（driver 无统一会话用户读取面）。
- `FullDocument=updateLookup` 每次 update 多一次点查，热表存在读放大。
- `drop`/`rename`/`invalidate` 只计数不入告警。
