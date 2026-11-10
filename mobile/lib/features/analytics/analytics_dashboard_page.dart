/// 分析大盘页（MOB-ANA-001）：facade 一次拉全部域摘要卡片。三态齐全
/// （加载/错误重试/空），卡片按 status 分色——ok 渲染指标、unavailable
/// 描错、pending_data_source 显示「数据源待建」占位（诚实边界，不伪造数字）。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'analytics_controller.dart';
import 'analytics_domain_page.dart';
import 'analytics_service.dart';

/// 域 → 中文标签（与 docs/design/game-analytics-requirements.md §3 对齐）。
const Map<String, String> kAnalyticsDomainLabels = {
  'overview': '宏观大盘',
  'behavior': '玩家行为',
  'payments': '商业化',
  'retention': '留存',
  'realtime': '实时监控',
  'invocations': '函数调用',
  'levels': '游戏进度',
  'warehouse': '数仓 DAU',
  'economy': '经济系统',
  'social': '社交',
  'stability': '稳定性',
  'anticheat': '反作弊',
  'campaign': '运营活动',
};

class AnalyticsDashboardPage extends ConsumerStatefulWidget {
  const AnalyticsDashboardPage({super.key});

  @override
  ConsumerState<AnalyticsDashboardPage> createState() =>
      _AnalyticsDashboardPageState();
}

class _AnalyticsDashboardPageState
    extends ConsumerState<AnalyticsDashboardPage> {
  @override
  void initState() {
    super.initState();
    Future.microtask(
      () => ref.read(analyticsControllerProvider.notifier).refresh(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(analyticsControllerProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('数据分析')),
      body: _buildBody(state),
    );
  }

  Widget _buildBody(AnalyticsState state) {
    final summary = state.summary;
    if (state.loading && summary == null) {
      return const Center(child: CircularProgressIndicator());
    }
    if (state.error != null && summary == null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 24),
              child: Text(state.error!),
            ),
            const SizedBox(height: 12),
            FilledButton(
              key: const ValueKey('analytics-retry'),
              onPressed: () =>
                  ref.read(analyticsControllerProvider.notifier).refresh(),
              child: const Text('重试'),
            ),
          ],
        ),
      );
    }
    if (summary == null) {
      return const SizedBox.shrink();
    }
    if (summary.domains.isEmpty) {
      return const Center(
        key: ValueKey('analytics-empty'),
        child: Text('暂无数据'),
      );
    }
    return RefreshIndicator(
      onRefresh: () =>
          ref.read(analyticsControllerProvider.notifier).refresh(),
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Text(
            '${summary.gameId} · ${summary.env} · ${summary.generatedAt}',
            style: const TextStyle(color: Colors.grey, fontSize: 12),
          ),
          const SizedBox(height: 12),
          for (final card in summary.domains) ...[
            _DomainCardTile(
              key: ValueKey('analytics-card-${card.domain}'),
              card: card,
              onTap: () => _openDomain(card),
            ),
            const SizedBox(height: 12),
          ],
        ],
      ),
    );
  }

  void _openDomain(DomainCardData card) {
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (context) => AnalyticsDomainPage(
          domain: card.domain,
          label: kAnalyticsDomainLabels[card.domain] ?? card.domain,
          pending: card.isPending,
        ),
      ),
    );
  }
}

class _DomainCardTile extends StatelessWidget {
  const _DomainCardTile({super.key, required this.card, this.onTap});

  final DomainCardData card;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: ListTile(
        onTap: onTap,
        title: Row(
          children: [
            Expanded(
              child: Text(
                kAnalyticsDomainLabels[card.domain] ?? card.domain,
                style: Theme.of(context).textTheme.titleMedium,
              ),
            ),
            _statusChip(),
          ],
        ),
        subtitle: _subtitle(),
      ),
    );
  }

  Widget _statusChip() {
    Color color;
    String label;
    switch (card.status) {
      case DomainCardData.statusOk:
        color = Colors.green;
        label = '正常';
      case DomainCardData.statusPending:
        color = Colors.orange;
        label = '待建';
      default:
        color = Colors.red;
        label = '不可用';
    }
    return Container(
      key: ValueKey('analytics-status-${card.domain}'),
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        label,
        style: TextStyle(color: color, fontSize: 12),
      ),
    );
  }

  Widget? _subtitle() {
    if (card.isPending) {
      return const Text('数据源待建：后端尚未接入该域，无合成数据');
    }
    if (card.isUnavailable) {
      return Text(
        card.error.isEmpty ? '查询失败' : card.error,
        style: const TextStyle(color: Colors.red),
      );
    }
    final lines = summarizeDomainData(card.data);
    if (lines.isEmpty) return null;
    return Text(lines.map((e) => '${e.key} ${e.value}').join(' · '));
  }
}

/// ok 卡片的指标摘要行：从动态 data 中提取前 4 个数值键值
/// （metrics / realtimeMetrics 子对象优先），列表形态报条目数。
List<MapEntry<String, String>> summarizeDomainData(Object? data) {
  if (data is Map) {
    final source = (data['metrics'] is Map)
        ? data['metrics'] as Map
        : (data['realtimeMetrics'] is Map)
            ? data['realtimeMetrics'] as Map
            : data;
    final lines = <MapEntry<String, String>>[];
    source.forEach((key, value) {
      if (lines.length >= 4) return;
      if (value is int) {
        lines.add(MapEntry(key.toString(), '$value'));
      } else if (value is double) {
        lines.add(MapEntry(key.toString(), _num(value)));
      }
    });
    return lines;
  }
  if (data is List) {
    return [MapEntry('条目数', '${data.length}')];
  }
  return const [];
}

String _num(double v) {
  if (v == v.roundToDouble()) return v.toInt().toString();
  return v.toStringAsFixed(2);
}
