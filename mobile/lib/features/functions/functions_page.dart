/// 函数目录页（设计稿 §2.5）：descriptors 目录 + 搜索 + risk/审批标签，
/// 点击进入调用页。scope 切换自动重查（控制器内监听）。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/function/function_spec.dart';
import 'function_invoke_page.dart';
import 'functions_controller.dart';

class FunctionsPage extends ConsumerStatefulWidget {
  const FunctionsPage({super.key});

  @override
  ConsumerState<FunctionsPage> createState() => _FunctionsPageState();
}

class _FunctionsPageState extends ConsumerState<FunctionsPage> {
  @override
  void initState() {
    super.initState();
    Future.microtask(
      () => ref.read(functionsControllerProvider.notifier).refresh(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(functionsControllerProvider);
    final controller = ref.read(functionsControllerProvider.notifier);
    return Scaffold(
      appBar: AppBar(title: const Text('函数调用')),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
            child: TextField(
              key: const ValueKey('function-search'),
              decoration: const InputDecoration(
                prefixIcon: Icon(Icons.search),
                hintText: '搜索函数 ID / 名称',
                border: OutlineInputBorder(),
                isDense: true,
              ),
              onChanged: controller.setQuery,
            ),
          ),
          Expanded(child: _body(state, controller)),
        ],
      ),
    );
  }

  Widget _body(FunctionsState state, FunctionsController controller) {
    if (state.loading && state.specs.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }
    if (state.error != null && state.specs.isEmpty) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(state.error!),
            const SizedBox(height: 12),
            FilledButton(
              key: const ValueKey('functions-retry'),
              onPressed: controller.refresh,
              child: const Text('重试'),
            ),
          ],
        ),
      );
    }
    final items = state.filtered;
    if (items.isEmpty) {
      return RefreshIndicator(
        onRefresh: controller.refresh,
        child: ListView(
          children: const [
            SizedBox(height: 120),
            Center(child: Text('当前 scope 下无可调用函数')),
          ],
        ),
      );
    }
    return RefreshIndicator(
      onRefresh: controller.refresh,
      child: ListView.separated(
        itemCount: items.length,
        separatorBuilder: (_, _) => const Divider(height: 1),
        itemBuilder: (context, index) => _FunctionTile(spec: items[index]),
      ),
    );
  }
}

class _FunctionTile extends StatelessWidget {
  const _FunctionTile({required this.spec});

  final FunctionSpec spec;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      key: ValueKey('function-${spec.id}'),
      title: Row(
        children: [
          Expanded(child: Text(spec.id)),
          if (spec.isHighRisk) _Tag('高危', Colors.red),
          if (spec.approvalRequired) _Tag('需审批', Colors.orange),
          if (!spec.executable) _Tag('未绑定', Colors.grey),
        ],
      ),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (spec.displayDescription.isNotEmpty) Text(spec.displayDescription),
          Text(
            '${spec.execution.isEmpty ? 'sync' : spec.execution} · ${spec.resource.isEmpty ? '-' : spec.resource}',
            style: const TextStyle(fontSize: 12, color: Colors.grey),
          ),
        ],
      ),
      trailing: const Icon(Icons.chevron_right),
      onTap: () => Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (context) => FunctionInvokePage(spec: spec),
        ),
      ),
    );
  }
}

class _Tag extends StatelessWidget {
  const _Tag(this.text, this.color);

  final String text;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(left: 6),
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.14),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(text, style: TextStyle(color: color, fontSize: 11)),
    );
  }
}
