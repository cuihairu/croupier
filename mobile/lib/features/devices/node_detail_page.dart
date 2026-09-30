/// 设备详情页（设计稿 §2.3）：基础信息卡 + 资源卡（无上报显示「暂无上报」）
/// + 函数卡（仅计数，清单引导回 Web）。历史在线状态不承诺（无落库数据）。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import '../monitoring/monitoring_format.dart';
import 'node_service.dart';

class NodeDetailPage extends ConsumerStatefulWidget {
  const NodeDetailPage({required this.nodeId, super.key});

  final String nodeId;

  @override
  ConsumerState<NodeDetailPage> createState() => _NodeDetailPageState();
}

class _NodeDetailPageState extends ConsumerState<NodeDetailPage> {
  NodeDevice? _node;
  String? _error;
  bool _loading = true;

  NodeService? _service;

  @override
  void initState() {
    super.initState();
    Future.microtask(_reload);
  }

  Future<void> _reload() async {
    final session = await ref.read(sessionStoreProvider).load();
    final serverUrl = session?.serverUrl ?? '';
    if (!mounted) return;
    if (serverUrl.isEmpty) {
      setState(() {
        _loading = false;
        _error = '未登录或缺少服务器地址';
      });
      return;
    }
    _service ??= NodeService(
      client: ref.read(apiClientFactoryProvider)(serverUrl),
    );
    try {
      final node = await _service!.detail(widget.nodeId);
      if (!mounted) return;
      setState(() {
        _node = node;
        _loading = false;
        _error = null;
      });
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = e.message;
      });
    } on StateError catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = e.message;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final node = _node;
    return Scaffold(
      appBar: AppBar(title: const Text('设备详情')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
          ? Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(_error!),
                  const SizedBox(height: 12),
                  FilledButton(
                    key: const ValueKey('node-detail-retry'),
                    onPressed: _reload,
                    child: const Text('重试'),
                  ),
                ],
              ),
            )
          : ListView(
              padding: const EdgeInsets.all(16),
              children: [
                _basicCard(node!),
                const SizedBox(height: 12),
                _resourceCard(node),
                const SizedBox(height: 12),
                _functionsCard(node),
              ],
            ),
    );
  }

  Widget _basicCard(NodeDevice node) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('基础信息', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            _row('ID', node.id),
            _row('主机名', node.hostname),
            _row('地址', node.addr),
            _row('游戏/环境', '${node.gameId} / ${node.env}'),
            _row('状态', _bandText(node.band)),
            _row('最近心跳', node.lastSeenRelative),
            if (node.expiresInSec > 0)
              _row('会话剩余', formatUptime(node.expiresInSec)),
            if (node.displayVersion.isNotEmpty) ...[
              _row('版本', node.displayVersion),
              if (node.version.isNotEmpty && node.sdkVersion.isNotEmpty)
                _row(
                  'SDK',
                  '${node.sdkName} ${node.sdkVersion}（${node.sdkLanguage}）',
                ),
            ],
            if (node.labels.isNotEmpty)
              _row(
                '标签',
                node.labels.entries.map((e) => '${e.key}=${e.value}').join('，'),
              ),
          ],
        ),
      ),
    );
  }

  Widget _resourceCard(NodeDevice node) {
    final hasReport =
        node.cpu != null || node.memory != null || node.disks.isNotEmpty;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('资源', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            if (!hasReport)
              const Text('暂无上报', style: TextStyle(color: Colors.grey))
            else ...[
              if (node.cpu != null)
                _row(
                  'CPU',
                  '${formatPercent(node.cpu!.usagePercent)}%（${node.cpu!.cores} 核）',
                ),
              if (node.memory != null)
                _row(
                  '内存',
                  '${formatPercent(node.memory!.usagePercent)}%（${formatBytes(node.memory!.usedBytes)} / ${formatBytes(node.memory!.totalBytes)}）',
                ),
              ...node.disks.map(
                (disk) => _row(
                  '磁盘 ${disk.mountPoint.isEmpty ? '-' : disk.mountPoint}',
                  '${formatPercent(disk.usagePercent)}%（${formatBytes(disk.usedBytes)} / ${formatBytes(disk.totalBytes)}）',
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _functionsCard(NodeDevice node) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('函数', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            _row('已注册', '${node.functions}'),
            const Text(
              '函数清单请在 Web 端查看',
              style: TextStyle(color: Colors.grey, fontSize: 12),
            ),
          ],
        ),
      ),
    );
  }

  String _bandText(DeviceBand band) => switch (band) {
    DeviceBand.online => '在线',
    DeviceBand.drained => '排空',
    DeviceBand.degraded => '异常',
    DeviceBand.offline => '离线',
    DeviceBand.unknown => '未知',
  };

  Widget _row(String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 110,
            child: Text(label, style: const TextStyle(color: Colors.grey)),
          ),
          Expanded(child: SelectableText(value)),
        ],
      ),
    );
  }
}
