/// 分析大盘控制器（MOB-ANA-001）：summary 拉取 / 刷新 /
/// scope 切换联动重查。gameId 取会话作用域（analytics 是 scoped 端）。
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import '../scope/scope_controller.dart';
import 'analytics_service.dart';

class AnalyticsState {
  const AnalyticsState({
    this.summary,
    this.loading = false,
    this.error,
  });

  final AnalyticsSummary? summary;
  final bool loading;
  final String? error;

  AnalyticsState copyWith({
    AnalyticsSummary? summary,
    bool? loading,
    String? error,
    bool clearError = false,
  }) {
    return AnalyticsState(
      summary: summary ?? this.summary,
      loading: loading ?? this.loading,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

final analyticsControllerProvider =
    NotifierProvider<AnalyticsController, AnalyticsState>(
      AnalyticsController.new,
    );

class AnalyticsController extends Notifier<AnalyticsState> {
  @override
  AnalyticsState build() {
    // scope 切换联动：analytics 是 scoped 端点，切后摘要重查。
    ref.listen(scopeControllerProvider, (prev, next) {
      if (prev?.selected != next.selected && next.selected != null) {
        refresh();
      }
    });
    return const AnalyticsState();
  }

  Future<void> refresh() async {
    final session = await ref.read(sessionStoreProvider).load();
    final serverUrl = session?.serverUrl ?? '';
    if (serverUrl.isEmpty) {
      state = state.copyWith(loading: false, error: '未登录或缺少服务器地址');
      return;
    }
    final gameId = session?.gameId.trim() ?? '';
    if (gameId.isEmpty) {
      state = state.copyWith(loading: false, error: '缺少游戏作用域');
      return;
    }
    final service = AnalyticsService(
      client: ref.read(apiClientFactoryProvider)(serverUrl),
    );
    state = state.copyWith(loading: true, clearError: true);
    try {
      final summary = await service.summary(
        gameId: gameId,
        env: session?.env.trim() ?? '',
      );
      state = state.copyWith(summary: summary, loading: false);
    } on ApiError catch (e) {
      state = state.copyWith(loading: false, error: e.message);
    } on StateError catch (e) {
      state = state.copyWith(loading: false, error: e.message);
    }
  }
}
