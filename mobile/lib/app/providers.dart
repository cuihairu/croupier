/// 全局 Provider 装配（M1）。
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api/api_client.dart';
import '../core/auth/login_service.dart';
import '../core/storage/session_store.dart';

/// 会话存储：生产用 secure storage；测试 override 为内存实现。
final sessionStoreProvider = Provider<SessionStore>(
  (ref) => SecureSessionStore(),
);

/// 冷启动会话探测：null = 未登录（进登录页）。
/// 登录成功 / 登出 / 401 时 invalidate 重建，App 根据此切页。
final sessionFutureProvider = FutureProvider<SessionData?>(
  (ref) => ref.watch(sessionStoreProvider).load(),
);

typedef LoginServiceFactory = LoginService Function(String baseUrl);

/// 按 serverUrl 现场装配登录服务——服务器地址登录前由用户输入，
/// 故用工厂而非常量 provider。
final loginServiceFactoryProvider = Provider<LoginServiceFactory>((ref) {
  final store = ref.watch(sessionStoreProvider);
  return (baseUrl) {
    final client = ApiClient.create(
      baseUrl: baseUrl,
      sessionStore: store,
      onUnauthorized: () => ref.invalidate(sessionFutureProvider),
    );
    return LoginService(client: client, sessionStore: store);
  };
});
