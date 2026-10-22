/// 审批列表控制器（设计稿 §2.1）：分页 / 刷新 / 状态过滤 /
/// scope 切换联动重查 / descMap 标签。
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import '../../core/function/function_spec.dart';
import '../../core/function/function_spec_service.dart';
import '../scope/scope_controller.dart';
import 'approval_service.dart';

class ApprovalsState {
  const ApprovalsState({
    this.items = const [],
    this.total = 0,
    this.page = 0,
    this.status = 'pending',
    this.loading = false,
    this.loadingMore = false,
    this.error,
    this.descs = const {},
  });

  final List<ApprovalItem> items;
  final int total;
  final int page;
  final String status;
  final bool loading;
  final bool loadingMore;
  final String? error;

  /// functionId → 描述符（高危 / 两人复核标签）。
  final Map<String, FunctionSpec> descs;

  bool get hasMore => items.length < total;

  ApprovalsState copyWith({
    List<ApprovalItem>? items,
    int? total,
    int? page,
    String? status,
    bool? loading,
    bool? loadingMore,
    String? error,
    bool clearError = false,
    Map<String, FunctionSpec>? descs,
  }) {
    return ApprovalsState(
      items: items ?? this.items,
      total: total ?? this.total,
      page: page ?? this.page,
      status: status ?? this.status,
      loading: loading ?? this.loading,
      loadingMore: loadingMore ?? this.loadingMore,
      error: clearError ? null : (error ?? this.error),
      descs: descs ?? this.descs,
    );
  }
}

final approvalsControllerProvider =
    NotifierProvider<ApprovalsController, ApprovalsState>(
      ApprovalsController.new,
    );

class ApprovalsController extends Notifier<ApprovalsState> {
  static const pageSize = 20;

  @override
  ApprovalsState build() {
    // scope 切换联动：approvals 是 scoped 端点，切后列表重查（§2.1）。
    ref.listen(scopeControllerProvider, (prev, next) {
      if (prev?.selected != next.selected && next.selected != null) {
        refresh();
      }
    });
    return const ApprovalsState();
  }

  Future<ApprovalService?> _serviceAsync() async {
    final session = await ref.read(sessionStoreProvider).load();
    final serverUrl = session?.serverUrl ?? '';
    if (serverUrl.isEmpty) return null;
    return ApprovalService(
      client: ref.read(apiClientFactoryProvider)(serverUrl),
    );
  }

  Future<void> refresh() => _fetch(reset: true);

  Future<void> loadMore() {
    if (state.loading || state.loadingMore || !state.hasMore) {
      return Future.value();
    }
    return _fetch(reset: false);
  }

  /// 切换状态过滤（pending/approved/rejected），重查第一页。
  Future<void> switchStatus(String status) async {
    if (status == state.status) return;
    state = state.copyWith(status: status);
    await _fetch(reset: true);
  }

  /// 描述符索引（高危 / 两人复核标签数据源）。缺 scope 或请求失败都不阻塞
  /// 列表——只是没有标签。
  Future<void> loadDescriptors() async {
    final service = await _specServiceAsync();
    if (service == null) return;
    try {
      final list = await service.list();
      state = state.copyWith(descs: {for (final d in list) d.id: d});
    } on ApiError {
      // 标签数据缺失不阻塞列表。
    } on StateError {
      // 同上。
    }
  }

  Future<FunctionSpecService?> _specServiceAsync() async {
    final session = await ref.read(sessionStoreProvider).load();
    final serverUrl = session?.serverUrl ?? '';
    if (serverUrl.isEmpty) return null;
    return FunctionSpecService(
      client: ref.read(apiClientFactoryProvider)(serverUrl),
    );
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
