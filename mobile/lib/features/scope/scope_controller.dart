/// Scope 控制器（设计稿 §3.3）：lastGameId/lastEnv 预选 → 列表选择 →
/// PUT /profile/scope 持久化 → 更新本地会话。全局生效（顶栏切换器）。
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import '../../core/storage/session_store.dart';
import 'scope_service.dart';

class ScopeSelection {
  const ScopeSelection({required this.gameId, required this.env});

  final String gameId;
  final String env;

  @override
  bool operator ==(Object other) =>
      other is ScopeSelection && other.gameId == gameId && other.env == env;

  @override
  int get hashCode => Object.hash(gameId, env);
}

class ScopeState {
  const ScopeState({
    this.games = const [],
    this.selected,
    this.loading = false,
    this.error,
  });

  final List<ScopeGame> games;
  final ScopeSelection? selected;
  final bool loading;
  final String? error;

  ScopeState copyWith({
    List<ScopeGame>? games,
    ScopeSelection? selected,
    bool clearSelected = false,
    bool? loading,
    String? error,
    bool clearError = false,
  }) {
    return ScopeState(
      games: games ?? this.games,
      selected: clearSelected ? null : (selected ?? this.selected),
      loading: loading ?? this.loading,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

final scopeControllerProvider = NotifierProvider<ScopeController, ScopeState>(
  ScopeController.new,
);

class ScopeController extends Notifier<ScopeState> {
  @override
  ScopeState build() => const ScopeState();

  ScopeService _service(String serverUrl) {
    final store = ref.read(sessionStoreProvider);
    return ScopeService(
      client: ref.read(apiClientFactoryProvider)(serverUrl),
      sessionStore: store,
    );
  }

  /// 拉取游戏列表并确定预选：lastGameId/lastEnv 仍可选 → 用之；
  /// 否则落第一个可用 (game, env)。幂等（loading 中忽略重复触发）。
  Future<void> load() async {
    if (state.loading) return;
    state = state.copyWith(loading: true, clearError: true);
    try {
      final session = await ref.read(sessionStoreProvider).load();
      final serverUrl = session?.serverUrl ?? '';
      if (serverUrl.isEmpty) {
        state = state.copyWith(loading: false, error: '未登录或缺少服务器地址');
        return;
      }
      final games = await _service(serverUrl).fetchGames();
      state = state.copyWith(
        games: games,
        selected: _resolveInitial(session, games),
        loading: false,
      );
    } on ApiError catch (e) {
      state = state.copyWith(loading: false, error: e.message);
    } on StateError catch (e) {
      state = state.copyWith(loading: false, error: e.message);
    }
  }

  ScopeSelection? _resolveInitial(SessionData? session, List<ScopeGame> games) {
    final gameId = session?.gameId ?? '';
    final env = session?.env ?? '';
    final keep = games.any((g) => g.gameId == gameId && g.envs.contains(env));
    if (keep) {
      return ScopeSelection(gameId: gameId, env: env);
    }
    for (final g in games) {
      if (g.envs.isNotEmpty) {
        return ScopeSelection(gameId: g.gameId, env: g.envs.first);
      }
    }
    return null;
  }

  /// 切换 scope：先 PUT 服务端持久化（失败不改本地），成功后更新会话与状态。
  Future<void> select(String gameId, String env) async {
    final session = await ref.read(sessionStoreProvider).load();
    final serverUrl = session?.serverUrl ?? '';
    if (serverUrl.isEmpty) {
      state = state.copyWith(error: '未登录或缺少服务器地址');
      return;
    }
    try {
      await _service(serverUrl).saveScope(gameId, env);
    } on ApiError catch (e) {
      state = state.copyWith(error: e.message);
      return;
    }
    await ref
        .read(sessionStoreProvider)
        .save(session!.copyWith(gameId: gameId, env: env));
    // 会话里的 scope 变了：让 App 根重查（占位壳正文等读会话的 UI 同步刷新）。
    ref.invalidate(sessionFutureProvider);
    state = state.copyWith(
      selected: ScopeSelection(gameId: gameId, env: env),
      clearError: true,
    );
  }
}
