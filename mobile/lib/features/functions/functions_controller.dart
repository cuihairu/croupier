/// 函数目录 + 调用控制器（设计稿 §2.5）：
/// descriptors 目录加载 / 搜索过滤 / invoke（同步结果或异步任务）/
/// 任务轮询（5s，页面可见时）/ 取消。
library;

import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import '../../core/function/function_spec.dart';
import '../../core/function/function_spec_service.dart';
import '../scope/scope_controller.dart';
import 'function_invoke_service.dart';

class FunctionsState {
  const FunctionsState({
    this.specs = const [],
    this.loading = false,
    this.error,
    this.query = '',
    this.submitting = false,
    this.invokeError,
    this.invokeResult,
    this.task,
    this.lastFunctionId = '',
  });

  final List<FunctionSpec> specs;
  final bool loading;
  final String? error;
  final String query;

  /// 正在提交调用（按钮禁用）。
  final bool submitting;

  /// 调用错误（snackbar 用）。
  final String? invokeError;

  /// 同步调用结果 / 审批提交通知。
  final InvokeResult? invokeResult;

  /// 异步任务当前状态（非空时任务跟进页活跃）。
  final TaskStatus? task;

  final String lastFunctionId;

  bool get taskRunning => task != null && task!.isRunning;

  List<FunctionSpec> get filtered {
    final q = query.trim().toLowerCase();
    if (q.isEmpty) return specs;
    return specs
        .where(
          (s) =>
              s.id.toLowerCase().contains(q) ||
              s.displayName.toLowerCase().contains(q) ||
              s.displayDescription.toLowerCase().contains(q),
        )
        .toList(growable: false);
  }

  FunctionsState copyWith({
    List<FunctionSpec>? specs,
    bool? loading,
    String? error,
    bool clearError = false,
    String? query,
    bool? submitting,
    String? invokeError,
    bool clearInvokeError = false,
    InvokeResult? invokeResult,
    bool clearInvokeResult = false,
    TaskStatus? task,
    bool clearTask = false,
    String? lastFunctionId,
  }) {
    return FunctionsState(
      specs: specs ?? this.specs,
      loading: loading ?? this.loading,
      error: clearError ? null : (error ?? this.error),
      query: query ?? this.query,
      submitting: submitting ?? this.submitting,
      invokeError: clearInvokeError ? null : (invokeError ?? this.invokeError),
      invokeResult: clearInvokeResult
          ? null
          : (invokeResult ?? this.invokeResult),
      task: clearTask ? null : (task ?? this.task),
      lastFunctionId: lastFunctionId ?? this.lastFunctionId,
    );
  }
}

final functionsControllerProvider =
    NotifierProvider<FunctionsController, FunctionsState>(
      FunctionsController.new,
    );

class FunctionsController extends Notifier<FunctionsState> {
  Timer? _poll;

  /// 目录请求代际（AUD-063）：新 refresh / 作用域切换使在途旧请求失效。
  /// 迟到的旧响应（含错误）按代际丢弃，不写 state——目录、错误、loading
  /// 均不被旧作用域请求污染；旧请求在途也不阻塞新请求。
  int _loadGen = 0;

  /// 任务轮询代次（AUD-067）：新任务 / 清结果 / 作用域切换都会换代；
  /// 在途轮询响应落地前核对代次，旧任务的终态不得覆盖当前任务或停掉
  /// 新任务的定时器。同一代次同时只保留一次在途轮询。
  int _taskGen = 0;
  int _pollInFlightGen = -1;

  @override
  FunctionsState build() {
    // scope 切换联动：descriptors 是 scoped 端点，切后清空旧目录并重查；
    // 任务与轮询属旧作用域，一并失效（AUD-067）。
    ref.listen(scopeControllerProvider, (prev, next) {
      if (prev?.selected != next.selected && next.selected != null) {
        refresh(clearCatalog: true);
        invalidateTask();
      }
    });
    ref.onDispose(() => _poll?.cancel());
    return const FunctionsState();
  }

  Future<FunctionSpecService?> _specServiceAsync() async {
    final session = await ref.read(sessionStoreProvider).load();
    final serverUrl = session?.serverUrl ?? '';
    if (serverUrl.isEmpty) return null;
    return FunctionSpecService(
      client: ref.read(apiClientFactoryProvider)(serverUrl),
    );
  }

  Future<FunctionInvokeService?> _invokeServiceAsync() async {
    final session = await ref.read(sessionStoreProvider).load();
    final serverUrl = session?.serverUrl ?? '';
    if (serverUrl.isEmpty) return null;
    return FunctionInvokeService(
      client: ref.read(apiClientFactoryProvider)(serverUrl),
    );
  }

  /// 拉取函数目录。并发语义（AUD-063）：每次调用开启新一代请求，取代
  /// 在途旧请求（不等待、不阻塞）；被取代请求的响应/错误迟到时按代际
  /// 丢弃。[clearCatalog] 在新查询发出前清空旧目录（作用域切换用，
  /// 避免旧作用域数据滞留）；手动刷新保留旧列表直到新数据到达。
  Future<void> refresh({bool clearCatalog = false}) async {
    final gen = ++_loadGen;
    state = state.copyWith(
      specs: clearCatalog ? const [] : null,
      loading: true,
      clearError: true,
    );
    final service = await _specServiceAsync();
    if (gen != _loadGen) return;
    if (service == null) {
      state = state.copyWith(loading: false, error: '未登录或缺少服务器地址');
      return;
    }
    try {
      final specs = await service.list();
      if (gen != _loadGen) return;
      state = state.copyWith(specs: specs, loading: false);
    } on ApiError catch (e) {
      if (gen != _loadGen) return;
      state = state.copyWith(loading: false, error: e.message);
    } on StateError catch (e) {
      if (gen != _loadGen) return;
      state = state.copyWith(loading: false, error: e.message);
    }
  }

  void setQuery(String query) => state = state.copyWith(query: query);

  /// 调用函数。[payload] 由表单组装。返回结果经 state 暴露给页面。
  Future<void> invoke({
    required String functionId,
    required Map<String, Object?> payload,
    String mode = '',
    String route = '',
    String targetServiceId = '',
    String hashKey = '',
  }) async {
    if (state.submitting) return;
    state = state.copyWith(
      submitting: true,
      clearInvokeError: true,
      clearInvokeResult: true,
      lastFunctionId: functionId,
    );
    final service = await _invokeServiceAsync();
    if (service == null) {
      state = state.copyWith(submitting: false, invokeError: '未登录或缺少服务器地址');
      return;
    }
    try {
      final result = await service.invoke(
        functionId: functionId,
        payload: payload,
        mode: mode,
        route: route,
        targetServiceId: targetServiceId,
        hashKey: hashKey,
      );
      state = state.copyWith(submitting: false, invokeResult: result);
      if (result.isTask) {
        // 异步任务：换代使旧任务的在途轮询失效（AUD-067），立即拉一次
        // 并启动 5s 轮询（页面可见时；见 page）。
        _taskGen++;
        state = state.copyWith(
          task: TaskStatus(id: result.taskId, status: 'running'),
        );
        await _pollTaskOnce();
        if (state.taskRunning) _startPolling();
      }
    } on ApiError catch (e) {
      state = state.copyWith(submitting: false, invokeError: e.message);
    } on StateError catch (e) {
      state = state.copyWith(submitting: false, invokeError: e.message);
    }
  }

  /// 页面可见性回调：resumed 启动轮询 / 不可见停止（后台零请求）。
  void onVisibility(bool visible) {
    if (visible) {
      if (state.taskRunning) _startPolling();
    } else {
      _poll?.cancel();
      _poll = null;
    }
  }

  void _startPolling() {
    _poll?.cancel();
    _poll = Timer.periodic(const Duration(seconds: 5), (_) => _pollTaskOnce());
  }

  Future<void> _pollTaskOnce() async {
    final current = state.task;
    if (current == null || !current.isRunning) {
      _poll?.cancel();
      _poll = null;
      return;
    }
    // 同一代次仅保留一次在途轮询（AUD-067）：上一 tick 未返回时本 tick
    // 跳过；代次已换代（新任务/清除）的旧在途轮询不阻塞新轮询。
    final gen = _taskGen;
    if (_pollInFlightGen == gen) return;
    _pollInFlightGen = gen;
    try {
      final service = await _invokeServiceAsync();
      if (gen != _taskGen) return;
      if (service == null) return;
      final task = await service.taskDetail(current.id);
      // 迟到响应（任务已切换/清除/作用域已切）按代次丢弃：不覆盖当前
      // 任务，也不停掉新任务的定时器。
      if (gen != _taskGen) return;
      state = state.copyWith(task: task);
      if (task.isDone) {
        _poll?.cancel();
        _poll = null;
      }
    } on ApiError {
      // 单次轮询失败不中断：下一 tick 重试（终态由用户手动刷新兜底）。
    } on StateError {
      // 同上。
    } finally {
      if (_pollInFlightGen == gen) _pollInFlightGen = -1;
    }
  }

  /// 清任务并使在途轮询响应失效（作用域切换 / 清结果共用，AUD-067）。
  void invalidateTask() {
    _taskGen++;
    _poll?.cancel();
    _poll = null;
    state = state.copyWith(clearTask: true);
  }

  /// 取消异步任务（终态由后续轮询/刷新确认）。
  Future<void> cancelTask() async {
    final task = state.task;
    if (task == null) return;
    final service = await _invokeServiceAsync();
    if (service == null) return;
    try {
      await service.cancelTask(task.id);
    } on ApiError catch (e) {
      state = state.copyWith(invokeError: e.message);
    }
    await _pollTaskOnce();
  }

  /// 手动刷新任务状态（轮询失败后的兜底）。
  Future<void> refreshTask() => _pollTaskOnce();

  /// 清空调用结果 / 任务（返回目录或再次调用前）。
  void clearResult() {
    invalidateTask();
    state = state.copyWith(clearInvokeResult: true);
  }
}
