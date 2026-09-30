/// 监控大盘页（设计稿 §2.2）：性能快照三块卡片（运行时/宿主机/过载描红）
/// + 全部只读。LB 统计（PromQL 代理）与设备/告警/审计入口按切片交付占位。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../devices/devices_page.dart';
import 'monitoring_controller.dart';
import 'monitoring_format.dart';
import 'performance_service.dart';

class MonitoringPage extends ConsumerStatefulWidget {
  const MonitoringPage({super.key});

  @override
  ConsumerState<MonitoringPage> createState() => _MonitoringPageState();
}

class _MonitoringPageState extends ConsumerState<MonitoringPage> {
  @override
  void initState() {
    super.initState();
    Future.microtask(
      () => ref.read(monitoringControllerProvider.notifier).refresh(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(monitoringControllerProvider);
    final snapshot = state.snapshot;
    if (state.loading && snapshot == null) {
      return const Center(child: CircularProgressIndicator());
    }
    if (state.error != null && snapshot == null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(state.error!),
            const SizedBox(height: 12),
            FilledButton(
              key: const ValueKey('monitor-retry'),
              onPressed: () =>
                  ref.read(monitoringControllerProvider.notifier).refresh(),
              child: const Text('重试'),
            ),
          ],
        ),
      );
    }
    if (snapshot == null) {
      return const SizedBox.shrink();
    }
    return RefreshIndicator(
      onRefresh: () =>
          ref.read(monitoringControllerProvider.notifier).refresh(),
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          _RuntimeCard(runtime: snapshot.runtime),
          const SizedBox(height: 12),
          _HostCard(host: snapshot.host, overload: snapshot.overload),
          const SizedBox(height: 12),
          const _EntriesCard(),
        ],
      ),
    );
  }
}

class _RuntimeCard extends StatelessWidget {
  const _RuntimeCard({required this.runtime});

  final PerformanceRuntime runtime;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('服务运行时', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            _metric('Goroutines', runtime.goroutines.toString()),
            _metric('堆内存', formatBytes(runtime.heapAllocBytes)),
            _metric('GC 次数', runtime.numGC.toString()),
            _metric('GC 累计暂停', '${formatPercent(runtime.gcPauseMs)}ms'),
            _metric('在线时长', formatUptime(runtime.uptimeSeconds)),
          ],
        ),
      ),
    );
  }
}

class _HostCard extends StatelessWidget {
  const _HostCard({required this.host, required this.overload});

  final PerformanceHost host;
  final PerformanceOverload overload;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('宿主机资源', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            _metric(
              'CPU',
              '${formatPercent(host.cpuPercent)}%',
              overloaded: overload.cpu,
            ),
            _metric(
              '内存',
              '${formatPercent(host.memoryUsedPct)}%（${formatBytes(host.memoryUsedBytes)} / ${formatBytes(host.memoryTotalBytes)}）',
              overloaded: overload.memory,
            ),
            _metric(
              '磁盘',
              '${formatPercent(host.diskUsedPct)}%（${formatBytes(host.diskUsedBytes)} / ${formatBytes(host.diskTotalBytes)}）',
              overloaded: overload.disk,
            ),
          ],
        ),
      ),
    );
  }
}

/// 过载行描红 +「过载」徽标（overload.*=true，设计稿 §2.2）。
Widget _metric(String label, String value, {bool overloaded = false}) {
  final valueStyle = overloaded
      ? const TextStyle(color: Colors.red, fontWeight: FontWeight.bold)
      : null;
  return Padding(
    padding: const EdgeInsets.symmetric(vertical: 2),
    child: Row(
      children: [
        SizedBox(
          width: 96,
          child: Text(label, style: const TextStyle(color: Colors.grey)),
        ),
        Expanded(child: Text(value, style: valueStyle)),
        if (overloaded)
          Container(
            key: ValueKey('overload-$label'),
            padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
            decoration: BoxDecoration(
              color: Colors.red.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(4),
            ),
            child: const Text(
              '过载',
              style: TextStyle(color: Colors.red, fontSize: 12),
            ),
          ),
      ],
    ),
  );
}

/// 二级页面入口：设备（已交付）/ 告警/审计随 M2 各切片交付启用；
/// LB 统计为 PromQL 代理（依赖服务端 Prometheus），不接入移动端。
class _EntriesCard extends StatelessWidget {
  const _EntriesCard();

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Column(
        children: [
          ListTile(
            key: const ValueKey('monitor-entry-devices'),
            leading: const Icon(Icons.dns_outlined),
            title: const Text('设备（Agent）'),
            subtitle: const Text('在线状态 / 资源 / 已注册函数'),
            enabled: true,
            onTap: () => Navigator.of(context).push(
              MaterialPageRoute<void>(
                builder: (context) => const DevicesPage(),
              ),
            ),
          ),
          const ListTile(
            key: ValueKey('monitor-entry-alerts'),
            leading: Icon(Icons.notifications_outlined),
            title: Text('告警'),
            subtitle: Text('M2 交付'),
            enabled: false,
          ),
          const ListTile(
            key: ValueKey('monitor-entry-audit'),
            leading: Icon(Icons.receipt_long_outlined),
            title: Text('审计查询'),
            subtitle: Text('M2 交付'),
            enabled: false,
          ),
          const ListTile(
            key: ValueKey('monitor-entry-lbstats'),
            leading: Icon(Icons.device_hub_outlined),
            title: Text('LB 统计'),
            subtitle: Text('依赖服务端 Prometheus（PromQL 代理），移动端不接入'),
            enabled: false,
          ),
        ],
      ),
    );
  }
}
