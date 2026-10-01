/// 审批详情页（设计稿 §2.1）：元数据 + payloadPreview 折叠 +
/// 两人规则只读横幅 + 批准/拒绝 + 409 竞态提示与自动刷新。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import 'approval_service.dart';

class ApprovalDetailPage extends ConsumerStatefulWidget {
  const ApprovalDetailPage({required this.approvalId, super.key});

  final String approvalId;

  @override
  ConsumerState<ApprovalDetailPage> createState() => _ApprovalDetailPageState();
}

class _ApprovalDetailPageState extends ConsumerState<ApprovalDetailPage> {
  ApprovalItem? _item;
  String? _error;
  bool _loading = true;
  bool _acting = false;

  ApprovalService? _service;

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
    _service ??= ApprovalService(
      client: ref.read(apiClientFactoryProvider)(serverUrl),
    );
    try {
      final item = await _service!.detail(widget.approvalId);
      if (!mounted) return;
      setState(() {
        _item = item;
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

  Future<void> _approve({String? otp}) async {
    if (_acting) return;
    setState(() => _acting = true);
    try {
      await _doApprove(otp: otp);
    } finally {
      if (mounted) setState(() => _acting = false);
    }
  }

  /// step-up 专属流程（不走通用 _act）：otp_required 需弹窗收集动态码后
  /// 重试，otp_invalid 重弹纠错，其余错误沿用 snackbar 文案。可重入
  ///（绕过 _acting 守卫），守卫只在入口 [_approve] 一处。
  Future<void> _doApprove({String? otp}) async {
    try {
      await _service!.approve(widget.approvalId, otp: otp);
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('操作成功')));
      await _reload();
    } on ApiError catch (e) {
      if (!mounted) return;
      if (e.status == 409) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('该审批已被处理：${e.message}')));
        await _reload();
      } else if (e.code == 'otp_required' ||
          (e.code == 'otp_invalid' && otp != null)) {
        // #75：高危审批必须带动态码；带码验证失败重弹纠错
        final code = await _promptOtp();
        if (code == null || code.trim().isEmpty) return;
        await _doApprove(otp: code.trim());
      } else {
        // 含 otp_not_enrolled：后端 message 已本地化（引导回 Web 绑定）
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(e.message)));
      }
    }
  }

  Future<String?> _promptOtp() {
    final controller = TextEditingController();
    return showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('高危审批'),
        content: TextField(
          key: const ValueKey('otp-input'),
          controller: controller,
          autofocus: true,
          keyboardType: TextInputType.number,
          maxLength: 6,
          decoration: const InputDecoration(labelText: '动态验证码（验证器 App 6 位）'),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: const Text('取消'),
          ),
          FilledButton(
            key: const ValueKey('otp-confirm'),
            onPressed: () {
              final text = controller.text.trim();
              if (text.isEmpty) return;
              Navigator.of(dialogContext).pop(text);
            },
            child: const Text('确认批准'),
          ),
        ],
      ),
    );
  }

  Future<void> _reject() async {
    final controller = TextEditingController();
    final reason = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('拒绝审批'),
        content: TextField(
          key: const ValueKey('reject-reason'),
          controller: controller,
          autofocus: true,
          maxLines: 3,
          decoration: const InputDecoration(labelText: '拒绝原因（必填）'),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: const Text('取消'),
          ),
          FilledButton(
            key: const ValueKey('reject-confirm'),
            onPressed: () {
              final text = controller.text.trim();
              if (text.isEmpty) return;
              Navigator.of(dialogContext).pop(text);
            },
            child: const Text('确认拒绝'),
          ),
        ],
      ),
    );
    if (reason == null || reason.trim().isEmpty) return;
    await _act(
      () => _service!.reject(widget.approvalId, reason: reason.trim()),
    );
  }

  Future<void> _act(Future<void> Function() action) async {
    if (_acting) return;
    setState(() => _acting = true);
    try {
      await action();
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('操作成功')));
      await _reload();
    } on ApiError catch (e) {
      if (!mounted) return;
      if (e.status == 409) {
        // 竞态（已被他人处理）：冲突提示 + 自动刷新详情（§2.1）。
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('该审批已被处理：${e.message}')));
        await _reload();
      } else {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(e.message)));
      }
    } finally {
      if (mounted) setState(() => _acting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final sessionAsync = ref.watch(sessionFutureProvider);
    final username = sessionAsync.value?.user['username'];
    final currentUsername = username is String ? username : '';
    final item = _item;
    final twoPersonRule =
        item != null && item.actor.isNotEmpty && item.actor == currentUsername;
    final pending = item?.state == 'pending';
    return Scaffold(
      appBar: AppBar(title: const Text('审批详情')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
          ? Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(_error!),
                  const SizedBox(height: 12),
                  FilledButton(onPressed: _reload, child: const Text('重试')),
                ],
              ),
            )
          : ListView(
              padding: const EdgeInsets.all(16),
              children: [
                if (twoPersonRule)
                  Container(
                    key: const ValueKey('two-person-banner'),
                    padding: const EdgeInsets.all(12),
                    margin: const EdgeInsets.only(bottom: 12),
                    color: Colors.orange.withValues(alpha: 0.15),
                    child: const Text('两人规则：申请人不能审批自己的申请'),
                  ),
                _row('函数', item!.functionId),
                _row('发起人', item.actor),
                if (item.approver.isNotEmpty) _row('审批人', item.approver),
                _row('状态', item.state),
                _row('模式', item.mode),
                _row('游戏/环境', '${item.gameId} / ${item.env}'),
                _row('创建时间', item.createdAt),
                if (item.payloadPreview.isNotEmpty) ...[
                  const SizedBox(height: 12),
                  ExpansionTile(
                    key: const ValueKey('payload-preview'),
                    title: const Text('请求参数（payloadPreview）'),
                    children: [
                      Padding(
                        padding: const EdgeInsets.all(12),
                        child: SelectableText(item.payloadPreview),
                      ),
                    ],
                  ),
                ],
                const SizedBox(height: 24),
                if (pending && !twoPersonRule)
                  Row(
                    children: [
                      Expanded(
                        child: FilledButton(
                          key: const ValueKey('approve-btn'),
                          onPressed: _acting ? null : _approve,
                          child: const Text('批准'),
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: OutlinedButton(
                          key: const ValueKey('reject-btn'),
                          onPressed: _acting ? null : _reject,
                          child: const Text('拒绝'),
                        ),
                      ),
                    ],
                  ),
              ],
            ),
    );
  }

  Widget _row(String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 88,
            child: Text(label, style: const TextStyle(color: Colors.grey)),
          ),
          Expanded(child: SelectableText(value)),
        ],
      ),
    );
  }
}
