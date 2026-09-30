/// 告警控制器（设计稿 §2.2）：分页 / level/status 筛选 / 静默 /
/// 静默规则列表。
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import 'alert_service.dart';

class AlertsState {
  const AlertsState({
    this.items = const [],
    this.total = 0,
    this.page = 0,
    this.level = '',
    this.status = '',
    this.loading = false,
    this.loadingMore = false,
    this.error,
    this.silences = const [],
    this.silencesLoading = false,
    this.silencesError,
  });

  final List<AlertItem> items;
  final int total;
  final int page;

  /// '' = 全部。
  final String level;
  final String status;

  final bool loading;
  final bool loadingMore;
  final String? error;

  final List<SilenceRule> silences;
  final bool silencesLoading;
  final String? silencesError;

  bool get hasMore => items.length < total;

  AlertsState copyWith({
    List<AlertItem>? items,
    int? total,
    int? page,
    String? level,
    String? status,
    bool? loading,
    bool? loadingMore,
    String? error,
    bool clearError = false,
    List<SilenceRule>? silences,
    bool? silencesLoading,
    String? silencesError,
    bool clearSilencesError = false,
  }) {
    return AlertsState(
      items: items ?? this.items,
      total: total ?? this.total,
      page: page ?? this.page,
      level: level ?? this.level,
      status: status ?? this.status,
      loading: loading ?? this.loading,
      loadingMore: loadingMore ?? this.loadingMore,
      error: clearError ? null : (error ?? this.error),
      silences: silences ?? this.silences,
      silencesLoading: silencesLoading ?? this.silencesLoading,
      silencesError: clearSilencesError
          ? null
          : (silencesError ?? this.silencesError),
    );
  }
}

final alertsControllerProvider =
    NotifierProvider<AlertsController, AlertsState>(AlertsController.new);

class AlertsController extends Notifier<AlertsState> {
  static const pageSize = 20;

  @override
  AlertsState build() => const AlertsState();

  Future<AlertService?> _serviceAsync() async {
    final session = await ref.read(sessionStoreProvider).load();
    final serverUrl = session?.serverUrl ?? '';
    if (serverUrl.isEmpty) return null;
    return AlertService(client: ref.read(apiClientFactoryProvider)(serverUrl));
  }

  Future<void> refresh() => _fetch(reset: true);

  Future<void> loadMore() {
    if (state.loading || state.loadingMore || !state.hasMore) {
      return Future.value();
    }
    return _fetch(reset: false);
  }

  Future<void> switchLevel(String level) async {
    if (level == state.level) return;
    state = state.copyWith(level: level);
    await _fetch(reset: true);
  }

  Future<void> switchStatus(String status) async {
    if (status == state.status) return;
    state = state.copyWith(status: status);
    await _fetch(reset: true);
  }

  /// 静默后刷新列表（触发中的告警状态可能变化）。
  Future<void> silence(
    String id, {
    required int durationMinutes,
    required String reason,
  }) async {
    final service = await _serviceAsync();
    if (service == null) {
      throw const ApiError(status: 0, code: 'no_session', message: '未登录');
    }
    await service.silence(id, durationMinutes: durationMinutes, reason: reason);
    await refresh();
  }

  /// 静默规则（只读，弹层打开时加载）。
  Future<void> loadSilences() async {
    final service = await _serviceAsync();
    if (service == null) {
      state = state.copyWith(silencesLoading: false, silencesError: '未登录');
      return;
    }
    state = state.copyWith(silencesLoading: true, clearSilencesError: true);
    try {
      final rules = await service.silences();
      state = state.copyWith(silences: rules, silencesLoading: false);
    } on ApiError catch (e) {
      state = state.copyWith(silencesLoading: false, silencesError: e.message);
    } on StateError catch (e) {
      state = state.copyWith(silencesLoading: false, silencesError: e.message);
    }
  }

  Future<void> _fetch({required bool reset}) async {
    final service = await _serviceAsync();
    if (service == null) {
      state = state.copyWith(loading: false, error: '未登录或缺少服务器地址');
      return;
    }
    final page = reset ? 1 : state.page + 1;
    state = state.copyWith(
      loading: reset,
      loadingMore: !reset,
      clearError: true,
    );
    try {
      final result = await service.list(
        page: page,
        pageSize: pageSize,
        level: state.level,
        status: state.status,
      );
      state = state.copyWith(
        items: reset ? result.items : [...state.items, ...result.items],
        total: result.total,
        page: page,
        loading: false,
        loadingMore: false,
      );
    } on ApiError catch (e) {
      state = state.copyWith(
        loading: false,
        loadingMore: false,
        error: e.message,
      );
    } on StateError catch (e) {
      state = state.copyWith(
        loading: false,
        loadingMore: false,
        error: e.message,
      );
    }
  }
}
