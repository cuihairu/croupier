/// 审计控制器（设计稿 §2.4）：筛选条件 / 分页 / 只读检索。
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import 'audit_service.dart';

/// 筛选条件（点「查询」整体提交；kinds 多选逗号串透传）。
class AuditFilters {
  const AuditFilters({
    this.actor = '',
    this.kinds = const {},
    this.env = '',
    this.ip = '',
    this.gameId = '',
    this.start,
    this.end,
  });

  final String actor;
  final Set<String> kinds;
  final String env;
  final String ip;
  final String gameId;

  /// 时间区间下/上界（日期当天 00:00:00 / 23:59:59 由页面归一）。
  final DateTime? start;
  final DateTime? end;

  AuditFilters copyWith({
    String? actor,
    Set<String>? kinds,
    String? env,
    String? ip,
    String? gameId,
    DateTime? start,
    DateTime? end,
  }) {
    return AuditFilters(
      actor: actor ?? this.actor,
      kinds: kinds ?? this.kinds,
      env: env ?? this.env,
      ip: ip ?? this.ip,
      gameId: gameId ?? this.gameId,
      start: start ?? this.start,
      end: end ?? this.end,
    );
  }

  @override
  bool operator ==(Object other) =>
      other is AuditFilters &&
      other.actor == actor &&
      other.env == env &&
      other.ip == ip &&
      other.gameId == gameId &&
      other.start == start &&
      other.end == end &&
      const SetEquality().equals(other.kinds, kinds);

  @override
  int get hashCode => Object.hash(
    actor,
    env,
    ip,
    gameId,
    start,
    end,
    Object.hashAllUnordered(kinds),
  );
}

/// Set 深比较（避免为它引 collection 包）。
class SetEquality {
  const SetEquality();

  bool equals(Set<String> a, Set<String> b) =>
      a.length == b.length && a.containsAll(b);
}

class AuditState {
  const AuditState({
    this.items = const [],
    this.total = 0,
    this.page = 0,
    this.loading = false,
    this.loadingMore = false,
    this.error,
    this.filters = const AuditFilters(),
  });

  final List<AuditItem> items;
  final int total;
  final int page;
  final bool loading;
  final bool loadingMore;
  final String? error;
  final AuditFilters filters;

  bool get hasMore => items.length < total;

  AuditState copyWith({
    List<AuditItem>? items,
    int? total,
    int? page,
    bool? loading,
    bool? loadingMore,
    String? error,
    bool clearError = false,
    AuditFilters? filters,
  }) {
    return AuditState(
      items: items ?? this.items,
      total: total ?? this.total,
      page: page ?? this.page,
      loading: loading ?? this.loading,
      loadingMore: loadingMore ?? this.loadingMore,
      error: clearError ? null : (error ?? this.error),
      filters: filters ?? this.filters,
    );
  }
}

final auditControllerProvider = NotifierProvider<AuditController, AuditState>(
  AuditController.new,
);

class AuditController extends Notifier<AuditState> {
  static const pageSize = 20;

  @override
  AuditState build() => const AuditState();

  Future<AuditService?> _serviceAsync() async {
    final session = await ref.read(sessionStoreProvider).load();
    final serverUrl = session?.serverUrl ?? '';
    if (serverUrl.isEmpty) return null;
    return AuditService(client: ref.read(apiClientFactoryProvider)(serverUrl));
  }

  /// 应用新筛选并重查第一页。
  Future<void> applyFilters(AuditFilters filters) async {
    if (filters == state.filters) return;
    state = state.copyWith(filters: filters);
    await refresh();
  }

  Future<void> refresh() => _fetch(reset: true);

  Future<void> loadMore() {
    if (state.loading || state.loadingMore || !state.hasMore) {
      return Future.value();
    }
    return _fetch(reset: false);
  }

  Future<void> _fetch({required bool reset}) async {
    final service = await _serviceAsync();
    if (service == null) {
      state = state.copyWith(loading: false, error: '未登录或缺少服务器地址');
      return;
    }
    final page = reset ? 1 : state.page + 1;
    final filters = state.filters;
    state = state.copyWith(
      loading: reset,
      loadingMore: !reset,
      clearError: true,
    );
    try {
      final result = await service.list(
        page: page,
        pageSize: pageSize,
        actor: filters.actor,
        kinds: filters.kinds.toList(),
        env: filters.env,
        ip: filters.ip,
        gameId: filters.gameId,
        start: filters.start,
        end: filters.end,
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
