/// 审计查询页（设计稿 §2.4，只读）：筛选区（actor/kind 芯片+自定义/env/ip/
/// 时间区间/gameId）+ 行展开 metadata JSON 折叠 + traceId + 分页。
/// 审计链 hash/prevHash 后端列表 DTO 不出参（§9 已记录），有值才显示的
/// 语义按可选字段预留。
library;

import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'audit_controller.dart';
import 'audit_service.dart' show AuditItem;

class AuditPage extends ConsumerStatefulWidget {
  const AuditPage({super.key});

  /// kind 常用闭集（对齐 Web OperationLogs 词表精选）；支持自定义追加。
  static const commonKinds = [
    'invoke',
    'start_job',
    'cancel_job',
    'approval_approve',
    'approval_reject',
    'user_create',
    'user_update',
    'user_delete',
  ];

  @override
  ConsumerState<AuditPage> createState() => _AuditPageState();
}

class _AuditPageState extends ConsumerState<AuditPage> {
  final _actor = TextEditingController();
  final _env = TextEditingController();
  final _ip = TextEditingController();
  final _gameId = TextEditingController();
  final _customKind = TextEditingController();
  DateTimeRange? _range;

  @override
  void initState() {
    super.initState();
    Future.microtask(
      () => ref.read(auditControllerProvider.notifier).refresh(),
    );
  }

  @override
  void dispose() {
    _actor.dispose();
    _env.dispose();
    _ip.dispose();
    _gameId.dispose();
    _customKind.dispose();
    super.dispose();
  }

  AuditFilters _collectFilters(Set<String> kinds) {
    final start = _range?.start;
    final endDay = _range?.end;
    final end = endDay == null
        ? null
        : DateTime(endDay.year, endDay.month, endDay.day, 23, 59, 59);
    return AuditFilters(
      actor: _actor.text.trim(),
      kinds: kinds,
      env: _env.text.trim(),
      ip: _ip.text.trim(),
      gameId: _gameId.text.trim(),
      start: start,
      end: end,
    );
  }

  Future<void> _pickRange() async {
    final now = DateTime.now();
    final picked = await showDateRangePicker(
      context: context,
      firstDate: DateTime(now.year - 2),
      lastDate: now.add(const Duration(days: 1)),
      initialDateRange:
          _range ??
          DateTimeRange(
            start: DateTime(now.year, now.month, now.day - 7),
            end: now,
          ),
    );
    if (picked != null) {
      setState(() => _range = picked);
    }
  }

  Future<void> _submitQuery(Set<String> kinds) async {
    Navigator.of(context).maybePop();
    await ref
        .read(auditControllerProvider.notifier)
        .applyFilters(_collectFilters(kinds));
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(auditControllerProvider);
    return Scaffold(
      appBar: AppBar(
        title: const Text('审计查询'),
        actions: [
          Builder(
            builder: (buttonContext) => IconButton(
              key: const ValueKey('audit-filter-open'),
              tooltip: '筛选',
              icon: const Icon(Icons.filter_list),
              onPressed: () => _openFilterSheet(buttonContext, state),
            ),
          ),
        ],
      ),
      body: _list(context, state),
    );
  }

  void _openFilterSheet(BuildContext anchorContext, AuditState state) {
    final kinds = {...state.filters.kinds};
    showModalBottomSheet<void>(
      context: anchorContext,
      isScrollControlled: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => SafeArea(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: ListView(
              shrinkWrap: true,
              children: [
                Text('筛选', style: Theme.of(sheetContext).textTheme.titleMedium),
                const SizedBox(height: 8),
                TextField(
                  key: const ValueKey('audit-filter-actor'),
                  controller: _actor,
                  decoration: const InputDecoration(
                    labelText: '操作者（actor，精确匹配）',
                  ),
                ),
                const SizedBox(height: 8),
                TextField(
                  key: const ValueKey('audit-filter-env'),
                  controller: _env,
                  decoration: const InputDecoration(labelText: '环境（env）'),
                ),
                const SizedBox(height: 8),
                TextField(
                  key: const ValueKey('audit-filter-game'),
                  controller: _gameId,
                  decoration: const InputDecoration(
                    labelText: '游戏 ID（gameId query 过滤）',
                  ),
                ),
                const SizedBox(height: 8),
                TextField(
                  key: const ValueKey('audit-filter-ip'),
                  controller: _ip,
                  decoration: const InputDecoration(labelText: 'IP'),
                ),
                const SizedBox(height: 12),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Wrap(
                    spacing: 8,
                    runSpacing: 4,
                    children: [
                      ActionChip(
                        key: const ValueKey('audit-filter-range'),
                        label: Text(
                          _range == null
                              ? '时间区间（全部）'
                              : '${_rangeDate(_range!.start)} ~ ${_rangeDate(_range!.end)}',
                        ),
                        avatar: const Icon(Icons.date_range, size: 18),
                        onPressed: () async {
                          await _pickRange();
                          setSheetState(() {});
                        },
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 12),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Wrap(
                    spacing: 8,
                    runSpacing: 4,
                    children: [
                      for (final kind in AuditPage.commonKinds)
                        FilterChip(
                          key: ValueKey('audit-kind-$kind'),
                          label: Text(kind),
                          selected: kinds.contains(kind),
                          onSelected: (selected) => setSheetState(() {
                            selected ? kinds.add(kind) : kinds.remove(kind);
                          }),
                        ),
                    ],
                  ),
                ),
                const SizedBox(height: 8),
                TextField(
                  key: const ValueKey('audit-kind-custom'),
                  controller: _customKind,
                  decoration: InputDecoration(
                    labelText: '自定义 kind（回车加入）',
                    suffixIcon: IconButton(
                      key: const ValueKey('audit-kind-add'),
                      icon: const Icon(Icons.add),
                      onPressed: () {
                        final text = _customKind.text.trim();
                        if (text.isEmpty) return;
                        setSheetState(() => kinds.add(text));
                        _customKind.clear();
                      },
                    ),
                  ),
                ),
                if (kinds.any((k) => !AuditPage.commonKinds.contains(k)))
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Align(
                      alignment: Alignment.centerLeft,
                      child: Wrap(
                        spacing: 8,
                        children: [
                          for (final kind in kinds.where(
                            (k) => !AuditPage.commonKinds.contains(k),
                          ))
                            FilterChip(
                              key: ValueKey('audit-kind-custom-$kind'),
                              label: Text(kind),
                              selected: true,
                              onSelected: (_) =>
                                  setSheetState(() => kinds.remove(kind)),
                            ),
                        ],
                      ),
                    ),
                  ),
                const SizedBox(height: 16),
                FilledButton(
                  key: const ValueKey('audit-filter-apply'),
                  onPressed: () => _submitQuery(kinds),
                  child: const Text('查询'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  String _rangeDate(DateTime t) =>
      '${t.year}-${t.month.toString().padLeft(2, '0')}-${t.day.toString().padLeft(2, '0')}';

  Widget _list(BuildContext context, AuditState state) {
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
              key: const ValueKey('audit-retry'),
              onPressed: () =>
                  ref.read(auditControllerProvider.notifier).refresh(),
              child: const Text('重试'),
            ),
          ],
        ),
      );
    }
    if (state.items.isEmpty) {
      return const Center(child: Text('暂无审计记录'));
    }
    return RefreshIndicator(
      onRefresh: () => ref.read(auditControllerProvider.notifier).refresh(),
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          if (state.total > 0)
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: Text(
                '共 ${state.total} 条',
                style: const TextStyle(color: Colors.grey),
              ),
            ),
          ...state.items.map(_AuditCard.new),
          if (state.hasMore)
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: OutlinedButton(
                key: const ValueKey('audit-load-more'),
                onPressed: state.loadingMore
                    ? null
                    : () =>
                          ref.read(auditControllerProvider.notifier).loadMore(),
                child: Text(state.loadingMore ? '加载中…' : '加载更多'),
              ),
            ),
        ],
      ),
    );
  }
}

class _AuditCard extends StatelessWidget {
  const _AuditCard(this.item);

  final AuditItem item;

  @override
  Widget build(BuildContext context) {
    final metadata = item.metadata;
    return Card(
      key: ValueKey('audit-card-${item.id}'),
      child: ExpansionTile(
        tilePadding: const EdgeInsets.symmetric(horizontal: 16),
        title: Text(
          item.action.isEmpty ? item.id : item.action,
          style: const TextStyle(fontWeight: FontWeight.w600),
        ),
        subtitle: Text(
          '${item.userId.isEmpty ? '-' : item.userId}'
          '${item.gameId.isEmpty ? '' : ' · ${item.gameId}/${item.env}'}'
          ' · ${item.createdAt}',
        ),
        trailing: item.result.isNotEmpty
            ? Text(
                item.result,
                style: TextStyle(
                  fontSize: 12,
                  color: item.result == 'success' || item.result == 'ok'
                      ? Colors.green
                      : Colors.orange,
                ),
              )
            : null,
        childrenPadding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
        children: [
          if (item.target.isNotEmpty) _row('目标', item.target),
          if (item.traceId.isNotEmpty) _row('TraceID', item.traceId),
          if (metadata != null && metadata.isNotEmpty) ...[
            const Align(
              alignment: Alignment.centerLeft,
              child: Text('metadata', style: TextStyle(color: Colors.grey)),
            ),
            const SizedBox(height: 4),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: Colors.grey.withValues(alpha: 0.08),
                borderRadius: BorderRadius.circular(6),
              ),
              child: SelectableText(
                const JsonEncoder.withIndent('  ').convert(metadata),
                style: const TextStyle(fontSize: 12),
              ),
            ),
          ],
          if (item.target.isEmpty &&
              item.traceId.isEmpty &&
              (metadata == null || metadata.isEmpty))
            const Align(
              alignment: Alignment.centerLeft,
              child: Text('无附加信息', style: TextStyle(color: Colors.grey)),
            ),
        ],
      ),
    );
  }

  Widget _row(String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 64,
            child: Text(label, style: const TextStyle(color: Colors.grey)),
          ),
          Expanded(child: SelectableText(value)),
        ],
      ),
    );
  }
}
