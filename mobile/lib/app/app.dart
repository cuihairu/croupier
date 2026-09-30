/// App 根装配：冷启动会话探测 → 登录页 / 占位首页
/// （底部 Tab router 壳在下一片替换 HomeStub）。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/storage/session_store.dart';
import '../features/login/login_page.dart';
import '../features/scope/scope_switcher.dart';
import 'providers.dart';

class CroupierApp extends ConsumerWidget {
  const CroupierApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sessionAsync = ref.watch(sessionFutureProvider);
    return MaterialApp(
      title: 'Croupier',
      theme: ThemeData(colorSchemeSeed: Colors.indigo, useMaterial3: true),
      home: sessionAsync.when(
        loading: () =>
            const Scaffold(body: Center(child: CircularProgressIndicator())),
        error: (Object error, StackTrace stack) => Scaffold(
          body: Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Text('会话读取失败'),
                const SizedBox(height: 8),
                Text('$error'),
                const SizedBox(height: 16),
                FilledButton(
                  onPressed: () => ref.invalidate(sessionFutureProvider),
                  child: const Text('重试'),
                ),
              ],
            ),
          ),
        ),
        data: (SessionData? session) =>
            session == null ? const LoginPage() : HomeStub(session: session),
      ),
    );
  }
}

/// 已登录占位首页：验证会话 + scope 链路用（底部 Tab router 壳在下一片替换）。
class HomeStub extends ConsumerWidget {
  const HomeStub({required this.session, super.key});

  final SessionData session;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final username = session.user['username'];
    return Scaffold(
      appBar: AppBar(
        title: Text(
          username is String && username.isNotEmpty ? username : '已登录',
        ),
        actions: [
          const ScopeSwitcher(),
          IconButton(
            icon: const Icon(Icons.logout),
            tooltip: '退出登录',
            onPressed: () async {
              await ref.read(sessionStoreProvider).clear();
              ref.invalidate(sessionFutureProvider);
            },
          ),
        ],
      ),
      body: Center(
        child: Text(
          session.hasCompleteScope
              ? '${session.gameId} / ${session.env}'
              : '未选择 scope',
        ),
      ),
    );
  }
}
