/// App 根装配：冷启动三态分流——
/// 无存储块（从未配置地址）→ 设置向导；已配置未登录（token 空）→ 登录页
/// （预填已存地址）；已登录 → 主壳（底部 Tab）。
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/storage/session_store.dart';
import '../features/approvals/approvals_page.dart';
import '../features/login/login_page.dart';
import '../features/monitoring/monitoring_page.dart';
import '../features/scope/scope_switcher.dart';
import '../features/settings/settings_page.dart';
import '../features/setup/server_setup_page.dart';
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
        data: (SessionData? session) {
          if (session == null) return const ServerSetupPage();
          if (session.token.isEmpty) {
            return LoginPage(initialServerUrl: session.serverUrl);
          }
          return MainShell(session: session);
        },
      ),
    );
  }
}

/// 已登录主壳：底部 Tab（§2.1 审批为默认着陆 Tab；
/// 监控大盘 M2 起真页面，设备/告警/审计由大盘入口进入）。
class MainShell extends ConsumerStatefulWidget {
  const MainShell({required this.session, super.key});

  final SessionData session;

  @override
  ConsumerState<MainShell> createState() => _MainShellState();
}

class _MainShellState extends ConsumerState<MainShell> {
  int _tab = 0;

  @override
  Widget build(BuildContext context) {
    final username = widget.session.user['username'];
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
      body: IndexedStack(
        index: _tab,
        children: const [ApprovalsPage(), MonitoringPage(), SettingsPage()],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _tab,
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.fact_check_outlined),
            selectedIcon: Icon(Icons.fact_check),
            label: '审批',
          ),
          NavigationDestination(
            icon: Icon(Icons.insights_outlined),
            label: '监控',
          ),
          NavigationDestination(
            icon: Icon(Icons.settings_outlined),
            label: '设置',
          ),
        ],
        onDestinationSelected: (index) => setState(() => _tab = index),
      ),
    );
  }
}
