/// 审批待办列表页（设计稿 §2.1）：状态过滤 + 下拉刷新 + 加载更多 +
/// 高危/两人复核标签 + scope 切换联动重查。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'approval_service.dart';
import 'approvals_controller.dart';
import 'approval_detail_page.dart';

class ApprovalsPage extends ConsumerStatefulWidget {
  const ApprovalsPage({super.key});

  @override
  ConsumerState<ApprovalsPage> createState() => _ApprovalsPageState();
}

class _ApprovalsPageState extends ConsumerState<ApprovalsPage> {
  @override
  void initState() {
    super.initState();
    Future.microtask(() {
      ref.read(approvalsControllerProvider.notifier).refresh();
      ref.read(approvalsControllerProvider.notifier).loadDescriptors();
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(approvalsControllerProvider);
    return Scaffold(
      appBar: AppBar(
        title: const Text('审批'),
        actions: [
          SegmentedButton<String>(
            segments: const [
              ButtonSegment(value: 'pending', label: Text('待批')),
              ButtonSegment(value: 'approved', label: Text('已批')),
              ButtonSegment(value: 'rejected', label: Text('已拒')),
            ],
            selected: {state.status},
            onSelectionChanged: (selection) => ref
                .read(approvalsControllerProvider.notifier)
                .switchStatus(selection.first),
          ),
          const SizedBox(width: 8),
        ],
      ),
      body: _buildBody(context, state),
    );
  }

  Widget _buildBody(BuildContext context, ApprovalsState state) {
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
              onPressed: () =>
                  ref.read(approvalsControllerProvider.notifier).refresh(),
              child: const Text('重试'),
            ),
          ],
        ),
      );
    }
    if (state.items.isEmpty) {
      return const Center(child: Text('暂无审批'));
    }
    return RefreshIndicator(
      onRefresh: () => ref.read(approvalsControllerProvider.notifier).refresh(),
      child: ListView.builder(
        itemCount: state.items.length + (state.hasMore ? 1 : 0),
        itemBuilder: (context, index) {
          if (index >= state.items.length) {
            return Padding(
              padding: const EdgeInsets.all(16),
              child: OutlinedButton(
                key: const ValueKey('approvals-load-more'),
                onPressed: state.loadingMore
                    ? null
                    : () => ref
                          .read(approvalsControllerProvider.notifier)
                          .loadMore(),
                child: state.loadingMore
                    ? const SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Text('加载更多'),
              ),
            );
          }
          final item = state.items[index];
          final desc = state.descs[item.functionId];
          return ApprovalListTile(
            key: ValueKey('approval-${item.id}'),
            item: item,
            desc: desc,
          );
        },
      ),
    );
  }
}

class ApprovalListTile extends StatelessWidget {
  const ApprovalListTile({required this.item, this.desc, super.key});

  final ApprovalItem item;
  final FunctionDescInfo? desc;

  @override
  Widget build(BuildContext context) {
    final risk = desc?.risk.toLowerCase() ?? '';
    final showTwoPerson = desc?.approvalRequired ?? false;
    return ListTile(
      title: Text(item.functionId, overflow: TextOverflow.ellipsis),
      subtitle: Text('${item.actor} · ${item.mode} · ${item.createdAt}'),
      trailing: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          if (risk == 'high')
            const _Tag(label: '高危', color: Colors.red)
          else if (showTwoPerson)
            const _Tag(label: '两人复核', color: Colors.orange),
        ],
      ),
      onTap: () {
        Navigator.of(context).push(
          MaterialPageRoute<void>(
            builder: (_) => ApprovalDetailPage(approvalId: item.id),
          ),
        );
      },
    );
  }
}

class _Tag extends StatelessWidget {
  const _Tag({required this.label, required this.color});

  final String label;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(label, style: TextStyle(color: color, fontSize: 12)),
    );
  }
}
