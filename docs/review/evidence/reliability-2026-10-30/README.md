# 2026-10-30 可靠性复审证据

结论及验收边界见 [复审报告](../../reliability-recheck-2026-10-30.md)。这些是缺陷诊断，不是应并入生产测试的通过门禁：普通诊断 PASS 表示当前缺陷仍可复现；race 预期退出非零。修复后需以正确行为编写正式回归，不能通过删除诊断或改断言制造完成。

从仓库根执行（使用现有依赖；部分包需本机监听端口与可写 Go 缓存）：

```bash
python3 docs/review/evidence/reliability-2026-10-30/replay.py
python3 docs/review/evidence/reliability-2026-10-30/replay.py --race
```

replay.py 临时构造 Go overlay，把 .go.txt 作为同包测试注入，退出后删除临时 overlay；不写业务包。它复用该版本的测试夹具，因此必须结合 source-sha256.json 和报告基线使用，不声称适配未来任意提交。

| 文件 | 证据 |
| --- | --- |
| govern-acl.go.txt | 显式函数 ACL 拒绝与 Govern 允许的差异；high 风险查询故障降档 |
| task-event-gap.go.txt | 终态更新后事件写失败，重试 ACK 但不补事件 |
| scheduler-recovery.go.txt | 跨槽恢复不处理旧 claimed |
| router-budget.go.txt | OpenBudget 不打断同步 Open |
| drain-accepted-task.go.txt | 已接受的 TaskRunner 工作在 drain ACK 后失败 |
| aggregate-race.go.txt | touchRevenue/flush 的生产字段竞争，独立于 runFlushInterval 测试竞争 |
| source-sha256.json | 审核基线、业务源与诊断哈希；审核期间已捕获的业务源无变化 |
| results.txt | 常规运行、诊断确认和预期 race 失败摘要 |

完整阻塞调度 SIGTERM、真实数据库故障、Redis/ClickHouse 崩溃/重复核对及真实游戏收据验收 NOT RUN。20 个有效 Go 定向包、124 个前端用例不能替代这些验收。
