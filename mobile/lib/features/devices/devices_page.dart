/// 设备列表页（设计稿 §2.3）：整表 + 状态徽标四档 + label 芯片过滤 +
/// 下拉刷新 + 前台 60s 定时刷新（离线推送未落地前的发现手段，§6.5）。
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'devices_controller.dart';
import 'node_detail_page.dart';
import 'node_service.dart';

class DevicesPage extends ConsumerStatefulWidget {
  const DevicesPage({super.key});

  @override
  ConsumerState<DevicesPage> createState() => _DevicesPageState();
}

class _DevicesPageState extends ConsumerState<DevicesPage>
    with WidgetsBindingObserver {
  Timer? _ticker;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    // 前台 60s 周期刷新；后台时 tick 空转不请求。
    _ticker = Timer.periodic(const Duration(seconds: 60), (_) {
      if (WidgetsBinding.instance.lifecycleState == AppLifecycleState.resumed) {
        ref.read(devicesControllerProvider.notifier).refresh();
      }
    });
    Future.microtask(
      () => ref.read(devicesControllerProvider.notifier).refresh(),
    );
  }

  @override
  void dispose() {
    _ticker?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // 回前台立即刷新一次（不等下一个 tick）。
    if (state == AppLifecycleState.resumed) {
      ref.read(devicesControllerProvider.notifier).refresh();
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(devicesControllerProvider);
    final nodes = state.filteredNodes;
    return Scaffold(
      appBar: AppBar(title: const Text('设备（Agent）')),
      body: RefreshIndicator(
        onRefresh: () => ref.read(devicesControllerProvider.notifier).refresh(),
        child: state.nodes.isEmpty && state.error != null
            ? ListView(
                children: [
                  Padding(
                    padding: const EdgeInsets.all(32),
                    child: Column(
                      children: [
                        Text(state.error!, textAlign: TextAlign.center),
                        const SizedBox(height: 12),
                        FilledButton(
                          key: const ValueKey('devices-retry'),
                          onPressed: () => ref
                              .read(devicesControllerProvider.notifier)
                              .refresh(),
                          child: const Text('重试'),
                        ),
                      ],
                    ),
                  ),
                ],
              )
            : ListView(
                padding: const EdgeInsets.all(16),
                children: [
                  if (state.allLabels.isNotEmpty) _labelChips(state),
                  if (state.nodes.isEmpty)
                    const Padding(
                      padding: EdgeInsets.all(32),
                      child: Center(child: Text('暂无设备')),
                    )
                  else
                    ...nodes.map(
                      (node) => _DeviceCard(
                        node: node,
                        onOpen: () => Navigator.of(context).push(
                          MaterialPageRoute<void>(
                            builder: (context) =>
                                NodeDetailPage(nodeId: node.id),
                          ),
                        ),
                      ),
                    ),
                  if (state.nodes.isNotEmpty && nodes.isEmpty)
                    const Padding(
                      padding: EdgeInsets.all(16),
                      child: Center(child: Text('无匹配 label 的设备')),
                    ),
                ],
              ),
      ),
    );
  }

  Widget _labelChips(DevicesState state) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Align(
        alignment: Alignment.centerLeft,
        child: Wrap(
          spacing: 8,
          runSpacing: 4,
          children: state.allLabels
              .map(
                (chip) => FilterChip(
                  key: ValueKey('label-chip-$chip'),
                  label: Text(chip),
                  selected: state.selectedLabels.contains(chip),
                  onSelected: (_) => ref
                      .read(devicesControllerProvider.notifier)
                      .toggleLabel(chip),
                ),
              )
              .toList(),
        ),
      ),
    );
  }
}

/// 状态徽标四档配色（设计稿 §2.3）：在线绿 / 排空灰 / 异常橙 / 离线红。
Color _bandColor(DeviceBand band) => switch (band) {
  DeviceBand.online => Colors.green,
  DeviceBand.drained => Colors.grey,
  DeviceBand.degraded => Colors.orange,
  DeviceBand.offline => Colors.red,
  DeviceBand.unknown => Colors.grey,
};

String _bandText(DeviceBand band) => switch (band) {
  DeviceBand.online => '在线',
  DeviceBand.drained => '排空',
  DeviceBand.degraded => '异常',
  DeviceBand.offline => '离线',
  DeviceBand.unknown => '未知',
};

class _DeviceCard extends StatelessWidget {
  const _DeviceCard({required this.node, required this.onOpen});

  final NodeDevice node;
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    final band = node.band;
    return Card(
      key: ValueKey('device-card-${node.id}'),
      child: ListTile(
        onTap: onOpen,
        title: Text(
          node.hostname.isNotEmpty ? node.hostname : node.id,
          style: const TextStyle(fontWeight: FontWeight.w600),
        ),
        subtitle: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('${node.id} · ${node.gameId}/${node.env}'),
            const SizedBox(height: 2),
            Text(
              '心跳 ${node.lastSeenRelative}'
              ' · 函数 ${node.functions}'
              '${node.displayVersion.isEmpty ? '' : ' · ${node.displayVersion}'}',
            ),
          ],
        ),
        isThreeLine: true,
        trailing: Container(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
          decoration: BoxDecoration(
            color: _bandColor(band).withValues(alpha: 0.12),
            borderRadius: BorderRadius.circular(6),
          ),
          child: Text(
            _bandText(band),
            style: TextStyle(color: _bandColor(band), fontSize: 12),
          ),
        ),
      ),
    );
  }
}
