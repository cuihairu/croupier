/// 告警列表页（设计稿 §2.2）：level/status 筛选芯片 + 分页 +
/// 静默（duration 分钟 + reason 必填）+ 静默规则只读弹层。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'alert_service.dart';
import 'alerts_controller.dart';

class AlertsPage extends ConsumerStatefulWidget {
  const AlertsPage({super.key});

  @override
  ConsumerState<AlertsPage> createState() => _AlertsPageState();
}

class _AlertsPageState extends ConsumerState<AlertsPage> {
  static const _levelOptions = ['', 'critical', 'warning', 'info'];
  static const _statusOptions = ['', 'firing', 'resolved'];

  @override
  void initState() {
    super.initState();
    Future.microtask(
      () => ref.read(alertsControllerProvider.notifier).refresh(),
    );
  }

  Future<void> _openSilenceDialog(AlertItem alert) async {
    final durationController = TextEditingController(text: '60');
    final reasonController = TextEditingController();
    final confirmed = await showDialog<({int duration, String reason})>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('静默告警'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              key: const ValueKey('silence-duration'),
              controller: durationController,
              keyboardType: TextInputType.number,
              decoration: const InputDecoration(
                labelText: '静默时长（分钟）',
                hintText: '60',
              ),
            ),
            const SizedBox(height: 12),
            TextField(
              key: const ValueKey('silence-reason'),
              controller: reasonController,
              maxLines: 2,
              autofocus: true,
              decoration: const InputDecoration(labelText: '静默原因（必填）'),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: const Text('取消'),
          ),
          FilledButton(
            key: const ValueKey('silence-confirm'),
            onPressed: () {
              final duration = int.tryParse(durationController.text.trim());
              final reason = reasonController.text.trim();
              if (duration == null || duration <= 0 || reason.isEmpty) return;
              Navigator.of(
                dialogContext,
              ).pop((duration: duration, reason: reason));
            },
            child: const Text('确认静默'),
          ),
        ],
      ),
    );
    if (confirmed == null) return;
    try {
      await ref
          .read(alertsControllerProvider.notifier)
          .silence(
            alert.id,
            durationMinutes: confirmed.duration,
            reason: confirmed.reason,
          );
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('已静默')));
    } on FormatException {
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('静默时长需为正整数')));
    }
  }

  Future<void> _openSilencesSheet() async {
    final notifier = ref.read(alertsControllerProvider.notifier);
    await notifier.loadSilences();
    if (!mounted) return;
    await showModalBottomSheet<void>(
      context: context,
      builder: (sheetContext) => Consumer(
        builder: (context, ref, _) {
          final state = ref.watch(alertsControllerProvider);
          return SafeArea(
            child: ListView(
              padding: const EdgeInsets.all(16),
              shrinkWrap: true,
              children: [
                Text(
                  '静默规则',
                  style: Theme.of(sheetContext).textTheme.titleMedium,
                ),
                const SizedBox(height: 8),
                if (state.silencesLoading)
                  const Padding(
                    padding: EdgeInsets.all(24),
                    child: Center(child: CircularProgressIndicator()),
                  )
                else if (state.silencesError != null)
                  Padding(
                    padding: const EdgeInsets.all(16),
                    child: Text(state.silencesError!),
                  )
                else if (state.silences.isEmpty)
                  const Padding(
                    padding: EdgeInsets.all(16),
                    child: Text('暂无静默规则'),
                  )
                else
                  ...state.silences.map(
                    (rule) => ListTile(
                      key: ValueKey('silence-rule-${rule.id}'),
                      leading: const Icon(Icons.notifications_paused_outlined),
                      title: Text(
                        rule.alertType.isNotEmpty ? rule.alertType : rule.id,
                      ),
                      subtitle: Text('${rule.startAt} ~ ${rule.endAt}'),
                      trailing: rule.createdBy.isNotEmpty
                          ? Text(rule.createdBy)
                          : null,
                    ),
                  ),
              ],
            ),
          );
        },
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(alertsControllerProvider);
    return Scaffold(
      appBar: AppBar(
        title: const Text('告警'),
        actions: [
          IconButton(
            key: const ValueKey('alerts-silences-open'),
            tooltip: '静默规则',
            icon: const Icon(Icons.notifications_paused_outlined),
            onPressed: _openSilencesSheet,
          ),
        ],
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
            child: Align(
              alignment: Alignment.centerLeft,
              child: Wrap(
                spacing: 8,
                runSpacing: 4,
                children: [
                  for (final level in _levelOptions)
                    ChoiceChip(
                      key: ValueKey(
                        'alerts-level-${level.isEmpty ? 'all' : level}',
                      ),
                      label: Text(level.isEmpty ? '全部级别' : level),
                      selected: state.level == level,
                      onSelected: (_) => ref
                          .read(alertsControllerProvider.notifier)
                          .switchLevel(level),
                    ),
                  for (final status in _statusOptions)
                    ChoiceChip(
                      key: ValueKey(
                        'alerts-status-${status.isEmpty ? 'all' : status}',
                      ),
                      label: Text(status.isEmpty ? '全部状态' : status),
                      selected: state.status == status,
                      onSelected: (_) => ref
                          .read(alertsControllerProvider.notifier)
                          .switchStatus(status),
                    ),
                ],
              ),
            ),
          ),
          Expanded(child: _list(context, state)),
        ],
      ),
    );
  }

  Widget _list(BuildContext context, AlertsState state) {
    if (state.loading && state.items.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }
    if (state.error != null && state.items.isEmpty) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(state.error!),
            const SizedBox(height: 12),
            FilledButton(
              key: const ValueKey('alerts-retry'),
              onPressed: () =>
                  ref.read(alertsControllerProvider.notifier).refresh(),
              child: const Text('重试'),
            ),
          ],
        ),
      );
    }
    if (state.items.isEmpty) {
      return const Center(child: Text('暂无告警'));
    }
    return RefreshIndicator(
      onRefresh: () => ref.read(alertsControllerProvider.notifier).refresh(),
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          ...state.items.map(
            (alert) =>
                _AlertCard(alert, onSilence: () => _openSilenceDialog(alert)),
          ),
          if (state.hasMore)
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: OutlinedButton(
                key: const ValueKey('alerts-load-more'),
                onPressed: state.loadingMore
                    ? null
                    : () => ref
                          .read(alertsControllerProvider.notifier)
                          .loadMore(),
                child: Text(state.loadingMore ? '加载中…' : '加载更多'),
              ),
            ),
        ],
      ),
    );
  }
}

/// level 色阶：critical 红 / warning 橙 / info 蓝 / 未知灰。
Color _levelColor(String level) => switch (level) {
  'critical' => Colors.red,
  'warning' => Colors.orange,
  'info' => Colors.blue,
  _ => Colors.grey,
};

class _AlertCard extends StatelessWidget {
  const _AlertCard(this.alert, {required this.onSilence});

  final AlertItem alert;
  final VoidCallback onSilence;

  @override
  Widget build(BuildContext context) {
    return Card(
      key: ValueKey('alert-card-${alert.id}'),
      child: ListTile(
        onLongPress: onSilence,
        title: Text(
          alert.message.isNotEmpty ? alert.message : alert.id,
          maxLines: 2,
          overflow: TextOverflow.ellipsis,
        ),
        subtitle: Text(
          '${alert.type.isEmpty ? '-' : alert.type}'
          ' · ${alert.source.isEmpty ? '-' : alert.source}'
          '${alert.createdAt.isEmpty ? '' : ' · ${alert.createdAt}'}',
        ),
        isThreeLine: true,
        trailing: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
              decoration: BoxDecoration(
                color: _levelColor(alert.level).withValues(alpha: 0.12),
                borderRadius: BorderRadius.circular(6),
              ),
              child: Text(
                alert.level.isEmpty ? '-' : alert.level,
                style: TextStyle(color: _levelColor(alert.level), fontSize: 12),
              ),
            ),
            const SizedBox(height: 4),
            SizedBox(
              height: 28,
              child: TextButton(
                key: ValueKey('alert-silence-${alert.id}'),
                style: TextButton.styleFrom(
                  padding: const EdgeInsets.symmetric(horizontal: 8),
                  textStyle: const TextStyle(fontSize: 12),
                ),
                onPressed: onSilence,
                child: const Text('静默'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
