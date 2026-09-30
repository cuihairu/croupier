/// 登录页（设计稿 §2 / §3.2）：双步表单，TOTP 输入框按需出现。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'login_controller.dart';

class LoginPage extends ConsumerStatefulWidget {
  const LoginPage({super.key});

  @override
  ConsumerState<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends ConsumerState<LoginPage> {
  // Android 模拟器访问宿主机回环地址的约定别名；用户可改。
  final _server = TextEditingController(text: 'http://10.0.2.2:18780');
  final _username = TextEditingController();
  final _password = TextEditingController();
  final _totp = TextEditingController();

  @override
  void dispose() {
    _server.dispose();
    _username.dispose();
    _password.dispose();
    _totp.dispose();
    super.dispose();
  }

  void _submit() {
    ref
        .read(loginControllerProvider.notifier)
        .submit(
          serverUrl: _server.text.trim(),
          username: _username.text.trim(),
          password: _password.text,
          totpCode: _totp.text.trim().isEmpty ? null : _totp.text.trim(),
        );
  }

  @override
  Widget build(BuildContext context) {
    ref.listen<LoginUiState>(loginControllerProvider, (prev, next) {
      if (next is LoginBlocked) {
        showDialog<void>(
          context: context,
          builder: (dialogContext) => AlertDialog(
            title: const Text('无法在移动端完成登录'),
            content: Text(next.message),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(dialogContext).pop(),
                child: const Text('知道了'),
              ),
            ],
          ),
        );
      }
    });
    final uiState = ref.watch(loginControllerProvider);
    final busy = uiState is LoginSubmitting;
    final needTotp = uiState is LoginNeedTotp;
    return Scaffold(
      appBar: AppBar(title: const Text('Croupier GM 登录')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          TextField(
            key: const ValueKey('login-server'),
            controller: _server,
            keyboardType: TextInputType.url,
            decoration: const InputDecoration(
              labelText: '服务器地址',
              hintText: 'https://gm.example.com',
            ),
          ),
          const SizedBox(height: 12),
          TextField(
            key: const ValueKey('login-username'),
            controller: _username,
            autofillHints: const [AutofillHints.username],
            decoration: const InputDecoration(labelText: '用户名'),
          ),
          const SizedBox(height: 12),
          TextField(
            key: const ValueKey('login-password'),
            controller: _password,
            obscureText: true,
            autofillHints: const [AutofillHints.password],
            decoration: const InputDecoration(labelText: '密码'),
          ),
          if (needTotp) ...[
            const SizedBox(height: 12),
            TextField(
              key: const ValueKey('login-totp'),
              controller: _totp,
              keyboardType: TextInputType.number,
              decoration: InputDecoration(
                labelText: '动态验证码（TOTP）',
                helperText: uiState.message,
              ),
            ),
          ],
          if (uiState is LoginFailure) ...[
            const SizedBox(height: 12),
            Text(
              uiState.message,
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          ],
          const SizedBox(height: 24),
          FilledButton(
            key: const ValueKey('login-submit'),
            onPressed: busy ? null : _submit,
            child: busy
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Text('登录'),
          ),
        ],
      ),
    );
  }
}
