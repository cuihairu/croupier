# 2026-11-13 代码逻辑审核证据

`review_regressions.py` 在 6 个包中通过 Go overlay 注入 8 个正常业务回归用例。
原业务实现和现有测试文件均不修改。

```bash
python3 docs/review/evidence/2026-11-13/review_regressions.py
```

**当前预期命令退出 1**：八个断言表达正确行为，当前实现没有满足它们。这与上一轮“PASS 表示复现错误行为”的探针不同；本轮不执行攻击式测试。

| 文件 | 用例 / 作用 |
| --- | --- |
| case_0_test.go.txt | 供给正常状态保存；实际 Gorm 适配器的竞争 CAS 返回应为错误而非 panic |
| case_1_test.go.txt | 事故详情 JSON 正常更新与读回 |
| case_2_test.go.txt | 合法 IPv6 ping 应答应成功 |
| case_3_test.go.txt | 活跃发版与回滚互斥；取消批次等待前应保留已完成进度 |
| case_4_test.go.txt | 慢插件启动不应挡住其他调用的 deadline |
| case_5_test.go.txt | 同一次每日计划跨分钟仍只能领取/派发一次 |
| regressions.log | 8 个回归断言的实际失败结果与错误堆栈 |
| runner.log | 执行器控制台结果，保留非零退出对应输出 |
| existing-tests.log | 原有相关包测试；与新增缺口分开记录 |
| source-sha256.json | 本轮 commit 与生产源文件指纹 |

所有数据库与业务执行使用临时夹具；没有 Terraform、真实云资源、真实程序换包或电源动作。
ICMP 使用正常 PacketConn 替身。插件用仓库已有 echo 测试子进程，退出时由测试清理。
两项 Runbook 生命周期发现为源码调用链结论，本目录没有将其标成已动态验证。
