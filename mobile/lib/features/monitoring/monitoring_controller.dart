/// 监控大盘控制器（设计稿 §2.2）：性能快照加载 / 刷新 / 错误面。
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import 'performance_service.dart';

class MonitoringState {
  const MonitoringState({this.snapshot, this.loading = false, this.error});

  final PerformanceSnapshot? snapshot;
  final bool loading;
  final String? error;

  MonitoringState copyWith({
    PerformanceSnapshot? snapshot,
    bool? loading,
    String? error,
    bool clearError = false,
  }) {
    return MonitoringState(
      snapshot: snapshot ?? this.snapshot,
      loading: loading ?? this.loading,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

final monitoringControllerProvider =
    NotifierProvider<MonitoringController, MonitoringState>(
      MonitoringController.new,
    );

class MonitoringController extends Notifier<MonitoringState> {
  @override
  MonitoringState build() => const MonitoringState();

  Future<PerformanceService?> _serviceAsync() async {
    final session = await ref.read(sessionStoreProvider).load();
    final serverUrl = session?.serverUrl ?? '';
    if (serverUrl.isEmpty) return null;
    return PerformanceService(
      client: ref.read(apiClientFactoryProvider)(serverUrl),
    );
  }

  Future<void> refresh() async {
    if (state.loading) return;
    final service = await _serviceAsync();
    if (service == null) {
      state = state.copyWith(loading: false, error: '未登录或缺少服务器地址');
      return;
    }
    state = state.copyWith(loading: true, clearError: true);
    try {
      final snapshot = await service.fetch();
      state = state.copyWith(snapshot: snapshot, loading: false);
    } on ApiError catch (e) {
      state = state.copyWith(loading: false, error: e.message);
    } on StateError catch (e) {
      state = state.copyWith(loading: false, error: e.message);
    }
  }
}
