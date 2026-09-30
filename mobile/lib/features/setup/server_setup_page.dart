/// 服务器地址设置向导（首次启动引导，设计稿插单 2026-09-30）：
/// 未配置过服务器地址（无会话存储块）时进入；输入域名/URL →
/// 格式校验 + 连通探测（GET /api/v1/public/site）→ 保存后进登录页。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import '../../core/server/server_probe.dart';
import '../../core/server/server_url.dart';

class ServerSetupPage extends ConsumerStatefulWidget {
  const ServerSetupPage({super.key});

  @override
  ConsumerState<ServerSetupPage> createState() => _ServerSetupPageState();
}

class _ServerSetupPageState extends ConsumerState<ServerSetupPage> {
  final _server = TextEditingController();
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _server.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_busy) return;
    final validation = validateServerUrl(_server.text);
    if (!validation.isValid) {
      setState(() => _error = validation.error);
      return;
    }
    final url = validation.url!;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await probeServer(ref.read(apiClientFactoryProvider)(url));
      // 地址落盘（块形态 token=''）→ invalidate 后 App 根按「已配置未登录」
      // 分支切登录页并预填本地址。
      await ref.read(sessionStoreProvider).saveServerUrl(url);
      ref.invalidate(sessionFutureProvider);
    } on ApiError catch (e) {
      setState(() {
        _busy = false;
        _error = e.message;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('设置服务器地址')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Text(
            '首次使用请填写 Croupier 服务器地址，保存后将进入登录。',
            style: Theme.of(context).textTheme.bodyMedium,
          ),
          const SizedBox(height: 16),
          TextField(
            key: const ValueKey('setup-server'),
            controller: _server,
            keyboardType: TextInputType.url,
            enabled: !_busy,
            decoration: const InputDecoration(
              labelText: '服务器地址',
              hintText: 'https://gm.example.com',
            ),
          ),
          if (_error != null) ...[
            const SizedBox(height: 12),
            Text(
              key: const ValueKey('setup-error'),
              _error!,
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          ],
          const SizedBox(height: 24),
          FilledButton(
            key: const ValueKey('setup-submit'),
            onPressed: _busy ? null : _submit,
            child: _busy
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Text('测试连接并继续'),
          ),
        ],
      ),
    );
  }
}
