# Croupier 运维/日志维护全量核对（需求清单 #54 核销归档）

## 状态

- 状态: 核对归档（2026-10-04，需求清单 #54）
- 日期: 2026-10-04
- 范围: 用户需求「日志配额配置、历史日志按时间清理（24 小时前/7 天前/30 天前）、复制器日志管理（目录、保留最新多少个）」
- 结论: **已落地（2026-10-02）+ 已线上复证 + 记录与代码一致，零新缺口，OPEN-ISSUES #54 核销**；本批无代码改动，仅归档与台账核销

## 一、需求 → 落地对照

| 用户需求                           | 落地                | 证据                                                                                                                                                                                                                                                                        |
| ---------------------------------- | ------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 日志配额配置                       | ✅（口径=保留天数） | L3 键 `log.retentionDays`（0 = 跟随配置文件 executionLog/taskLog 各自缺省 7 天；>0 统一覆盖两类留痕），`RetentionConfig.ResolveDays` 每轮周期清理前读取（热生效无需重启）。边界③：行数/字节配额未实现，保留天数即配额机制                                                   |
| 按时间清理（24 小时/7 天/30 天前） | ✅                  | `POST /api/v1/ops/logs/cleanup`：scope=execution/task/all × beforeHours 1-87600（预设 24/168/720），走 `Retention.PurgeBefore`（multiGame 逐 game 库 fanout 同周期清理，ExecutionLog 关闭时回退 meta 库直清），只删 cutoff 前；前端手动清理卡预设按钮 + Popconfirm 危险确认 |
| 复制器日志管理（目录、保留数）     | ⚠️ 占位（诚实登记） | `log.copierDir`/`log.copierKeep` 为已声明占位键未接线；服务器日志轮转参数（output/file/目录/maxSizeMB/maxBackups/maxAgeDays/compress）为配置文件级，GET 只读视图展示 + 目录同前缀轮转文件计数（lumberjack 备份剥扩展名 `croupier-<ts>.log`）；改动需重启                    |

## 二、既有记录 vs 代码核验（2026-10-04）

- 键族：`internal/platform/settings/layered.go` 93-96 —— `log.retentionDays`（真键）+ `log.cleanupCron`/`log.copierDir`/`log.copierKeep`（三占位键，注释明示「未接线」）✓
- 路由：`internal/handler/routes.go` 583-585 —— `/ops` 组 `GET /logs` + `PUT /logs`（仅收 log.retentionDays，未知键/负数/越界 >36500 均 400）+ `POST /logs/cleanup` ✓
- 清理实现：`internal/platform/executionlog/retention.go` —— `RetentionConfig.ResolveDays`（:19 动态保留期来源）+ `Retention.PurgeBefore`（:116）+ `defaultSweepInterval = time.Hour`（:12，清理节奏固定每小时一轮）；`internal/svc/service_context.go` 336-341 每轮清理前读 L3 覆盖接线 ✓
- handler：`internal/api/ops/logs.go`（快照：L3 覆盖值 + 实际生效保留期 + 服务器日志只读视图 + 轮转文件计数 + 三留痕表行数/最老记录，查询失败 Rows=-1 降级）+ `logs_test.go` ✓
- 前端：`web/src/pages/System/SiteSettings/LogsTab.tsx`（保留策略卡 + 手动清理卡 + 服务器日志只读卡 + 留痕表体量表；占位键边界在 UI 提示文案中明示）+ `LogsTab.test.tsx` 6 用例 ✓
- 线上复证：2026-10-02 deploy run 36937558114，gitCommit 444d7f0，双实例 healthy ✓

## 三、边界（原登记四条，核对后无变化）

1. `log.cleanupCron`/`log.copierDir`/`log.copierKeep` 为已声明占位键未接线（清理节奏固定每小时一轮；服务器日志轮转参数为配置文件级，本批只读展示）。
2. audit_records（哈希链审计）永不参与清理（链完整性）。
3. 行数/字节配额未实现（保留天数即配额机制）。
4. 留痕表行数统计为全表 COUNT，大表上为 O(n) 读。

## 四、核销动作

- OPEN-ISSUES.md #54 行追加「已核销（2026-10-04）」注记。
- todo.md 增 #54 核对段。
- 无代码改动，无新测试；后续顺延 #55 SMTP 归运维核对 → #56 安全与限制。
