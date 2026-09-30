/// 设置页（M1 最小集）：会话信息展示 + 服务器地址更换
/// （更换 = 清会话回登录页，新地址预填登录表单）。
/// ntfy 推送 / 生物门禁开关为 M2/M3 交付（设计稿 §2.3/§6）。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';

class SettingsPage extends ConsumerWidget {
  const SettingsPage({super.key});

  Future<void> _changeServerUrl(BuildContext context, WidgetRef ref) async {
    final session = await ref.read(sessionStoreProvider).load();
    if (!context.mounted) return;
    final controller = TextEditingController(text: session?.serverUrl ?? '');
    final newUrl = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('更换服务器地址'),
        content: TextField(
          key: const ValueKey('settings-server-input'),
          controller: controller,
          keyboardType: TextInputType.url,
          decoration: const InputDecoration(
            labelText: '服务器地址',
            hintText: 'https://gm.example.com',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: const Text('取消'),
          ),
          FilledButton(
            key: const ValueKey('settings-server-save'),
            onPressed: () {
              final text = controller.text.trim();
              if (text.isEmpty) return;
              Navigator.of(dialogContext).pop(text);
            },
            child: const Text('保存并重新登录'),
          ),
        ],
      ),
    );
    if (newUrl == null || newUrl.trim().isEmpty) return;
    // 换服务器 = 全新会话：清会话回登录页，新地址预填登录表单。
    ref.read(pendingServerUrlProvider.notifier).state = newUrl.trim();
    await ref.read(sessionStoreProvider).clear();
    if (!context.mounted) return;
    ref.invalidate(sessionFutureProvider);
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sessionAsync = ref.watch(sessionFutureProvider);
    final session = sessionAsync.value;
    final username = session?.user['username'];
    final scope = session?.hasCompleteScope ?? false
        ? '${session?.gameId} / ${session?.env}'
        : '未选择';
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        ListTile(
          leading: const Icon(Icons.person_outline),
          title: const Text('当前用户'),
          subtitle: Text(
            username is String && username.isNotEmpty ? username : '-',
          ),
        ),
        ListTile(
          leading: const Icon(Icons.category_outlined),
          title: const Text('当前 scope'),
          subtitle: Text(scope),
        ),
        ListTile(
          key: const ValueKey('settings-server'),
          leading: const Icon(Icons.dns_outlined),
          title: const Text('服务器地址'),
          subtitle: Text(session?.serverUrl ?? '-'),
          trailing: TextButton(
            key: const ValueKey('settings-change-server'),
            onPressed: () => _changeServerUrl(context, ref),
            child: const Text('更换'),
          ),
        ),
        const Divider(height: 32),
        const ListTile(
          leading: Icon(Icons.notifications_outlined),
          title: Text('推送通知（ntfy）'),
          subtitle: Text('M3 交付（自托管 ntfy 订阅，设计稿 §6）'),
          enabled: false,
        ),
        const ListTile(
          leading: Icon(Icons.fingerprint),
          title: Text('生物门禁'),
          subtitle: Text('M2 交付（local_auth，设计稿 §4.1）'),
          enabled: false,
        ),
      ],
    );
  }
}
