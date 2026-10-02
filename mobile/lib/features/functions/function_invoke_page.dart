/// 函数调用页（设计稿 §2.5）：描述符驱动动态表单 + 同步/异步 invoke +
/// 异步任务跟进（5s 轮询 / 取消）。
library;

import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/function/function_spec.dart';
import 'function_invoke_service.dart';
import 'functions_controller.dart';
import 'schema_form.dart';
import 'schema_form_model.dart';

class FunctionInvokePage extends ConsumerStatefulWidget {
  const FunctionInvokePage({required this.spec, super.key});

  final FunctionSpec spec;

  @override
  ConsumerState<FunctionInvokePage> createState() => _FunctionInvokePageState();
}

class _FunctionInvokePageState extends ConsumerState<FunctionInvokePage>
    with WidgetsBindingObserver {
  late final SchemaFormSpec _formSpec = SchemaFormSpec.fromSchema(
    widget.spec.inputSchema,
  );
  late final SchemaFormController _form = SchemaFormController(_formSpec);

  // 高级路由区。
  String _route = '';
  final _targetController = TextEditingController();
  final _hashKeyController = TextEditingController();

  // 异步任务开关：execution=task 的函数默认开。
  late bool _async = widget.spec.defaultsToTask;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void dispose() {
    _form.dispose();
    WidgetsBinding.instance.removeObserver(this);
    _targetController.dispose();
    _hashKeyController.dispose();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // 页面可见性：resumed 恢复任务轮询，后台停止（零请求契约）。
    ref
        .read(functionsControllerProvider.notifier)
        .onVisibility(state == AppLifecycleState.resumed);
  }

  Future<void> _submit() async {
    FocusScope.of(context).unfocus();
    if (_formSpec.rawEditor) {
      final text = _form.rawJson?.trim() ?? '';
      try {
        final decoded = text.isEmpty ? <String, Object?>{} : jsonDecode(text);
        if (decoded is! Map) {
          _snack('请求参数须为 JSON 对象');
          return;
        }
        await _doInvoke(Map<String, Object?>.from(decoded));
      } on FormatException {
        _snack('JSON 解析失败，请检查格式');
      }
      return;
    }
    final err = _form.validate(_formSpec);
    if (err != null) {
      _snack('参数校验失败：$err');
      return;
    }
    await _doInvoke(_form.payload(_formSpec));
  }

  Future<void> _doInvoke(Map<String, Object?> payload) async {
    // 只读复杂参数：以原默认值合并（用户无法编辑）。
    for (final field in _formSpec.readOnly) {
      if (field.defaultValue != null) payload[field.name] = field.defaultValue;
    }
    await ref
        .read(functionsControllerProvider.notifier)
        .invoke(
          functionId: widget.spec.id,
          payload: payload,
          mode: _async ? 'async' : 'sync',
          route: _route,
          targetServiceId: _targetController.text.trim(),
          hashKey: _hashKeyController.text.trim(),
        );
    if (!mounted) return;
    final err = ref.read(functionsControllerProvider).invokeError;
    if (err != null) _snack(err);
  }

  void _snack(String message) {
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(functionsControllerProvider);
    final spec = widget.spec;
    return Scaffold(
      appBar: AppBar(title: Text(spec.id)),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          _Header(spec: spec),
          const SizedBox(height: 12),
          if (spec.approvalRequired)
            _banner(
              key: 'approval-banner',
              color: Colors.orange,
              text: '该函数需审批：提交后等待第二审批人，去审批中心跟进',
            ),
          if (!spec.executable)
            _banner(
              key: 'unbound-banner',
              color: Colors.grey,
              text: '函数未绑定运行时，调用将返回 executor_unbound',
            ),
          Text('参数', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 8),
          SchemaForm(spec: _formSpec, controller: _form),
          const SizedBox(height: 16),
          _Advanced(
            route: _route,
            onRoute: (v) => setState(() => _route = v),
            targetController: _targetController,
            hashKeyController: _hashKeyController,
          ),
          const SizedBox(height: 8),
          SwitchListTile(
            key: const ValueKey('async-switch'),
            contentPadding: EdgeInsets.zero,
            title: const Text('异步任务'),
            subtitle: const Text('长耗时调用走任务生命周期，可轮询进度 / 取消'),
            value: _async,
            onChanged: (v) => setState(() => _async = v),
          ),
          const SizedBox(height: 8),
          FilledButton(
            key: const ValueKey('invoke-submit'),
            onPressed: state.submitting ? null : _submit,
            child: state.submitting
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Text('调用'),
          ),
          const SizedBox(height: 16),
          _ResultSection(
            state: state,
            controller: ref.read(functionsControllerProvider.notifier),
          ),
        ],
      ),
    );
  }

  Widget _banner({
    required String key,
    required Color color,
    required String text,
  }) {
    return Container(
      key: ValueKey(key),
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.14),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(text, style: TextStyle(color: color, fontSize: 13)),
    );
  }
}

class _Header extends StatelessWidget {
  const _Header({required this.spec});

  final FunctionSpec spec;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (spec.displayDescription.isNotEmpty) Text(spec.displayDescription),
        const SizedBox(height: 4),
        Text(
          '${spec.execution.isEmpty ? 'sync' : spec.execution} · risk=${spec.risk.isEmpty ? 'unknown' : spec.risk} · ${spec.permission.isEmpty ? '无权限声明' : spec.permission}',
          style: const TextStyle(fontSize: 12, color: Colors.grey),
        ),
      ],
    );
  }
}

class _Advanced extends StatelessWidget {
  const _Advanced({
    required this.route,
    required this.onRoute,
    required this.targetController,
    required this.hashKeyController,
  });

  final String route;
  final ValueChanged<String> onRoute;
  final TextEditingController targetController;
  final TextEditingController hashKeyController;

  @override
  Widget build(BuildContext context) {
    return ExpansionTile(
      key: const ValueKey('advanced'),
      title: const Text('高级路由（默认 lb，通常无需设置）'),
      childrenPadding: const EdgeInsets.all(8),
      children: [
        DropdownButtonFormField<String>(
          key: const ValueKey('route-select'),
          initialValue: route.isEmpty ? null : route,
          decoration: const InputDecoration(
            labelText: '路由',
            border: OutlineInputBorder(),
            isDense: true,
          ),
          items: const [
            DropdownMenuItem(value: 'lb', child: Text('lb（默认负载均衡）')),
            DropdownMenuItem(value: 'targeted', child: Text('targeted（指定实例）')),
            DropdownMenuItem(value: 'hash', child: Text('hash（一致性哈希）')),
            DropdownMenuItem(value: 'broadcast', child: Text('broadcast（广播）')),
          ],
          onChanged: (v) => onRoute(v ?? ''),
        ),
        const SizedBox(height: 8),
        TextField(
          key: const ValueKey('target-service'),
          controller: targetController,
          decoration: const InputDecoration(
            labelText: 'targetServiceId（route=targeted 必填）',
            border: OutlineInputBorder(),
            isDense: true,
          ),
        ),
        const SizedBox(height: 8),
        TextField(
          key: const ValueKey('hash-key'),
          controller: hashKeyController,
          decoration: const InputDecoration(
            labelText: 'hashKey（route=hash 必填）',
            border: OutlineInputBorder(),
            isDense: true,
          ),
        ),
      ],
    );
  }
}

class _ResultSection extends StatelessWidget {
  const _ResultSection({required this.state, required this.controller});

  final FunctionsState state;
  final FunctionsController controller;

  @override
  Widget build(BuildContext context) {
    final task = state.task;
    if (task != null) {
      return _TaskCard(task: task, controller: controller);
    }
    final result = state.invokeResult;
    if (result == null) return const SizedBox.shrink();
    if (result.approvalRequired) {
      return Card(
        key: const ValueKey('approval-submitted'),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Text(
            '已提交审批${result.approvalId.isEmpty ? '' : '（${result.approvalId}）'}，等待第二审批人',
          ),
        ),
      );
    }
    final text = result.result == null
        ? '（无返回数据）'
        : const JsonEncoder.withIndent('  ').convert(result.result);
    return Card(
      key: const ValueKey('sync-result'),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('执行结果', style: TextStyle(fontWeight: FontWeight.bold)),
            const SizedBox(height: 8),
            SelectableText(text),
          ],
        ),
      ),
    );
  }
}

class _TaskCard extends StatelessWidget {
  const _TaskCard({required this.task, required this.controller});

  final TaskStatus task;
  final FunctionsController controller;

  @override
  Widget build(BuildContext context) {
    final color = task.isSuccess
        ? Colors.green
        : task.isFailed
        ? Colors.red
        : task.isCanceled
        ? Colors.grey
        : Colors.blue;
    return Card(
      key: const ValueKey('task-card'),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text('异步任务', style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(width: 8),
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 6,
                    vertical: 2,
                  ),
                  decoration: BoxDecoration(
                    color: color.withValues(alpha: 0.15),
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: Text(
                    task.status,
                    style: TextStyle(color: color, fontSize: 12),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            LinearProgressIndicator(
              value: task.isDone ? 1 : (task.progress / 100).clamp(0, 1),
            ),
            const SizedBox(height: 4),
            Text('任务 ${task.id} · 进度 ${task.progress}%'),
            if (task.message.isNotEmpty) Text(task.message),
            if (task.error.isNotEmpty)
              Text(task.error, style: const TextStyle(color: Colors.red)),
            if (task.isSuccess && task.result != null) ...[
              const SizedBox(height: 8),
              const Text('执行结果', style: TextStyle(fontWeight: FontWeight.bold)),
              SelectableText(
                const JsonEncoder.withIndent('  ').convert(task.result),
              ),
            ],
            const SizedBox(height: 8),
            Row(
              children: [
                if (task.isRunning)
                  OutlinedButton(
                    key: const ValueKey('task-cancel'),
                    onPressed: controller.cancelTask,
                    child: const Text('取消任务'),
                  ),
                const SizedBox(width: 8),
                TextButton(
                  key: const ValueKey('task-refresh'),
                  onPressed: controller.refreshTask,
                  child: const Text('刷新状态'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
