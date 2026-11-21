/// 分析大盘控制器（MOB-ANA-001）：summary 拉取 / 刷新 /
/// scope 切换联动重查。gameId 取会话作用域（analytics 是 scoped 端）。
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import '../scope/scope_controller.dart';
import 'analytics_service.dart';

class AnalyticsState {
  const AnalyticsState({this.summary, this.loading = false, this.error});

  final AnalyticsSummary? summary;
  final bool loading;
  final String? error;

  AnalyticsState copyWith({
    AnalyticsSummary? summary,
    bool clearSummary = false,
    bool? loading,
    String? error,
    bool clearError = false,
  }) {
    return AnalyticsState(
      summary: clearSummary ? null : (summary ?? this.summary),
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
  /// 请求序号（AUD-003）：每次 refresh 递增；只有序号仍是当前值的响应可写
  /// 状态。作用域切换会递增它，使在途请求全部过期——乱序响应无法把旧作用
  /// 域的摘要写成当前结果。
  int _requestSeq = 0;

  @override
  AnalyticsState build() {
    // scope 切换联动：analytics 是 scoped 端点，切后先清空旧摘要（不把上一
    // 作用域的卡片留作当前结果），再重查。清空用 clearSummary 而非传 null——
    // copyWith 的 `summary ?? this.summary` 无法显式置空。
    ref.listen(scopeControllerProvider, (prev, next) {
      if (prev?.selected != next.selected) {
        _requestSeq++; // 使在途请求过期
        state = state.copyWith(
          clearSummary: true,
          loading: false,
          clearError: true,
        );
        if (next.selected != null) {
          refresh();
        }
      }
    });
    return const AnalyticsState();
  }

  Future<void> refresh() async {
    final seq = ++_requestSeq;
    final session = await ref.read(sessionStoreProvider).load();
    if (seq != _requestSeq) return; // 会话加载期间作用域已变：丢弃
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
      if (seq != _requestSeq) return; // 过期响应丢弃（乱序/已切作用域）
      state = state.copyWith(summary: summary, loading: false);
    } on ApiError catch (e) {
      if (seq != _requestSeq) return;
      state = state.copyWith(loading: false, error: e.message);
    } on StateError catch (e) {
      if (seq != _requestSeq) return;
      state = state.copyWith(loading: false, error: e.message);
    }
  }
}
