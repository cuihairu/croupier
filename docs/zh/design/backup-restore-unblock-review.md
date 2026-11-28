# 备份恢复演练：解阻决策评审

日期：2026-11-28。状态：**proposal awaiting owner decision**（提案待 owner 决策）。
DEPLOY-002 具名阻塞点（deploy-inventory-2026-11-28.md §9）与 OPS-008 Stage C
预留的形态决策（「恢复演练模式…暂缓，不盲建」）的交付物。**本文不实现任何
东西**——不改镜像、不建备份、不跑演练。提案把 owner 决策收窄到一次签核。

## 1. Grounding（代码 + 现场事实，2026-11-28）

| 事实                                                                                                                                         | 来源                                                             |
| -------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| postgres 执行器经 PATH 外呼：`run(ctx, "pg_dump", …)`，参数 `--format=plain --no-owner --no-privileges`                                      | `internal/platform/backupexec/executor.go:261`（`dumpPostgres`） |
| plain-SQL dump → 恢复只需 `psql -f`，不需要 `pg_restore`                                                                                     | 上行执行器参数                                                   |
| sqlite 是唯一进程内 dump 先例（`VACUUM INTO`）                                                                                               | `executor.go` `dumpSQLite`                                       |
| server 运行时基底 `gcr.io/distroless/static:nonroot`；CGO 禁用静态二进制；busybox 已用 `COPY --from` + `--install` 装入；`USER nonroot` 运行 | `docker/Dockerfile.server`                                       |
| `distroless/static` 无动态 loader——musl 链接的 pg_dump 不能直接 COPY 进来                                                                    | deploy-inventory §9（容器内两次探测 rc=1）                       |
| 线上 postgres 为 `postgres:15-alpine`（major 15）；pg_dump major 必须 ≥ server major                                                         | deploy-inventory §3                                              |
| 已有一个 compose 服务以 `distroless/base-debian12:nonroot`（含 glibc loader）为运行时基底                                                    | `docker/docker-compose.yml:232`（sdk-demo-go）                   |
| `backups` / `db_sources` 双 0 行；调度器未配置                                                                                               | deploy-inventory §9                                              |
| 平台级备份路径已存在：无注册数据源 → 回落 `Config.Database.Driver/DataSource`（meta DB）                                                     | `internal/api/ops/backups.go:128-135,209-223`                    |
| 线上为单库模式 → meta DB 一份 dump 即覆盖生产全部表                                                                                          | deploy-inventory §1                                              |

## 2. D1 —— server 镜像如何获得 pg_dump

| 选项                                              | 改动                                                                                                                                                                     | 评估                                                                                                                                                                                                                                  |
| ------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| (a) **base-debian12 + COPY pg_dump 闭包**（推荐） | 仅 server 的 `RUNTIME_IMAGE=gcr.io/distroless/base-debian12:nonroot`；builder 阶段 `FROM postgres:15-bookworm` 把 `pg_dump` 及其 `ldd` 闭包拷入 `/out`，再 `COPY --from` | 保持 distroless 无包管理器姿态；glibc loader 在场（仓内先例：sdk-demo-go）；二进制来自官方 Debian postgres 镜像、钉住线上 major 15；库闭包在构建期用 `ldd` 机械推导——不手工枚举；CI 冒烟：构建镜像内以 nonroot 跑 `pg_dump --version` |
| (b) backupexec 增加 pure-Go postgres dump         | Go 侧逻辑 dump                                                                                                                                                           | 不动镜像，但 postgres 没有sqlite `VACUUM INTO` 那样的单语句等价物；手写 dump 是 DR 关键路径的保真度风险，且恰好在一切皆坏的时刻运行；库生态本轮 NOT VERIFIED。仅在 (a) 因镜像策略被拒时作后备                                         |
| (c) 整体换 debian-slim/alpine 基底 + 包管理器     | 运行时镜像带 apt/apk                                                                                                                                                     | 否决：放弃 distroless 姿态；Dockerfile 自身设计注释即「不引入包管理器」                                                                                                                                                               |
| (d) COPY musl 版 pg_dump 进 distroless/static     | —                                                                                                                                                                        | 否决：static 基底无 loader（deploy-inventory §9 已排除）                                                                                                                                                                              |
| (e) sidecar dump 容器 / 宿主 cron dump            | —                                                                                                                                                                        | 否决：绕开 backupexec → ObjectStore → sha256 校验链，违反验收条款「不新增未经设计的恢复服务」——与 §9 否决宿主手工 dump 同一裁定                                                                                                       |

## 3. D2 —— 首份真实备份的范围

- (a) **先走平台 meta DB 路径**（推荐）：`backups.go` 既有回落不需要
  `db_sources` 行；线上单库模式下一份 dump 覆盖生产全部表。D1 落地后立即
  解阻 A2/A3。
- (b) game 作用域数据源：需登记 `db_sources` 行（带凭据的运维动作）——有意
  后置；不是恢复演练的前置。

## 4. D3 —— 恢复演练形态（即 OPS-008 Stage C 的定形）

- (a) **runbook 先行演练**（推荐）：部署宿主一次性 `postgres:15` 容器；经
  既有 download 端点或 file store 取产物；`Verify`（sha256 读回）通过；
  `psql -f` 恢复进 scratch 库；对照 deploy-inventory 已记录的线上计数做
  schema + 行数抽检；把 RPO（备份时间戳 → 演练时刻）与 RTO（恢复挂钟时长）
  记入带日期的评审文档；演练后容器销毁。**不新增任何平台代码。**
- (b) 周期演练端点（ops API「演练模式」）：仅当 owner 需要定期演练；另立
  设计，本文不提案。

副作用边界（承 deploy-inventory §9，对演练有约束力）：恢复库绝不接回线上
链路；live server 绝不指向 scratch 库。registry 会话、在途审批、调度器
状态、SSE 订阅者是内存态、不进任何 dump——恢复库未经重热语义不得当 live
server 启动；该回放问题明确不在本文范围。

## 5. D4 —— 业务凭据核对的范围

- 平台备份成功即证明所配置 DB 凭据可 dump；演练的 `psql` 恢复证明产物可
  restore。
- 凭据**轮换**（deploy-inventory §4 发现线上为模板默认开发口令）是 DEPLOY-002
  之外的运维动作——具名为边界，本提案不覆盖。
- mysql 路径（`mysqldump`）存在同样的镜像缺口；线上仅 postgres，记为已知
  边界，不在本文处理。

## 6. 验收映射

| #   | 场景                                                                                | 对应        |
| --- | ----------------------------------------------------------------------------------- | ----------- |
| A1  | 构建出的 server 镜像以 nonroot 跑 `pg_dump --version`，rc=0，major 15               | D1(a)       |
| A2  | 经既有 API 创建平台备份 → `succeeded` + sha256 + 对象落存储；`backups` 行 > 0       | D1(a)+D2(a) |
| A3  | `Verify` 读回重算 sha256 对照存储对象 → 通过                                        | 既有链路    |
| A4  | 一次性 postgres:15 容器：`psql -f` 恢复 API 产物；schema/行数抽检记录；RPO/RTO 记录 | D3(a)       |
| A5  | 演练隔离：scratch 库在一次性容器上、演练后销毁；线上链路不受触碰                    | D3 边界     |
| A6  | 内存态边界记录在案（会话/审批/调度器/SSE 不进 dump）                                | D3          |

## 7. 待 owner 决策

- [ ] D1：批准 (a) base-debian12 + pg_dump 闭包 COPY（或选 b/后备）。
- [ ] D2：确认首份备份范围 = 平台 meta DB。
- [ ] D3：批准 runbook 先行演练 (a) 并授权部署宿主操作窗口（线上宿主纪律），或再次搁置演练形态（OPS-008 Stage C 维持暂缓）。
- [ ] D4：确认凭据轮换不进 DEPLOY-002 范围。

## 8. 边界

- 不实现任何东西：不改 Dockerfile、不构建镜像、不建备份、不跑演练——只有
  本文。实现（A1/A2）跟随 D1/D2 签核；演练本体（A4/A5）还需 D3 的部署宿主
  窗口授权。
- pg_dump 库闭包清单有意不手工枚举——若 D1(a) 获批，必须在 builder 阶段
  以 `ldd` 机械推导。pure-Go dump 库生态：NOT VERIFIED（仅 D1(b) 被选中时
  才相关）。
