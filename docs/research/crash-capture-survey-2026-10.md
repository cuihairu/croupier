---
title: 崩溃快照与跨语言抓取调研——语言原生机制、OS 级通用层与分级方案（#67 配套）
---

# 崩溃快照与跨语言抓取调研

## 状态

- 状态: 调研归档（2026-10-08，Agent Supervisor #67「先调研后拍板」配套；本文只调研不实现）
- 日期: 2026-10-08
- 范围: 被监管进程崩溃时拿到内存快照的可行路径——①各语言原生机制（Go/Python/Node/JVM）；②OS 级语言无关抓取层（core_pattern/systemd-coredump、Windows WER、Crashpad/Breakpad）；③解析层的语言相关性；④对比与分级方案建议 + 可拍板项
- 结论速览: **没有单点通吃的应用级工具**；「OS 层抓取层零集成通吃（Linux core_pattern 管道 / Windows WER LocalDumps）+ 语言原生机制做轻量级画像 + 解析层分语言」是证据支撑的组合。Croupier 特有杠杆：被监管进程由 agent spawn（`ManagedProcesses` 配置 command/env），**spawn 时可注入语言原生开关**，轻量级方案的可行性因此显著高于一般场景
- 关联: [Agent Supervisor 设计简档](../design/agent-supervisor-design.md) §8 S3（快照方案待本文拍板后并入）

## 一、问题定义

Supervisor 监管的游戏服进程语言不一（Go/C++/Python/Node/JVM 都可能）。崩溃排查需要内存快照，但快照的**抓取**与**解析**是两件事：抓取层可以语言无关（OS 在进程终止时天然拥有全部内存），解析层必然语言相关（符号表、运行时对象布局）。调研按此两层展开。

## 二、语言原生机制

### 2.1 Go

| 机制               | 事实                                                                                                                                                    | 来源                                                                                                            |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| runtime/pprof heap | 预定义 `heap`/`allocs` profile，`WriteTo` 输出 pprof 快照；heap profile 反映「最近一次完成的 GC」时点；`net/http/pprof` 可挂端点实时拉取                | [pkg.go.dev/runtime/pprof](https://pkg.go.dev/runtime/pprof)                                                    |
| 采样开销           | `runtime.MemProfileRate` 默认 512×1024（平均每 512KiB 分配采一个样本），置 1 全采、0 关闭；`runtime.MemProfile` 数据滞后至多两个 GC 周期——常驻开销低    | [pkg.go.dev/runtime](https://pkg.go.dev/runtime)                                                                |
| GOTRACEBACK=crash  | 五档 none/single(默认)/all/system/crash/wer；crash 档「以 OS 特定方式崩溃」——Unix 上 raise SIGABRT 触发 core dump                                       | [pkg.go.dev/runtime](https://pkg.go.dev/runtime)                                                                |
| SIGQUIT            | 源码 sigtable 登记 `_SigNotify+_SigThrow`，中继给全部 m 后 crash/exit——即全 goroutine 栈转储后退出（栈快照≠内存快照）                                   | [signal_unix.go](https://github.com/golang/go/blob/master/src/runtime/signal_unix.go)                           |
| core dump 调试链   | 官方 Wiki：`GOTRACEBACK=crash` + Ctrl+\ 触发 core；`gcore <pid>` 不杀进程取 core；`dlv core ./app core` 检查；`ulimit -c` 默认 0 需放开；端到端仅 Linux | [CoreDumpDebugging wiki](https://go.dev/wiki/CoreDumpDebugging) · [diagnostics](https://go.dev/doc/diagnostics) |
| Delve attach       | `dlv attach <pid>` 可对运行中进程调试（远程取堆信息的手段，但 attach 有暂停与权限成本）                                                                 | [dlv_attach](https://github.com/go-delve/delve/blob/master/Documentation/usage/dlv_attach.md)                   |

### 2.2 Python

| 机制         | 事实                                                                                                                                                                                  | 来源                                                                |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- |
| faulthandler | 崩溃/超时/信号时转储**仅 traceback**（ASCII、限 100 帧 100 线程、无源码、不含内存内容）；C 实现，可挂 SIGSEGV/FPE/ABRT/BUS/ILL，sigaltstack 兼容栈溢出，与 Apport 类系统 handler 兼容 | [faulthandler](https://docs.python.org/3/library/faulthandler.html) |
| tracemalloc  | 运行时分配追踪，`take_snapshot()`/`dump()` 取 Python 分配块快照（非崩溃专用）；追踪本身有内存/CPU 开销，可 `get_tracemalloc_memory()` 度量                                            | [tracemalloc](https://docs.python.org/3/library/tracemalloc.html)   |
| gdb py-bt    | 未抓到官方页面，未证实                                                                                                                                                                | —                                                                   |

### 2.3 Node.js

| 机制                  | 事实                                                                                                                                                                                                                         | 来源                                                                                                     |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| --heapsnapshot-signal | v12+，信号触发「write a heap dump」（`Heap.*.heapsnapshot`），默认关闭；写完后进程是否存活文档未说明（未证实）                                                                                                               | [nodejs cli](https://nodejs.org/api/cli.html)                                                            |
| Diagnostic Report     | JSON 报告：JS/原生栈、V8 堆统计、libuv 句柄、**环境变量**；`--report-on-signal`（默认 SIGUSR2，Windows 不支持）、`--report-on-fatalerror`（OOM 等致命错误）；生成会打断 JS 与事件循环；`--report-exclude-env` 可剔除环境变量 | [nodejs report](https://nodejs.org/api/report.html)                                                      |
| CDP HeapProfiler      | inspector 协议 `takeHeapSnapshot`；采样式堆剖析 `startSampling` 默认泊松分布、平均 32768 字节/样本、栈深 128                                                                                                                 | [CDP js_protocol](https://github.com/ChromeDevTools/devtools-protocol/blob/master/json/js_protocol.json) |

### 2.4 JVM

| 机制                       | 事实                                                                                                                         | 来源                                                                          |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| jcmd GC.heap_dump          | 生成 HPROF 全堆转储，官方 Impact 标注「High — depends on the Java heap size and content」，默认请求 full GC（`-all` 可避开） | [jcmd](https://docs.oracle.com/en/java/javase/21/docs/specs/man/jcmd.html)    |
| HeapDumpOnOutOfMemoryError | `-XX:+HeapDumpOnOutOfMemoryError` + `-XX:HeapDumpPath`，OOM 时自动落 `java_pid<pid>.hprof`                                   | [java(1)](https://docs.oracle.com/en/java/javase/21/docs/specs/man/java.html) |
| jmap -dump                 | 可用但官方标注「experimental and unsupported」                                                                               | [jmap](https://docs.oracle.com/en/java/javase/21/docs/specs/man/jmap.html)    |
| HeapDumpOnCrash            | java(1) 手册可见部分未出现，未证实                                                                                           | —                                                                             |

## 三、OS 级通用抓取层（语言无关）

### 3.1 Linux：core dump / core_pattern 管道 / systemd-coredump

- core dump 即「termination 时进程内存的镜像」，天然语言无关；`RLIMIT_CORE` 限大小，但 **core_pattern 管道模式不受其约束**；`/proc/sys/kernel/core_pattern` 首字符为 `|` 时 dump 经 stdin 交给用户态程序（以 root 在初始 namespace 运行，Linux 2.6.19+）——[core(5)](https://man7.org/linux/man-pages/man5/core.5.html)。
- systemd-coredump 经 `/usr/lib/sysctl.d/50-coredump.conf` 接管 core_pattern：journal 记录摘要 + backtrace，core 镜像存 `/var/lib/systemd/coredump/`（.zst 压缩、数日后自动删除），`ProcessSizeMax`/`ExternalSizeMax` 64 位默认 **32G**（journal 内限 767M）；`coredumpctl list/debug` 直接交 gdb——[systemd-coredump(8)](https://manpages.ubuntu.com/manpages/noble/en/man8/systemd-coredump.8.html) · [coredump.conf(5)](https://manpages.ubuntu.com/manpages/noble/en/man5/coredump.conf.5.html)。
- 该路由**对任意语言进程零集成通吃**；Breakpad 自身提供 `core_handler`（接 core_pattern 管道生成全系统 minidump）作为「管道→minidump」先例——[linux_core_handler](https://github.com/google/breakpad/blob/main/docs/linux_core_handler.md)。

### 3.2 Windows：WER LocalDumps

- 注册表 `HKLM\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps`（全局 + 每应用子键覆盖）：`DumpType` 1=Mini（默认）/2=Full/0=Custom（`CustomDumpFlags`），`DumpFolder` 默认 `%LOCALAPPDATA%\CrashDumps`，`DumpCount` 默认 10；**默认未启用**，需管理员写注册表；原文限定「应用自行做自定义崩溃上报的不被此特性支持」——[Microsoft Learn](https://learn.microsoft.com/en-us/windows/win32/wer/collecting-user-mode-dumps)。

### 3.3 macOS

崩溃报告经 Xcode Crashes organizer / 设备转移收集，jetsam（高内存杀）为独立报告类型——[Apple 文档](https://developer.apple.com/documentation/xcode/diagnosing-issues-using-crash-reports-and-device-logs)。`~/Library/Logs/DiagnosticReports` 路径与 `diagnose` 工具未能从可抓取的官方页面证实（标未证实）。服务器部署以 Linux 为主，macOS 列为非目标。

### 3.4 Crashpad / Breakpad（应用级跨平台采集器）

- **Crashpad**：handler 为独立进程，client 库编译期链入可执行模块；崩溃时进程被挂起，handler 生成快照（模块/寄存器/栈内存/OS 状态）写 minidump；设计动机「崩溃进程状态不可信，应在其中执行尽可能少的代码」。平台状态 status.md：macOS/Windows/Linux/Android/Fuchsia 客户端 complete，iOS 开发中——[README](https://chromium.googlesource.com/crashpad/crashpad/+/main/README.md) · [overview_design](https://chromium.googlesource.com/crashpad/crashpad/+/main/doc/overview_design.md) · [status](https://chromium.googlesource.com/crashpad/crashpad/+/main/doc/status.md)。**语言无关性有边界**：快照内容是 OS 级实体（与实现语言无关），但 client 是编译期 C++ 集成（sentry-native 亦要求 C++17 并随程序分发 crashpad_handler）——[sentry-native](https://github.com/getsentry/sentry-native)。
- **Breakpad**：官方未见 deprecated 声明（仓库 2026-10 仍有提交）；Crashpad 是其在 Chromium 的继任者。Linux 默认在崩溃进程内装信号处理器写 dump，官方文档自警「崩溃进程内写 minidump 不安全」——[getting_started](https://chromium.googlesource.com/breakpad/breakpad/+/master/docs/getting_started_with_breakpad.md)。

## 四、解析层（语言相关）

- minidump 是 Windows 用户态格式，「崩溃 dump 的有用子集、小到可经网络发送」；client 只写「uninterpreted byte streams」（无函数名/行号）。符号链：`dump_syms`（读 DWARF/PDB，须在 strip 前产出）→ `.sym` → `minidump_stackwalk` 符号化——[minidump-files](https://learn.microsoft.com/en-us/windows/win32/debug/minidump-files) · [getting_started](https://chromium.googlesource.com/breakpad/breakpad/+/master/docs/getting_started_with_breakpad.md) · [Chromium 解码指南](https://www.chromium.org/developers/decoding-crash-dumps/)。
- 现代替代：Mozilla rust-minidump（minidump-stackwalk/breakpad-symbols），README 自述「heavy modeled after Google Breakpad」——[rust-minidump](https://github.com/rust-minidump/rust-minidump)。
- **Go**：编译器默认产 DWARF（[cmd/compile](https://pkg.go.dev/cmd/compile)），理论上可喂 dump_syms；但 **Breakpad/Crashpad 官方 Go 客户端/支持文档未找到——成熟先例未证实**。
- **Java/Python**：minidump 只存原始栈字节+寄存器，语言级栈要靠语言侧工具（`jstack` 输出全类名/方法名/行号）——[jstack](https://docs.oracle.com/en/java/javase/21/docs/specs/man/jstack.html)。OS dump 对托管语言的价值主要是寄存器/原生帧与内存内容，语言栈仍需原生工具双轨。

## 五、对比表

| 方案                 | 覆盖语言                      | 平台                    | 接入成本                       | 开销                             | 产物                                | 解析工具                         |
| -------------------- | ----------------------------- | ----------------------- | ------------------------------ | -------------------------------- | ----------------------------------- | -------------------------------- |
| 语言原生（pprof 等） | 单语言                        | 随运行时                | 低（spawn 注入 env/flags）     | 低（采样式）到高（jcmd full GC） | pprof/hprof/heapsnapshot            | 各语言工具链                     |
| core_pattern 管道    | 任意（零集成）                | Linux                   | 低（handler 脚本/程序）        | 全量内存快照，体积大             | core 或 minidump（经 core_handler） | gdb / minidump 工具链            |
| systemd-coredump     | 任意                          | Linux（systemd）        | 极低（系统默认）               | 32G 量级预留、压缩落盘           | core（.zst）                        | coredumpctl → gdb                |
| WER LocalDumps       | 任意用户态进程                | Windows                 | 极低（注册表）                 | per-app 可配 dump 类型           | minidump/full                       | WinDbg                           |
| Crashpad/Breakpad    | OS 级通用；client 需 C++ 嵌入 | Win/macOS/Linux/Android | 高（源码/构建 + 分发 handler） | 挂起→快照，低侵入                | minidump(+扩展)                     | minidump_stackwalk/rust-minidump |

## 六、分级方案建议

**前提事实**：croupier agent 的被监管进程由 agent spawn（`ManagedProcesses`：command/args/env），**agent 控制进程的启动环境**——这是语言原生方案在 croupier 场景可行的关键杠杆（一般场景做不到）。

| 档位 | 方案                                                                                                                                                                                                                                                                                                           | 适用                                                             |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| 轻   | **spawn 注入语言原生开关**（按进程配置的 snapshot 档位）：Go→`GOTRACEBACK=crash`+pprof 端点；Node→`--heapsnapshot-signal`/`--report-on-fatalerror`（配 `--report-exclude-env`）；Python→`PYTHONFAULTHANDLER=1`（栈）+tracemalloc 按需；JVM→`-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=<agent 管理目录>` | 默认档：开销低、产物小、agent 面板可直接下载                     |
| 中   | **OS 层接管兜底**：Linux 侧写 core_pattern 管道 handler（或接受宿主机 systemd-coredump 现状），Windows 侧 WER LocalDumps per-app 注册；agent 扫描 dump 产物目录列表面板+下载                                                                                                                                   | 任意语言崩溃的兜底真值；体积大、敏感度高，默认关/白名单          |
| 重   | Crashpad/Breakpad C++ 嵌入 + minidump 符号链                                                                                                                                                                                                                                                                   | 仅当有 C++ 游戏服且需要跨平台统一 minidump；当前无此诉求，不立项 |

**推荐组合**：轻（spawn 注入）为默认 + 中（Linux core_pattern 兜底）为可选开关；不引入 Crashpad。依赖事实：spawn 控制权（代码事实）、core.5 管道语义零集成通吃、原生机制开销数据（§2）、Crashpad 集成成本与 Go 支持未证实（§3.4/§4）。

## 七、敏感数据与生产纪律

- **full dump/core/heapsnapshot 含进程内存明文**（凭据、密钥、PII 都在内存里）；Node Diagnostic Report 默认内嵌**环境变量**（常含密钥），须显式 `--report-exclude-env`（[nodejs report](https://nodejs.org/api/report.html)）；systemd-coredump 官方亦提醒含全部内存属敏感数据。
- 生产纪律建议：快照默认关、按进程白名单开；落盘目录 ACL 收紧 + 保留期上限（systemd-coredump 的自动过期可参照）；面板下载走 ops 权限位并登记审计；产物体积配额（超限丢弃并记事件，防磁盘打满）。

## 八、可拍板项

1. **分级起步档位**：推荐「轻=spawn 注入（默认）+中=core_pattern 兜底（可选）」，否决 Crashpad/Breakpad 立项（C++ 集成成本 vs 当前无 C++ 服诉求；Go 支持无成熟先例）。**已批复「按调研分级方案」执行**（agent-supervisor 设计简档落档时，2026-10-09）。
2. **轻档的配置形态**：`ManagedProcesses[x].snapshotProfile: none|go|node|python|jvm`（显式声明，不做语言自动探测——误判的代价是开关注入无效或污染进程环境）。**已拍板（2026-10-10 用户授权代拍）：同意，随 #67 S3 批次落地**。
3. **中档的宿主机前提**：core_pattern 是宿主机全局资源（root sysctl），多游戏服混布时改它影响面超出 agent。**已拍板（2026-10-10 用户授权代拍）：agent 只读提示**——扫描宿主机现状（是否 core_pattern 管道模式）+ 在面板/日志显示建议命令供运维一键复制，agent 不直接写宿主配置（权限/安全边界，教学与生产都低危）。
4. **保留期/配额/下载权限**：建议保留 72h、单进程目录配额 2GiB、下载要求 `ops:operate` + 审计；数字可调。**仍待拍板**（未在本次授权范围内）。

## 九、来源索引

Go：[runtime](https://pkg.go.dev/runtime) · [runtime/pprof](https://pkg.go.dev/runtime/pprof) · [diagnostics](https://go.dev/doc/diagnostics) · [CoreDumpDebugging](https://go.dev/wiki/CoreDumpDebugging) · [signal_unix.go](https://github.com/golang/go/blob/master/src/runtime/signal_unix.go) · [dlv attach](https://github.com/go-delve/delve/blob/master/Documentation/usage/dlv_attach.md) · [cmd/compile](https://pkg.go.dev/cmd/compile)；Python：[faulthandler](https://docs.python.org/3/library/faulthandler.html) · [tracemalloc](https://docs.python.org/3/library/tracemalloc.html)；Node：[cli](https://nodejs.org/api/cli.html) · [report](https://nodejs.org/api/report.html) · [CDP 协议](https://github.com/ChromeDevTools/devtools-protocol/blob/master/json/js_protocol.json)；JVM：[jcmd](https://docs.oracle.com/en/java/javase/21/docs/specs/man/jcmd.html) · [java(1)](https://docs.oracle.com/en/java/javase/21/docs/specs/man/java.html) · [jmap](https://docs.oracle.com/en/java/javase/21/docs/specs/man/jmap.html) · [jstack](https://docs.oracle.com/en/java/javase/21/docs/specs/man/jstack.html)；平台：[core(5)](https://man7.org/linux/man-pages/man5/core.5.html) · [systemd-coredump(8)](https://manpages.ubuntu.com/manpages/noble/en/man8/systemd-coredump.8.html) · [coredump.conf(5)](https://manpages.ubuntu.com/manpages/noble/en/man5/coredump.conf.5.html) · [WER LocalDumps](https://learn.microsoft.com/en-us/windows/win32/wer/collecting-user-mode-dumps) · [minidump-files](https://learn.microsoft.com/en-us/windows/win32/debug/minidump-files) · [Apple crash reports](https://developer.apple.com/documentation/xcode/diagnosing-issues-using-crash-reports-and-device-logs)；采集器：[Crashpad README](https://chromium.googlesource.com/crashpad/crashpad/+/main/README.md) · [design](https://chromium.googlesource.com/crashpad/crashpad/+/main/doc/overview_design.md) · [status](https://chromium.googlesource.com/crashpad/crashpad/+/main/doc/status.md) · [Breakpad getting_started](https://chromium.googlesource.com/breakpad/breakpad/+/master/docs/getting_started_with_breakpad.md) · [linux_core_handler](https://github.com/google/breakpad/blob/main/docs/linux_core_handler.md) · [rust-minidump](https://github.com/rust-minidump/rust-minidump) · [sentry-native](https://github.com/getsentry/sentry-native) · [Chromium 解码](https://www.chromium.org/developers/decoding-crash-dumps/)。未证实项：gdb py-bt 官方文档、`--heapsnapshot-signal` 写后进程存活性、JVM HeapDumpOnCrash、macOS DiagnosticReports 路径、Breakpad 官方 Go 支持、Valve/游戏工作室具体方案。
