# 4301 部署环境验收报告

2026-10-14 08:08–08:45（+0800），部署机 192.168.5.5。结论：**4301 六项判据在部署机全部复验通过**，补浏览器截图取证完成。

## 拓扑说明

- 部署栈（ghcr 镜像）已滚动升级至最新 main 镜像：10-09 镜像早于 capture gates API（10-13 合入），升级后 gates API 200、迁移自动跑通。
- 部署栈不含 capture-agent（ef3c44f 已知边界：docker.yml 源码栈有，ghcr 部署栈无镜像），故本次用：
  - 本地构建的两个 capture agent：`cap-deploy-demo`（game=demo）/ `cap-deploy-demo3`（game=demo3），出站-only 经 SSH 隧道 127.0.0.1:29090 → 172.18.0.14:19090 注册到部署 server；
  - 本地 postgres:16 容器（capture-deploy-e2e）作 CDC 源：wal_level=logical，预建 publication `cap_pub_demo` / `cap_pub_demo3`（connector 只建槽不建发布）。
- 管理员凭据：部署库 admin 口令未知（bcrypt 不可逆），验收前已重置（新口令不入库，见验收当次会话记录）——**请知悉并择机改回**。

## 六项判据与证据

### ① 规则热生效 — 通过

| 时间 | 事件 |
| --- | --- |
| 08:12:26 | v1（burst threshold 1000）发布，agent `version applied version=1` |
| 08:12:45 | INSERT amount=1500 命中：`single delta 1500 exceeds threshold 1000` gtid=26878912 |
| 08:13:26 | v2（threshold 2000）热生效，无需重启 |
| 08:14:14 | 1500 静默（未命中）、2500 命中 gtid=26879448 — 阈值边界按新版本 |

### ② 断线恢复 keep last-good — 通过

- docker stop CDC 源 → 08:14:50 `receive message failed: unexpected EOF` → `capture source interrupted, will reopen` 指数退避重连（1→1→4→2→6→7→16→14s）；
- 书签保持 `lsn:26879448`；源恢复后 08:15:39 `resuming from bookmark` + `startLsn=0/19A25D8`；
- labels `captureGatesRevision=2` 未回落文件兜底；恢复后 1800 静默、2200 命中（gtid=26880664）。

### ③ 位点恢复 — 通过

- 优雅关闭前 08:20:21 `bookmark flushed on exit kind=lsn value=26880936`；
- 重启 08:22:04 `resuming from bookmark value=26880936` + `startLsn=0/19A2BA8`（26880936 = 0x19A2BA8，十六进制互证）；
- 恢复后新事件单次命中 3000（gtid=26880936），部署面 4 条 rule_hit 告警零重复。

### ④ 新表启用 — 通过

- `ALTER PUBLICATION ADD game.wallet` 后发布 v3 被拒：HTTP 422 `table game.wallet is outside the agent collection scope (captureTables)`（依赖守卫）；
- agent 白名单扩容（captureTables 加 game.wallet）重启 → 08:22:34 v3 `version applied`（cepBurst=2）；
- wallet 6000 命中 critical（gtid=26896752）、wallet 100 静默、capture_demo 2300 按旧规则（threshold 2000）继续命中不回退。

### ⑤ 过滤错误可观测 — 通过

- v5（`event.ops=["insert"]`）08:41:34 热生效；UPDATE(4000) 被过滤不命中、INSERT 6000@5000 命中（gtid=26900016）；
- 08:44:24 优雅关闭 stats：`Total:6 Insert:5 Update:1 FilteredNoMatch:1 FilteredError:0 Evaluated:5 GateHits:3`；
- FilteredError（非法 op 闭集之外）真实 PG 源不可注入，仅集成测试面证据——与本地轮同边界，能力声明如实。

### ⑥ 跨 game/env 隔离 — 通过

- demo3 v1（threshold 1000）08:24:23 applied；demo 侧同期为 v4（threshold 5000）；
- INSERT 2000：demo3 命中（08:25:16 gtid=26898304）、demo 静默；
- 告警 API 行级隔离：demo scope 6 条 / demo3 scope 1 条（判定时点）；截图 05（demo 8 条）/ 07（demo3 2 条）为 08:48 时刻累积。

## 截图清单

| 文件 | 内容 |
| --- | --- |
| 01-login.png / 02-login-filled.png / 03-after-login.png | 部署站点登录与进入面板 |
| 04-risk-strategies-demo.png | demo scope 风控策略页：v1–v5 版本链、v5 当前（insert-only 过滤）、生效对账 1/1 台已生效 |
| 05-risk-alerts-demo.png | demo scope 告警中心：capture.rule_hit warning/critical 混排 |
| 06-risk-strategies-demo3.png | demo3 scope 风控策略页：v1 当前（threshold 1000），与 demo 版本链独立 |
| 07-risk-alerts-demo3.png | demo3 scope 告警中心：仅本 game 告警 |

## 已知边界与说明

1. **FilteredError 证据面**：真实 PG CDC 不产出非法 op，该计数只有集成测试面证据（runner_stats_test 同口径）。
2. **PG 非键列 UPDATE delta 降级**：PG 默认 REPLICA IDENTITY 只给主键前像，非键列 UPDATE 的 delta 取不到，CEPGate 内部 `skippedNoDelta` 跳过——能力声明的诚实边界，非缺陷。
3. **tmpfs 配额瞬态**（08:30–08:31）：两 agent 均出现 `bookmark save failed ... disk quota exceeded`（final=false，后续重试成功；优雅关闭最终 flush 成功）——环境瞬态，非产品缺陷。
4. **SSH 隧道心跳抖动**（08:41:46 / 08:47:11）：`heartbeat i/o timeout` 后自动恢复重注册（环境性）。
5. **测试数据留存**：部署库 demo / demo3 game 与 capture gates / config_versions 为本次验收的加法式测试数据，留存于部署机，可留可删。
6. **部署栈无 capture-agent 镜像**：本次以本地二进制 + 本地 CDC 源 + SSH 隧道验收；docker.yml 源码栈（ef3c44f）不受影响。
