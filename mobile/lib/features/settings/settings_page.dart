/// 设置页（M1 最小集 + 插单）：会话信息展示 + 服务器地址更换
/// （格式校验 + 连通探测通过才放行；生效 = 新地址落盘 → 清会话回登录页，
/// API 客户端工厂按新地址重建，登录页预填新地址）。
/// ntfy 推送 / 生物门禁开关为 M2/M3 交付（设计稿 §2.3/§6）。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import '../../core/server/server_probe.dart';
import '../../core/server/server_url.dart';

class SettingsPage extends ConsumerWidget {
  const SettingsPage({super.key});

  Future<void> _changeServerUrl(BuildContext context, WidgetRef ref) async {
    final session = await ref.read(sessionStoreProvider).load();
    if (!context.mounted) return;
    final controller = TextEditingController(text: session?.serverUrl ?? '');
    final newUrl = await showDialog<String>(
      context: context,
      builder: (dialogContext) {
        // busy/error 持有在 StatefulBuilder.builder 之外的闭包里——
        // 放 builder 内会随每次 rebuild 重置。
        var busy = false;
        String? error;
        return StatefulBuilder(
          builder: (dialogContext, setDialogState) {
            Future<void> submit() async {
              if (busy) return;
              final validation = validateServerUrl(controller.text);
              if (!validation.isValid) {
                setDialogState(() => error = validation.error);
                return;
              }
              setDialogState(() {
                busy = true;
                error = null;
              });
              try {
                await probeServer(
                  ref.read(apiClientFactoryProvider)(validation.url!),
                );
              } on ApiError catch (e) {
                setDialogState(() {
                  busy = false;
                  error = e.message;
                });
                return;
              }
              if (!dialogContext.mounted) return;
              Navigator.of(dialogContext).pop(validation.url);
            }

            return AlertDialog(
              title: const Text('更换服务器地址'),
              content: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  TextField(
                    key: const ValueKey('settings-server-input'),
                    controller: controller,
                    keyboardType: TextInputType.url,
                    enabled: !busy,
                    decoration: const InputDecoration(
                      labelText: '服务器地址',
                      hintText: 'https://gm.example.com',
                    ),
                  ),
                  if (error != null) ...[
                    const SizedBox(height: 8),
                    Align(
                      alignment: Alignment.centerLeft,
                      child: Text(
                        key: const ValueKey('settings-server-error'),
                        error!,
                        style: TextStyle(
                          color: Theme.of(dialogContext).colorScheme.error,
                        ),
                      ),
                    ),
                  ],
                ],
              ),
              actions: [
                TextButton(
                  onPressed: busy
                      ? null
                      : () => Navigator.of(dialogContext).pop(),
                  child: const Text('取消'),
                ),
                FilledButton(
                  key: const ValueKey('settings-server-save'),
                  onPressed: busy ? null : submit,
                  child: busy
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Text('保存并重新登录'),
                ),
              ],
            );
          },
        );
      },
    );
    if (newUrl == null || newUrl.trim().isEmpty) return;
    // 即时生效：新地址先落盘（探测已通过），再清会话回登录页；
    // 清会话保留地址（配置与凭据分层），API 客户端工厂按新地址重建。
    final store = ref.read(sessionStoreProvider);
    await store.saveServerUrl(newUrl.trim());
    await store.clear();
    ref.read(pendingServerUrlProvider.notifier).state = newUrl.trim();
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
