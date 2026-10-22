# Croupier 运维/性能参数设置全量核对（需求清单 #53 核销归档）

## 状态

- 状态: 核对归档（2026-10-04，需求清单 #53）
- 日期: 2026-10-04
- 范围: 用户需求「内存缓存大小、Redis 配置、系统性能监控（最大占用 CPU/内存/磁盘阈值）、并发请求、线程数量、磁盘大小、系统内存统计、缓存之类」
- 结论: **已落地（2026-10-02）+ 已线上复证 + 记录与代码一致，OPEN-ISSUES #53 核销**；仅「Redis 配置」一项登记边界（如实，不扩代码）；本批无代码改动，仅归档与台账核销

## 一、需求 → 落地对照

| 用户需求           | 落地        | 证据                                                                                                                                                                                                      |
| ------------------ | ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 内存缓存大小       | ✅          | `perf.cacheSize`（字节存储，前端 MB 编辑换算）+ 快照回显                                                                                                                                                  |
| Redis 配置         | ⚠️ 边界登记 | settings 层无 redis.* 键；部署级 `cache.type: redis` 由 config 文件层支持（configs/server.yaml 示例）。运行时切换缓存后端属部署级变更（重连语义/数据面风险），settings 面板不暴露——如实登记，另行批次评估 |
| 性能监控：CPU 阈值 | ✅          | `perf.maxCpuPct`（0 = 不过滤）+ 宿主机 CPU 采样（gopsutil，自上次采样窗口值）+ overload 红色注记                                                                                                          |
| 性能监控：内存阈值 | ✅          | `perf.maxMemoryPct` + 运行时/宿主机内存双列 + overload 注记                                                                                                                                               |
| 性能监控：磁盘阈值 | ✅          | `perf.maxDiskPct` + 宿主机磁盘占用（进程工作目录所在盘）+ overload 注记                                                                                                                                   |
| 并发请求           | ✅          | `perf.maxConcurrent`（0 = 无限制）——仅注记不拦截（边界①）                                                                                                                                                 |
| 线程数量           | ✅          | `perf.maxThreadCount`——仅记录不热改 GOMAXPROCS（边界②）                                                                                                                                                   |
| 磁盘大小           | ✅          | 宿主机磁盘占用统计（见上）                                                                                                                                                                                |
| 系统内存统计       | ✅          | `GET /api/v1/ops/performance` 运行时统计：GOMAXPROCS/Goroutines/堆/总内存/GC 次数                                                                                                                         |
| 缓存之类           | ✅          | `perf.cacheSize`（注记边界③：internal/cache 无容量上限语义可接，值先存储回显）                                                                                                                            |

## 二、既有记录 vs 代码核验（2026-10-04）

- 六键：`internal/platform/settings/layered.go` 83-88 —— `perf.{maxCpuPct,maxMemoryPct,maxDiskPct,maxConcurrent,maxThreadCount,cacheSize}`，注释含「0 = 不过滤/无限制」语义 ✓
- 路由：`internal/handler/routes.go` 578-579 —— `/ops` 组 `GET /performance` + `PUT /performance` ✓
- handler：`internal/api/ops/performance.go`（PerformanceSnapshotResponse L2∧L3 合成 + 逐键来源；Get=参数+运行时快照；Put=逐键 L3 覆盖，未知键/负数/百分比 >100 均 400，写后热重载）+ `performance_test.go` 在册 ✓
- 前端：`web/src/pages/System/SiteSettings/PerformanceTab.tsx`（六键表单 + 来源徽标 + 运行时/宿主机双列统计 + 超限红色注记）+ `PerformanceTab.test.tsx` 5 用例 ✓
- 边界四条（阈值仅注记不拦截 / maxThreadCount 不热改 / cacheSize 仅存储回显 / CPU 采样窗口与磁盘采样盘）与注记一致 ✓
- 线上复证：2026-10-02 deploy run 36937558114，gitCommit 444d7f0，双实例 healthy ✓

## 三、边界（原登记四条 + 本批追加一条）

1. 阈值本批仅注记不拦截请求（请求限流中间件另行批次）。
2. maxThreadCount 仅记录不热改 GOMAXPROCS（全局调度变更风险另行评估）。
3. cacheSize 仅存储回显（internal/cache 无容量上限语义可接）。
4. CPU 百分比为自上次采样窗口值；宿主机磁盘采样进程工作目录所在盘。
5. **（本批追加）Redis 配置未纳入 settings 面板**：`cache.type: redis` 仅部署 config 层支持，运行时切换缓存后端（重连/数据面语义）另行批次评估。

## 四、核销动作

- OPEN-ISSUES.md #53 行追加「已核销（2026-10-04）」注记（含 Redis 边界登记）。
- todo.md 增 #53 核对段。
- 无代码改动，无新测试；后续顺延 #54 日志维护核对 → #55 SMTP 归运维。
