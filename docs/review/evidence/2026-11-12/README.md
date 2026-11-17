# 2026-11-12 深度审核复现证据

基线为本次报告所列 HEAD；生产源码的 SHA-256 见 `source-sha256.json`。
这些是**缺陷复现探针**：PASS 表示观察到了报告所述错误行为，不能解释为问题已修复。

所有输入均为合成数据。Go 使用 overlay 注入测试，不往业务源码目录写测试文件；
数据库、文件、监听端口均为临时夹具。移动端请求由 Dio interceptor 返回，不请求真实服务；
C++ 通过链接器包裹 send 注入合法的短写结果。没有真实电源操作、发布或部署。

从仓库根目录执行：

```bash
python3 docs/review/evidence/2026-11-12/run_probes.py
node docs/review/evidence/2026-11-12/web-probe.cjs
python3 docs/review/evidence/2026-11-12/ci-probe.py
python3 docs/review/evidence/2026-11-12/run_cpp_probe.py
python3 docs/review/evidence/2026-11-12/run_mobile_probe.py
python3 docs/review/evidence/2026-11-12/python-transport-probe.py
```

移动端 runner 支持 `FLUTTER_BIN` 指定已有工具链，并使用 `--no-pub`，不更新锁文件。
本次使用 Flutter 3.47.9 / Dart 3.13.7；这不是 CI 固定的 Flutter 3.35.1。
因此移动端探针成功不等于 CI 工具链兼容性已验证。

| 证据 | 内容 |
| --- | --- |
| `go-probes.log` / `probe-runner.log` | 10 个包、11 个 overlay 探针；其中模板写入是行为观察，其余关联确认问题 |
| `probe_0_test.go.txt`..`probe_9_test.go.txt` | 本次执行的 Go 探针原文；生成入口是 `run_probes.py` 中 SOURCES |
| `mobile_probe.dart.txt` / `mobile-probe.log` | 真实 AnalyticsController 请求乱序复现 |
| `web-probe.cjs` / `web-probe.log` | 使用实际 TypeScript SSE 解析源码，比较同字节不同分块结果 |
| `ci-probe.py` / `ci-probe.log` | 提取实际 workflow 的覆盖率 shell 步骤，100% lcov 仍失败 |
| `cpp-short-send.cpp` / `cpp-probe.log` | 编译实际 C++ 响应 writer，140 字节帧仅写出 5 字节后静默返回 |
| `python-transport-probe.py` / 对应 `.log` | 实际 Python reader 处理超长帧后不清连接状态、不唤醒 pending |
| `go-baseline.log` | 根模块 14 个包的原有测试；完整命令见主报告 |
| `go-sdk-baseline.log` | Go SDK 独立 module 中 transport 与 croupier 包测试 |
| `go-baseline-input-error.log` | 首次命令误把 SDK 独立 module 目录作为根 module 包；属审核命令错误，已纠正并独立重跑，不计产品缺陷 |
| `web-baseline.log` | Web 3 套件 45 用例；指定的 scope store 路径未匹配到额外套件，不能算作该文件已测试 |
| `golden-vectors.log` | Go/Python/JS 三语言 golden 向量测试，strict 模式无跳过 |
| `go-build.log` | `go build ./...`（成功时空文件） |
| `web-tsc.log` | Web TypeScript 检查 |
| `dashboard-guard.log` / `version-guard.log` | 页面契约守卫与版本一致性守卫 |
| `docs-build.log` | 文档构建日志 |

未运行六语言全量编译、CI 上的 Flutter 3.35.1、Android/iOS 实机、远端发布签名和灾备恢复。
C++ 短写由注入确定性复现；未以真实拥塞网络复现并发帧交错。
修复后探针可能因不再出现旧行为而失败，应把它们转为断言安全结果的正式回归测试。
